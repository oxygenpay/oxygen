package merchantapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	queryParamFrom   = "from"
	queryParamTo     = "to"
	queryParamAmount = "amount"
)

func (h *Handler) GetCurrencyConvert(c echo.Context) error {
	ctx := c.Request().Context()

	from := c.QueryParam(queryParamFrom)
	to := c.QueryParam(queryParamTo)
	amount := c.QueryParam(queryParamAmount)

	conv, err := h.blockchain.Convert(ctx, from, to, amount)
	switch {
	case errors.Is(err, blockchain.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case errors.Is(err, blockchain.ErrCurrencyNotFound):
		return common.ValidationErrorResponse(c, "unsupported currency")
	case err != nil:
		return errors.Wrapf(err, "unable to perform conversion from %q to %q", from, to)
	}

	return c.JSON(http.StatusOK, conversionToResponse(conv))
}

func conversionToResponse(conv blockchain.Conversion) *model.Conversion {
	return &model.Conversion{
		From:            conv.From.Ticker(),
		FromType:        string(conv.From.Type()),
		To:              conv.To.Ticker(),
		ToType:          string(conv.To.Type()),
		SelectedAmount:  conv.From.String(),
		ExchangeRate:    conv.Rate,
		ConvertedAmount: conv.To.String(),
	}
}
