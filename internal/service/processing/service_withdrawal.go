package processing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// BatchCreateWithdrawals ingests list of withdrawals and creates & broadcasts transactions.
func (s *Service) BatchCreateWithdrawals(ctx context.Context, withdrawalIDs []int64) (*TransferResult, error) {
	withdrawals, err := s.payments.ListWithdrawals(ctx, payment.StatusPending, withdrawalIDs)
	if err != nil {
		return nil, err
	}

	// 1.Validate payments
	if errValidate := s.validateWithdrawals(withdrawals); errValidate != nil {
		return nil, errValidate
	}

	// 2. Get OUTBOUND wallets and balances
	outboundWallets, outboundBalances, err := s.getOutboundWalletsWithBalancesAsMap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get outbound wallets with balances")
	}

	result := &TransferResult{}

	// 3. For each withdrawal:
	// - Resolve currency
	// - Resolve outbound system wallet & balance
	// - Resolve merchant balance & withdrawal address
	// - Create withdrawal
	// - Rollback failed withdrawal
	group := errgroup.Group{}
	group.SetLimit(8)
	for i := range withdrawals {
		withdrawal := withdrawals[i]
		group.Go(func() error {
			currency, err := s.blockchain.GetCurrencyByTicker(withdrawal.Price.Ticker())
			if err != nil {
				result.registerErr(errors.Wrap(err, "unable to get withdrawal currency"))
				return nil
			}

			systemBalanceKey := balanceKey(&wallet.Balance{
				EntityType: wallet.EntityTypeWallet,
				NetworkID:  currency.ChooseNetwork(withdrawal.IsTest),
				Currency:   currency.Ticker,
			})

			systemBalance, ok := outboundBalances[systemBalanceKey]
			if !ok {
				result.registerErr(errors.New("unable to get withdrawal wallet balance"))
				return nil
			}

			withdrawalWallet, ok := outboundWallets[systemBalance.EntityID]
			if !ok {
				result.registerErr(errors.New("unable to get withdrawal wallet"))
				return nil
			}

			merchantBalance, err := s.wallets.GetBalanceByID(
				ctx,
				wallet.EntityTypeMerchant,
				withdrawal.MerchantID,
				withdrawal.WithdrawalBalanceID(),
			)
			if err != nil {
				result.registerErr(errors.Wrap(err, "unable to get merchant balance"))
				return nil
			}

			merchantAddress, err := s.merchants.GetMerchantAddressByID(
				ctx,
				withdrawal.MerchantID,
				withdrawal.WithdrawalAddressID(),
			)
			if err != nil {
				result.registerErr(errors.Wrap(err, "unable to get merchant address"))
				return nil
			}

			params := withdrawalInput{
				Withdrawal:      withdrawal,
				Wallet:          withdrawalWallet,
				SystemBalance:   systemBalance,
				MerchantBalance: merchantBalance,
				MerchantAddress: merchantAddress,
			}

			output, errWithdrawal := s.createWithdrawal(ctx, params)

			if errWithdrawal != nil {
				s.logger.Error().Err(errWithdrawal).
					Int64("payment_id", withdrawal.ID).
					Int64("merchant_id", withdrawal.MerchantID).
					Msg("unable to create withdrawal. performing rollback")

				errRollback := s.rollbackWithdrawal(ctx, params, output, errWithdrawal)
				result.registerErr(errRollback)

				if errRollback != nil {
					return errors.Wrap(errRollback, "unable to rollback withdrawal")
				}

				s.logger.Info().
					Str("operation", "withdrawal").
					Int64("payment_id", withdrawal.ID).
					Int64("merchant_id", withdrawal.MerchantID).
					Msg("rollback completed")

				if output.Transaction != nil {
					result.addRollbackID(output.Transaction.ID)
				}

				return nil
			}

			result.addTransaction(output.Transaction)

			return nil
		})
	}

	return result, group.Wait()
}

func (s *Service) BatchCheckWithdrawals(ctx context.Context, transactionIDs []int64) error {
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
			if err := s.checkWithdrawalTransaction(ctx, txID); err != nil {
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
		Ints64("failed_transaction_ids", failedTXs).
		Ints64("transaction_ids", transactionIDs).
		Msg("Checked withdrawal transactions")

	return err
}

type withdrawalInput struct {
	Withdrawal      *payment.Payment
	Wallet          *wallet.Wallet
	SystemBalance   *wallet.Balance
	MerchantBalance *wallet.Balance
	MerchantAddress *merchant.Address
}

type withdrawalOutput struct {
	Transaction         *transaction.Transaction
	TransactionRaw      string
	TransactionHashID   string
	MarkPaymentAsFailed bool
	ServiceFee          money.Money
	BalanceDecremented  bool
	IsTest              bool
}

//nolint:gocyclo
func (s *Service) createWithdrawal(ctx context.Context, params withdrawalInput) (withdrawalOutput, error) {
	out := withdrawalOutput{}

	// 0. Get currency & baseCurrency (e.g. ETH and ETH_USDT)
	baseCurrency, err := s.blockchain.GetNativeCoin(params.MerchantBalance.Blockchain())
	if err != nil {
		return out, errors.Wrap(err, "unable to get base currency")
	}

	currency, err := s.blockchain.GetCurrencyByTicker(params.MerchantBalance.Currency)
	if err != nil {
		return out, errors.Wrap(err, "unable to get currency")
	}

	isTest := currency.NetworkID != params.MerchantBalance.NetworkID
	out.IsTest = isTest

	// 1. Calculate withdrawal fee in USD
	withdrawalFeeUSD, err := s.blockchain.CalculateWithdrawalFeeUSD(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return out, errors.Wrapf(err, "unable to get currency withdrawal fee in USD")
	}

	withdrawalFeeCrypto, err := s.blockchain.FiatToCrypto(ctx, withdrawalFeeUSD, currency)
	if err != nil {
		return out, errors.Wrapf(err, "unable to get currency withdrawal fee in crypto")
	}

	amount := params.Withdrawal.Price
	serviceFee := withdrawalFeeCrypto.To
	out.ServiceFee = serviceFee

	// 2. Ensure that merchant balance & outbound wallet have enough funds
	if errBalance := params.MerchantBalance.Covers(amount, serviceFee); errBalance != nil {
		// (!) That's important: explicitly mark withdrawal as failed
		out.MarkPaymentAsFailed = true

		return out, errors.Wrap(errBalance, "merchant balance has not enough funds for withdrawal")
	}
	if errBalance := params.SystemBalance.Covers(amount); errBalance != nil {
		return out, errors.Wrap(errBalance, "system balance has not enough funds for withdrawal")
	}

	// 3. Create signed transaction via KMS
	txNetworkFee, err := s.blockchain.CalculateFee(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return out, errors.Wrap(err, "unable to calculate network fee")
	}

	txRaw, err := s.wallets.CreateSignedTransaction(
		ctx,
		params.Wallet,
		params.MerchantAddress.Address,
		currency,
		amount,
		txNetworkFee,
		isTest,
	)
	if err != nil {
		return out, errors.Wrapf(err, "unable to create raw signed transaction")
	}

	out.TransactionRaw = txRaw

	// 5. Convert price to USD
	conv, err := s.blockchain.CryptoToFiat(ctx, amount, money.USD)
	if err != nil {
		return out, errors.Wrapf(err, "unable to convert %s to USD", amount.Ticker())
	}

	// 6. Create transaction in the DB
	tx, err := s.transactions.Create(ctx, params.Withdrawal.MerchantID, transaction.CreateTransaction{
		Type:             transaction.TypeWithdrawal,
		EntityID:         params.Withdrawal.ID,
		SenderWallet:     params.Wallet,
		RecipientAddress: params.MerchantAddress.Address,
		Currency:         currency,
		Amount:           amount,
		USDAmount:        conv.To,
		ServiceFee:       serviceFee,
		IsTest:           isTest,
	})
	if err != nil {
		return out, errors.Wrap(err, "unable to create database transaction")
	}

	out.Transaction = tx

	// 7. Inside a DB tx: decrement merchant balance and decrement wallet balance
	// take service fee & network fee into account
	err = s.wallets.UpdateBalancesForWithdrawal(ctx, wallet.UpdateBalancesForWithdrawal{
		Operation:     wallet.OperationDecrement,
		TransactionID: tx.ID,
		System:        params.SystemBalance,
		Merchant:      params.MerchantBalance,
		Amount:        amount,
		ServiceFee:    serviceFee,
		Comment:       "Decrementing balances for withdrawal",
	})

	switch {
	case errors.Is(err, wallet.ErrInsufficienceMerchantBalance):
		// (!) That's important: explicitly mark withdrawal as failed
		out.MarkPaymentAsFailed = true
		fallthrough
	case err != nil:
		return out, errors.Wrap(err, "unable to decrement balances for withdrawal")
	}

	out.BalanceDecremented = true

	// 8. Broadcast transaction to blockchain
	transactionHashID, err := s.blockchain.BroadcastTransaction(ctx, currency.Blockchain, txRaw, isTest)

	switch {
	case errors.Is(err, blockchain.ErrInsufficientFunds):
		// (!) That's important: explicitly mark withdrawal as failed
		out.MarkPaymentAsFailed = true
		fallthrough
	case err != nil:
		return out, errors.Wrapf(err, "unable to broadcast transaction to %s", currency.Blockchain)
	}

	out.TransactionHashID = transactionHashID

	// 9. Update transaction hash
	errUpdate := s.transactions.UpdateTransactionHash(ctx, params.Withdrawal.MerchantID, tx.ID, transactionHashID)
	if errUpdate != nil {
		// todo
		//  well, this shouldn't happen, but tx is already broadcasted
		//  think about possible solutions
		s.logger.Error().Err(errUpdate).
			Int64("transaction_id", tx.ID).Str("transaction_hash_id", transactionHashID).
			Msg("unable to update database tx hash id")
	}

	// 10. Update payment's status
	_, err = s.payments.Update(
		ctx,
		params.Withdrawal.MerchantID,
		params.Withdrawal.ID,
		payment.UpdateProps{Status: payment.StatusInProgress},
	)
	if err != nil {
		return out, errors.Wrap(err, "unable to update payment")
	}

	// 11. if currency is TOKEN, then "steal" COIN balance and decrement it.
	// UPD: we can do it when receiving confirmation webhook "transaction processed"
	// because otherwise it's impossible to determine exact tx fees.

	return out, nil
}

func (s *Service) rollbackWithdrawal(
	ctx context.Context,
	in withdrawalInput,
	out withdrawalOutput,
	errOut error,
) error {
	if out.TransactionRaw != "" {
		if err := s.wallets.DecrementPendingTransaction(ctx, in.Wallet.ID, out.IsTest); err != nil {
			return errors.Wrap(err, "unable to decrement pending transaction")
		}
	}

	if out.Transaction != nil {
		msg := fmt.Sprintf("withdrawal rollback. Reason: %s", errOut.Error())
		err := s.transactions.Cancel(ctx, out.Transaction, transaction.StatusCancelled, msg, nil)
		if err != nil {
			return errors.Wrap(err, "unable to cancel transaction")
		}
	}

	if out.BalanceDecremented {
		err := s.wallets.UpdateBalancesForWithdrawal(ctx, wallet.UpdateBalancesForWithdrawal{
			Operation:     wallet.OperationIncrement,
			TransactionID: in.Withdrawal.ID,
			System:        in.SystemBalance,
			Merchant:      in.MerchantBalance,
			Amount:        in.Withdrawal.Price,
			ServiceFee:    out.ServiceFee,
			Comment:       "Balance rollback after failed transaction",
		})
		if err != nil {
			return errors.Wrap(err, "unable to update balances")
		}
	}

	if out.MarkPaymentAsFailed {
		_, err := s.payments.Update(
			ctx,
			in.Withdrawal.MerchantID,
			in.Withdrawal.ID,
			payment.UpdateProps{Status: payment.StatusFailed},
		)
		if err != nil {
			return errors.Wrap(err, "unable to update withdrawal")
		}
	}

	return nil
}

func (s *Service) checkWithdrawalTransaction(ctx context.Context, txID int64) error {
	tx, err := s.transactions.GetByID(ctx, transaction.MerchantIDWildcard, txID)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction")
	}

	if tx.HashID == nil {
		return errors.New("empty transaction hash")
	}

	if tx.SenderWalletID == nil {
		return errors.New("empty sender wallet id")
	}

	receipt, err := s.blockchain.GetTransactionReceipt(ctx, tx.Currency.Blockchain, *tx.HashID, tx.IsTest)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction receipt")
	}

	if !receipt.IsConfirmed {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Bool("is_test", tx.IsTest).
			Str("transaction_hash", *tx.HashID).Msg("withdrawal transaction is not confirmed yet")

		return nil
	}

	if !receipt.Success {
		if err := s.cancelWithdrawal(ctx, tx, receipt); err != nil {
			return errors.Wrap(err, "unable to cancel withdrawal")
		}

		return nil
	}

	if err := s.confirmWithdrawal(ctx, tx, receipt); err != nil {
		return errors.Wrap(err, "unable to confirm withdrawal")
	}

	return nil
}

func (s *Service) confirmWithdrawal(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Info().Int64("transaction_id", tx.ID).Msg("confirming withdrawal")

	// 1. Confirm nonce
	if err := s.wallets.IncrementConfirmedTransaction(ctx, *tx.SenderWalletID, tx.IsTest); err != nil {
		return errors.Wrap(err, "unable to confirm nonce")
	}

	// 2. Mark tx as completed
	_, err := s.transactions.Confirm(ctx, tx.MerchantID, tx.ID, transaction.ConfirmTransaction{
		Status:          transaction.StatusCompleted,
		SenderAddress:   *tx.SenderAddress,
		TransactionHash: *tx.HashID,
		FactAmount:      tx.Amount,
		NetworkFee:      receipt.NetworkFee,
		MetaData:        tx.MetaData,
	})
	if err != nil {
		return errors.Wrap(err, "unable to confirm transaction")
	}

	// 3. Mark payment as successful
	_, err = s.payments.Update(ctx, tx.MerchantID, tx.EntityID, payment.UpdateProps{Status: payment.StatusSuccess})
	if err != nil {
		return errors.Wrap(err, "unable to mark withdrawal as successful")
	}

	return nil
}

// cancelWithdrawal cancels withdrawal after system received confirmation of tx failure (revert).
// This can happen when tx exceeded gas limit.
func (s *Service) cancelWithdrawal(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Error().
		Int64("transaction_id", tx.ID).
		Str("blockchain", receipt.Blockchain.String()).
		Str("network_id", tx.NetworkID()).
		Str("transaction_hash", receipt.Hash).
		Msg("canceling withdrawal")

	// 1. Confirm nonce
	if err := s.wallets.IncrementConfirmedTransaction(ctx, *tx.SenderWalletID, tx.IsTest); err != nil {
		return errors.Wrap(err, "unable to confirm nonce")
	}

	// 2. Mark tx as failed
	err := s.transactions.Cancel(ctx, tx, transaction.StatusFailed, revertReason, &receipt.NetworkFee)
	if err != nil {
		return errors.Wrap(err, "unable to cancel transaction")
	}

	// 3. Restore balances to previous state
	ticker := tx.Currency.Ticker
	networkID := tx.NetworkID()

	senderBalance, err := s.wallets.GetWalletsBalance(ctx, *tx.SenderWalletID, ticker, networkID)
	if err != nil {
		return errors.Wrap(err, "unable to get sender wallet balance")
	}

	recipientBalance, err := s.wallets.GetMerchantBalance(ctx, tx.MerchantID, ticker, networkID)
	if err != nil {
		return errors.Wrap(err, "unable to get merchant wallet balance")
	}

	err = s.wallets.UpdateBalancesForWithdrawal(ctx, wallet.UpdateBalancesForWithdrawal{
		Operation:     wallet.OperationIncrement,
		TransactionID: tx.ID,
		System:        senderBalance,
		Merchant:      recipientBalance,
		Amount:        tx.Amount,
		ServiceFee:    tx.ServiceFee,
		Comment:       "transaction was reverted by blockchain",
	})
	if err != nil {
		return errors.Wrap(err, "unable update balances for withdrawal")
	}

	// 4. Mark payment as failed
	_, err = s.payments.Update(ctx, tx.MerchantID, tx.EntityID, payment.UpdateProps{Status: payment.StatusFailed})
	if err != nil {
		return errors.Wrap(err, "unable to mark withdrawal as failed")
	}

	return nil
}

func (s *Service) validateWithdrawals(withdrawals []*payment.Payment) error {
	for _, pt := range withdrawals {
		if pt.Type != payment.TypeWithdrawal {
			return errors.Wrap(ErrInvalidInput, "payment is not withdrawal")
		}

		if pt.Status != payment.StatusPending {
			return errors.Wrap(ErrInvalidInput, "withdrawal is not pending")
		}

		if pt.MerchantID == 0 {
			return errors.Wrap(ErrInvalidInput, "invalid merchant id")
		}

		if pt.WithdrawalBalanceID() < 1 {
			return errors.Wrap(ErrInvalidInput, "invalid balance id")
		}

		if pt.WithdrawalAddressID() < 1 {
			return errors.Wrap(ErrInvalidInput, "invalid address id")
		}
	}

	return nil
}
