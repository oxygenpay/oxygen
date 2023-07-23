package fakes

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type FeeCalculator struct {
	t              *testing.T
	mu             sync.RWMutex
	fees           map[string]blockchain.Fee
	withdrawalFees map[string]money.Money
}

func newFeeCalculator(t *testing.T) *FeeCalculator {
	return &FeeCalculator{
		t:              t,
		fees:           make(map[string]blockchain.Fee),
		withdrawalFees: make(map[string]money.Money),
	}
}

func (m *FeeCalculator) CalculateFee(
	_ context.Context,
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
) (blockchain.Fee, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.key(baseCurrency, currency, isTest)
	if fee, ok := m.fees[key]; ok {
		return fee, nil
	}

	return blockchain.Fee{}, errors.New("unexpected call of (*FeeCalculatorMock).CalculateFee for " + key)
}

func (m *FeeCalculator) CalculateWithdrawalFeeUSD(
	_ context.Context,
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
) (money.Money, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.key(baseCurrency, currency, isTest)
	if fee, ok := m.withdrawalFees[key]; ok {
		return fee, nil
	}

	return money.Money{}, errors.New("unexpected call of (*FeeCalculatorMock).CalculateWithdrawalFeeUSD for " + key)
}

func (m *FeeCalculator) SetupCalculateFee(
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
	fee blockchain.Fee,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fees[m.key(baseCurrency, currency, isTest)] = fee
}

func (m *FeeCalculator) SetupCalculateWithdrawalFeeUSD(
	baseCurrency, currency money.CryptoCurrency,
	isTest bool,
	fee money.Money,
) {
	if fee.Ticker() != money.USD.String() {
		panic("invalid fee provided")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.withdrawalFees[m.key(baseCurrency, currency, isTest)] = fee
}

func (m *FeeCalculator) SetupAllFees(t *testing.T, service *blockchain.Service) {
	getCurrency := func(ticker string) money.CryptoCurrency {
		c, err := service.GetCurrencyByTicker(ticker)
		require.NoError(t, err)

		return c
	}

	now := time.Now().UTC()

	// ETH
	eth := getCurrency("ETH")
	ethUSDT := getCurrency("ETH_USDT")
	ethUSDC := getCurrency("ETH_USDC")
	ethFee := blockchain.EthFee{
		GasUnits:     21000,
		GasPrice:     "52860219500",
		PriorityFee:  "118797707",
		TotalCostWEI: "1112559361347000",
		TotalCostETH: "0.001112559361347",
		TotalCostUSD: "1.83",
	}

	m.SetupCalculateFee(eth, eth, false, blockchain.NewFee(eth, now, false, ethFee))
	m.SetupCalculateFee(eth, eth, true, blockchain.NewFee(eth, now, true, ethFee))
	m.SetupCalculateFee(eth, ethUSDT, false, blockchain.NewFee(eth, now, false, ethFee))
	m.SetupCalculateFee(eth, ethUSDT, true, blockchain.NewFee(eth, now, true, ethFee))
	m.SetupCalculateFee(eth, ethUSDC, false, blockchain.NewFee(eth, now, false, ethFee))
	m.SetupCalculateFee(eth, ethUSDC, true, blockchain.NewFee(eth, now, true, ethFee))

	// withdrawal fees
	m.SetupCalculateWithdrawalFeeUSD(eth, eth, false, lo.Must(money.USD.MakeAmount("100")))
	m.SetupCalculateWithdrawalFeeUSD(eth, eth, true, lo.Must(money.USD.MakeAmount("100")))
	m.SetupCalculateWithdrawalFeeUSD(eth, ethUSDT, false, lo.Must(money.USD.MakeAmount("300")))
	m.SetupCalculateWithdrawalFeeUSD(eth, ethUSDT, true, lo.Must(money.USD.MakeAmount("300")))
	m.SetupCalculateWithdrawalFeeUSD(eth, ethUSDC, true, lo.Must(money.USD.MakeAmount("300")))
	m.SetupCalculateWithdrawalFeeUSD(eth, ethUSDC, true, lo.Must(money.USD.MakeAmount("300")))

	// MATIC
	matic := getCurrency("MATIC")
	maticUSDT := getCurrency("MATIC_USDT")
	maticUSDC := getCurrency("MATIC_USDC")
	maticFee := blockchain.MaticFee{
		GasUnits:       21000,
		GasPrice:       "115243093692",
		PriorityFee:    "30000000000",
		TotalCostWEI:   "9440801089980000",
		TotalCostMATIC: "0.00944080108998",
		TotalCostUSD:   "0.01",
	}

	m.SetupCalculateFee(matic, matic, false, blockchain.NewFee(matic, now, false, maticFee))
	m.SetupCalculateFee(matic, matic, true, blockchain.NewFee(matic, now, true, maticFee))
	m.SetupCalculateFee(matic, maticUSDT, false, blockchain.NewFee(matic, now, false, maticFee))
	m.SetupCalculateFee(matic, maticUSDT, true, blockchain.NewFee(matic, now, true, maticFee))
	m.SetupCalculateFee(matic, maticUSDC, false, blockchain.NewFee(matic, now, false, maticFee))
	m.SetupCalculateFee(matic, maticUSDC, true, blockchain.NewFee(matic, now, true, maticFee))

	// withdrawal fees
	m.SetupCalculateWithdrawalFeeUSD(matic, matic, false, lo.Must(money.USD.MakeAmount("10")))
	m.SetupCalculateWithdrawalFeeUSD(matic, matic, true, lo.Must(money.USD.MakeAmount("10")))
	m.SetupCalculateWithdrawalFeeUSD(matic, maticUSDT, false, lo.Must(money.USD.MakeAmount("20")))
	m.SetupCalculateWithdrawalFeeUSD(matic, maticUSDT, true, lo.Must(money.USD.MakeAmount("20")))
	m.SetupCalculateWithdrawalFeeUSD(matic, maticUSDC, false, lo.Must(money.USD.MakeAmount("20")))
	m.SetupCalculateWithdrawalFeeUSD(matic, maticUSDC, true, lo.Must(money.USD.MakeAmount("20")))

	// BSC
	bnb := getCurrency("BNB")
	bscFee := blockchain.BSCFee{
		GasUnits:     21000,
		GasPrice:     "115243093692",
		PriorityFee:  "30000000000",
		TotalCostWEI: "9440801089980000",
		TotalCostBNB: "0.00944080108998",
		TotalCostUSD: "0.01",
	}

	m.SetupCalculateFee(bnb, bnb, false, blockchain.NewFee(bnb, now, false, bscFee))
	m.SetupCalculateFee(bnb, bnb, true, blockchain.NewFee(bnb, now, true, bscFee))

	// withdrawal fees
	m.SetupCalculateWithdrawalFeeUSD(bnb, bnb, false, lo.Must(money.USD.MakeAmount("10")))
	m.SetupCalculateWithdrawalFeeUSD(bnb, bnb, true, lo.Must(money.USD.MakeAmount("10")))

	// TRON
	tron := getCurrency("TRON")
	tronUSDT := getCurrency("TRON_USDT")
	tronFee := blockchain.TronFee{
		FeeLimitSun: 3500000,
		FeeLimitTRX: "0.35",
		FeeLimitUSD: "0.02",
	}

	m.SetupCalculateFee(tron, tron, false, blockchain.NewFee(tron, now, false, tronFee))
	m.SetupCalculateFee(tron, tron, true, blockchain.NewFee(tron, now, true, tronFee))
	m.SetupCalculateFee(tron, tronUSDT, false, blockchain.NewFee(tron, now, false, tronFee))
	m.SetupCalculateFee(tron, tronUSDT, true, blockchain.NewFee(tron, now, true, tronFee))

	// withdrawal fees
	m.SetupCalculateWithdrawalFeeUSD(tron, tron, false, lo.Must(money.USD.MakeAmount("50")))
	m.SetupCalculateWithdrawalFeeUSD(tron, tron, true, lo.Must(money.USD.MakeAmount("50")))
	m.SetupCalculateWithdrawalFeeUSD(tron, tronUSDT, false, lo.Must(money.USD.MakeAmount("80")))
	m.SetupCalculateWithdrawalFeeUSD(tron, tronUSDT, true, lo.Must(money.USD.MakeAmount("80")))
}

func (m *FeeCalculator) key(baseCurrency, currency money.CryptoCurrency, isTest bool) string {
	return fmt.Sprintf("%s/%s/test:%t", baseCurrency.Ticker, currency.Ticker, isTest)
}
