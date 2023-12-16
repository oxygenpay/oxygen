package merchant

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
}

type Service struct {
	repo       *repository.Queries
	blockchain BlockchainService
	logger     *zerolog.Logger
}

var (
	ErrMerchantNotFound     = errors.New("merchant not found")
	ErrAddressNotFound      = errors.New("merchant address not found")
	ErrAddressAlreadyExists = errors.New("merchant address already exists")
	ErrAddressReserved      = errors.New("this address is reserved")
)

func New(
	repo *repository.Queries,
	blockchainService BlockchainService,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "merchant_service").Logger()

	return &Service{
		repo:       repo,
		blockchain: blockchainService,
		logger:     &log,
	}
}

func (s *Service) GetByID(ctx context.Context, id int64, withTrashed bool) (*Merchant, error) {
	entry, err := s.repo.GetMerchantByID(ctx, repository.GetMerchantByIDParams{
		ID:          id,
		WithTrashed: withTrashed,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrMerchantNotFound
	case err != nil:
		return nil, err
	}

	return entryToMerchant(entry), nil
}

func (s *Service) GetByUUIDAndCreatorID(
	ctx context.Context,
	merchantUUID uuid.UUID,
	creatorID int64,
	withTrashed bool,
) (*Merchant, error) {
	entry, err := s.repo.GetMerchantByUUIDAndCreatorID(ctx, repository.GetMerchantByUUIDAndCreatorIDParams{
		Uuid:        merchantUUID,
		CreatorID:   creatorID,
		WithTrashed: withTrashed,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrMerchantNotFound
	case err != nil:
		return nil, err
	}

	return entryToMerchant(entry), nil
}

func (s *Service) Create(ctx context.Context, creatorID int64, name, website string, settings Settings) (*Merchant, error) {
	entry, err := s.repo.CreateMerchant(ctx, repository.CreateMerchantParams{
		Uuid:      uuid.New(),
		Name:      name,
		Website:   website,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: sql.NullTime{},
		CreatorID: creatorID,
		Settings:  settings.toJSONB(),
	})

	if err != nil {
		return nil, err
	}

	return entryToMerchant(entry), nil
}

func (s *Service) Update(ctx context.Context, id int64, name, website string) (*Merchant, error) {
	if _, err := s.GetByID(ctx, id, false); err != nil {
		return nil, err
	}

	entry, err := s.repo.UpdateMerchant(ctx, repository.UpdateMerchantParams{
		ID:        id,
		UpdatedAt: time.Now(),
		Name:      name,
		Website:   website,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrMerchantNotFound
	case err != nil:
		return nil, err
	}

	return entryToMerchant(entry), nil
}

func (s *Service) ListByCreatorID(ctx context.Context, creatorID int64) ([]*Merchant, error) {
	entries, err := s.repo.ListMerchantsByCreatorID(ctx, repository.ListMerchantsByCreatorIDParams{
		CreatorID:   creatorID,
		WithTrashed: false,
	})

	var merchants = make([]*Merchant, len(entries))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}

	for i := range entries {
		merchants[i] = entryToMerchant(entries[i])
	}

	return merchants, nil
}

type SupportedCurrency struct {
	Currency money.CryptoCurrency
	Enabled  bool
}

func (s *Service) ListSupportedCurrencies(_ context.Context, merchant *Merchant) ([]SupportedCurrency, error) {
	all := s.blockchain.ListSupportedCurrencies(false)
	enabledTickers := util.Set(merchant.Settings().PaymentMethods())

	// if merchant didn't set this parameter yet, let's treat that as "all currencies enabled"
	if len(enabledTickers) == 0 {
		fn := func(c money.CryptoCurrency) SupportedCurrency { return SupportedCurrency{Currency: c, Enabled: true} }
		return util.MapSlice(all, fn), nil
	}

	results := make([]SupportedCurrency, len(all))
	for i := range all {
		_, enabled := enabledTickers[all[i].Ticker]

		results[i] = SupportedCurrency{Currency: all[i], Enabled: enabled}
	}

	return results, nil
}

func (s *Service) UpsertSettings(ctx context.Context, merchant *Merchant, settings Settings) error {
	for prop, value := range settings {
		merchant.settings[prop] = value
	}

	return s.repo.UpdateMerchantSettings(ctx, repository.UpdateMerchantSettingsParams{
		ID:        merchant.ID,
		UpdatedAt: time.Now(),
		Settings:  merchant.settings.toJSONB(),
	})
}

func (s *Service) UpdateSupportedMethods(ctx context.Context, merchant *Merchant, tickers []string) error {
	if len(tickers) == 0 {
		return errors.New("tickers are empty")
	}

	// check that tickers are valid
	tickersSet := util.Set(tickers)
	availableTickersSet := util.Set(
		util.MapSlice(
			s.blockchain.ListSupportedCurrencies(false),
			func(c money.CryptoCurrency) string { return c.Ticker },
		),
	)

	for ticker := range tickersSet {
		if _, exists := availableTickersSet[ticker]; !exists {
			return fmt.Errorf("ticker %q doesn't exist", ticker)
		}
	}

	return s.UpsertSettings(ctx, merchant, Settings{
		// example: "ETH,ETH_USDT"
		PropertyPaymentMethods: strings.Join(util.Keys(tickersSet), ","),
	})
}

func (s *Service) DeleteByUUID(ctx context.Context, merchantUUID uuid.UUID) error {
	return s.repo.SoftDeleteMerchantByUUID(ctx, merchantUUID)
}

func entryToMerchant(entry repository.Merchant) *Merchant {
	settings := make(Settings)
	_ = json.Unmarshal(entry.Settings.Bytes, &settings)

	return &Merchant{
		ID:        entry.ID,
		UUID:      entry.Uuid,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
		Name:      entry.Name,
		Website:   entry.Website,
		CreatorID: entry.CreatorID,
		settings:  settings,
	}
}
