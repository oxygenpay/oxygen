// Package kms represents Key Management Generator as separate application
package kms

import (
	"context"
	cryptorand "crypto/rand"
	"net/http"
	"os"

	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/oxygenpay/oxygen/internal/db/connection/bolt"
	"github.com/oxygenpay/oxygen/internal/kms/api"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	httpServer "github.com/oxygenpay/oxygen/internal/server/http"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/rs/zerolog"
	"go.etcd.io/bbolt"
)

type App struct {
	ctx    context.Context
	config *config.Config
	logger *zerolog.Logger
	db     *bbolt.DB
}

func NewApp(ctx context.Context, cfg *config.Config) *App {
	hostname, _ := os.Hostname()
	logger := log.New(cfg.Logger, "kms", cfg.GitVersion, cfg.Env, hostname)

	return &App{
		ctx:    ctx,
		config: cfg,
		logger: &logger,
	}
}

func (app *App) Run() {
	app.connectToDB()
	app.runWebServer(app.ctx)
}

func (app *App) Logger() *zerolog.Logger {
	return app.logger
}

func (app *App) connectToDB() {
	conn, err := bolt.Open(app.config.KMS.Bolt, app.logger)
	if err != nil {
		app.logger.Fatal().Err(err).Msg("unable to run kms without db")
	}

	if err := conn.LoadBuckets(); err != nil {
		app.logger.Fatal().Err(err).Msg("unable to load kms bolt db buckets")
	}

	app.db = conn.DB()
}

func (app *App) runWebServer(ctx context.Context) {
	walletGenerator := wallet.NewGenerator().
		AddProvider(&wallet.EthProvider{Blockchain: wallet.ETH, CryptoReader: cryptorand.Reader}).
		AddProvider(&wallet.EthProvider{Blockchain: wallet.MATIC, CryptoReader: cryptorand.Reader}).
		AddProvider(&wallet.EthProvider{Blockchain: wallet.BSC, CryptoReader: cryptorand.Reader}).
		AddProvider(&wallet.BitcoinProvider{Blockchain: wallet.BTC, CryptoReader: cryptorand.Reader}).
		AddProvider(&wallet.TronProvider{
			Blockchain:   wallet.TRON,
			Trongrid:     trongrid.New(app.config.Providers.Trongrid, app.logger),
			CryptoReader: cryptorand.Reader,
		})

	walletRepo := wallet.NewRepository(app.db)
	kmsService := wallet.New(walletRepo, walletGenerator, app.logger)

	if app.config.KMS.IsEmbedded {
		app.config.KMS.Server.Port = "14000"
	}

	srv := httpServer.New(
		app.config.KMS.Server,
		app.config.Debug,
		httpServer.WithRecover(),
		httpServer.WithLogger(app.logger),
		httpServer.WithBodyDump(),
		api.SetupRoutes(api.New(kmsService, app.logger)),
	)

	go func() {
		app.logger.Info().Str("address", srv.Address()).Msg("starting http server")
		if err := srv.Run(); err != nil && err != http.ErrServerClosed {
			app.logger.Error().Err(err).Msg("unable to run http server")
			graceful.ShutdownNow()
		}
	}()

	graceful.AddCallback(func() error {
		app.logger.Info().Msg("shutting down http server")
		return srv.Shutdown(ctx)
	})
}
