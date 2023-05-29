package user

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/auth"
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

func New(
	store repository.Storage,
	publisher bus.Publisher,
	registryService *registry.Service,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "user_service").Logger()

	return &Service{
		store:     store,
		publisher: publisher,
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

func (s *Service) ResolveWithGoogle(
	ctx context.Context,
	googleUser *auth.GoogleUser,
) (*User, error) {
	entry, err := s.store.GetUserByGoogleID(ctx, repository.StringToNullable(googleUser.Sub))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return s.registerUser(ctx, googleUser)
	case err != nil:
		return nil, errors.Wrap(err, "unable to get user")
	}

	return s.updateUser(ctx, entry.ID, googleUser)
}

func (s *Service) registerUser(ctx context.Context, googleUser *auth.GoogleUser) (*User, error) {
	if err := s.guardRegistration(ctx, googleUser.Email); err != nil {
		return nil, err
	}

	entry, err := s.store.CreateUser(ctx, repository.CreateUserParams{
		Name:            googleUser.Name,
		Email:           googleUser.Email,
		Uuid:            uuid.New(),
		GoogleID:        repository.StringToNullable(googleUser.Sub),
		ProfileImageUrl: repository.StringToNullable(googleUser.Picture),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeletedAt:       sql.NullTime{},
		Settings:        pgtype.JSONB{Status: pgtype.Null},
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to create user")
	}

	event := bus.UserRegisteredEvent{UserID: entry.ID}
	if err := s.publisher.Publish(bus.TopicUserRegistered, event); err != nil {
		s.logger.Error().Err(err).Msg("unable to publish event")
	}

	return entryToUser(entry), nil
}

func (s *Service) updateUser(ctx context.Context, userID int64, googleUser *auth.GoogleUser) (*User, error) {
	entry, err := s.store.UpdateUser(ctx, repository.UpdateUserParams{
		ID:              userID,
		Name:            googleUser.Name,
		ProfileImageUrl: repository.StringToNullable(googleUser.Picture),
		UpdatedAt:       time.Now(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to update user")
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
