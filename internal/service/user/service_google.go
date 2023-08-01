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
	"github.com/pkg/errors"
)

func (s *Service) ResolveWithGoogle(ctx context.Context, user *auth.GoogleUser) (*User, error) {
	entry, err := s.store.GetUserByEmail(ctx, user.Email)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return s.registerGoogleUser(ctx, user)
	case err != nil:
		return nil, errors.Wrap(err, "unable to get user")
	}

	return s.updateGoogleUser(ctx, entry.ID, user)
}

func (s *Service) registerGoogleUser(ctx context.Context, user *auth.GoogleUser) (*User, error) {
	if err := s.guardRegistration(ctx, user.Email); err != nil {
		return nil, err
	}

	entry, err := s.store.CreateUser(ctx, repository.CreateUserParams{
		Name:            user.Name,
		Email:           user.Email,
		Uuid:            uuid.New(),
		GoogleID:        repository.StringToNullable(user.Sub),
		ProfileImageUrl: repository.StringToNullable(user.Picture),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		DeletedAt:       sql.NullTime{},
		Settings:        pgtype.JSONB{Status: pgtype.Null},
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to create user")
	}

	event := bus.UserRegisteredEvent{UserID: entry.ID}
	if err = s.publisher.Publish(bus.TopicUserRegistered, event); err != nil {
		s.logger.Error().Err(err).Msg("unable to publish event")
	}

	return entryToUser(entry)
}

func (s *Service) updateGoogleUser(ctx context.Context, userID int64, user *auth.GoogleUser) (*User, error) {
	entry, err := s.store.UpdateUser(ctx, repository.UpdateUserParams{
		ID:              userID,
		SetGoogleID:     true,
		GoogleID:        repository.StringToNullable(user.Sub),
		Name:            user.Name,
		ProfileImageUrl: repository.StringToNullable(user.Picture),
		UpdatedAt:       time.Now(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to update user")
	}

	return entryToUser(entry)
}
