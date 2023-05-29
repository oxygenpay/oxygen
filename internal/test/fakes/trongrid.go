package fakes

import (
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/rs/zerolog"
)

type Trongrid struct {
	mu                    sync.Mutex
	txRequests            map[trongrid.TransactionRequest]trongrid.Transaction
	contractCallsRequests map[trongrid.ContractCallRequest]trongrid.Transaction
}

// contractCallParameter every TRC-20 call generates unique hex-encoded ABI string.
// In order to ease testing suite let's assume that it doesn't change.
const contractCallParameter = "static-mock"

func NewTrongrid(logger *zerolog.Logger) (*trongrid.Provider, *Trongrid) {
	mock := &Trongrid{
		mu:                    sync.Mutex{},
		txRequests:            make(map[trongrid.TransactionRequest]trongrid.Transaction),
		contractCallsRequests: make(map[trongrid.ContractCallRequest]trongrid.Transaction),
	}

	e := echo.New()
	e.Use(middleware.Logger())

	e.POST("/wallet/createtransaction", mock.createTransaction)
	e.POST("/wallet/triggersmartcontract", mock.triggerSmartContract)

	srv := httptest.NewServer(e)

	cfg := trongrid.Config{
		MainnetBaseURL: srv.URL,
		TestnetBaseURL: srv.URL,
		APIKey:         "mock-api-key",
	}

	return trongrid.New(cfg, logger), mock
}

func (m *Trongrid) SetupCreateTransaction(req trongrid.TransactionRequest, res trongrid.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.txRequests[req] = res
}

func (m *Trongrid) SetupTriggerSmartContract(req trongrid.ContractCallRequest, res trongrid.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req.Parameter = contractCallParameter

	m.contractCallsRequests[req] = res
}

func (m *Trongrid) createTransaction(c echo.Context) error {
	var req trongrid.TransactionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, trongrid.Transaction{Error: "INVALID REQUEST"})
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	response, exists := m.txRequests[req]
	if !exists {
		response = trongrid.Transaction{Error: "UNEXPECTED CALL"}
	}

	return c.JSON(http.StatusOK, response)
}

func (m *Trongrid) triggerSmartContract(c echo.Context) error {
	var req trongrid.ContractCallRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusOK, trongrid.Transaction{Error: "INVALID REQUEST"})
	}

	req.Parameter = contractCallParameter

	m.mu.Lock()
	defer m.mu.Unlock()

	tx, exists := m.contractCallsRequests[req]

	if !exists {
		return c.JSON(http.StatusOK, trongrid.CallResponse{
			Result: trongrid.CallResponseResult{
				Result:  false,
				Code:    "FAIL",
				Message: "UNEXPECTED CALL",
			},
		})
	}

	return c.JSON(http.StatusOK, trongrid.CallResponse{
		Result:      trongrid.CallResponseResult{Result: true},
		Transaction: tx,
	})
}
