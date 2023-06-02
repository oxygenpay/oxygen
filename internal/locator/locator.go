// package locator represents simple Service Locator pattern.
package locator

import (
	"context"
	"sync"

	"github.com/go-openapi/strfmt"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/oxygenpay/oxygen/internal/db/connection/pg"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/lock"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/registry"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/pkg/api-kms/v1/client"
	"github.com/oxygenpay/oxygen/pkg/graceful"
	"github.com/rs/zerolog"
)

type Locator struct {
	ctx    context.Context
	config *config.Config
	once   map[string]*sync.Once

	logger *zerolog.Logger

	// Database
	db    *pg.Connection
	repo  *repository.Queries
	store *repository.Store

	// Event
	eventBus *bus.PubSub

	// Provides
	tatumProvider    *tatum.Provider
	trongridProvider *trongrid.Provider

	// Clients
	kmsClient *client.KMSInternalAPI

	// Services
	registryService    *registry.Service
	blockchainService  *blockchain.Service
	userService        *user.Service
	locker             *lock.Locker
	merchantService    *merchant.Service
	tokenManager       *auth.TokenAuthManager
	googleAuth         *auth.GoogleOAuthManager
	transactionService *transaction.Service
	paymentService     *payment.Service
	walletService      *wallet.Service
	processingService  *processing.Service
	jobLogger          *log.JobLogger
}

func New(ctx context.Context, cfg *config.Config, logger *zerolog.Logger) *Locator {
	return &Locator{
		config: cfg,
		ctx:    ctx,
		logger: logger,
		once:   make(map[string]*sync.Once, 128),
	}
}

func (loc *Locator) DB() *pg.Connection {
	loc.init("db", func() {
		db, err := pg.Open(loc.ctx, loc.config.Oxygen.Postgres, loc.logger)
		if err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to open pg database")
			return
		}

		if err := db.Ping(loc.ctx); err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to ping postgres")
			return
		}

		loc.db = db

		graceful.AddCallback(db.Shutdown)
	})

	return loc.db
}

func (loc *Locator) Repository() *repository.Queries {
	loc.init("repo", func() {
		loc.repo = repository.New(loc.DB())
	})

	return loc.repo
}

func (loc *Locator) Store() *repository.Store {
	loc.init("store", func() {
		loc.store = repository.NewStore(loc.DB())
	})

	return loc.store
}

func (loc *Locator) EventBus() *bus.PubSub {
	loc.init("event.bus", func() {
		loc.eventBus = bus.NewPubSub(loc.ctx, true, loc.logger)
	})

	return loc.eventBus
}

func (loc *Locator) Locker() *lock.Locker {
	loc.init("locker", func() {
		loc.locker = lock.New(loc.Store())
	})

	return loc.locker
}

func (loc *Locator) TatumProvider() *tatum.Provider {
	loc.init("provider.tatum", func() {
		loc.tatumProvider = tatum.New(loc.config.Providers.Tatum, loc.RegistryService(), loc.logger)
	})

	return loc.tatumProvider
}

func (loc *Locator) TrongridProvider() *trongrid.Provider {
	loc.init("provider.trongrid", func() {
		loc.trongridProvider = trongrid.New(loc.config.Providers.Trongrid, loc.logger)
	})

	return loc.trongridProvider
}

func (loc *Locator) KMSClient() *client.KMSInternalAPI {
	loc.init("client.kms", func() {
		kms := client.NewHTTPClientWithConfig(strfmt.Default, &client.TransportConfig{
			Host:     loc.config.Providers.KmsClient.Host,
			BasePath: loc.config.Providers.KmsClient.BasePath,
			Schemes:  loc.config.Providers.KmsClient.Schemes,
		})

		// transport wrapper
		kms.SetTransport(log.ClientTransport(kms.Transport))

		loc.kmsClient = kms
	})

	return loc.kmsClient
}

func (loc *Locator) RegistryService() *registry.Service {
	loc.init("service.registry", func() {
		loc.registryService = registry.New(loc.Repository(), loc.logger)
	})

	return loc.registryService
}

func (loc *Locator) BlockchainService() *blockchain.Service {
	loc.init("service.blockchain", func() {
		currencies := blockchain.NewCurrencies()
		if err := blockchain.DefaultSetup(currencies); err != nil {
			loc.logger.Fatal().Err(err).Msg("unable to setup currencies")
		}

		loc.blockchainService = blockchain.New(
			currencies,
			blockchain.Providers{
				Tatum:    loc.TatumProvider(),
				Trongrid: loc.TrongridProvider(),
			},
			true,
			loc.logger,
		)
	})

	return loc.blockchainService
}

func (loc *Locator) UserService() *user.Service {
	loc.init("service.user", func() {
		loc.userService = user.New(loc.Store(), loc.EventBus(), loc.RegistryService(), loc.logger)
	})

	return loc.userService
}

func (loc *Locator) MerchantService() *merchant.Service {
	loc.init("service.merchant", func() {
		loc.merchantService = merchant.New(loc.Repository(), loc.BlockchainService(), loc.logger)
	})

	return loc.merchantService
}

func (loc *Locator) TokenManagerService() *auth.TokenAuthManager {
	loc.init("service.tokenManager", func() {
		loc.tokenManager = auth.NewTokenAuth(loc.Repository(), loc.logger)
	})

	return loc.tokenManager
}

func (loc *Locator) GoogleAuth() *auth.GoogleOAuthManager {
	loc.init("service.auth.google", func() {
		loc.googleAuth = auth.NewGoogleOAuth(loc.config.Oxygen.Auth.Google, loc.logger)
	})

	return loc.googleAuth
}

func (loc *Locator) TransactionService() *transaction.Service {
	loc.init("service.transaction", func() {
		loc.transactionService = transaction.New(
			loc.Store(),
			loc.BlockchainService(),
			loc.WalletService(),
			loc.logger,
		)
	})

	return loc.transactionService
}

func (loc *Locator) PaymentService() *payment.Service {
	loc.init("service.payment", func() {
		loc.paymentService = payment.New(
			loc.Repository(),
			loc.config.Oxygen.Processing.PaymentFrontendPath(),
			loc.TransactionService(),
			loc.MerchantService(),
			loc.WalletService(),
			loc.BlockchainService(),
			loc.EventBus(),
			loc.logger,
		)
	})

	return loc.paymentService
}

func (loc *Locator) WalletService() *wallet.Service {
	loc.init("service.wallet", func() {
		loc.walletService = wallet.New(loc.KMSClient().Wallet, loc.BlockchainService(), loc.Store(), loc.logger)
	})

	return loc.walletService
}

func (loc *Locator) ProcessingService() *processing.Service {
	loc.init("service.processing", func() {
		loc.processingService = processing.New(
			loc.config.Oxygen.Processing,
			loc.WalletService(),
			loc.MerchantService(),
			loc.PaymentService(),
			loc.TransactionService(),
			loc.BlockchainService(),
			loc.TatumProvider(),
			loc.EventBus(),
			loc.Locker(),
			loc.logger,
		)
	})

	return loc.processingService
}

func (loc *Locator) JobLogger() *log.JobLogger {
	loc.init("service.jogLogger", func() {
		loc.jobLogger = log.NewJobLogger(loc.Store())
	})

	return loc.jobLogger
}

func (loc *Locator) init(key string, f func()) {
	if loc.once[key] == nil {
		loc.once[key] = &sync.Once{}
	}

	loc.once[key].Do(f)
}
