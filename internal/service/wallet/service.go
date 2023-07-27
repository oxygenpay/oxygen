package wallet

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	kmsclient "github.com/oxygenpay/oxygen/pkg/api-kms/v1/client/wallet"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	ErrNotFound                     = errors.New("wallet not found")
	ErrBalanceNotFound              = errors.New("balance not found")
	ErrInvalidBlockchain            = errors.New("invalid blockchain provided")
	ErrInvalidType                  = errors.New("invalid type provided")
	ErrInsufficientBalance          = errors.New("insufficient balance")
	ErrInsufficienceMerchantBalance = errors.Wrap(ErrInsufficientBalance, "merchant")
)

const (
	TypeInbound  Type = "inbound"
	TypeOutbound Type = "outbound"
)

type BlockchainService interface {
	blockchain.Convertor
}

type Service struct {
	kms        kmsclient.ClientService
	blockchain BlockchainService
	store      repository.Storage
	logger     *zerolog.Logger
}

type Wallet struct {
	ID                           int64
	CreatedAt                    time.Time
	UUID                         uuid.UUID
	Address                      string
	Blockchain                   kmswallet.Blockchain
	Type                         Type
	TatumSubscription            TatumSubscription
	ConfirmedMainnetTransactions int64
	PendingMainnetTransactions   int64
	ConfirmedTestnetTransactions int64
	PendingTestnetTransactions   int64
}

type Type string

type Pagination struct {
	Start              int64
	Limit              int32
	FilterByBlockchain kmswallet.Blockchain
	FilterByType       Type
}

type TatumSubscription struct {
	MainnetSubscriptionID string
	TestnetSubscriptionID string
}

func New(
	kmsClient kmsclient.ClientService,
	blockchainService BlockchainService,
	store *repository.Store,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "wallet_service").Logger()

	return &Service{
		kms:        kmsClient,
		blockchain: blockchainService,
		store:      store,
		logger:     &log,
	}
}

func (s *Service) Create(ctx context.Context, bc kmswallet.Blockchain, walletType Type) (*Wallet, error) {
	if !bc.IsValid() {
		return nil, ErrInvalidBlockchain
	}

	if walletType != TypeOutbound && walletType != TypeInbound {
		return nil, ErrInvalidType
	}

	return s.create(ctx, s.store, bc, walletType)
}

// create can be fired both in transactional and non-transactional ways.
func (s *Service) create(
	ctx context.Context,
	q repository.Querier,
	bc kmswallet.Blockchain,
	walletType Type,
) (*Wallet, error) {
	// 1. Create wallet in KMS
	res, err := s.kms.CreateWallet(&kmsclient.CreateWalletParams{
		Context: ctx,
		Data: &kmsmodel.CreateWalletRequest{
			Blockchain: kmsmodel.Blockchain(bc),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "kmsClient.Wallet.StoreWallet error")
	}

	// 2. Create wallet in DB
	entry, err := q.CreateWallet(ctx, repository.CreateWalletParams{
		CreatedAt:  time.Now(),
		Uuid:       uuid.MustParse(res.Payload.ID),
		Address:    res.Payload.Address,
		Blockchain: string(res.Payload.Blockchain),
		Type:       repository.StringToNullable(string(walletType)),
	})

	if err != nil {
		return nil, errors.Wrap(err, "repo.StoreWallet error")
	}

	return entryToWallet(entry), nil
}

func (s *Service) BulkCreateWallets(ctx context.Context, bc kmswallet.Blockchain, amount int64) ([]*Wallet, error) {
	wallets := make([]*Wallet, amount)

	for i := range wallets {
		w, err := s.Create(ctx, bc, TypeInbound)
		if err != nil {
			return nil, errors.Wrapf(err, "error at wallet %d", i)
		}

		wallets[i] = w
	}

	return wallets, nil
}

// EnsureOutboundWallet finds or creates outbound wallet for specified blockchain.
// Outbound wallets are used for funds withdrawal.
func (s *Service) EnsureOutboundWallet(ctx context.Context, bc kmswallet.Blockchain) (*Wallet, bool, error) {
	params := repository.PaginateWalletsByIDParams{
		Blockchain:         bc.String(),
		Type:               repository.StringToNullable(string(TypeOutbound)),
		FilterByType:       true,
		FilterByBlockchain: true,
		Limit:              1,
	}

	wallets, err := s.store.PaginateWalletsByID(ctx, params)

	if err != nil {
		return nil, false, errors.Wrap(err, "unable to list wallets")
	}

	if len(wallets) == 1 {
		return entryToWallet(wallets[0]), false, nil
	}

	wallet, err := s.Create(ctx, bc, TypeOutbound)
	if err != nil {
		return nil, false, errors.Wrap(err, "unable to create wallet")
	}

	return wallet, true, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*Wallet, error) {
	w, err := s.store.GetWalletByID(ctx, id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToWallet(w), nil
}

func (s *Service) GetByUUID(ctx context.Context, id uuid.UUID) (*Wallet, error) {
	w, err := s.store.GetWalletByUUID(ctx, id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToWallet(w), nil
}

func (s *Service) List(ctx context.Context, pagination Pagination) ([]*Wallet, *int64, error) {
	results, err := s.store.PaginateWalletsByID(ctx, repository.PaginateWalletsByIDParams{
		ID:                 pagination.Start,
		Limit:              pagination.Limit,
		FilterByBlockchain: pagination.FilterByBlockchain != "",
		Blockchain:         string(pagination.FilterByBlockchain),
		FilterByType:       pagination.FilterByType != "",
		Type:               repository.StringToNullable(string(pagination.FilterByType)),
	})

	if err != nil || len(results) == 0 {
		return nil, nil, err
	}

	wallets := make([]*Wallet, len(results))
	for i := range results {
		wallets[i] = entryToWallet(results[i])
	}

	// request next pagination info
	lastID := wallets[len(wallets)-1].ID
	nextPageResults, err := s.store.PaginateWalletsByID(ctx, repository.PaginateWalletsByIDParams{
		ID:                 lastID + 1,
		Limit:              1,
		FilterByBlockchain: pagination.FilterByBlockchain != "",
		Blockchain:         string(pagination.FilterByBlockchain),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return wallets, nil, nil
	case err != nil:
		return nil, nil, err
	}

	if len(nextPageResults) == 0 {
		return wallets, nil, nil
	}

	nextPageFirstResultsID := nextPageResults[0].ID

	return wallets, &nextPageFirstResultsID, nil
}

// UpdateTatumSubscription updates tatum_* fields and reflects changes in *Wallet argument.
func (s *Service) UpdateTatumSubscription(ctx context.Context, wallet *Wallet, subscription TatumSubscription) error {
	entry, err := s.store.UpdateWalletTatumFields(ctx, repository.UpdateWalletTatumFieldsParams{
		TatumMainnetSubscriptionID: repository.StringToNullable(subscription.MainnetSubscriptionID),
		TatumTestnetSubscriptionID: repository.StringToNullable(subscription.TestnetSubscriptionID),
		ID:                         wallet.ID,
	})

	if err != nil {
		return err
	}

	*wallet = *entryToWallet(entry)

	return nil
}

func entryToWallet(entry repository.Wallet) *Wallet {
	return &Wallet{
		ID:         entry.ID,
		CreatedAt:  entry.CreatedAt,
		Type:       Type(entry.Type.String),
		UUID:       entry.Uuid,
		Address:    entry.Address,
		Blockchain: kmswallet.Blockchain(entry.Blockchain),
		TatumSubscription: TatumSubscription{
			MainnetSubscriptionID: entry.TatumMainnetSubscriptionID.String,
			TestnetSubscriptionID: entry.TatumTestnetSubscriptionID.String,
		},
		ConfirmedMainnetTransactions: entry.ConfirmedMainnetTransactions,
		PendingMainnetTransactions:   entry.PendingMainnetTransactions,
		ConfirmedTestnetTransactions: entry.ConfirmedTestnetTransactions,
		PendingTestnetTransactions:   entry.PendingTestnetTransactions,
	}
}
