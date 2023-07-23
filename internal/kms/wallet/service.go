package wallet

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	repo      *Repository
	generator *Generator
	logger    *zerolog.Logger
}

type CreateTransactionParams struct {
	Type            AssetType
	Recipient       string
	AmountRaw       string
	NetworkID       string
	ContractAddress *string
}

var (
	ErrInvalidAddress         = errors.New("invalid address")
	ErrInvalidContractAddress = errors.New("invalid contract address")
	ErrInvalidAmount          = errors.New("invalid amount")
	ErrInvalidNetwork         = errors.New("invalid network")
	ErrInvalidGasSettings     = errors.New("invalid network gas settings")
	ErrInvalidNonce           = errors.New("invalid nonce")
	ErrTronResponse           = errors.New("invalid response from TRON node")
	ErrInsufficientBalance    = errors.New("sender balance is insufficient")
	ErrUnknownBlockchain      = errors.New("unknown blockchain")
)

func New(repo *Repository, generator *Generator, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "kms_service").Logger()

	return &Service{
		repo:      repo,
		generator: generator,
		logger:    &log,
	}
}

func (s *Service) CreateWallet(_ context.Context, blockchain Blockchain) (*Wallet, error) {
	wallet, err := s.generator.CreateWallet(blockchain)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Set(wallet); err != nil {
		msg := "unable to persist wallet"
		s.logger.Error().Err(err).Msg(msg)

		return nil, errors.Wrap(err, msg)
	}

	return wallet, nil
}

func (s *Service) GetWallet(_ context.Context, id uuid.UUID, withTrashed bool) (*Wallet, error) {
	return s.repo.Get(id, withTrashed)
}

func (s *Service) DeleteWallet(ctx context.Context, id uuid.UUID) error {
	wallet, err := s.GetWallet(ctx, id, false)
	if err != nil {
		return err
	}

	return s.repo.SoftDelete(wallet)
}

// CreateEthereumTransaction creates and sings new raw Ethereum transaction based on provided input.
func (s *Service) CreateEthereumTransaction(_ context.Context, wt *Wallet, params EthTransactionParams) (string, error) {
	if _, ok := s.generator.providers[ETH]; !ok {
		return "", errors.New("ETH provider not found")
	}

	eth, ok := s.generator.providers[ETH].(*EthProvider)
	if !ok {
		return "", errors.New("ETH provider is invalid")
	}

	return eth.NewTransaction(wt, params)
}

func (s *Service) CreateMaticTransaction(_ context.Context, wt *Wallet, params EthTransactionParams) (string, error) {
	if _, ok := s.generator.providers[MATIC]; !ok {
		return "", errors.New("MATIC provider not found")
	}

	matic, ok := s.generator.providers[MATIC].(*EthProvider)
	if !ok {
		return "", errors.New("MATIC provider is invalid")
	}

	return matic.NewTransaction(wt, params)
}

func (s *Service) CreateBSCTransaction(_ context.Context, wt *Wallet, params EthTransactionParams) (string, error) {
	if _, ok := s.generator.providers[BSC]; !ok {
		return "", errors.New("BSC provider not found")
	}

	bsc, ok := s.generator.providers[BSC].(*EthProvider)
	if !ok {
		return "", errors.New("BSC provider is invalid")
	}

	return bsc.NewTransaction(wt, params)
}

func (s *Service) CreateTronTransaction(
	ctx context.Context, wallet *Wallet, params TronTransactionParams,
) (TronTransaction, error) {
	if _, ok := s.generator.providers[TRON]; !ok {
		return TronTransaction{}, errors.New("TRON provider not found")
	}

	tron, ok := s.generator.providers[TRON].(*TronProvider)
	if !ok {
		return TronTransaction{}, errors.New("TRON provider is invalid")
	}

	return tron.NewTransaction(ctx, wallet, params)
}
