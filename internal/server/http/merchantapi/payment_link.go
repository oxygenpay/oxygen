package merchantapi

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const paramPaymentLinkID = "paymentLinkId"

func (h *Handler) ListPaymentLinks(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	links, err := h.payments.ListPaymentLinks(ctx, mt.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, &model.PaymentLinksPagination{
		Results: util.MapSlice(links, linkToResponse),
	})
}

func (h *Handler) GetPaymentLink(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	id, err := common.UUID(c, paramPaymentLinkID)
	if err != nil {
		return nil
	}

	link, err := h.payments.GetPaymentLinkByPublicID(ctx, mt.ID, id)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment link not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, linkToResponse(link))
}

func (h *Handler) DeletePaymentLink(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	id, err := common.UUID(c, paramPaymentLinkID)
	if err != nil {
		return nil
	}

	err = h.payments.DeletePaymentLinkByPublicID(ctx, mt.ID, id)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment link not found")
	case err != nil:
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) CreatePaymentLink(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreatePaymentLinkRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	currency, err := money.MakeFiatCurrency(req.Currency)
	if err != nil {
		return common.ValidationErrorItemResponse(c, "currency", "invalid currency")
	}

	price, err := money.FiatFromFloat64(currency, req.Price)
	if err != nil {
		return common.ValidationErrorItemResponse(c, "price", "price should be between %.2f and %.0f", money.FiatMin, money.FiatMax)
	}

	mt := middleware.ResolveMerchant(c)

	link, err := h.payments.CreatePaymentLink(ctx, mt.ID, payment.CreateLinkProps{
		Name:           req.Name,
		Price:          price,
		Description:    req.Description,
		SuccessAction:  payment.SuccessAction(req.SuccessAction),
		RedirectURL:    req.RedirectURL,
		SuccessMessage: req.SuccessMessage,
		IsTest:         false,
	})

	switch {
	case errors.Is(err, payment.ErrLinkValidation):
		return common.ValidationErrorResponse(c, err.Error())
	case err != nil:
		return err
	}

	return c.JSON(http.StatusCreated, linkToResponse(link))
}

func linkToResponse(link *payment.Link) *model.PaymentLink {
	return &model.PaymentLink{
		ID:        link.PublicID.String(),
		CreatedAt: strfmt.DateTime(link.CreatedAt),
		URL:       link.URL,

		Name:        link.Name,
		Description: link.Description,

		Currency: link.Price.Ticker(),
		Price:    link.Price.String(),

		SuccessAction:  string(link.SuccessAction),
		RedirectURL:    link.RedirectURL,
		SuccessMessage: link.SuccessMessage,
	}
}
