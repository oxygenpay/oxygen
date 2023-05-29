package cmd

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/kms"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/spf13/cobra"
)

var kmsServerCmd = &cobra.Command{
	Use:   "kms-server",
	Short: "Start Key Management Server",
	Run:   kmsServer,
}

func kmsServer(_ *cobra.Command, _ []string) {
	service := kms.NewApp(resolveConfig())
	service.Run(context.Background())

	if err := graceful.WaitShutdown(); err != nil {
		service.Logger().Error().Err(err).Msg("unable to shutdown service gracefully")
		return
	}

	service.Logger().Info().Msg("shutdown complete")
}
