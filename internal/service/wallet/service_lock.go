package wallet

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

// AcquireLock finds and locks wallet of specified type currency for selected merchantID.
// If required wallet isn't found, then the new one is being created. This method uses transactions.
func (s *Service) AcquireLock(ctx context.Context, merchantID int64, currency money.CryptoCurrency, isTest bool) (*Wallet, error) {
	var acquiredWallet *Wallet

	blockchainNetwork := currency.ChooseNetwork(isTest)

	// case 1: get + lock
	err := s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		entry, err := q.GetAvailableWallet(ctx, repository.GetAvailableWalletParams{
			Blockchain: currency.Blockchain.String(),
			Currency:   currency.Ticker,
			NetworkID:  blockchainNetwork,
			Type:       repository.StringToNullable(string(TypeInbound)),
		})

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrNotFound
		case err != nil:
			return errors.Wrap(err, "unable to GetAvailableWalletByBlockchain")
		}

		acquiredWallet = entryToWallet(entry)

		lock := lockParams{
			merchantID: merchantID,
			wallet:     acquiredWallet,
			ticker:     currency.Ticker,
			networkID:  blockchainNetwork,
			isTest:     isTest,
		}

		if err := s.lockWallet(ctx, q, lock); err != nil {
			return errors.Wrap(err, "unable to lock wallet")
		}

		return nil
	})

	// wallet found & locked
	if err == nil {
		return acquiredWallet, nil
	}

	// error occurred
	if !errors.Is(err, ErrNotFound) {
		return nil, errors.Wrap(err, "unable to find & acquire wallet")
	}

	// case 2: create + lock
	err = s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		bc := kmswallet.Blockchain(currency.Blockchain.String())
		acquiredWallet, err = s.create(ctx, q, bc, TypeInbound)
		if err != nil {
			return errors.Wrap(err, "unable to create wallet")
		}

		lock := lockParams{
			merchantID: merchantID,
			wallet:     acquiredWallet,
			ticker:     currency.Ticker,
			networkID:  blockchainNetwork,
			isTest:     isTest,
		}

		if errLock := s.lockWallet(ctx, q, lock); errLock != nil {
			return errors.Wrap(err, "unable to lock wallet")
		}

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to create & lock the wallet")
	}

	return acquiredWallet, nil
}

type lockParams struct {
	merchantID int64
	wallet     *Wallet
	ticker     string
	networkID  string
	isTest     bool
}

func (s *Service) lockWallet(ctx context.Context, q repository.Querier, params lockParams) error {
	_, errLock := q.CreateWalletLock(ctx, repository.CreateWalletLockParams{
		WalletID:   params.wallet.ID,
		MerchantID: params.merchantID,
		Currency:   params.ticker,
		NetworkID:  params.networkID,
		LockedAt:   time.Now(),
		// in the future we might want to create locks for definite period of time.
		LockedUntil: sql.NullTime{},
	})

	if errLock != nil {
		return errors.Wrap(errLock, "unable to CreateWalletLock")
	}

	return nil
}

// ReleaseLock does the opposite of AcquireLock.
func (s *Service) ReleaseLock(ctx context.Context, walletID int64, currency, networkID string) error {
	return s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		return ReleaseLock(ctx, q, walletID, currency, networkID)
	})
}

func ReleaseLock(ctx context.Context, q repository.Querier, walletID int64, currency, networkID string) error {
	params := repository.GetWalletLockParams{
		WalletID:  walletID,
		Currency:  currency,
		NetworkID: networkID,
	}

	lock, err := q.GetWalletLock(ctx, params)
	if err != nil {
		return errors.Wrap(err, "unable to get wallet lock")
	}

	return q.ReleaseWalletLock(ctx, lock.ID)
}
