package webhook

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/pkg/errors"
)

func (h *Handler) ReceiveTatum(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. Parse request params
	networkID := c.Param(paramNetworkID)

	walletID, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	// 2. Verify signature
	signature := c.Request().Header.Get(headerTatumHMAC)
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}

	if err := h.processing.ValidateWebhookSignature(body, signature); err != nil {
		h.logger.Error().Err(err).
			EmbedObject(log.Ctx(ctx)).
			Str("body", string(body)).
			Str("signature", signature).
			Msg("invalid signature")

		return common.ValidationErrorResponse(c, errors.New("invalid signature"))
	}

	// 3. Parse request
	var req processing.TatumWebhook
	if err := json.Unmarshal(body, &req); err != nil {
		return err
	}

	// 4. Process incoming webhook
	if err := h.processing.ProcessIncomingWebhook(ctx, walletID, networkID, req); err != nil {
		h.logger.Error().Err(err).
			Str("wallet_id", walletID.String()).Interface("webhook", req).
			Msg("unable to process tatum webhook")

		return c.JSON(http.StatusBadRequest, "unable to process tatum webhook")
	}

	h.logger.Info().
		Str("wallet_id", walletID.String()).
		Interface("webhook", req).
		Msg("processed incoming tatum webhook")

	return c.NoContent(http.StatusNoContent)
}
