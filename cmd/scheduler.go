package cmd

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/spf13/cobra"
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Start Oxygen Scheduler",
	Run:   startScheduler,
}

func startScheduler(_ *cobra.Command, _ []string) {
	ctx := context.Background()

	service := app.New(ctx, resolveConfig())
	service.RunScheduler()

	if err := graceful.WaitShutdown(); err != nil {
		service.Logger().Error().Err(err).Msg("unable to shutdown service gracefully")
		return
	}

	service.Logger().Info().Msg("shutdown complete")
}
