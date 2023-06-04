package cmd

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/kms"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/spf13/cobra"
)

var allInOneCommand = &cobra.Command{
	Use:   "all-in-one",
	Short: "Runs server, scheduler, and KMS in a single instance",
	Run:   allInOne,
}

func allInOne(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	cfg := resolveConfig()

	// "embed" KMS
	cfg.KMS.IsEmbedded = true
	cfg.Providers.KmsClient.Host = "localhost:14000"

	service := app.New(ctx, cfg)
	kmsService := kms.NewApp(ctx, cfg)

	setupOnBeforeRun(service, cfg)

	service.RunServer()
	service.RunScheduler()
	kmsService.Run()

	if err := graceful.WaitShutdown(); err != nil {
		service.Logger().Error().Err(err).Msg("unable to shutdown service gracefully")
		return
	}

	service.Logger().Info().Msg("shutdown complete")
}
