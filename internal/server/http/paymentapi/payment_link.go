package paymentapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/pkg/api-payment/v1/model"
	"github.com/pkg/errors"
)

const paramPaymentLinkSlug = "paymentLinkSlug"

func (h *Handler) GetPaymentLink(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param(paramPaymentLinkSlug)

	link, err := h.payments.GetPaymentLinkBySlug(ctx, slug)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment link not found")
	case err != nil:
		return err
	}

	mt, err := h.merchants.GetByID(ctx, link.MerchantID, false)
	if err != nil {
		return err
	}

	price, err := link.Price.FiatToFloat64()
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &model.PaymentLink{
		MerchantName: mt.Name,
		Currency:     link.Price.Ticker(),
		Price:        price,
		Description:  link.Description,
	})
}

func (h *Handler) CreatePaymentFromLink(c echo.Context) error {
	ctx := c.Request().Context()
	slug := c.Param(paramPaymentLinkSlug)

	link, err := h.payments.GetPaymentLinkBySlug(ctx, slug)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment link not found")
	case err != nil:
		return err
	}

	pt, err := h.payments.CreatePaymentFromLink(ctx, link)
	if err != nil {
		return errors.Wrap(err, "unable to create payment from link")
	}

	return c.JSON(http.StatusCreated, &model.PaymentRedirectInfo{ID: pt.PublicID.String()})
}
