package user

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/service/registry"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
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
	ErrNotFound      = errors.New("user not found")
	ErrWrongPassword = errors.New("wrong password provided")
	ErrAlreadyExists = errors.New("user already exists")
	ErrRestricted    = errors.New("access restricted")
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

	return entryToUser(entry)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	entry, err := s.store.GetUserByEmail(ctx, email)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry)
}

func (s *Service) GetByEmailWithPasswordCheck(ctx context.Context, email, password string) (*User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	entry, err := s.store.GetUserByEmail(ctx, email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	if !checkPass(entry.Password.String, password) {
		return nil, ErrWrongPassword
	}

	return entryToUser(entry)
}

// Register registers user via email. If user already exists, return User and ErrAlreadyExists
func (s *Service) Register(ctx context.Context, email, pass string) (*User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	if len(pass) < 8 {
		return nil, errors.New("password should have minimum length of 8")
	}

	// check if exists
	u, err := s.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return u, ErrAlreadyExists
	case errors.Is(err, ErrNotFound):
		// do nothing
	case err != nil:
		return nil, err
	}

	hashedPass, err := hashPass(pass)
	if err != nil {
		return nil, err
	}

	entry, err := s.store.CreateUser(ctx, repository.CreateUserParams{
		Name:      email[:strings.IndexByte(email, '@')],
		Email:     email,
		Password:  repository.StringToNullable(hashedPass),
		Uuid:      uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Settings:  pgtype.JSONB{Status: pgtype.Null},
	})
	if err != nil {
		return nil, err
	}

	event := bus.UserRegisteredEvent{UserID: entry.ID}
	if err = s.publisher.Publish(bus.TopicUserRegistered, event); err != nil {
		s.logger.Error().Err(err).Msg("unable to publish event")
	}

	return entryToUser(entry)
}

func (s *Service) UpdatePassword(ctx context.Context, id int64, pass string) (*User, error) {
	if len(pass) < 8 {
		return nil, errors.New("password should have minimum length of 8")
	}

	hashedPass, err := hashPass(pass)
	if err != nil {
		return nil, err
	}

	entry, err := s.store.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{
		ID:        id,
		Password:  repository.StringToNullable(hashedPass),
		UpdatedAt: time.Now(),
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToUser(entry)
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

func entryToUser(entry repository.User) (*User, error) {
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
	}, nil
}

func validateEmail(email string) error {
	if _, err := mail.ParseAddress(email); err != nil {
		return err
	}

	return nil
}

func hashPass(pass string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func checkPass(hashed, pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(pass)) == nil
}
