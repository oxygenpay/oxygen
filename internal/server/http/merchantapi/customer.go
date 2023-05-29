package merchantapi

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	paramCustomerID = "customerId"
)

func (h *Handler) ListCustomers(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	pagination, err := common.QueryPagination(c)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	customers, nextCursor, err := h.payments.ListCustomers(ctx, mt.ID, payment.ListOptions{
		Limit:        pagination.Limit,
		Cursor:       pagination.Cursor,
		ReverseOrder: pagination.ReverseSort,
	})

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, "invalid query")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, &model.CustomersPagination{
		Cursor:  nextCursor,
		Limit:   int64(pagination.Limit),
		Results: util.MapSlice(customers, customerToResponse),
	})
}

func (h *Handler) GetCustomerDetails(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	id, err := common.UUID(c, paramCustomerID)
	if err != nil {
		return err
	}

	customerDetails, err := h.payments.GetCustomerDetailsByUUID(ctx, mt.ID, id)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "customer not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, customerDetailsToResponse(customerDetails))
}

func customerToResponse(c *payment.Customer) *model.Customer {
	return &model.Customer{
		CreatedAt: strfmt.DateTime(c.CreatedAt),
		Email:     c.Email,
		ID:        c.UUID.String(),
	}
}

func customerDetailsToResponse(ct *payment.CustomerDetails) *model.Customer {
	customer := customerToResponse(&ct.Customer)

	mapPayments := func(pt *payment.Payment) *model.CustomerPayment {
		return &model.CustomerPayment{
			ID:        pt.MerchantOrderUUID.String(),
			CreatedAt: strfmt.DateTime(pt.CreatedAt),
			Currency:  pt.Price.Ticker(),
			Price:     pt.Price.String(),
			Status:    pt.PublicStatus().String(),
		}
	}

	customer.Details = &model.CustomerDetails{
		SuccessfulPayments: ct.SuccessfulPayments,
		Payments:           util.MapSlice(ct.RecentPayments, mapPayments),
	}

	return customer
}
