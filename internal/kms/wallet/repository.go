package wallet

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/db/connection/bolt"
	"go.etcd.io/bbolt"
)

type Repository struct {
	db *bbolt.DB
}

var ErrNotFound = errors.New("wallet not found")

func NewRepository(db *bbolt.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Get(id uuid.UUID, withTrashed bool) (*Wallet, error) {
	found := false
	w := &Wallet{}

	err := r.db.View(func(tx *bbolt.Tx) error {
		b := mustResolveBucket(tx, bolt.WalletsBucket)

		rawValue := b.Get(uuidToKey(id))
		if len(rawValue) == 0 {
			return nil
		}

		if err := json.Unmarshal(rawValue, w); err != nil {
			return err
		}

		found = true

		return nil
	})

	if !found {
		return nil, ErrNotFound
	}

	if w.DeletedAt != nil && !withTrashed {
		return nil, err
	}

	return w, err
}

func (r *Repository) Set(w *Wallet) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		rawValue, err := json.Marshal(w)
		if err != nil {
			return err
		}

		b := mustResolveBucket(tx, bolt.WalletsBucket)
		return b.Put(uuidToKey(w.UUID), rawValue)
	})
}

func (r *Repository) SoftDelete(w *Wallet) error {
	if w.DeletedAt == nil {
		now := time.Now()
		w.DeletedAt = &now
	}

	return r.Set(w)
}

func mustResolveBucket(tx *bbolt.Tx, name string) *bbolt.Bucket {
	bucket := tx.Bucket([]byte(name))
	if bucket == nil {
		panic("unable to resolve bucket " + name)
	}

	return bucket
}

func uuidToKey(id uuid.UUID) []byte {
	return []byte(id.String())
}
