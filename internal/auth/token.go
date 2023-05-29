package auth

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/rs/zerolog"
)

type TokenAuthManager struct {
	repo   *repository.Queries
	logger *zerolog.Logger
}

type TokenType string

type APIToken struct {
	ID         int64
	EntityType TokenType
	EntityID   int64
	CreatedAt  time.Time
	Token      string
	Name       *string
	UUID       uuid.UUID
	Settings   []byte
}

var (
	ErrNotFound = errors.New("token not found")
)

const (
	TokenTypeUser     TokenType = "user"
	TokenTypeMerchant TokenType = "merchant"

	tokenRuneSource = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + "0123456789" + ".$;"
)

func NewTokenAuth(repo *repository.Queries, logger *zerolog.Logger) *TokenAuthManager {
	log := logger.With().Str("channel", "token_oauth").Logger()

	return &TokenAuthManager{
		repo:   repo,
		logger: &log,
	}
}

func (m *TokenAuthManager) GetToken(ctx context.Context, tokenType TokenType, token string) (*APIToken, error) {
	entry, err := m.repo.GetAPIToken(ctx, repository.GetAPITokenParams{
		EntityType: string(tokenType),
		Token:      token,
	})

	if err != nil {
		return nil, err
	}

	return entryToAPIToken(entry), nil
}

func (m *TokenAuthManager) GetTokenByUUID(ctx context.Context, id uuid.UUID) (*APIToken, error) {
	entry, err := m.repo.GetAPITokenByUUID(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToAPIToken(entry), nil
}

func (m *TokenAuthManager) GetMerchantTokenByUUID(ctx context.Context, merchantID int64, id uuid.UUID) (*APIToken, error) {
	token, err := m.GetTokenByUUID(ctx, id)
	if err != nil {
		return nil, err
	}

	if token.EntityType != TokenTypeMerchant || token.EntityID != merchantID {
		return nil, ErrNotFound
	}

	return token, nil
}

func (m *TokenAuthManager) CreateUserToken(ctx context.Context, merchantID int64, name string) (*APIToken, error) {
	return m.createToken(ctx, TokenTypeUser, merchantID, name)
}

func (m *TokenAuthManager) CreateMerchantToken(ctx context.Context, merchantID int64, name string) (*APIToken, error) {
	return m.createToken(ctx, TokenTypeMerchant, merchantID, name)
}

func (m *TokenAuthManager) createToken(
	ctx context.Context,
	tokenType TokenType,
	entityID int64, name string,
) (*APIToken, error) {
	entry, err := m.repo.CreateAPIToken(ctx, repository.CreateAPITokenParams{
		EntityType: string(tokenType),
		EntityID:   entityID,
		CreatedAt:  time.Now(),
		Token:      generateToken(64),
		Uuid:       uuid.New(),
		Name: sql.NullString{
			Valid:  true,
			String: name,
		},
		Settings: pgtype.JSONB{
			Status: pgtype.Null,
		},
	})

	if err != nil {
		return nil, err
	}

	return entryToAPIToken(entry), nil
}

func (m *TokenAuthManager) ListByEntityType(
	ctx context.Context,
	tokenType TokenType,
	entityID int64,
) ([]*APIToken, error) {
	entries, err := m.repo.ListAPITokensByEntity(ctx, repository.ListAPITokensByEntityParams{
		EntityID:   entityID,
		EntityType: string(tokenType),
	})

	var tokens = make([]*APIToken, len(entries))

	if err != nil {
		if err == pgx.ErrNoRows {
			return tokens, nil
		}

		return nil, err
	}
	for i := range entries {
		tokens[i] = entryToAPIToken(entries[i])
	}

	return tokens, nil
}

func (m *TokenAuthManager) DeleteToken(ctx context.Context, id int64) error {
	return m.repo.DeleteAPITokenByID(ctx, id)
}

func entryToAPIToken(entry repository.ApiToken) *APIToken {
	return &APIToken{
		ID:         entry.ID,
		EntityType: TokenType(entry.EntityType),
		EntityID:   entry.EntityID,
		CreatedAt:  entry.CreatedAt,
		Token:      entry.Token,
		Name:       repository.NullableStringToPointer(entry.Name),
		UUID:       entry.Uuid,
		Settings:   []byte{}, // todo
	}
}

func generateToken(length int) string {
	result := make([]byte, length)

	for i := range result {
		// nolint gosec
		j := rand.Intn(len(tokenRuneSource))
		result[i] = tokenRuneSource[j]
	}

	return string(result)
}
