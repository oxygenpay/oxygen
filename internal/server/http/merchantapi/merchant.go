package merchantapi

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

func (h *Handler) ListMerchants(c echo.Context) error {
	ctx := c.Request().Context()
	user := middleware.ResolveUser(c)

	merchants, err := h.merchants.ListByCreatorID(ctx, user.ID)

	if err != nil {
		h.logger.Error().Err(err).Msg("unable to list merchants")
		return common.ErrorResponse(c, "internal_error")
	}

	var merchantList = make([]*model.MerchantListItem, len(merchants))
	for i, mt := range merchants {
		merchantList[i] = &model.MerchantListItem{
			ID:      mt.UUID.String(),
			Name:    mt.Name,
			Website: mt.Website,
		}
	}

	return c.JSON(http.StatusOK, &model.MerchantList{Results: merchantList})
}

func (h *Handler) CreateMerchant(c echo.Context) error {
	req := &model.CreateMerchantRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	if _, err := url.ParseRequestURI(req.Website); err != nil {
		return common.ValidationErrorItemResponse(c, "website", "website should be a valid URL (e.g. https://o2pay.co)")
	}

	ctx := c.Request().Context()
	user := middleware.ResolveUser(c)

	mt, err := h.merchants.Create(
		ctx,
		user.ID,
		req.Name,
		req.Website,
		nil,
	)

	if err != nil {
		h.logger.Error().Err(err).Msg("unable to store merchant")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.JSON(http.StatusCreated, &model.Merchant{
		ID:      mt.UUID.String(),
		Name:    mt.Name,
		Website: mt.Website,
	})
}

func (h *Handler) GetMerchant(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	methods, err := h.merchants.ListSupportedCurrencies(ctx, mt)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &model.Merchant{
		ID:      mt.UUID.String(),
		Name:    mt.Name,
		Website: mt.Website,
		WebhookSettings: &model.WebhookSettings{
			Secret: mt.Settings().WebhookSignatureSecret(),
			URL:    mt.Settings().WebhookURL(),
		},
		SupportedPaymentMethods: util.MapSlice(methods, func(sc merchant.SupportedCurrency) *model.SupportedPaymentMethod {
			return &model.SupportedPaymentMethod{
				Blockchain:     sc.Currency.Blockchain.String(),
				BlockchainName: sc.Currency.BlockchainName,
				DisplayName:    sc.Currency.DisplayName(),
				Name:           sc.Currency.Name,
				Ticker:         sc.Currency.Ticker,
				Enabled:        sc.Enabled,
			}
		}),
	})
}

func (h *Handler) UpdateMerchant(c echo.Context) error {
	req := &model.UpdateMerchantRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	if _, err := url.ParseRequestURI(req.Website); err != nil {
		return common.ValidationErrorItemResponse(c, "website", "website should be a valid URL (e.g. https://o2pay.co)")
	}

	m := middleware.ResolveMerchant(c)
	ctx := c.Request().Context()

	if _, err := h.merchants.Update(ctx, m.ID, req.Name, req.Website); err != nil {
		h.logger.Error().Err(err).Msg("merchants.UpdateMerchant")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DeleteMerchant(c echo.Context) error {
	m := middleware.ResolveMerchant(c)
	ctx := c.Request().Context()

	if err := h.merchants.DeleteByUUID(ctx, m.UUID); err != nil {
		h.logger.Error().Err(err).Msg("merchants.DeleteByUUID")
		return common.ErrorResponse(c, "internal_error")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) UpdateMerchantWebhook(c echo.Context) error {
	var req model.WebhookSettings
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	if _, err := url.ParseRequestURI(req.URL); err != nil {
		return common.ValidationErrorResponse(c, "url is invalid")
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	upsert := merchant.Settings{
		merchant.PropertyWebhookURL:      req.URL,
		merchant.PropertySignatureSecret: req.Secret,
	}

	if err := h.merchants.UpsertSettings(ctx, mt, upsert); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) UpdateMerchantSupportedMethods(c echo.Context) error {
	var req model.UpdateSupportedPaymentMethodsRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	tickers := req.SupportedPaymentMethods

	if len(tickers) != len(util.Set(tickers)) {
		return common.ValidationErrorResponse(c, "duplicate tickers")
	}

	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	if err := h.merchants.UpdateSupportedMethods(ctx, mt, tickers); err != nil {
		return err
	}

	return c.NoContent(http.StatusNoContent)
}
