package bolt

import (
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

type Config struct {
	DataSource string `yaml:"path" env:"KMS_DB_DATA_SOURCE" env-description:"KMS vault data source. Example: '/opt/oxygen/kms.db'"`
}

type Connection struct {
	db     *bbolt.DB
	logger *zerolog.Logger
}

const WalletsBucket = "wallets"

const chmodReadWrite = 0660

func Open(cfg Config, logger *zerolog.Logger) (*Connection, error) {
	log := logger.With().Str("channel", "bolt").Logger()

	if err := util.EnsureFile(cfg.DataSource, chmodReadWrite); err != nil {
		return nil, errors.Wrap(err, "unable to ensure KMS database file")
	}

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
