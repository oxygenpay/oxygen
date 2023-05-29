package paymentapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
}

type Handler struct {
	payments   *payment.Service
	merchants  *merchant.Service
	blockchain BlockchainService
	processing *processing.Service
	logger     *zerolog.Logger
}

func New(
	payments *payment.Service,
	merchants *merchant.Service,
	blockchainService BlockchainService,
	core *processing.Service,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "payment_api").Logger()

	return &Handler{
		payments:   payments,
		merchants:  merchants,
		blockchain: blockchainService,
		processing: core,
		logger:     &log,
	}
}

func (h *Handler) PaymentService() *payment.Service {
	return h.payments
}

// GetCookie sets CSRF cookie for customer's session and attaches
// X-CSRF-Token to response headers
func (h *Handler) GetCookie(c echo.Context) error {
	tokenRaw := c.Get("csrf")
	token, ok := tokenRaw.(string)
	if !ok {
		return c.JSON(http.StatusInternalServerError, "err")
	}

	c.Response().Header().Set(echo.HeaderXCSRFToken, token)
	c.Response().Header().Set(echo.HeaderAccessControlExposeHeaders, middleware.CSRFTokenHeader)

	return c.NoContent(http.StatusNoContent)
}
