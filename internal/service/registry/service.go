package registry

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	repo   *repository.Queries
	logger *zerolog.Logger
}

func New(
	repo *repository.Queries,

	logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "registry_service").Logger()

	return &Service{repo: repo, logger: &log}
}

type Value struct {
	Key, Value string
}

var ErrNotFound = errors.New("registry item not found")

const globalConfigMerchantID = 0

func (s *Service) GetBoolSafe(ctx context.Context, key string, defaultValue bool) bool {
	raw := s.GetValueSafe(ctx, key, fmt.Sprintf("%t", defaultValue))
	b, _ := strconv.ParseBool(raw.Value)

	return b
}

func (s *Service) GetStringsSafe(ctx context.Context, key string) []string {
	str := s.GetValueSafe(ctx, key, "").Value
	if str == "" {
		return nil
	}

	items := strings.Split(str, ",")

	return util.MapSlice(items, strings.TrimSpace)
}

func (s *Service) GetValueSafe(ctx context.Context, key, defaultValue string) Value {
	v, err := s.Get(ctx, key)
	if err != nil {
		return Value{Key: key, Value: defaultValue}
	}

	return v
}

func (s *Service) Get(ctx context.Context, key string) (Value, error) {
	entry, err := s.repo.GetRegistryItemByKey(ctx, repository.GetRegistryItemByKeyParams{
		MerchantID: globalConfigMerchantID,
		Key:        key,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Value{}, errors.Wrap(ErrNotFound, key)
	case err != nil:
		return Value{}, errors.Wrap(err, "unable to GetRegistryItemByKey")
	}

	return entryToValue(entry)
}

func (s *Service) Set(ctx context.Context, key, value string) (Value, error) {
	v, err := s.Get(ctx, key)

	// create entry
	if errors.Is(err, ErrNotFound) {
		entry, errCreate := s.repo.CreateRegistryItem(ctx, repository.CreateRegistryItemParams{
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			MerchantID: globalConfigMerchantID,
			Key:        key,
			Value:      value,
		})

		if errCreate != nil {
			return Value{}, errors.Wrap(err, "unable to update registry item")
		}

		return entryToValue(entry)
	}

	// update entry
	if err == nil {
		entry, errUpdate := s.repo.UpdateRegistryItem(ctx, repository.UpdateRegistryItemParams{
			Key:        v.Key,
			MerchantID: globalConfigMerchantID,
			UpdatedAt:  time.Now(),
			Value:      value,
		})

		if errUpdate != nil {
			return Value{}, errors.Wrap(err, "unable to update registry item")
		}

		return entryToValue(entry)
	}

	// unknown error
	return Value{}, errors.Wrap(err, "unable to set registry item")
}

func entryToValue(entry repository.Registry) (Value, error) {
	return Value{Key: entry.Key, Value: entry.Value}, nil
}
