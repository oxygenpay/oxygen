package transaction

import (
	"context"
	"math/big"
	"time"

	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

func (c *CreateTransaction) validateForSystemTopup() error {
	if !c.Type.valid() {
		return errors.New("invalid type")
	}

	if !c.ServiceFee.IsZero() {
		return errors.New("service fee should be empty")
	}

	if c.Currency.Ticker != c.Amount.Ticker() {
		return errors.New("invalid currency specified")
	}

	if c.USDAmount.Type() != money.Fiat || c.USDAmount.Ticker() != money.USD.String() {
		return errors.New("invalid usd amount")
	}

	if c.Amount.IsZero() {
		return errors.New("amount can't be zero")
	}

	if c.Type != TypeVirtual {
		return errors.New("invalid type")
	}

	if c.EntityID == 0 {
		return errors.New("entity id should be present")
	}

	if c.SenderAddress != "" || c.SenderWallet != nil {
		return errors.New("sender should be empty")
	}

	if c.RecipientAddress != "" || c.RecipientWallet != nil {
		return errors.New("recipient should be empty")
	}

	if c.TransactionHash != "" {
		return errors.New("transaction hash should be empty")
	}

	return nil
}

func (s *Service) CreateSystemTopup(ctx context.Context, merchantID int64, params CreateTransaction) (*Transaction, error) {
	tx, err := s.createSystemTopup(ctx, merchantID, params)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create system transaction")
	}

	if tx.FactAmount == nil {
		return nil, errors.New("fact amount is nil")
	}

	tx, err = s.confirm(ctx, s.store, merchantID, tx.ID, ConfirmTransaction{
		Status:              StatusCompleted,
		FactAmount:          *tx.FactAmount,
		MetaData:            nil,
		allowZeroNetworkFee: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to confirm system transaction")
	}

	return tx, nil
}

func (s *Service) createSystemTopup(ctx context.Context, merchantID int64, params CreateTransaction) (*Transaction, error) {
	if err := params.validateForSystemTopup(); err != nil {
		return nil, err
	}

	networkCurrency, err := s.blockchain.GetNativeCoin(params.Currency.Blockchain)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get network currency")
	}

	var (
		now    = time.Now()
		status = StatusInProgress
		meta   = MetaData{MetaComment: "internal system topup"}
	)

	create := repository.CreateTransactionParams{
		CreatedAt: now,
		UpdatedAt: now,

		MerchantID: merchantID,
		EntityID:   repository.Int64ToNullable(params.EntityID),

		Status: string(status),
		Type:   string(params.Type),

		Blockchain:      params.Currency.Blockchain.String(),
		NetworkID:       repository.StringToNullable(params.Currency.ChooseNetwork(params.IsTest)),
		CurrencyType:    string(params.Currency.Type),
		Currency:        params.Currency.Ticker,
		Decimals:        int32(params.Amount.Decimals()),
		NetworkDecimals: int32(networkCurrency.Decimals),

		// note that amount == factAmount
		Amount:     repository.MoneyToNumeric(params.Amount),
		FactAmount: repository.MoneyToNumeric(params.Amount),
		UsdAmount:  repository.MoneyToNumeric(params.USDAmount),

		// note that fees are set to zero.
		NetworkFee: repository.BigIntToNumeric(big.NewInt(0)),
		ServiceFee: repository.BigIntToNumeric(big.NewInt(0)),

		Metadata: meta.toJSONB(),
		IsTest:   params.IsTest,
	}

	tx, err := s.store.CreateTransaction(ctx, create)
	if err != nil {
		return nil, err
	}

	return s.entryToTransaction(tx)
}
