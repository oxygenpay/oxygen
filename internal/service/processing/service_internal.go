package processing

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

// transferableCoinPercentage determines how much funds to transfer from inbound to outbound wallet in case of native
// network coin. We cant' transfer 100% because we need to make sure there are enough fees for other transactions.
const transferableCoinPercentage = 0.90

const revertReason = "blockchain confirmed failure (revert)"

type TransferResult struct {
	mu sync.Mutex

	CreatedTransactions      []*transaction.Transaction
	RollbackedTransactionIDs []int64
	UnhandledErrors          []error

	// len(UnhandledErrors) + len(RollbackedTransactionIDs)
	TotalErrors int64
}

func (r *TransferResult) addTransaction(tx *transaction.Transaction) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.CreatedTransactions = append(r.CreatedTransactions, tx)
}

func (r *TransferResult) addRollbackID(txID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.RollbackedTransactionIDs = append(r.RollbackedTransactionIDs, txID)
}

func (r *TransferResult) registerErr(err error) {
	atomic.AddInt64(&r.TotalErrors, 1)

	if err != nil {
		r.mu.Lock()
		r.UnhandledErrors = append(r.UnhandledErrors, err)
		r.mu.Unlock()
	}
}

// BatchCreateInternalTransfers receives INBOUND balances and transfers funds to OUTBOUND ones.
func (s *Service) BatchCreateInternalTransfers(
	ctx context.Context,
	inboundBalances []*wallet.Balance,
) (*TransferResult, error) {
	// 1. Validate INBOUND balances
	if err := s.validateInboundBalances(ctx, inboundBalances); err != nil {
		return nil, errors.Wrap(err, "validation error: balances are invalid")
	}

	// 2. Fetch OUTBOUND wallets & balances
	outboundWallets, outboundBalances, err := s.getOutboundWalletsWithBalancesAsMap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get outbound wallets with balances")
	}

	result := &TransferResult{}

	// 3. For each INBOUND balance:
	// - Match exact balance for OUTBOUND balance (e.g. same currency/network etc...)
	// - Create internal transfer
	// - Rollback if error occurred
	group := errgroup.Group{}
	group.SetLimit(8)
	for i := range inboundBalances {
		b := inboundBalances[i]
		group.Go(func() error {
			senderWallet, err := s.wallets.GetByID(ctx, b.EntityID)
			if err != nil {
				result.registerErr(errors.Wrap(err, "unable to get wallet"))
				return nil
			}

			recipientBalanceKey := balanceKey(b)
			recipientBalance, ok := outboundBalances[recipientBalanceKey]
			if !ok {
				result.registerErr(errors.New("unable to find outbound balance for " + recipientBalanceKey))
				return nil
			}

			recipientWallet, ok := outboundWallets[recipientBalance.EntityID]
			if !ok {
				result.registerErr(errors.New("unable to find outbound wallet for " + b.Currency))
				return nil
			}

			amount, err := determineInternalTransferAmount(b.CurrencyType, b.Amount)
			if err != nil {
				result.registerErr(errors.Wrapf(err, "unable to calculate amount to transfer"))
				return nil
			}

			params := internalTransferInput{
				SenderWallet:    senderWallet,
				SenderBalance:   b,
				RecipientWallet: recipientWallet,
				Amount:          amount,
			}

			output, errTransfer := s.createInternalTransfer(ctx, senderWallet, params)

			if errTransfer != nil {
				s.logger.Error().Err(errTransfer).
					Str("sender_address", senderWallet.Address).
					Int64("sender_wallet_id", senderWallet.ID).
					Int64("sender_balance_id", b.ID).
					Msg("unable to create internal transfer. performing rollback")

				errRollback := s.rollbackInternalTransfer(ctx, params, output, errTransfer)
				result.registerErr(errRollback)

				if errRollback != nil {
					return errors.Wrap(errRollback, "unable to rollback internal transfer")
				}

				s.logger.Info().
					Str("operation", "internalTransfer").
					Int64("sender_wallet_id", senderWallet.ID).
					Int64("sender_balance_id", b.ID).
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

// determineInternalTransferAmount calculates suitable amount of internal transfer.
// We can confidently transfer all ERC-20 tokens, but for coins let's reserve some for gas fees.
func determineInternalTransferAmount(crypto money.CryptoCurrencyType, amount money.Money) (money.Money, error) {
	if crypto == money.Token {
		return amount, nil
	}

	return amount.MultiplyFloat64(transferableCoinPercentage)
}

func (s *Service) BatchCheckInternalTransfers(ctx context.Context, transactionIDs []int64) error {
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
			if err := s.checkInternalTransaction(ctx, txID); err != nil {
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
		Msg("Checked internal transactions")

	return err
}

type internalTransferInput struct {
	SenderWallet    *wallet.Wallet
	SenderBalance   *wallet.Balance
	RecipientWallet *wallet.Wallet
	Amount          money.Money
}

type internalTransferOutput struct {
	Transaction        *transaction.Transaction
	TransactionRaw     string
	TransactionHashID  string
	BalanceDecremented bool
	IsTest             bool
}

func (s *Service) createInternalTransfer(
	ctx context.Context,
	sender *wallet.Wallet,
	params internalTransferInput,
) (internalTransferOutput, error) {
	out := internalTransferOutput{}

	// 0. Get currency & baseCurrency (e.g. ETH and ETH_USDT)
	baseCurrency, err := s.blockchain.GetNativeCoin(sender.Blockchain.ToMoneyBlockchain())
	if err != nil {
		return out, errors.Wrap(err, "unable to get base currency")
	}

	// e.g. ETH / ETH_USDT
	currency, err := s.blockchain.GetCurrencyByTicker(params.Amount.Ticker())
	if err != nil {
		return out, errors.Wrap(err, "unable to get currency")
	}

	isTest := currency.TestNetworkID == params.SenderBalance.NetworkID
	out.IsTest = isTest

	txNetworkFee, err := s.blockchain.CalculateFee(ctx, baseCurrency, currency, isTest)
	if err != nil {
		return out, errors.Wrapf(err, "unable to calculate fee")
	}

	// 1. Create signed transaction via KMS
	txRaw, err := s.wallets.CreateSignedTransaction(
		ctx,
		sender,
		params.RecipientWallet.Address,
		currency,
		params.Amount,
		txNetworkFee,
		isTest,
	)
	if err != nil {
		return out, errors.Wrapf(err, "unable to create raw signed transaction")
	}

	out.TransactionRaw = txRaw

	// 2. Convert amount to USD
	conv, err := s.blockchain.CryptoToFiat(ctx, params.Amount, money.USD)
	if err != nil {
		return out, errors.Wrapf(err, "unable to convert %s to USD", currency.Ticker)
	}

	// 3. Create transaction in the DB
	tx, err := s.transactions.Create(ctx, 0, transaction.CreateTransaction{
		Type:            transaction.TypeInternal,
		RecipientWallet: params.RecipientWallet,
		SenderWallet:    params.SenderWallet,
		Currency:        currency,
		Amount:          params.Amount,
		USDAmount:       conv.To,
		IsTest:          isTest,
	})
	if err != nil {
		return out, errors.Wrap(err, "unable to create database transaction")
	}

	out.Transaction = tx

	// 4. Decrement balance sender's balance
	_, err = s.wallets.UpdateBalanceByID(ctx, params.SenderBalance.ID, wallet.UpdateBalanceByIDQuery{
		Operation: wallet.OperationDecrement,
		Amount:    params.Amount,
		Comment:   "locking balance for internal transaction",
		MetaData: wallet.MetaData{
			wallet.MetaTransactionID:     strconv.Itoa(int(tx.ID)),
			wallet.MetaSenderWalletID:    strconv.Itoa(int(params.SenderWallet.ID)),
			wallet.MetaRecipientWalletID: strconv.Itoa(int(params.RecipientWallet.ID)),
		},
	})

	if err != nil {
		return out, errors.Wrap(err, "unable to decrement sender balance")
	}

	out.BalanceDecremented = true

	// 5. Broadcast tx
	transactionHashID, err := s.blockchain.BroadcastTransaction(ctx, currency.Blockchain, txRaw, isTest)
	if err != nil {
		return out, errors.Wrapf(err, "unable to broadcast transaction to %s", currency.Blockchain)
	}

	out.TransactionHashID = transactionHashID

	if err := s.transactions.UpdateTransactionHash(ctx, 0, tx.ID, transactionHashID); err != nil {
		// todo
		//  well, this shouldn't happen, but tx is already broadcasted
		//  think about possible solutions
		s.logger.Error().Err(err).
			Int64("transaction_id", tx.ID).Str("transaction_hash_id", transactionHashID).
			Msg("unable to update database tx hash id")
	}

	// 5. if currency is TOKEN, then "steal" COIN balance and decrement it.
	// UPD: we can do it when receiving confirmation webhook "transaction processed"
	// because otherwise it's impossible to determine exact tx fees.

	return out, nil
}

func (s *Service) rollbackInternalTransfer(
	ctx context.Context,
	in internalTransferInput,
	out internalTransferOutput,
	errOut error,
) error {
	if out.TransactionRaw != "" {
		if err := s.wallets.DecrementPendingTransaction(ctx, in.SenderWallet.ID, out.IsTest); err != nil {
			return errors.Wrap(err, "unable to decrement pending transaction")
		}
	}

	if out.Transaction != nil {
		msg := fmt.Sprintf("internal transfer rollback. Reason: %s", errOut.Error())
		err := s.transactions.Cancel(ctx, out.Transaction, transaction.StatusCancelled, msg, nil)
		if err != nil {
			return errors.Wrap(err, "unable to cancel transaction")
		}
	}

	if out.BalanceDecremented {
		_, err := s.wallets.UpdateBalanceByID(ctx, in.SenderBalance.ID, wallet.UpdateBalanceByIDQuery{
			Operation: wallet.OperationIncrement,
			Amount:    in.Amount,
			Comment:   "Unlocking balance due to internal transfer rollback",
			MetaData: wallet.MetaData{
				wallet.MetaTransactionID: strconv.Itoa(int(out.Transaction.ID)),
			},
		})

		if err != nil {
			return errors.Wrap(err, "unable to rollback balance")
		}
	}

	if errOut != nil {
		s.logger.Error().Err(errOut).
			Interface("input", in).
			Interface("out", out).
			Msg("error occurred while creating internal transfer")
	}

	return nil
}

func (s *Service) checkInternalTransaction(ctx context.Context, txID int64) error {
	tx, err := s.transactions.GetByID(ctx, transaction.SystemMerchantID, txID)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction")
	}

	switch {
	case tx.Type != transaction.TypeInternal:
		return errors.New("invalid transaction type")
	case tx.HashID == nil:
		return errors.New("empty transaction hash")
	case tx.SenderWalletID == nil:
		return errors.New("empty sender wallet id")
	case tx.RecipientWalletID == nil:
		return errors.New("empty recipient wallet id")
	}

	receipt, err := s.blockchain.GetTransactionReceipt(ctx, tx.Currency.Blockchain, *tx.HashID, tx.IsTest)
	if err != nil {
		return errors.Wrap(err, "unable to get transaction receipt")
	}

	if !receipt.IsConfirmed {
		s.logger.Info().
			Int64("transaction_id", tx.ID).
			Bool("is_test", tx.IsTest).
			Str("transaction_hash", *tx.HashID).Msg("internal transaction is not confirmed yet")

		return nil
	}

	if !receipt.Success {
		if err := s.cancelInternalTransfer(ctx, tx, receipt); err != nil {
			return errors.Wrap(err, "unable to cancel internal transfer")
		}

		return nil
	}

	if err := s.confirmInternalTransfer(ctx, tx, receipt); err != nil {
		return errors.Wrap(err, "unable to confirm internal transfer")
	}

	return nil
}

func (s *Service) confirmInternalTransfer(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Info().Int64("transaction_id", tx.ID).Msg("confirming internal transfer")

	var (
		senderWalletID    = *tx.SenderWalletID
		recipientWalletID = *tx.RecipientWalletID
		txHashID          = *tx.HashID
	)

	senderWallet, err := s.wallets.GetByID(ctx, senderWalletID)
	if err != nil {
		return errors.Wrap(err, "unable to get sender wallet id")
	}

	// 1. Confirm wallet's nonce
	if err = s.wallets.IncrementConfirmedTransaction(ctx, senderWallet.ID, tx.IsTest); err != nil {
		return errors.Wrap(err, "unable to confirm wallet's nonce")
	}

	// 2. Confirm transaction
	confirmation := transaction.ConfirmTransaction{
		Status:          transaction.StatusCompleted,
		SenderAddress:   senderWallet.Address,
		TransactionHash: txHashID,
		FactAmount:      tx.Amount,
		NetworkFee:      receipt.NetworkFee,
		MetaData:        tx.MetaData,
	}

	// some blockchains (e.g. tron) may have zero network fees on certain txs
	confirmation.AllowZeroNetworkFee()

	tx, err = s.transactions.Confirm(ctx, transaction.SystemMerchantID, tx.ID, confirmation)
	if err != nil {
		return errors.Wrap(err, "unable to update transaction")
	}

	s.logger.Info().
		Int64("transaction_id", tx.ID).
		Int64("sender_waller_id", senderWalletID).
		Int64("recipient_waller_id", recipientWalletID).
		Msg("processed internal transaction")

	return nil
}

func (s *Service) cancelInternalTransfer(
	ctx context.Context,
	tx *transaction.Transaction,
	receipt *blockchain.TransactionReceipt,
) error {
	s.logger.Error().
		Int64("transaction_id", tx.ID).
		Str("blockchain", receipt.Blockchain.String()).
		Str("network_id", tx.NetworkID()).
		Str("transaction_hash", receipt.Hash).
		Msg("canceling internal transfer")

	// 1. Confirm nonce
	if err := s.wallets.IncrementConfirmedTransaction(ctx, *tx.SenderWalletID, tx.IsTest); err != nil {
		return errors.Wrap(err, "unable to confirm nonce")
	}

	// 2. Mark tx as failed
	err := s.transactions.Cancel(ctx, tx, transaction.StatusFailed, revertReason, &receipt.NetworkFee)
	if err != nil {
		return errors.Wrap(err, "unable to cancel transaction")
	}

	// 3. Restore sender balance to previous state
	ticker := tx.Currency.Ticker
	networkID := tx.NetworkID()

	senderBalance, err := s.wallets.GetWalletsBalance(ctx, *tx.SenderWalletID, ticker, networkID)
	if err != nil {
		return errors.Wrap(err, "unable to get sender wallet balance")
	}

	_, err = s.wallets.UpdateBalanceByID(ctx, senderBalance.ID, wallet.UpdateBalanceByIDQuery{
		Operation: wallet.OperationIncrement,
		Amount:    tx.Amount,
		Comment:   "transaction was reverted by blockchain",
		MetaData: wallet.MetaData{
			wallet.MetaTransactionID: strconv.Itoa(int(tx.ID)),
		},
	})
	if err != nil {
		return errors.Wrap(err, "unable to increment sender's wallet balance")
	}

	return nil
}

// getOutboundBalancesAsMap returns map of outbound balances as {ticker:balance}
func (s *Service) getOutboundWalletsWithBalancesAsMap(ctx context.Context) (
	map[int64]*wallet.Wallet,
	map[string]*wallet.Balance,
	error,
) {
	wallets, _, err := s.wallets.List(ctx, wallet.Pagination{
		Limit:        100,
		FilterByType: wallet.TypeOutbound,
	})

	walletsMap := util.KeyFunc(wallets, func(w *wallet.Wallet) int64 { return w.ID })

	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to list wallets")
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(3)

	results := make(map[string]*wallet.Balance)
	mu := sync.Mutex{}

	for i := range wallets {
		w := wallets[i]
		group.Go(func() error {
			balances, err := s.wallets.ListBalances(ctx, wallet.EntityTypeWallet, w.ID, false)
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()

			for _, b := range balances {
				results[balanceKey(b)] = b
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, nil, err
	}

	return walletsMap, results, nil
}

func (s *Service) validateInboundBalances(ctx context.Context, balances []*wallet.Balance) error {
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(8)

	idMapper := func(b *wallet.Balance) int64 { return b.ID }
	if len(lo.UniqBy(balances, idMapper)) != len(balances) {
		return errors.New("balances contain duplicates")
	}

	for i := range balances {
		b := balances[i]

		if b.EntityType != wallet.EntityTypeWallet {
			return errors.Wrap(ErrInboundWallet, "balance is not related to oxygen wallet")
		}

		group.Go(func() error {
			if b.Amount.IsZero() {
				return errors.New("balance is empty")
			}

			w, err := s.wallets.GetByID(ctx, b.EntityID)
			if err != nil {
				return errors.Wrap(err, "unable to get wallet by id")
			}

			if w.Type != wallet.TypeInbound {
				return errors.New("wallet is not inbound")
			}

			minAmount, err := s.blockchain.GetUSDMinimalInternalTransferByTicker(b.Currency)
			if err != nil {
				return errors.Wrapf(err, "unable to get minimal internal transfer for %q", b.Currency)
			}

			currency, err := s.blockchain.GetCurrencyByTicker(b.Currency)
			if err != nil {
				return errors.Wrapf(err, "unable to get currency by ticker")
			}

			conv, err := s.blockchain.FiatToCrypto(ctx, minAmount, currency)
			if err != nil {
				return errors.Wrapf(err, "unable to convert %s to %s", minAmount.Ticker(), b.Currency)
			}

			if b.Amount.LessThan(conv.To) {
				return fmt.Errorf("insufficient amount for %q", b.Amount.Ticker())
			}

			return nil
		})
	}

	return group.Wait()
}

func balanceKey(b *wallet.Balance) string {
	return fmt.Sprintf("%s/%s/%s", b.EntityType, b.NetworkID, b.Currency)
}
