package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/oxygenpay/oxygen/internal/event/paymentevents"
	"github.com/oxygenpay/oxygen/internal/event/userevents"
	"github.com/oxygenpay/oxygen/internal/locator"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/scheduler"
	httpServer "github.com/oxygenpay/oxygen/internal/server/http"
	"github.com/oxygenpay/oxygen/internal/server/http/internalapi"
	"github.com/oxygenpay/oxygen/internal/server/http/merchantapi"
	merchantauth "github.com/oxygenpay/oxygen/internal/server/http/merchantapi/auth"
	"github.com/oxygenpay/oxygen/internal/server/http/paymentapi"
	"github.com/oxygenpay/oxygen/internal/server/http/webhook"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	uidashboard "github.com/oxygenpay/oxygen/ui-dashboard"
	uipayment "github.com/oxygenpay/oxygen/ui-payment"
	"github.com/oxygenpay/oxygen/web"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

type App struct {
	config    *config.Config
	ctx       context.Context
	logger    *zerolog.Logger
	services  *locator.Locator
	beforeRun []BeforeRun
}

type BeforeRun func(ctx context.Context, app *App) error

func New(ctx context.Context, cfg *config.Config) *App {
	hostname, _ := os.Hostname()
	logger := log.New(cfg.Logger, "oxygen", cfg.GitVersion, cfg.Env, hostname)

	return &App{
		config:   cfg,
		ctx:      ctx,
		logger:   &logger,
		services: locator.New(ctx, cfg, &logger),
	}
}

func (app *App) Locator() *locator.Locator {
	return app.services
}

func (app *App) OnBeforeRun(fn BeforeRun) {
	app.beforeRun = append(app.beforeRun, fn)
}

func (app *App) Logger() *zerolog.Logger {
	return app.logger
}

func (app *App) RunServer() {
	// HTTP Handlers
	merchantAPIHandler := merchantapi.NewHandler(
		app.services.MerchantService(),
		app.services.TokenManagerService(),
		app.services.PaymentService(),
		app.services.WalletService(),
		app.services.BlockchainService(),
		app.services.EventBus(),
		app.Logger(),
	)

	dashboardAuthHandler := merchantauth.NewHandler(
		app.services.GoogleAuth(),
		app.services.UserService(),
		app.config.Oxygen.Auth.EnabledProviders(),
		app.Logger(),
	)

	paymentAPIHandler := paymentapi.New(
		app.services.PaymentService(),
		app.services.MerchantService(),
		app.services.BlockchainService(),
		app.services.ProcessingService(),
		app.Logger(),
	)

	// handler for providers webhook (e.g. Tatum)
	incomingWebhooksHandler := webhook.New(
		app.services.ProcessingService(),
		app.Logger(),
	)

	schedulerHandler := scheduler.New(
		app.services.PaymentService(),
		app.services.BlockchainService(),
		app.services.WalletService(),
		app.services.ProcessingService(),
		app.services.TransactionService(),
		app.services.JobLogger(),
	)

	withInternalAPI := app.config.Oxygen.Server.EnableInternalAPI

	srv := httpServer.New(
		app.config.Oxygen.Server,
		app.config.Debug,
		httpServer.WithRecover(),
		httpServer.WithLogger(app.logger),
		httpServer.WithMerchantAPI(merchantAPIHandler, app.services.TokenManagerService()),
		httpServer.WithDashboardAPI(
			app.config.Oxygen.Server,
			merchantAPIHandler,
			dashboardAuthHandler,
			app.services.TokenManagerService(),
			app.services.UserService(),
			app.config.Oxygen.Auth.Email.Enabled,
			app.config.Oxygen.Auth.Google.Enabled,
		),
		httpServer.WithPaymentAPI(paymentAPIHandler, app.config.Oxygen.Server),
		httpServer.WithWebhookAPI(incomingWebhooksHandler),
		httpServer.When(
			app.config.EmbedFrontend,
			httpServer.WithEmbeddedFrontend(uidashboard.Files(), uipayment.Files()),
		),
		httpServer.When(withInternalAPI, httpServer.WithInternalAPI(
			internalapi.New(
				app.services.WalletService(),
				app.services.BlockchainService(),
				schedulerHandler,
				app.logger,
			),
		)),
		httpServer.When(withInternalAPI, httpServer.WithDocs(web.SwaggerFiles())),
		httpServer.When(withInternalAPI, httpServer.WithAuthDebug(web.AuthDebugFiles())),
	)

	app.registerEventHandlers()

	app.OnBeforeRun(func(ctx context.Context, app *App) error {
		cfg := app.config.Oxygen.Auth.Email
		if !cfg.Enabled {
			return nil
		}

		u, err := app.services.UserService().Register(ctx, cfg.FirstUserEmail, cfg.FirstUserPass)
		switch {
		case errors.Is(err, user.ErrAlreadyExists):
			app.Logger().Info().Msg("Skipped user registration from config: already exists")
			return nil
		case err != nil:
			return errors.Wrapf(err, "unable to create user %q from config", cfg.FirstUserEmail)
		}

		app.Logger().Info().Str("email", u.Email).Msg("Registered user from config")

		return nil
	})

	for _, fn := range app.beforeRun {
		if err := fn(app.ctx, app); err != nil {
			app.logger.Fatal().Err(err).Msg("error while running onBeforeRun")
			return
		}
	}

	go func() {
		app.logger.Info().Str("address", srv.Address()).Msg("starting http server")
		if err := srv.Run(); err != nil && err != http.ErrServerClosed {
			app.logger.Error().Err(err).Msg("unable to run http server")
			graceful.ShutdownNow()
		}
	}()

	graceful.AddCallback(func() error {
		app.logger.Info().Msg("shutting down http server")
		return srv.Shutdown(app.ctx)
	})

	graceful.AddCallback(app.services.EventBus().Shutdown)
}

func (app *App) RunScheduler() {
	logger := app.logger.With().Str("channel", "scheduler").Logger()

	register := makeCron(app.ctx, &logger, app.services.JobLogger())

	jobs := scheduler.New(
		app.services.PaymentService(),
		app.services.BlockchainService(),
		app.services.WalletService(),
		app.services.ProcessingService(),
		app.services.TransactionService(),
		app.services.JobLogger(),
	)

	app.registerEventHandlers()

	register("@every 30s", "checkIncomingTransactionsProgress", jobs.CheckIncomingTransactionsProgress, false)

	register("@every 10m", "performInternalWalletTransfer", jobs.PerformInternalWalletTransfer, true)
	register("@every 2m", "checkInternalTransferProgress", jobs.CheckInternalTransferProgress, false)

	register("@every 10m", "performWithdrawalsCreation", jobs.PerformWithdrawalsCreation, true)
	register("@every 2m", "checkWithdrawalsProgress", jobs.CheckWithdrawalsProgress, false)

	register("@every 2m", "cancelExpiredPayments", jobs.CancelExpiredPayments, false)
}

func (app *App) registerEventHandlers() {
	handlers := []bus.Handler{
		paymentevents.New(
			app.services.MerchantService(),
			app.services.ProcessingService(),
			app.services.PaymentService(),
			app.config.Notifications.SlackWebhookURL,
			app.logger,
		),
		userevents.New(
			app.config.Env,
			app.config.Notifications.SlackWebhookURL,
			app.services.UserService(),
			app.logger,
		),
	}

	for _, h := range handlers {
		if err := app.services.EventBus().RegisterHandler(h); err != nil {
			panic(errors.Wrapf(err, "unable to register handler %T", h))
		}
	}
}

type registerFunc func(cronSpec, name string, job jobFunc, enableTableLogging bool)

type jobFunc func(ctx context.Context) error

func makeCron(ctx context.Context, stdoutLogger *zerolog.Logger, jobLogger *log.JobLogger) registerFunc {
	crontab := cron.New(cron.WithLocation(time.UTC), cron.WithSeconds())
	crontab.Start()

	graceful.AddCallback(func() error {
		stdoutLogger.Info().Msg("stopping scheduler...")
		crontab.Stop()

		return nil
	})

	return func(cronSpec, name string, job jobFunc, enableTableLogging bool) {
		// 1. Setup logger & context
		jobID := fmt.Sprintf("%s-%d", name, time.Now().UTC().Unix())

		stdoutLogger := stdoutLogger.With().
			Str("cron_spec", cronSpec).
			Str("job", name).
			Str("job_id", jobID).
			Logger()

		ctx = stdoutLogger.WithContext(ctx)
		ctx = context.WithValue(ctx, scheduler.ContextJobID{}, jobID)
		ctx = context.WithValue(ctx, scheduler.ContextJobEnableTableLogger{}, enableTableLogging)

		jobLog := func(level log.Level, message string, meta map[string]any) {
			if enableTableLogging {
				jobLogger.Log(ctx, level, jobID, message, meta)
			}
		}

		// 2. Register function
		_, errRegister := crontab.AddFunc(cronSpec, func() {
			start := time.Now()
			withMeta := func(err error) map[string]any {
				errorMsg := ""
				if err != nil {
					errorMsg = err.Error()
				}

				return map[string]any{
					"time_taken": timeTaken(start),
					"error":      errorMsg,
				}
			}

			jobLog(log.Info, "starting job", nil)

			defer func() {
				if err := recover(); err != nil {
					stdoutLogger.Error().Interface("panic", err).Msg("job failed (panic!)")
					jobLog(log.Error, "job failed (panic!)", withMeta(nil))
				}
			}()

			if err := job(ctx); err != nil {
				stdoutLogger.Err(err).Msg("job failed")
				jobLog(log.Error, "job failed", withMeta(err))

				return
			}

			stdoutLogger.Info().Msg("job completed")
			jobLog(log.Error, "job completed", withMeta(nil))
		})

		if errRegister != nil {
			stdoutLogger.Fatal().Err(errRegister).Msg("unable to register job")
		}
	}
}

func timeTaken(since time.Time) string {
	return time.Since(since).String()
}
