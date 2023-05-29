package paymentapi

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-payment/v1/model"
	"github.com/pkg/errors"
)

const ParamPaymentID = "paymentId"

const (
	queryParamFiatCurrency   = "fiatCurrency"
	queryParamFiatAmount     = "fiatAmount"
	queryParamCryptoCurrency = "cryptoCurrency"
)

func (h *Handler) GetPayment(c echo.Context) error {
	ctx := c.Request().Context()

	pt, err := middleware.ResolvePayment(c)
	if err != nil {
		return err
	}

	detailedPayment, err := h.processing.GetDetailedPayment(ctx, pt.MerchantID, pt.ID)
	if err != nil {
		return err
	}

	price, err := detailedPayment.Payment.Price.FiatToFloat64()
	if err != nil {
		return err
	}

	response := &model.Payment{
		ID:           detailedPayment.Payment.PublicID.String(),
		Currency:     detailedPayment.Payment.Price.Ticker(),
		Price:        price,
		IsLocked:     !detailedPayment.Payment.IsEditable(),
		MerchantName: detailedPayment.Merchant.Name,
		Description:  detailedPayment.Payment.Description,
	}

	if detailedPayment.Customer != nil {
		response.Customer = customerToResponse(detailedPayment.Customer)
	}

	if detailedPayment.PaymentMethod != nil {
		response.PaymentMethod = paymentMethodToResponse(detailedPayment.PaymentMethod)
	}

	if detailedPayment.PaymentInfo != nil {
		response.PaymentInfo = paymentInfoToResponse(detailedPayment.PaymentInfo)
	}

	return c.JSON(http.StatusOK, response)
}

// CreateCustomer upserts customer for the payment.
func (h *Handler) CreateCustomer(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateCustomerRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	p, err := middleware.ResolvePayment(c)
	if err != nil {
		return err
	}

	person, err := h.payments.AssignCustomerByEmail(ctx, p, req.Email)

	switch {
	case errors.Is(err, payment.ErrValidation):
		return common.ValidationErrorItemResponse(c, "email", "invalid email provided")
	case errors.Is(err, payment.ErrPaymentLocked):
		return common.ValidationErrorResponse(c, payment.ErrPaymentLocked)
	case err != nil:
		h.logger.
			Warn().Err(err).
			Int64(ParamPaymentID, p.ID).
			Msg("unable to assign customer by email")

		return common.ErrorResponse(c, common.StatusInternalError)
	}

	return c.JSON(http.StatusCreated, customerToResponse(person))
}

//nolint:gocritic
func (h *Handler) GetSupportedMethods(c echo.Context) error {
	ctx := c.Request().Context()

	p, err := middleware.ResolvePayment(c)
	if err != nil {
		return err
	}

	mt, err := h.merchants.GetByID(ctx, p.MerchantID, false)
	if err != nil {
		return err
	}

	currencies, err := h.merchants.ListSupportedCurrencies(ctx, mt)
	if err != nil {
		return err
	}

	availableOnly := util.FilterSlice(
		currencies,
		func(sc merchant.SupportedCurrency) bool { return sc.Enabled },
	)

	return c.JSON(http.StatusOK, &model.SupportedPaymentMethods{
		AvailableMethods: util.MapSlice(availableOnly, func(sc merchant.SupportedCurrency) *model.SupportedPaymentMethod {
			return &model.SupportedPaymentMethod{
				Blockchain:     sc.Currency.Blockchain.String(),
				BlockchainName: sc.Currency.BlockchainName,
				DisplayName:    sc.Currency.DisplayName(),
				Name:           sc.Currency.Name,
				Ticker:         sc.Currency.Ticker,
			}
		}),
	})
}

func (h *Handler) CreatePaymentMethod(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreatePaymentMethodRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	p, err := middleware.ResolvePayment(c)
	if err != nil {
		return err
	}

	method, err := h.processing.SetPaymentMethod(ctx, p, req.Ticker)

	switch {
	case errors.Is(err, blockchain.ErrCurrencyNotFound):
		return common.ValidationErrorResponse(c, errors.New("invalid ticker"))
	case errors.Is(err, processing.ErrStatusInvalid):
		return common.ValidationErrorResponse(c, errors.New("payment method is already set and locked"))
	case err != nil:
		h.logger.
			Error().Err(err).
			Int64("payment_id", p.ID).Str("ticker", req.Ticker).
			Msg("unable to set payment method")

		return common.ErrorResponse(c, common.StatusInternalError)
	}

	return c.JSON(http.StatusCreated, paymentMethodToResponse(method))
}

func (h *Handler) LockPaymentOptions(c echo.Context) error {
	ctx := c.Request().Context()

	p, err := middleware.ResolvePayment(c)
	if err != nil {
		return err
	}

	err = h.processing.LockPaymentOptions(ctx, p.MerchantID, p.ID)

	switch {
	case errors.Is(err, processing.ErrPaymentOptionsMissing):
		return common.ValidationErrorResponse(c, processing.ErrPaymentOptionsMissing)
	case err != nil:
		h.logger.
			Warn().Err(err).
			Int64("payment_id", p.ID).
			Msg("unable to lock payment options")

		return common.ErrorResponse(c, common.StatusInternalError)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) GetExchangeRate(c echo.Context) error {
	from := c.QueryParam(queryParamFiatCurrency)
	amount := c.QueryParam(queryParamFiatAmount)
	to := c.QueryParam(queryParamCryptoCurrency)

	conv, err := h.blockchain.Convert(c.Request().Context(), from, to, amount)

	switch {
	case errors.Is(err, blockchain.ErrValidation):
		return common.ValidationErrorResponse(c, err)
	case errors.Is(err, blockchain.ErrCurrencyNotFound):
		return common.ValidationErrorResponse(c, "unsupported currency")
	case err != nil:
		return errors.Wrapf(err, "unable to perform conversion from %q to %q", from, to)
	case conv.Type != blockchain.ConversionTypeFiatToCrypto:
		return common.ValidationErrorResponse(c, "invalid fiat currency")
	}

	fiatAmount, _ := conv.From.FiatToFloat64()
	crypto, _ := h.blockchain.GetCurrencyByTicker(to)

	return c.JSON(http.StatusOK, &model.CurrencyExchangeRate{
		FiatCurrency: conv.From.Ticker(),
		FiatAmount:   fiatAmount,

		CryptoCurrency: conv.To.Ticker(),
		CryptoAmount:   conv.To.String(),
		Network:        crypto.BlockchainName,
		DisplayName:    crypto.DisplayName(),

		ExchangeRate: conv.Rate,
	})
}

func customerToResponse(c *payment.Customer) *model.Customer {
	return &model.Customer{
		Email: c.Email,
		ID:    c.UUID.String(),
	}
}

func paymentMethodToResponse(m *payment.Method) *model.PaymentMethod {
	return &model.PaymentMethod{
		Blockchain:     m.Currency.Blockchain.String(),
		BlockchainName: m.Currency.BlockchainName,
		DisplayName:    m.Currency.DisplayName(),
		Name:           m.Currency.Name,
		Ticker:         m.Currency.Ticker,
		NetworkID:      m.NetworkID,
		IsTest:         m.IsTest,
	}
}

func paymentInfoToResponse(i *processing.PaymentInfo) *model.PaymentInfo {
	var successAction *string
	if i.SuccessAction != nil {
		successAction = (*string)(i.SuccessAction)
	}

	return &model.PaymentInfo{
		Status:           i.Status.String(),
		RecipientAddress: i.RecipientAddress,
		PaymentLink:      i.PaymentLink,

		Amount:          i.Amount,
		AmountFormatted: i.AmountFormatted,

		ExpiresAt:             strfmt.DateTime(i.ExpiresAt),
		ExpirationDurationMin: i.ExpirationDurationMin,

		SuccessAction:  successAction,
		SuccessURL:     i.SuccessURL,
		SuccessMessage: i.SuccessMessage,
	}
}
