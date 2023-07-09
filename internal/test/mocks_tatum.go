package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/money"
	tatumprovider "github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/registry"
	"github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/rs/zerolog"
)

const tatumMockToken = "api-key"
const tatumMockTestToken = "api-test-key"

type TatumMock struct {
	mu               sync.Mutex
	rates            map[string]map[string]float64
	wallets          map[string]string
	processedWallets map[string]struct{}
}

func NewTatum(registrySvc *registry.Service, logger *zerolog.Logger) (*tatumprovider.Provider, *TatumMock) {
	mock := &TatumMock{
		mu:               sync.Mutex{},
		rates:            map[string]map[string]float64{},
		wallets:          map[string]string{},
		processedWallets: map[string]struct{}{},
	}

	srv := echo.New()
	srv.GET("/v3/tatum/rate/:from", mock.ratesEndpoint)
	srv.POST("/v3/subscription", mock.subscriptionEndpoint)

	cfg := tatumprovider.Config{
		APIKey:       tatumMockToken,
		TestAPIKey:   tatumMockTestToken,
		BasePath:     httptest.NewServer(srv).URL,
		HMACSecret:   "",
		HMACForceSet: false,
	}

	return tatumprovider.New(cfg, registrySvc, logger), mock
}

func (m *TatumMock) SetupRates(from string, to money.FiatCurrency, rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	from = blockchain.NormalizeTicker(from)

	if m.rates[from] == nil {
		m.rates[from] = make(map[string]float64, 0)
	}

	m.rates[from][string(to)] = rate
}

func (m *TatumMock) SetupSubscription(chain, address string, isTest bool, resultID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.wallets[tatumSubKey(chain, address, isTest)] = resultID
}

func (m *TatumMock) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.wallets = map[string]string{}
	m.processedWallets = map[string]struct{}{}
}

func (m *TatumMock) ratesEndpoint(c echo.Context) error {
	from := blockchain.NormalizeTicker(c.Param("from"))
	to := c.QueryParam("basePair")

	rate, exists := m.rates[from][to]
	if !exists || rate <= 0 {
		return c.JSON(http.StatusBadRequest, "invalid")
	}

	return c.JSON(http.StatusOK, tatum.ExchangeRate{
		Id:        from,
		BasePair:  to,
		Value:     fmt.Sprintf("%f", rate),
		Timestamp: float64(time.Now().UnixMilli()),
		Source:    "MockSource",
	})
}

func (m *TatumMock) subscriptionEndpoint(c echo.Context) error {
	var req tatum.CreateSubscriptionNotification
	if err := c.Bind(&req); err != nil {
		return err
	}

	isTest := false
	if c.Request().Header.Get(tatumprovider.TokenHeader) == tatumMockTestToken {
		isTest = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check that method wasn't used yet
	key := tatumSubKey(req.Attr.Chain, req.Attr.Address, isTest)
	if _, alreadyCreated := m.processedWallets[key]; alreadyCreated {
		msg := fmt.Sprintf("SetupRates subscription for %q was already created", key)
		return c.JSON(http.StatusBadRequest, msg)
	}

	id, exists := m.wallets[key]

	if !exists {
		return c.JSON(http.StatusBadRequest, fmt.Sprintf("Unexpected call for %q", key))
	}

	m.processedWallets[key] = struct{}{}

	return c.JSON(http.StatusOK, tatumprovider.SubscriptionResponse{ID: id})
}

func tatumSubKey(chain, address string, isTest bool) string {
	return fmt.Sprintf("%s/%s/is_test:%t", chain, address, isTest)
}
