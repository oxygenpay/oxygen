package internalapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/scheduler"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Broadcaster
	blockchain.FeeCalculator
}

type Handler struct {
	wallet     *wallet.Service
	blockchain BlockchainService
	scheduler  *scheduler.Handler
	logger     *zerolog.Logger
}

func New(
	walletService *wallet.Service,
	blockchainService BlockchainService,
	schedulerHandler *scheduler.Handler,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "admin_api").Logger()

	return &Handler{
		wallet:     walletService,
		blockchain: blockchainService,
		scheduler:  schedulerHandler,
		logger:     &log,
	}
}

var selectedMethods = map[string]struct{}{
	http.MethodGet:    {},
	http.MethodPost:   {},
	http.MethodPut:    {},
	http.MethodPatch:  {},
	http.MethodDelete: {},
}

// GetRouter dumps all routes routes.
func (h *Handler) GetRouter(c echo.Context) error {
	paths := make(map[string][]string)

	for _, r := range c.Echo().Routes() {
		if _, exists := selectedMethods[r.Method]; exists {
			paths[r.Path] = append(paths[r.Path], r.Method)
		}
	}

	formatted := map[string]string{}
	for path, methods := range paths {
		sort.Strings(methods)
		formatted[path] = strings.Join(methods, ", ")
	}

	return c.JSON(http.StatusOK, formatted)
}
