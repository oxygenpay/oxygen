package processing

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type TopupInput struct {
	Currency money.CryptoCurrency
	Amount   money.Money
	Comment  string
	IsTest   bool
}

type TopupOut struct {
	Payment         *payment.Payment
	Transaction     *transaction.Transaction
	MerchantBalance *wallet.Balance
}

// TopupMerchantFromSystem performs an internal transfer from a non-materialized system balance to merchant's balance.
// Useful for resolving customer support requests.
func (s *Service) TopupMerchantFromSystem(ctx context.Context, merchantID int64, in TopupInput) (TopupOut, error) {
	merchant, err := s.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		return TopupOut{}, errors.Wrap(err, "unable to find merchant")
	}

	systemBalance, err := s.locateSystemBalance(ctx, in.Currency, in.IsTest)
	if err != nil {
		return TopupOut{}, errors.Wrap(err, "unable to find system balance")
	}

	if err = systemBalance.Covers(in.Amount); err != nil {
		return TopupOut{}, errors.Wrap(err, "system balance has no sufficient funds")
	}

	conv, err := s.blockchain.CryptoToFiat(ctx, in.Amount, money.USD)
	if err != nil {
		return TopupOut{}, errors.Wrap(err, "unable to convert crypto to USD")
	}

	usdAmount := conv.To

	paymentProps := payment.CreateInternalPaymentProps{
		MerchantOrderUUID: uuid.New(),
		Money:             in.Amount,
		Description:       fmt.Sprintf("*internal* topup of %s %s (%s)", in.Amount.String(), in.Currency.Ticker, in.Comment),
		IsTest:            in.IsTest,
	}

	pt, err := s.payments.CreateSystemTopup(ctx, merchant.ID, paymentProps)
	if err != nil {
		return TopupOut{}, errors.Wrap(err, "unable to create the payment")
	}

	txProps := transaction.CreateTransaction{
		Type:      transaction.TypeVirtual,
		EntityID:  pt.ID,
		Currency:  in.Currency,
		Amount:    in.Amount,
		USDAmount: usdAmount,
		IsTest:    in.IsTest,
	}

	tx, err := s.transactions.CreateSystemTopup(ctx, merchant.ID, txProps)
	if err != nil {
		if errFail := s.payments.Fail(ctx, pt); errFail != nil {
			return TopupOut{}, errors.Wrap(errFail, "unable to delete internal payment after failed system topup tx")
		}

		return TopupOut{}, errors.Wrap(err, "unable to create internal transaction")
	}

	balance, err := s.wallets.GetMerchantBalance(ctx, merchant.ID, in.Currency.Ticker, in.Currency.ChooseNetwork(in.IsTest))
	if err != nil {
		return TopupOut{}, errors.Wrap(err, "unable to get merchants balance")
	}

	return TopupOut{
		Payment:         pt,
		Transaction:     tx,
		MerchantBalance: balance,
	}, nil
}

// this operation might be slow and expensive in the future as it lists ALL balances
// and calculates "system" ones under the hood.
func (s *Service) locateSystemBalance(ctx context.Context, currency money.CryptoCurrency, isTest bool) (*wallet.Balance, error) {
	balances, err := s.wallets.ListAllBalances(ctx, wallet.ListAllBalancesOpts{WithSystemBalances: true})
	if err != nil {
		return nil, err
	}

	desiredNetworkID := currency.ChooseNetwork(isTest)

	search := func(b *wallet.Balance) bool {
		tickerMatches := b.Currency == currency.Ticker
		networkMatches := b.NetworkID == desiredNetworkID

		return tickerMatches && networkMatches
	}

	systemBalance, ok := lo.Find(balances[wallet.EntityTypeSystem], search)
	if !ok {
		return nil, errors.New("system balance not found")
	}

	return systemBalance, nil
}
