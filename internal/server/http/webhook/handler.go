package webhook

import (
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/rs/zerolog"
)

type Handler struct {
	processing *processing.Service
	logger     *zerolog.Logger
}

const (
	paramWalletID   = "walletId"
	paramNetworkID  = "networkId"
	headerTatumHMAC = "x-payload-hash"
)

func New(processingService *processing.Service, logger *zerolog.Logger) *Handler {
	log := logger.With().Str("channel", "webhook").Logger()

	return &Handler{
		processing: processingService,
		logger:     &log,
	}
}
