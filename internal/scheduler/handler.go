package scheduler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Handler scheduler handler. Be aware that each ctx has zerolog.Logger instance!
type Handler struct {
	payments     *payment.Service
	blockchains  *blockchain.Service
	wallets      *wallet.Service
	processing   ProcessingService
	transactions *transaction.Service
	tableLogger  *log.JobLogger
}

type ContextJobID struct{}
type ContextJobEnableTableLogger struct{}

type ProcessingService interface {
	BatchCheckIncomingTransactions(ctx context.Context, transactionIDs []int64) error
	BatchCreateInternalTransfers(ctx context.Context, balances []*wallet.Balance) (*processing.TransferResult, error)
	BatchCheckInternalTransfers(ctx context.Context, transactionIDs []int64) error
	BatchCreateWithdrawals(ctx context.Context, paymentsIDs []int64) (*processing.TransferResult, error)
	BatchCheckWithdrawals(ctx context.Context, transactionIDs []int64) error
	EnsureOutboundWallet(ctx context.Context, chain money.Blockchain) (*wallet.Wallet, bool, error)
	BatchExpirePayments(ctx context.Context, paymentsIDs []int64) error
}

func New(
	payments *payment.Service,
	blockchains *blockchain.Service,
	wallets *wallet.Service,
	processingService ProcessingService,
	transactions *transaction.Service,
	jobLogger *log.JobLogger,
) *Handler {
	return &Handler{
		payments:     payments,
		wallets:      wallets,
		blockchains:  blockchains,
		processing:   processingService,
		transactions: transactions,
		tableLogger:  jobLogger,
	}
}

func (h *Handler) JobLogger() *log.JobLogger {
	return h.tableLogger
}

func (h *Handler) CheckIncomingTransactionsProgress(ctx context.Context) error {
	// it will be definitely enough for first months of usage.
	const limit = 200

	filter := transaction.Filter{
		Types:    []transaction.Type{transaction.TypeIncoming},
		Statuses: []transaction.Status{transaction.StatusInProgress, transaction.StatusInProgressInvalid},
	}

	txs, err := h.transactions.ListByFilter(ctx, filter, limit)
	if err != nil {
		return errors.Wrap(err, "unable to list incoming transactions")
	}

	ids := util.MapSlice(txs, func(t *transaction.Transaction) int64 { return t.ID })

	if err := h.processing.BatchCheckIncomingTransactions(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch check incoming transactions")
	}

	return nil
}

// PerformInternalWalletTransfer performs money transfer from INBOUND wallets to OUTBOUND ones
// so later customers can withdraw their assets.
func (h *Handler) PerformInternalWalletTransfer(ctx context.Context) error {
	jobID := ctx.Value(ContextJobID{}).(string)
	logger := zerolog.Ctx(ctx)

	// 1. Ensure outbound wallets exist in DB
	if err := h.EnsureOutboundWallets(ctx); err != nil {
		return errors.Wrap(err, "unable to ensure outbound wallets")
	}

	// 2. Get all inbound wallets
	var (
		start          int64
		inboundWallets []*wallet.Wallet
	)

	for {
		wallets, nextID, err := h.wallets.List(ctx, wallet.Pagination{
			Start:        start,
			Limit:        300,
			FilterByType: wallet.TypeInbound,
		})

		if err != nil {
			return errors.Wrap(err, "unable to list inbound wallets")
		}

		inboundWallets = append(inboundWallets, wallets...)

		if nextID != nil {
			start = *nextID
			continue
		}

		break
	}

	logger.Info().Int("wallets_count", len(inboundWallets)).Msg("fetched inbound wallets")
	h.tableLogger.Log(ctx, log.Info, jobID, "fetched inbound wallets", map[string]any{
		"inboundWalletsCount": strconv.Itoa(len(inboundWallets)),
	})

	// 3. Get INBOUND wallets balances and match only those that have minimum required funds.
	var matchedBalances []*wallet.Balance

	for _, inboundWallet := range inboundWallets {
		walletBalances, err := h.wallets.ListBalances(ctx, wallet.EntityTypeWallet, inboundWallet.ID, false)
		if err != nil {
			return errors.Wrap(err, "unable to list balances")
		}

		for _, b := range walletBalances {
			if b.Amount.IsZero() {
				continue
			}

			minAmountUSD, err := h.blockchains.GetUSDMinimalInternalTransferByTicker(b.Currency)
			if err != nil {
				return errors.Wrapf(err, "unable to get minimal internal transfer for %q", b.Currency)
			}

			currency, err := h.blockchains.GetCurrencyByTicker(b.Currency)
			if err != nil {
				return errors.Wrapf(err, "unable to get currency by ticker")
			}

			conv, err := h.blockchains.FiatToCrypto(ctx, minAmountUSD, currency)
			if err != nil {
				return errors.Wrapf(err, "unable to convert %s to %s", minAmountUSD.Ticker(), b.Currency)
			}

			if b.Amount.GreaterThanOrEqual(conv.To) {
				matchedBalances = append(matchedBalances, b)
			}
		}
	}

	h.tableLogger.Log(ctx, log.Info, jobID, "gathered matching outbound balances", map[string]any{
		"matchedInboundBalancesCount": len(matchedBalances),
		"matchedInboundBalances": util.MapSlice(matchedBalances, func(b *wallet.Balance) string {
			return fmt.Sprintf("balance#%d with %s amount of %s", b.ID, b.Currency, b.Amount.String())
		}),
	})

	result, err := h.processing.BatchCreateInternalTransfers(ctx, matchedBalances)
	if err != nil {
		return errors.Wrap(err, "unable to transfer money from inbound to outbound wallets")
	}

	h.tableLogger.Log(ctx, log.Info, jobID, "created internal transactions", map[string]any{
		"transactionIDs": util.MapSlice(result.CreatedTransactions, func(tx *transaction.Transaction) int64 { return tx.ID }),
		"transactionsList": util.MapSlice(result.CreatedTransactions, func(tx *transaction.Transaction) string {
			return fmt.Sprintf("tx#%d: send %s of %s to %s",
				tx.ID,
				tx.Amount.String(),
				tx.Amount.Ticker(),
				tx.RecipientAddress,
			)
		}),
		"rollbackedTransactionIDs": result.RollbackedTransactionIDs,
		"totalErrors":              result.TotalErrors,
		"errorMessages":            util.MapSlice(result.UnhandledErrors, func(e error) string { return e.Error() }),
	})

	return nil
}

func (h *Handler) CheckInternalTransferProgress(ctx context.Context) error {
	// it will be definitely enough for first months of usage.
	const limit = 200

	filter := transaction.Filter{
		Types:    []transaction.Type{transaction.TypeInternal},
		Statuses: []transaction.Status{transaction.StatusPending, transaction.StatusInProgress},
	}

	txs, err := h.transactions.ListByFilter(ctx, filter, limit)
	if err != nil {
		return errors.Wrap(err, "unable to list internal transactions")
	}

	ids := util.MapSlice(txs, func(t *transaction.Transaction) int64 { return t.ID })

	if err := h.processing.BatchCheckInternalTransfers(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch check internal transfers")
	}

	return nil
}

// PerformWithdrawalsCreation searches for pending payments with type = withdrawal
// and creates transactions.
func (h *Handler) PerformWithdrawalsCreation(ctx context.Context) error {
	jobID := ctx.Value(ContextJobID{}).(string)
	logger := zerolog.Ctx(ctx)

	// 1. Ensure outbound wallets exist in DB
	if err := h.EnsureOutboundWallets(ctx); err != nil {
		return errors.Wrap(err, "unable to ensure outbound wallets")
	}

	// 2. List pending withdrawals
	withdrawals, err := h.payments.ListWithdrawals(ctx, payment.StatusPending, nil)
	if err != nil {
		return errors.Wrap(err, "unable to list pending withdrawals")
	}

	logger.Info().Int("withdrawals_count", len(withdrawals)).Msg("fetched pending withdrawals")
	h.tableLogger.Log(ctx, log.Info, jobID, "fetched inbound wallets", map[string]any{
		"pendingWithdrawalsCount": strconv.Itoa(len(withdrawals)),
	})

	ids := util.MapSlice(withdrawals, func(p *payment.Payment) int64 { return p.ID })

	result, err := h.processing.BatchCreateWithdrawals(ctx, ids)
	if err != nil {
		return errors.Wrap(err, "unable to create withdrawals")
	}

	h.tableLogger.Log(ctx, log.Info, jobID, "created withdrawal transactions", map[string]any{
		"transactionIDs": util.MapSlice(result.CreatedTransactions, func(tx *transaction.Transaction) int64 { return tx.ID }),
		"transactionsList": util.MapSlice(result.CreatedTransactions, func(tx *transaction.Transaction) string {
			return fmt.Sprintf("tx#%d: send %s of %s to %s",
				tx.ID,
				tx.Amount.String(),
				tx.Amount.Ticker(),
				tx.RecipientAddress,
			)
		}),
		"rollbackedTransactionIDs": result.RollbackedTransactionIDs,
		"totalErrors":              result.TotalErrors,
		"errorMessages":            util.MapSlice(result.UnhandledErrors, func(e error) string { return e.Error() }),
	})

	return nil
}

func (h *Handler) EnsureOutboundWallets(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(4)

	for _, bc := range h.blockchains.ListSupportedBlockchains() {
		bc := bc
		group.Go(func() error {
			w, justCreated, err := h.processing.EnsureOutboundWallet(ctx, bc)
			if err != nil {
				return errors.Wrapf(err, "unable to ensure outbound wallet for %s", bc)
			}

			logger.Info().
				Str("blockchain", string(bc)).
				Int64("wallet_id", w.ID).
				Bool("just_created", justCreated).
				Msg("Ensured outbound wallet")

			return nil
		})
	}

	return group.Wait()
}

func (h *Handler) CheckWithdrawalsProgress(ctx context.Context) error {
	// it will be definitely enough for first months of usage.
	const limit = 200

	filter := transaction.Filter{
		Types:    []transaction.Type{transaction.TypeWithdrawal},
		Statuses: []transaction.Status{transaction.StatusPending, transaction.StatusInProgress},
	}

	txs, err := h.transactions.ListByFilter(ctx, filter, limit)
	if err != nil {
		return errors.Wrap(err, "unable to list withdrawal transactions")
	}

	ids := util.MapSlice(txs, func(t *transaction.Transaction) int64 { return t.ID })

	if err := h.processing.BatchCheckWithdrawals(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch check withdrawals")
	}

	return nil
}

func (h *Handler) CancelExpiredPayments(ctx context.Context) error {
	// it will be definitely enough for first months of usage.
	const limit = 200

	payments, err := h.payments.GetBatchExpired(ctx, limit)
	if err != nil {
		return errors.Wrap(err, "unable to get batch expired payments")
	}

	ids := util.MapSlice(payments, func(pt *payment.Payment) int64 { return pt.ID })

	if err := h.processing.BatchExpirePayments(ctx, ids); err != nil {
		return errors.Wrap(err, "unable to batch expire payments")
	}

	return nil
}
