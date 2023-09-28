package payment

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
)

type CreateWithdrawalProps struct {
	BalanceID uuid.UUID
	AddressID uuid.UUID
	AmountRaw string // "0.123"
}

func (s *Service) ListWithdrawals(ctx context.Context, status Status, filterByIDs []int64) ([]*Payment, error) {
	results, err := s.repo.GetPaymentsByType(ctx, repository.GetPaymentsByTypeParams{
		Type:        string(TypeWithdrawal),
		Status:      string(status),
		FilterByIds: len(filterByIDs) > 0,
		ID:          util.MapSlice(filterByIDs, func(id int64) int32 { return int32(id) }),
		Limit:       200,
	})

	if err != nil {
		return nil, err
	}

	if len(filterByIDs) > 0 && len(results) != len(filterByIDs) {
		return nil, fmt.Errorf("withdrawals filter mismatch for status %q", status)
	}

	payments := make([]*Payment, len(results))
	for i := range results {
		pt, err := s.entryToPayment(results[i])
		if err != nil {
			return nil, err
		}

		payments[i] = pt
	}

	return payments, nil
}

func (s *Service) CreateWithdrawal(ctx context.Context, merchantID int64, props CreateWithdrawalProps) (*Payment, error) {
	// 1. Resolve address, balance & parse amount
	address, err := s.merchants.GetMerchantAddressByUUID(ctx, merchantID, props.AddressID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get merchant address")
	}

	balance, err := s.wallets.GetMerchantBalanceByUUID(ctx, merchantID, props.BalanceID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get merchant balance")
	}

	if string(address.Blockchain) != balance.Network {
		return nil, ErrAddressBalanceMismatch
	}

	ticker := balance.Amount.Ticker()

	amount, err := money.CryptoFromStringFloat(ticker, props.AmountRaw, balance.Amount.Decimals())
	if err != nil {
		return nil, err
	}
	if !amount.CompatibleTo(balance.Amount) {
		return nil, ErrAddressBalanceMismatch
	}

	// 2. Check if balance has sufficient funds
	withdrawalFee, err := s.GetWithdrawalFee(ctx, merchantID, balance.UUID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get withdrawal fee")
	}
	if errCovers := balance.Covers(amount, withdrawalFee.CryptoFee); errCovers != nil {
		return nil, errors.WithMessagef(
			ErrWithdrawalInsufficientBalance,
			"balance of %s %s is less than requested %s %s + withdrawal fee of %s %s ($%s)",
			balance.Amount.String(),
			balance.Amount.Ticker(),
			amount.String(),
			amount.Ticker(),
			withdrawalFee.CryptoFee.String(),
			withdrawalFee.CryptoFee.Ticker(),
			withdrawalFee.USDFee.String(),
		)
	}

	// 2. Check if amount more than minimal
	minimalUSDLimit, err := s.blockchain.GetMinimalWithdrawalByTicker(ticker)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get minimal withdrawal amount for %q", ticker)
	}

	currency, err := s.blockchain.GetCurrencyByTicker(ticker)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get currency %q", ticker)
	}

	conversion, err := s.blockchain.FiatToCrypto(ctx, minimalUSDLimit, currency)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to convert USD to crypto %q", ticker)
	}

	// minimal amount that can be withdrawn from merchant's balance
	minimalCryptoLimit := conversion.To

	if !amount.CompatibleTo(minimalCryptoLimit) {
		return nil, money.ErrIncompatibleMoney
	}

	if amount.LessThan(minimalCryptoLimit) {
		return nil, errors.Wrapf(
			ErrWithdrawalAmountTooSmall,
			"minimum withdrawal amount is %s %s ($%s)",
			minimalCryptoLimit.String(),
			minimalCryptoLimit.Ticker(),
			minimalUSDLimit.String(),
		)
	}

	// 3. Create withdrawal
	publicID := uuid.New()
	isTest := balance.NetworkID != currency.NetworkID

	p, err := s.repo.CreatePayment(ctx, repository.CreatePaymentParams{
		PublicID:          publicID,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Type:              TypeWithdrawal.String(),
		Status:            StatusPending.String(),
		MerchantID:        merchantID,
		MerchantOrderUuid: publicID,
		Price:             repository.MoneyToNumeric(amount),
		Decimals:          int32(amount.Decimals()),
		Currency:          amount.Ticker(),
		Description:       repository.StringToNullable("Balance withdrawal"),
		IsTest:            isTest,
		Metadata: Metadata{
			MetaBalanceID: strconv.Itoa(int(balance.ID)),
			MetaAddressID: strconv.Itoa(int(address.ID)),
		}.ToJSONB(),
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to create payment")
	}

	err = s.publisher.Publish(bus.TopicWithdrawals, bus.WithdrawalCreatedEvent{
		MerchantID: p.MerchantID,
		PaymentID:  p.ID,
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to publish WithdrawalCreatedEvent event")
	}

	return s.entryToPayment(p)
}

type WithdrawalFee struct {
	CalculatedAt time.Time
	Blockchain   money.Blockchain
	Currency     string
	USDFee       money.Money
	CryptoFee    money.Money
	IsTest       bool
}

func (s *Service) GetWithdrawalFee(ctx context.Context, merchantID int64, balanceID uuid.UUID) (*WithdrawalFee, error) {
	balance, err := s.wallets.GetMerchantBalanceByUUID(ctx, merchantID, balanceID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get merchant balance")
	}

	// e.g. ETH_USDT
	currency, err := s.blockchain.GetCurrencyByTicker(balance.Currency)
	if err != nil {
		return nil, errors.Wrap(err, "unable to  get currency by ticker")
	}

	// e.g. ETH
	baseCurrency, err := s.blockchain.GetNativeCoin(currency.Blockchain)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get currency by ticker")
	}

	isTest := balance.NetworkID != currency.NetworkID

	usdFee, err := s.blockchain.CalculateWithdrawalFeeUSD(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get fee")
	}

	conv, err := s.blockchain.FiatToCrypto(ctx, usdFee, currency)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get currency withdrawal fee in crypto")
	}

	return &WithdrawalFee{
		CalculatedAt: time.Now(),
		Blockchain:   currency.Blockchain,
		Currency:     currency.Ticker,
		IsTest:       isTest,
		USDFee:       usdFee,
		CryptoFee:    conv.To,
	}, nil
}
