package repository

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/connection/pg"
)

// Store combines features of a repo and db connection (e.g. transaction support)
type Store struct {
	Queries
	db *pg.Connection
}

type Storage interface {
	Querier
	RunTransaction(ctx context.Context, callback TxCallback) error
}

// NewStore Store constructor.
func NewStore(db *pg.Connection) *Store {
	return &Store{
		Queries: Queries{db: db},
		db:      db,
	}
}

type TxCallback func(context.Context, Querier) error

func (s *Store) RunTransaction(ctx context.Context, callback TxCallback) error {
	return s.db.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return callback(ctx, s.WithTx(tx))
	})
}
