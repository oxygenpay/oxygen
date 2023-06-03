package user

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/service/registry"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	store     repository.Storage
	publisher bus.Publisher
	registry  *registry.Service
	logger    *zerolog.Logger
}

type User struct {
	ID              int64
	Name            string
	Email           string
	UUID            uuid.UUID
	GoogleID        *string
	ProfileImageURL *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
	Settings        []byte
}

var (
	ErrNotFound   = errors.New("user not found")
	ErrRestricted = errors.New("access restricted")
)

const (
	registryRegistrationWhitelistOnly = "registration.is_whitelist_only"
	registryRegistrationWhitelist     = "registration.whitelist"
)

func New(store repository.Storage, pub bus.Publisher, registryService *registry.Service, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "user_service").Logger()

	return &Service{
		store:     store,
		publisher: pub,
		registry:  registryService,
		logger:    &log,
	}
}

func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	entry, err := s.store.GetUserByID(ctx, id)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry), nil
}

// guardRegistration restricts user from registration is registration by whitelist is enabled.
func (s *Service) guardRegistration(ctx context.Context, email string) error {
	bouncer := s.registry.GetBoolSafe(ctx, registryRegistrationWhitelistOnly, false)
	if !bouncer {
		return nil
	}

	var matched bool

	whitelist := s.registry.GetStringsSafe(ctx, registryRegistrationWhitelist)
	for _, e := range whitelist {
		if e == email {
			matched = true
			break
		}
	}

	if !matched {
		s.logger.Error().Str("email", email).Msg("Restricted user registration due to enabled whitelist")
		return ErrRestricted
	}

	return nil
}

func entryToUser(entry repository.User) *User {
	return &User{
		ID:              entry.ID,
		Name:            entry.Name,
		Email:           entry.Email,
		UUID:            entry.Uuid,
		GoogleID:        repository.NullableStringToPointer(entry.GoogleID),
		ProfileImageURL: repository.NullableStringToPointer(entry.ProfileImageUrl),
		CreatedAt:       entry.CreatedAt,
		UpdatedAt:       entry.UpdatedAt,
		DeletedAt:       nil,
		Settings:        nil,
	}
}
