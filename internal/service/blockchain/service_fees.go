package blockchain

import (
	"context"
	"math/big"
	"time"

	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

type FeeCalculator interface {
	CalculateFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error)
	CalculateWithdrawalFeeUSD(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (money.Money, error)
}

// withdrawalNetworkFeeMultiplier when customer wants to withdraw his assets from the system, we already spent
// 1x network fee for INBOUND -> OUTBOUND processing. In total o2pay would pay x2 network fee in order to withdraw
// assets. So it should be kinda fair if customer pays for 0.5x INBOUND -> OUTBOUND & x1 for OUTBOUND -> EXTERNAL.
const withdrawalNetworkFeeMultiplier = 1.5

// CalculateFee calculates blockchain transaction fee for selected currency.
func (s *Service) CalculateFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	if baseCurrency.Type != money.Coin || baseCurrency.Blockchain != currency.Blockchain {
		return Fee{}, errors.New("invalid arguments")
	}

	switch kmswallet.Blockchain(currency.Blockchain) {
	case kmswallet.ETH:
		return s.ethFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.MATIC:
		return s.maticFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.BSC:
		return s.bscFee(ctx, baseCurrency, currency, isTest)
	case kmswallet.TRON:
		return s.tronFee(ctx, baseCurrency, currency, isTest)
	}

	return Fee{}, errors.New("unsupported blockchain for fees calculations " + currency.Ticker)
}

// CalculateWithdrawalFeeUSD withdrawal fees are tied to network fee but calculated in USD
// Example: usdFee, err := CalculateWithdrawalFeeUSD(ctx, eth, ethUSD, false)
func (s *Service) CalculateWithdrawalFeeUSD(
	ctx context.Context,
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
) (money.Money, error) {
	fee, err := s.CalculateFee(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return money.Money{}, err
	}

	var usdFee money.Money

	switch kmswallet.Blockchain(fee.Currency.Blockchain) {
	case kmswallet.ETH:
		f, _ := fee.ToEthFee()
		usdFee = f.totalCostUSD
	case kmswallet.MATIC:
		f, _ := fee.ToMaticFee()
		usdFee = f.totalCostUSD
	case kmswallet.BSC:
		f, _ := fee.ToBSCFee()
		usdFee = f.totalCostUSD
	case kmswallet.TRON:
		f, _ := fee.ToTronFee()
		usdFee = f.feeLimitUSD
	default:
		return money.Money{}, ErrCurrencyNotFound
	}

	// Sometimes crypto fee lower than 1 cent, so du to rounding error we can get usdFee = $0.0.
	// We shouldn't allow that, so let's force it to 1 cent
	if usdFee.IsZero() {
		return money.FiatFromFloat64(money.USD, 0.01)
	}

	return usdFee.MultiplyFloat64(withdrawalNetworkFeeMultiplier)
}

type Fee struct {
	CalculatedAt time.Time
	Currency     money.CryptoCurrency
	IsTest       bool
	raw          any
}

func NewFee(currency money.CryptoCurrency, at time.Time, isTest bool, fee any) Fee {
	return Fee{
		CalculatedAt: at,
		Currency:     currency,
		IsTest:       isTest,
		raw:          fee,
	}
}

type EthFee struct {
	GasUnits     uint   `json:"gasUnits"`
	GasPrice     string `json:"gasPrice"`
	PriorityFee  string `json:"priorityFee"`
	TotalCostWEI string `json:"totalCostWei"`
	TotalCostETH string `json:"totalCostEth"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToEthFee() (EthFee, error) {
	if fee, ok := f.raw.(EthFee); ok {
		return fee, nil
	}

	return EthFee{}, errors.New("invalid fee type assertion for ETH")
}

func (s *Service) ethFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.15
	)

	bigIntToETH := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	// 1. Connect to ETH node
	client, err := s.providers.Tatum.EthereumRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceETH, err := bigIntToETH(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make ETH from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceETHConfident, err := gasPriceETH.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply ETH gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest ETH gas tip cap")
	}

	priorityFeeETH, err := bigIntToETH(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make ETH from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceETHConfident.Add(priorityFeeETH)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, EthFee{
		GasUnits:     uint(gasUnits),
		GasPrice:     gasPriceETHConfident.StringRaw(),
		PriorityFee:  priorityFeeETH.StringRaw(),
		TotalCostWEI: totalCost.StringRaw(),
		TotalCostETH: totalCost.String(),
		TotalCostUSD: conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type MaticFee struct {
	GasUnits       uint   `json:"gasUnits"`
	GasPrice       string `json:"gasPrice"`
	PriorityFee    string `json:"priorityFee"`
	TotalCostWEI   string `json:"totalCostWei"`
	TotalCostMATIC string `json:"totalCostMatic"`
	TotalCostUSD   string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToMaticFee() (MaticFee, error) {
	if fee, ok := f.raw.(MaticFee); ok {
		return fee, nil
	}

	return MaticFee{}, errors.New("invalid fee type assertion for MATIC")
}

func (s *Service) maticFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.10
	)

	bigIntToMATIC := func(i *big.Int) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, i, baseCurrency.Decimals)
	}

	// 1. Connect to MATIC node
	client, err := s.providers.Tatum.MaticRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceMATIC, err := bigIntToMATIC(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make MATIC from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceMATICConfident, err := gasPriceMATIC.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply MATIC gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest MATIC gas tip cap")
	}

	priorityFeeMATIC, err := bigIntToMATIC(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make MATIC from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceMATICConfident.Add(priorityFeeMATIC)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, MaticFee{
		GasUnits:       uint(gasUnits),
		GasPrice:       gasPriceMATICConfident.StringRaw(),
		PriorityFee:    priorityFeeMATIC.StringRaw(),
		TotalCostWEI:   totalCost.StringRaw(),
		TotalCostMATIC: totalCost.String(),
		TotalCostUSD:   conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type BSCFee struct {
	GasUnits     uint   `json:"gasUnits"`
	GasPrice     string `json:"gasPrice"`
	PriorityFee  string `json:"priorityFee"`
	TotalCostWEI string `json:"totalCostWei"`
	TotalCostBNB string `json:"totalCostBNB"`
	TotalCostUSD string `json:"totalCostUsd"`

	totalCostUSD money.Money
}

func (f *Fee) ToBSCFee() (BSCFee, error) {
	if fee, ok := f.raw.(BSCFee); ok {
		return fee, nil
	}

	return BSCFee{}, errors.New("invalid fee type assertion for BSC")
}

func (s *Service) bscFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		gasUnitsForCoin  = 21_000
		gasUnitsForToken = 65_000

		gasConfidentRate = 1.10
	)

	// 1. Connect to BSC node
	client, err := s.providers.Tatum.BinanceSmartChainRPC(ctx, isTest)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to setup RPC")
	}

	// 2. Calculate gasPrice
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest gas price")
	}

	gasPriceMATIC, err := baseCurrency.MakeAmountFromBigInt(gasPrice)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make BSC from gas price")
	}

	// In order to be confident that tx will be processed, let's multiply price by gasConfidentRate
	gasPriceMATICConfident, err := gasPriceMATIC.MultiplyFloat64(gasConfidentRate)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to multiply BSC gas price")
	}

	// 3. Calculate priorityFee
	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest BSC gas tip cap")
	}

	priorityFeeBSC, err := baseCurrency.MakeAmountFromBigInt(priorityFee)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to suggest make BSC from priorityFee")
	}

	// 4. Calculate gasUnits and total cost in WEI
	totalFeePerGas, err := gasPriceMATICConfident.Add(priorityFeeBSC)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total fee per gas")
	}

	gasUnits := gasUnitsForCoin
	if currency.Type == money.Token {
		gasUnits = gasUnitsForToken
	}

	totalCost, err := totalFeePerGas.MultiplyFloat64(float64(gasUnits))
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total tx cost")
	}

	conv, err := s.CryptoToFiat(ctx, totalCost, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, BSCFee{
		GasUnits:     uint(gasUnits),
		GasPrice:     gasPriceMATICConfident.StringRaw(),
		PriorityFee:  priorityFeeBSC.StringRaw(),
		TotalCostWEI: totalCost.StringRaw(),
		TotalCostBNB: totalCost.String(),
		TotalCostUSD: conv.To.String(),

		totalCostUSD: conv.To,
	}), nil
}

type TronFee struct {
	FeeLimitSun uint64 `json:"feeLimit"`
	FeeLimitTRX string `json:"feeLimitTrx"`
	FeeLimitUSD string `json:"feeLimitUsd"`

	feeLimitUSD money.Money
}

func (f *Fee) ToTronFee() (TronFee, error) {
	if fee, ok := f.raw.(TronFee); ok {
		return fee, nil
	}

	return TronFee{}, errors.New("invalid fee type assertion for TRON")
}

func (s *Service) tronFee(ctx context.Context, baseCurrency, currency money.CryptoCurrency, isTest bool) (Fee, error) {
	const (
		bandwidthSunCost      = int64(1000)
		coinTransferBandwidth = int64(350)

		// 30.01.23: based on avg tronscan data ~ 15 trx
		// 14.06.23: https://support.ledger.com/hc/en-us/articles/8085235615133-Tether-USDT-transaction-on-Tron-failed-and-ran-out-of-energy
		tokenTransactionSun = int64(30 * 1_000_000)
	)

	intToTRON := func(i int64) (money.Money, error) {
		return money.NewFromBigInt(money.Crypto, baseCurrency.Ticker, big.NewInt(i), baseCurrency.Decimals)
	}

	feeLimit := bandwidthSunCost * coinTransferBandwidth
	if currency.Type == money.Token {
		feeLimit = tokenTransactionSun
	}

	feeLimitTRON, err := intToTRON(feeLimit)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to make TRON from int")
	}

	conv, err := s.CryptoToFiat(ctx, feeLimitTRON, money.USD)
	if err != nil {
		return Fee{}, errors.Wrap(err, "unable to calculate total cost in USD")
	}

	return NewFee(currency, time.Now().UTC(), isTest, TronFee{
		FeeLimitSun: uint64(feeLimit),
		FeeLimitTRX: feeLimitTRON.String(),
		FeeLimitUSD: conv.To.String(),

		feeLimitUSD: conv.To,
	}), nil
}
