package cmd

import (
	"context"
	"errors"

	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/spf13/cobra"
)

var serveWebCommand = &cobra.Command{
	Use:   "serve-web",
	Short: "Start Oxygen Server",
	Run:   serveWeb,
}

func serveWeb(_ *cobra.Command, _ []string) {
	ctx := context.Background()
	cfg := resolveConfig()

	service := app.New(ctx, cfg)

	setupOnBeforeRun(service, cfg)

	service.RunServer()
	if err := graceful.WaitShutdown(); err != nil {
		service.Logger().Error().Err(err).Msg("unable to shutdown service gracefully")
		return
	}

	service.Logger().Info().Msg("shutdown complete")
}

func setupOnBeforeRun(service *app.App, cfg *config.Config) {
	service.OnBeforeRun(func(ctx context.Context, a *app.App) error {
		if cfg.Oxygen.Postgres.MigrateOnStart {
			a.Logger().Info().Msg("Enabled migration on start")
			performMigration(ctx, cfg, "up", true)
		}

		return nil
	})

	service.OnBeforeRun(func(ctx context.Context, a *app.App) error {
		if len(cfg.Oxygen.Auth.EnabledProviders()) == 0 {
			return errors.New("unable to run server: at least one auth provider should be enabled")
		}

		return nil
	})
}
