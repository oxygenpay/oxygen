package blockchain

import (
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	client "github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	ErrValidation         = errors.New("invalid data provided")
	ErrCurrencyNotFound   = errors.New("currency not found")
	ErrNoTokenAddress     = errors.New("token should have contract address filled")
	ErrParseMoney         = errors.New("unable to parse money value")
	ErrInsufficientFunds  = errors.New("wallet has insufficient funds")
	ErrInvalidTransaction = errors.New("transaction is invalid")
)

type Providers struct {
	Tatum    *tatum.Provider
	Trongrid *trongrid.Provider
}

type Service struct {
	*CurrencyResolver
	providers Providers
	logger    *zerolog.Logger

	ratesCache *ttlcache.Cache[string, client.ExchangeRate]
}

const exchangeRateCacheTTL = time.Second * 30

func New(currencies *CurrencyResolver, providers Providers, enableCache bool, logger *zerolog.Logger) *Service {
	log := logger.With().Str("channel", "blockchain_service").Logger()

	s := &Service{
		CurrencyResolver: currencies,
		providers:        providers,
		logger:           &log,
	}

	// Cache for storing exchange rates
	// I know that it looks a bit of spaghetti but atm we have only one exchange provider,
	// so lets KISS and then refactor to more generic approach
	if enableCache {
		withTTL := ttlcache.WithTTL[string, client.ExchangeRate](exchangeRateCacheTTL)
		s.ratesCache = ttlcache.New[string, client.ExchangeRate](withTTL)

		go s.ratesCache.Start()
	}

	return s
}
