package wallet

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/pkg/errors"
)

var (
	ErrTxConfirm  = errors.New("nothing to confirm")
	ErrTxRollback = errors.New("nothing to rollback")
)

// IncrementPendingTransaction updates Wallet's nonce parameter and returns nonce for next tx.
func (s *Service) IncrementPendingTransaction(ctx context.Context, walletID int64, isTest bool) (int, error) {
	var nonce int

	err := s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		w, err := q.GetWalletForUpdateByID(ctx, walletID)
		if err != nil {
			return errors.Wrap(err, "unable to get wallet for update")
		}

		if isTest {
			nonce = int(w.ConfirmedTestnetTransactions + w.PendingTestnetTransactions)

			return q.UpdateWalletTestnetTransactionCounters(ctx, repository.UpdateWalletTestnetTransactionCountersParams{
				ID:                           walletID,
				ConfirmedTestnetTransactions: w.ConfirmedTestnetTransactions,
				PendingTestnetTransactions:   w.PendingTestnetTransactions + 1,
			})
		}

		nonce = int(w.ConfirmedMainnetTransactions + w.PendingMainnetTransactions)

		return q.UpdateWalletMainnetTransactionCounters(ctx, repository.UpdateWalletMainnetTransactionCountersParams{
			ID:                           walletID,
			ConfirmedMainnetTransactions: w.ConfirmedMainnetTransactions,
			PendingMainnetTransactions:   w.PendingMainnetTransactions + 1,
		})
	})

	return nonce, err
}

func (s *Service) IncrementConfirmedTransaction(ctx context.Context, walletID int64, isTest bool) error {
	return s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		w, err := q.GetWalletForUpdateByID(ctx, walletID)
		if err != nil {
			return errors.Wrap(err, "unable to get wallet for update")
		}

		if isTest {
			if w.PendingTestnetTransactions == 0 {
				return ErrTxConfirm
			}

			return q.UpdateWalletTestnetTransactionCounters(ctx, repository.UpdateWalletTestnetTransactionCountersParams{
				ID:                           walletID,
				ConfirmedTestnetTransactions: w.ConfirmedTestnetTransactions + 1,
				PendingTestnetTransactions:   w.PendingTestnetTransactions - 1,
			})
		}

		if w.PendingMainnetTransactions == 0 {
			return ErrTxConfirm
		}

		return q.UpdateWalletMainnetTransactionCounters(ctx, repository.UpdateWalletMainnetTransactionCountersParams{
			ID:                           walletID,
			ConfirmedMainnetTransactions: w.ConfirmedMainnetTransactions + 1,
			PendingMainnetTransactions:   w.PendingMainnetTransactions - 1,
		})
	})
}

func (s *Service) DecrementPendingTransaction(ctx context.Context, walletID int64, isTest bool) error {
	return s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		w, err := q.GetWalletForUpdateByID(ctx, walletID)
		if err != nil {
			return errors.Wrap(err, "unable to get wallet for update")
		}

		if isTest {
			if w.PendingTestnetTransactions == 0 {
				return ErrTxRollback
			}

			return q.UpdateWalletTestnetTransactionCounters(ctx, repository.UpdateWalletTestnetTransactionCountersParams{
				ID:                           walletID,
				ConfirmedTestnetTransactions: w.ConfirmedTestnetTransactions,
				PendingTestnetTransactions:   w.PendingTestnetTransactions - 1,
			})
		}

		if w.PendingMainnetTransactions == 0 {
			return ErrTxRollback
		}

		return q.UpdateWalletMainnetTransactionCounters(ctx, repository.UpdateWalletMainnetTransactionCountersParams{
			ID:                           walletID,
			ConfirmedMainnetTransactions: w.ConfirmedMainnetTransactions,
			PendingMainnetTransactions:   w.PendingMainnetTransactions - 1,
		})
	})
}
