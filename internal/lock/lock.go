// Package lock implements generic application-level locking mechanism based on pg "advisory lock" feature.
// see https://www.postgresql.org/docs/current/explicit-locking.html#ADVISORY-LOCKS.
package lock

import (
	"context"
	"fmt"
	"hash/crc32"

	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/pkg/errors"
)

// Key represents unique lock key
type Key interface {
	Int64() int64
}

type Locker struct {
	store *repository.Store
}

func New(store *repository.Store) *Locker {
	return &Locker{store: store}
}

// Do acquires exclusive lock for provided resource
func (l *Locker) Do(ctx context.Context, key Key, run func() error) error {
	lockID := key.Int64()

	return l.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		// lock will be released automatically after tx commit/rollback
		if err := q.AdvisoryTxLock(ctx, lockID); err != nil {
			return errors.Wrap(err, "unable to set advisory lock")
		}

		if err := run(); err != nil {
			return errors.Wrap(err, "callback execution error")
		}

		return nil
	})
}

type RowKey struct {
	Table string
	ID    int64
}

func (k RowKey) Int64() int64 {
	return hash("%s.%d", k.Table, k.ID)
}

type StringKey struct {
	Key string
}

func (k StringKey) Int64() int64 {
	return hash(k.Key)
}

func hash(tpl string, args ...any) int64 {
	str := fmt.Sprintf(tpl, args...)
	checksum := crc32.ChecksumIEEE([]byte(str))

	return int64(checksum)
}
