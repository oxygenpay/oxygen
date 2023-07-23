package merchant

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/pkg/errors"
)

func (s *Service) ListMerchantAddresses(ctx context.Context, merchantID int64) ([]*Address, error) {
	entries, err := s.repo.ListMerchantAddresses(ctx, merchantID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list merchant addresses")
	}

	results := make([]*Address, len(entries))
	for i := range entries {
		result, err := s.entryToAddress(entries[i])
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert entry to merchant address")
		}
		results[i] = result
	}

	return results, nil
}

type CreateMerchantAddressParams struct {
	Name       string
	Blockchain wallet.Blockchain
	Address    string
}

// GetMerchantAddressByUUID returns saved merchant address for withdrawals.
func (s *Service) GetMerchantAddressByUUID(ctx context.Context, merchantID int64, id uuid.UUID) (*Address, error) {
	entry, err := s.repo.GetMerchantAddressByUUID(ctx, repository.GetMerchantAddressByUUIDParams{
		MerchantID: merchantID,
		Uuid:       id,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrAddressNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToAddress(entry)
}

func (s *Service) GetMerchantAddressByID(ctx context.Context, merchantID, id int64) (*Address, error) {
	entry, err := s.repo.GetMerchantAddressByID(ctx, repository.GetMerchantAddressByIDParams{
		MerchantID: merchantID,
		ID:         id,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrAddressNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToAddress(entry)
}

func (s *Service) CreateMerchantAddress(ctx context.Context, merchantID int64, params CreateMerchantAddressParams) (*Address, error) {
	if err := wallet.ValidateAddress(params.Blockchain, params.Address); err != nil {
		return nil, err
	}

	existingAddress, _ := s.repo.GetMerchantAddressByAddress(ctx, repository.GetMerchantAddressByAddressParams{
		MerchantID: merchantID,
		Blockchain: string(params.Blockchain),
		Address:    params.Address,
	})
	if existingAddress.ID != 0 {
		return nil, ErrAddressAlreadyExists
	}

	systemWallet, _ := s.repo.CheckSystemWalletExistsByAddress(ctx, params.Address)
	if systemWallet.ID != 0 {
		return nil, ErrAddressReserved
	}

	entry, err := s.repo.CreateMerchantAddress(ctx, repository.CreateMerchantAddressParams{
		Uuid:       uuid.New(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MerchantID: merchantID,
		Name:       params.Name,
		Blockchain: string(params.Blockchain),
		Address:    params.Address,
	})

	if err != nil {
		return nil, err
	}

	return s.entryToAddress(entry)
}

func (s *Service) UpdateMerchantAddress(ctx context.Context, merchantID int64, id uuid.UUID, name string) (*Address, error) {
	address, err := s.GetMerchantAddressByUUID(ctx, merchantID, id)
	if err != nil {
		return nil, err
	}

	entry, err := s.repo.UpdateMerchantAddress(ctx, repository.UpdateMerchantAddressParams{
		MerchantID: merchantID,
		ID:         address.ID,
		Name:       name,
		UpdatedAt:  time.Now(),
	})

	if err != nil {
		return nil, err
	}

	return s.entryToAddress(entry)
}

func (s *Service) DeleteMerchantAddress(ctx context.Context, merchantID int64, id uuid.UUID) error {
	address, err := s.GetMerchantAddressByUUID(ctx, merchantID, id)
	if err != nil {
		return err
	}

	return s.repo.DeleteMerchantAddress(ctx, repository.DeleteMerchantAddressParams{
		MerchantID: merchantID,
		ID:         address.ID,
	})
}

func (s *Service) entryToAddress(entry repository.MerchantAddress) (*Address, error) {
	blockchain := wallet.Blockchain(entry.Blockchain)
	coin, _ := s.blockchain.GetNativeCoin(blockchain.ToMoneyBlockchain())

	return &Address{
		ID:             entry.ID,
		UUID:           entry.Uuid,
		CreatedAt:      entry.CreatedAt,
		UpdatedAt:      entry.UpdatedAt,
		Name:           entry.Name,
		MerchantID:     entry.MerchantID,
		Blockchain:     blockchain,
		BlockchainName: coin.BlockchainName,
		Address:        entry.Address,
	}, nil
}
