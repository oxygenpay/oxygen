package pg

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Config struct {
	DataSource     string `yaml:"data_source" env:"DB_DATA_SOURCE" env-description:"Postgres connection string. Example: 'host=localhost sslmode=disable dbname=oxygen user=oxygen password=qwerty pool_max_conns=32'"`
	MigrateOnStart bool   `yaml:"migrate_on_start" env:"DB_MIGRATE_ON_START" env-default:"true" env-description:"Apply database migrations on start"`
}

type Connection struct {
	*pgxpool.Pool
	logger *zerolog.Logger
}

func Open(ctx context.Context, cfg Config, logger *zerolog.Logger) (*Connection, error) {
	dbConfig, err := pgxpool.ParseConfig(cfg.DataSource)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse db config")
	}

	pgConnection, err := pgxpool.ConnectConfig(ctx, dbConfig)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to pg database")
	}

	log := logger.With().Str("channel", "postgres").Logger()

	connection := &Connection{
		Pool:   pgConnection,
		logger: logger,
	}

	log.Info().
		Str("db_host", dbConfig.ConnConfig.Host).
		Str("db_name", dbConfig.ConnConfig.Database).
		Str("db_user", dbConfig.ConnConfig.User).
		Int32("db_min_connections", dbConfig.MaxConns).
		Int32("db_max_connections", dbConfig.MinConns).
		Msg("connected to postgres")

	return connection, nil
}

func (c *Connection) Shutdown() error {
	stats := c.Pool.Stat()
	c.logger.Info().Interface("stats", stats).Msg("shutting down postgres connections")
	c.Pool.Close()
	return nil
}
