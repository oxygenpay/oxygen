package processing

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	ErrInvalidInput = errors.New("invalid incoming input")
	ErrTransaction  = errors.New("transaction error")
)

type Input struct {
	Currency      money.CryptoCurrency
	Amount        money.Money
	SenderAddress string
	TransactionID string
	NetworkID     string
}

func (i Input) validate() error {
	if i.Currency.Ticker == "" {
		return errors.Wrap(ErrInvalidInput, "missing currency")
	}

	if i.Amount.Ticker() == "" {
		return errors.Wrap(ErrInvalidInput, "missing amount")
	}

	if i.SenderAddress == "" {
		return errors.Wrap(ErrInvalidInput, "missing SenderAddress")
	}

	if i.TransactionID == "" {
		return errors.Wrap(ErrInvalidInput, "missing TransactionID")
	}

	if i.NetworkID == "" {
		return errors.Wrap(ErrInvalidInput, "missing networkID")
	}

	return nil
}

// ProcessInboundTransaction implements correct business logic for transaction processing
func (s *Service) ProcessInboundTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	wt *wallet.Wallet,
	input Input,
) error {
	if err := input.validate(); err != nil {
		return err
	}

	if err := s.determineIncomingStatus(ctx, tx, input); err != nil {
		return err
	}

	// Step 1: Process transaction
	tx, err := s.transactions.Receive(ctx, tx.MerchantID, tx.ID, transaction.ReceiveTransaction{
		Status:          tx.Status,
		SenderAddress:   input.SenderAddress,
		TransactionHash: input.TransactionID,
		FactAmount:      input.Amount,
		MetaData:        tx.MetaData,
	})
	if err != nil {
		return errors.Wrap(err, "unable to update transaction")
	}

	paymentID := tx.EntityID

	if tx.Status != transaction.StatusInProgress {
		s.logger.Warn().
			Int64("wallet_id", wt.ID).
			Int64("transaction_id", tx.ID).
			Str("expected_amount", tx.Amount.String()).
			Str("actual_amount", input.Amount.String()).
			Msg("received invalid transaction that has not expected amount")

		return nil
	}

	// Step 2: Process payment
	pt, err := s.payments.GetByID(ctx, tx.MerchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	_, err = s.payments.Update(ctx, tx.MerchantID, pt.ID, payment.UpdateProps{Status: payment.StatusInProgress})
	if err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Info().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", paymentID).
		Msg("marked payment as in progress")

	return nil
}

func (s *Service) createUnexpectedTransaction(ctx context.Context, wt *wallet.Wallet, input Input) error {
	isTest := input.Currency.NetworkID != input.NetworkID

	conv, err := s.blockchain.CryptoToFiat(ctx, input.Amount, money.USD)
	if err != nil {
		return errors.Wrapf(err, "unable to convert %s to USD", input.Currency.Ticker)
	}

	params := transaction.CreateTransaction{
		Type:            transaction.TypeIncoming,
		SenderAddress:   input.SenderAddress,
		RecipientWallet: wt,
		TransactionHash: input.TransactionID,
		Currency:        input.Currency,
		Amount:          input.Amount,
		USDAmount:       conv.To,
		IsTest:          isTest,
	}

	_, err = s.transactions.Create(ctx, transaction.SystemMerchantID, params, transaction.IncomingUnexpected())
	if err != nil {
		return errors.Wrap(err, "unable to store unexpected transaction")
	}

	return nil
}

func (s *Service) determineIncomingStatus(ctx context.Context, tx *transaction.Transaction, input Input) error {
	if input.Amount.Equals(tx.Amount) {
		tx.Status = transaction.StatusInProgress
		return nil
	}

	if input.Amount.GreaterThan(tx.Amount) {
		tx.Status = transaction.StatusInProgress
		tx.MetaData[transaction.MetaComment] = "incoming tx amount is higher than expected"

		return nil
	}

	// If amount is less than expected we can tolerate $0.01 round error
	oneCent, err := money.USD.MakeAmount("1")
	if err != nil {
		return err
	}

	conv, err := s.blockchain.FiatToCrypto(ctx, oneCent, tx.Currency)
	if err != nil {
		return err
	}

	amountWithOneCent, err := input.Amount.Add(conv.To)
	if err != nil {
		return err
	}

	if amountWithOneCent.GreaterThanOrEqual(tx.Amount) {
		tx.Status = transaction.StatusInProgress
		return nil
	}

	// Even when adding $0.01 in crypto to input.Amount it's still less than required.
	// In that case let's mark tx as inProgressInvalid
	tx.Status = transaction.StatusInProgressInvalid
	tx.MetaData[transaction.MetaErrorReason] = "incoming tx amount is less than expected"

	return nil
}

func (s *Service) BatchCheckIncomingTransactions(ctx context.Context, transactionIDs []int64) error {
	var (
		group     errgroup.Group
		checked   int64
		failedTXs []int64
		mu        sync.Mutex
	)

	group.SetLimit(8)

	for i := range transactionIDs {
		txID := transactionIDs[i]
		group.Go(func() error {
			if err := s.checkIncomingTransaction(ctx, txID); err != nil {
				mu.Lock()
				failedTXs = append(failedTXs, txID)
				mu.Unlock()

				return err
			}

			atomic.AddInt64(&checked, 1)

			return nil
		})
	}

	err := group.Wait()

	evt := s.logger.Info()
	if err != nil {
		evt = s.logger.Error().Err(err)
	}

	evt.Int64("checked_transactions_count", checked).
		Ints64("transaction_ids", transactionIDs).
		Ints64("failed_transaction_ids", failedTXs).
		Msg("Checked incoming transactions")

	return err
}

func (s *Service) checkIncomingTransaction(ctx context.Context, txID int64) error {
	tx, err := s.transactions.GetByID(ctx, transaction.MerchantIDWildcard, txID)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction")
	}

	switch {
	case tx.Type != transaction.TypeIncoming:
		return errors.New("invalid transaction type")
	case tx.HashID == nil:
		return errors.New("empty transaction hash")
	case tx.SenderAddress == nil:
		return errors.New("empty sender address")
	case tx.RecipientWalletID == nil:
		return errors.New("empty recipient wallet id")
	case !tx.IsInProgress():
		return nil
	}

	receipt, err := s.blockchain.GetTransactionReceipt(ctx, tx.Currency.Blockchain, *tx.HashID, tx.IsTest)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction receipt")
	}

	if !receipt.IsConfirmed {
		// check later
		return nil
	}

	if !receipt.Success {
		return s.cancelIncomingTransaction(ctx, tx)
	}

	return s.confirmIncomingTransaction(ctx, tx, receipt)
}

func (s *Service) confirmIncomingTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Info().Int64("transaction_id", tx.ID).Msg("confirming incoming transaction")

	setTXStatus := transaction.StatusCompleted
	setPaymentStatus := payment.StatusSuccess

	if tx.Status == transaction.StatusInProgressInvalid {
		setTXStatus = transaction.StatusCompletedInvalid
		setPaymentStatus = payment.StatusFailed
	}

	confirmation := transaction.ConfirmTransaction{
		Status:          setTXStatus,
		SenderAddress:   *tx.SenderAddress,
		TransactionHash: *tx.HashID,
		FactAmount:      *tx.FactAmount,
		NetworkFee:      receipt.NetworkFee,
		MetaData:        tx.MetaData,
	}

	confirmation.AllowZeroNetworkFee()

	tx, err := s.transactions.Confirm(ctx, tx.MerchantID, tx.ID, confirmation)
	if err != nil {
		return errors.Wrap(err, "unable to confirm transaction")
	}

	if tx.MerchantID == transaction.SystemMerchantID {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Str("transaction_status", string(tx.Status)).
			Msg("processed unexpected incoming transaction")

		return nil
	}

	paymentID := tx.EntityID

	pt, err := s.payments.GetByID(ctx, tx.MerchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	pt, err = s.payments.Update(ctx, tx.MerchantID, pt.ID, payment.UpdateProps{Status: setPaymentStatus})
	if err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Info().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", paymentID).
		Str("transaction_status", string(tx.Status)).
		Str("payment_status", string(pt.Status)).
		Msg("processed payment")

	return nil
}

func (s *Service) cancelIncomingTransaction(ctx context.Context, tx *transaction.Transaction) error {
	err := s.transactions.Cancel(ctx, tx, transaction.StatusFailed, revertReason, nil)
	if err != nil {
		return errors.Wrap(err, "unable to cancel transaction")
	}

	if tx.MerchantID == transaction.SystemMerchantID {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Str("transaction_status", string(tx.Status)).
			Msg("canceled unexpected incoming transaction")

		return nil
	}

	paymentID := tx.EntityID

	_, err = s.payments.Update(ctx, tx.MerchantID, paymentID, payment.UpdateProps{Status: payment.StatusFailed})
	if err != nil {
		return errors.Wrap(err, "unable to update payment")
	}

	s.logger.Error().
		Int64("transaction_id", tx.ID).
		Int64("payment_id", paymentID).
		Str("transaction_hash", *tx.HashID).
		Msg("incoming transaction has failed")

	return nil
}

func (s *Service) BatchExpirePayments(ctx context.Context, paymentsIDs []int64) error {
	var (
		group        errgroup.Group
		expiredCount int64
		failedIDs    []int64
		mu           sync.Mutex
	)

	group.SetLimit(8)

	for i := range paymentsIDs {
		paymentID := paymentsIDs[i]
		group.Go(func() error {
			if err := s.expirePayment(ctx, paymentID); err != nil {
				mu.Lock()
				failedIDs = append(failedIDs, paymentID)
				mu.Unlock()

				return err
			}

			atomic.AddInt64(&expiredCount, 1)

			return nil
		})
	}

	err := group.Wait()

	evt := s.logger.Info()
	if err != nil {
		evt = s.logger.Error().Err(err)
	}

	evt.Int64("expired_payments_count", expiredCount).
		Ints64("payments_ids", paymentsIDs).
		Ints64("failed_payments_ids", failedIDs).
		Msg("canceled expired payments")

	return err
}

func (s *Service) expirePayment(ctx context.Context, paymentID int64) error {
	pt, err := s.payments.GetByID(ctx, payment.MerchantIDWildcard, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get payment")
	}

	if pt.Type != payment.TypePayment {
		return errors.Errorf("invalid payment type %q", pt.Type)
	}

	if pt.Status != payment.StatusPending && pt.Status != payment.StatusLocked {
		return errors.Errorf("invalid payment status %q", pt.Status)
	}

	// 1. Cancel if tx exists
	tx, err := s.transactions.GetLatestByPaymentID(ctx, pt.ID)
	switch {
	case errors.Is(err, transaction.ErrNotFound):
		// that's expected, do nothing
	case err != nil:
		return errors.Wrap(err, "unable to get transaction")
	}

	if tx != nil && tx.Status != transaction.StatusCancelled {
		errCancel := s.transactions.Cancel(ctx, tx, transaction.StatusCancelled, "payment expired", nil)
		if errCancel != nil {
			return errors.Wrap(err, "unable to cancel transaction")
		}
	}

	// 2. Cancel payment itself
	if errFail := s.payments.Fail(ctx, pt); errFail != nil {
		return errors.Wrap(errFail, "unable to expire payment")
	}

	return nil
}
