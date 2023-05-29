package merchantapi

import (
	"net/http"
	"strconv"

	apierrors "github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	queryParamBalanceID = "balanceId"
)

func (h *Handler) CreateWithdrawal(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateWithdrawalRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	props, err := parseWithdrawalRequest(req)
	if err != nil {
		return common.ValidationErrorResponse(c, err)
	}

	mt := middleware.ResolveMerchant(c)

	p, err := h.payments.CreateWithdrawal(ctx, mt.ID, props)

	switch {
	case errors.Is(err, merchant.ErrAddressNotFound):
		return common.ValidationErrorItemResponse(c, "addressId", "address not found")
	case errors.Is(err, wallet.ErrBalanceNotFound):
		return common.ValidationErrorItemResponse(c, "balanceId", "balance not found")
	case errors.Is(err, payment.ErrAddressBalanceMismatch):
		return common.ValidationErrorItemResponse(c, "balanceId", "balance does not match to address")
	case errors.Is(err, money.ErrParse):
		return common.ValidationErrorItemResponse(c, "amount", "withdrawal amount is invalid")
	case errors.Is(err, payment.ErrWithdrawalInsufficientBalance):
		return common.ValidationErrorItemResponse(c, "amount", err.Error())
	case errors.Is(err, payment.ErrWithdrawalAmountTooSmall):
		return common.ValidationErrorItemResponse(c, "amount", err.Error())
	case err != nil:
		return err
	}

	return c.JSON(http.StatusCreated, &model.Withdrawal{PaymentID: p.MerchantOrderUUID.String()})
}

func (h *Handler) GetWithdrawalFee(c echo.Context) error {
	ctx := c.Request().Context()

	balanceID, err := common.UUID(c, queryParamBalanceID)
	if err != nil {
		return nil
	}

	mt := middleware.ResolveMerchant(c)

	fee, err := h.payments.GetWithdrawalFee(ctx, mt.ID, balanceID)

	switch {
	case errors.Is(err, wallet.ErrBalanceNotFound):
		return common.ValidationErrorItemResponse(c, "balanceId", "balance not found")
	case err != nil:
		return errors.Wrap(err, "unable to get withdrawal fee")
	}

	return c.JSON(http.StatusOK, withdrawalFeeToResponse(fee))
}

func parseWithdrawalRequest(req model.CreateWithdrawalRequest) (payment.CreateWithdrawalProps, error) {
	errComposite := &apierrors.CompositeError{}

	balanceID, err := uuid.Parse(req.BalanceID)
	if err != nil {
		errComposite.Errors = append(errComposite.Errors, common.WrapErrorItem(&model.ErrorResponseItem{
			Field:   "balanceId",
			Message: "invalid uuid",
		}))
	}

	addressID, err := uuid.Parse(req.AddressID)
	if err != nil {
		errComposite.Errors = append(errComposite.Errors, common.WrapErrorItem(&model.ErrorResponseItem{
			Field:   "addressId",
			Message: "invalid uuid",
		}))
	}

	amount := req.Amount
	if amountAsFloat, err := strconv.ParseFloat(req.Amount, 64); err != nil || amountAsFloat <= 0 {
		errComposite.Errors = append(errComposite.Errors, common.WrapErrorItem(&model.ErrorResponseItem{
			Field:   "amount",
			Message: "invalid amount",
		}))
	}

	if len(errComposite.Errors) > 0 {
		return payment.CreateWithdrawalProps{}, errComposite
	}

	return payment.CreateWithdrawalProps{
		BalanceID: balanceID,
		AddressID: addressID,
		AmountRaw: amount,
	}, nil
}

func withdrawalFeeToResponse(fee *payment.WithdrawalFee) *model.WithdrawalFee {
	usdFee := "0"
	if !fee.IsTest {
		usdFee = fee.USDFee.String()
	}

	return &model.WithdrawalFee{
		Blockchain:   fee.Blockchain.String(),
		CalculatedAt: strfmt.DateTime(fee.CalculatedAt),
		Currency:     fee.Currency,
		UsdFee:       usdFee,
		CurrencyFee:  fee.CryptoFee.String(),
		IsTest:       fee.IsTest,
	}
}
