package merchantapi

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	paramPaymentID = "paymentId"
	queryParamType = "type"
)

func (h *Handler) ListPayments(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	pagination, err := common.QueryPagination(c)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	var filterByType []payment.Type

	ptType := payment.Type(c.QueryParam(queryParamType))
	if ptType != "" {
		if ptType == payment.TypePayment || ptType == payment.TypeWithdrawal {
			filterByType = append(filterByType, ptType)
		} else {
			return common.ValidationErrorItemResponse(c, "type", "unknown type %q", ptType)
		}
	}

	payments, nextCursor, err := h.payments.ListWithRelations(ctx, mt.ID, payment.ListOptions{
		Limit:        pagination.Limit,
		Cursor:       pagination.Cursor,
		ReverseOrder: pagination.ReverseSort,
		FilterByType: filterByType,
	})

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, "invalid query")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, &model.PaymentsPagination{
		Cursor:  nextCursor,
		Limit:   int64(pagination.Limit),
		Results: util.MapSlice(payments, paymentToResponse),
	})
}

func (h *Handler) GetPayment(c echo.Context) error {
	ctx := c.Request().Context()

	paymentID, err := uuid.Parse(c.Param(paramPaymentID))
	if err != nil {
		return common.ValidationErrorResponse(c, "invalid payment id")
	}

	mt := middleware.ResolveMerchant(c)

	pt, err := h.payments.GetByMerchantOrderIDWithRelations(ctx, mt.ID, paymentID)

	switch {
	case errors.Is(err, payment.ErrNotFound):
		return common.NotFoundResponse(c, "payment not found")
	case err != nil:
		h.logger.Error().Err(err).
			Int64("merchant_id", mt.ID).Str("payment_uuid", paymentID.String()).
			Msg("unable to get payment")

		return err
	}

	return c.JSON(http.StatusOK, paymentToResponse(pt))
}

func (h *Handler) CreatePayment(c echo.Context) error {
	var req model.CreatePaymentRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	merchantOrderUUID, err := uuid.Parse(req.ID.String())
	if err != nil {
		return common.ValidationErrorResponse(c, "order id is invalid")
	}

	currency, err := money.MakeFiatCurrency(req.Currency)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	if req.Price <= 0 {
		return common.ValidationErrorResponse(c, errors.New("price should be positive"))
	}

	price, err := money.FiatFromFloat64(currency, req.Price)
	if err != nil {
		return common.ValidationErrorItemResponse(c, "price", "price should be between %.2f and %.0f", money.FiatMin, money.FiatMax)
	}

	pt, err := h.payments.CreatePayment(ctx, mt.ID, payment.CreatePaymentProps{
		MerchantOrderUUID: merchantOrderUUID,
		MerchantOrderID:   req.OrderID,
		Money:             price,
		Description:       req.Description,
		RedirectURL:       req.RedirectURL,
		IsTest:            req.IsTest,
	})

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case errors.Is(err, payment.ErrAlreadyExists):
		return common.ValidationErrorResponse(c, err)
	case err != nil:
		h.logger.Err(err).Msg("unable to create payment")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusCreated, paymentToResponse(
		payment.PaymentWithRelations{Payment: pt}),
	)
}

func paymentToResponse(pr payment.PaymentWithRelations) *model.Payment {
	pt := pr.Payment
	tx := pr.Transaction
	customer := pr.Customer
	addr := pr.Address
	balance := pr.Balance

	res := &model.Payment{
		ID:      pt.MerchantOrderUUID.String(),
		OrderID: pt.MerchantOrderID,

		CreatedAt: strfmt.DateTime(pt.CreatedAt),

		Price:    pt.Price.String(),
		Currency: pt.Price.Ticker(),

		Status: pt.PublicStatus().String(),
		Type:   pt.Type.String(),

		PaymentURL:  pt.PaymentURL,
		RedirectURL: pt.RedirectURL,

		Description: pt.Description,
		IsTest:      pt.IsTest,
	}

	if pt.Type == payment.TypePayment {
		info := &model.AdditionalPaymentInfo{}

		if tx != nil {
			info.SelectedCurrency = util.Ptr(tx.Currency.DisplayName())
			info.ServiceFee = util.Ptr(tx.ServiceFee.String())
		}

		if customer != nil {
			info.CustomerEmail = &customer.Email
		}

		res.AdditionalInfo = &model.PaymentAdditionalInfo{Payment: info}
	}

	if pt.Type == payment.TypeWithdrawal {
		info := &model.AdditionalWithdrawalInfo{}

		if addr != nil {
			info.AddressID = addr.UUID.String()
		}
		if balance != nil {
			info.BalanceID = balance.UUID.String()
		}
		if tx != nil {
			info.ServiceFee = tx.ServiceFee.String()

			if tx.HashID != nil {
				info.TransactionHash = tx.HashID

				link, _ := tx.ExplorerLink()
				info.ExplorerLink = &link
			}
		}

		res.AdditionalInfo = &model.PaymentAdditionalInfo{Withdrawal: info}
	}

	return res
}
