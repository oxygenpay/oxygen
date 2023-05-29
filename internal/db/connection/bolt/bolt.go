package bolt

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

type Config struct {
	DataSource string `yaml:"path" env:"KMS_DB_DATA_SOURCE"`
}

type Connection struct {
	db     *bbolt.DB
	logger *zerolog.Logger
}

const WalletsBucket = "wallets"

func Open(cfg Config, logger *zerolog.Logger) (*Connection, error) {
	log := logger.With().Str("channel", "bolt").Logger()

	log.Info().Str("db_path", cfg.DataSource).Msg("connecting to bolt")
	db, err := bbolt.Open(cfg.DataSource, 0660, nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to open bolt db file")
	}

	connection := &Connection{
		db:     db,
		logger: logger,
	}

	log.Info().Str("db_path", cfg.DataSource).Msg("connected to bolt")

	return connection, nil
}

func (c *Connection) LoadBuckets() error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(WalletsBucket))
		return err
	})
}

func (c *Connection) DB() *bbolt.DB {
	return c.db
}
