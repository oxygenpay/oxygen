package merchantapi

import (
	"net/http"

	"github.com/go-openapi/strfmt"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	paramTokenID = "tokenId"
)

func (h *Handler) ListMerchantTokens(c echo.Context) error {
	m := middleware.ResolveMerchant(c)
	ctx := c.Request().Context()

	tokens, err := h.tokens.ListByEntityType(ctx, auth.TokenTypeMerchant, m.ID)
	if err != nil {
		h.logger.Error().Err(err).Msg("unable to store merchant token")
		return common.ErrorResponse(c, "internal_error")
	}

	var tokensFormatted = make([]*model.APIToken, len(tokens))
	for i, token := range tokens {
		var name string
		if token.Name != nil {
			name = *token.Name
		}

		tokensFormatted[i] = &model.APIToken{
			ID:        token.UUID.String(),
			CreatedAt: strfmt.DateTime(token.CreatedAt),
			Name:      name,
			Token:     &token.Token,
		}
	}

	return c.JSON(http.StatusOK, &model.TokenList{Results: tokensFormatted})
}

func (h *Handler) CreateMerchantToken(c echo.Context) error {
	req := &model.CreateMerchantTokenRequest{}
	if isValid := common.BindAndValidateRequest(c, req); !isValid {
		return nil
	}

	mt := middleware.ResolveMerchant(c)
	ctx := c.Request().Context()

	token, err := h.tokens.CreateMerchantToken(ctx, mt.ID, req.Name)
	if err != nil {
		return errors.Wrap(err, "unable to create merchant token")
	}

	var name string
	if token.Name != nil {
		name = *token.Name
	}

	return c.JSON(http.StatusCreated, &model.APIToken{
		Name:  name,
		Token: &token.Token,
		ID:    token.UUID.String(),
	})
}

func (h *Handler) DeleteMerchantTokens(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	id, err := common.UUID(c, paramTokenID)
	if err != nil {
		return nil
	}

	token, err := h.tokens.GetMerchantTokenByUUID(ctx, mt.ID, id)

	switch {
	case errors.Is(err, auth.ErrNotFound):
		return common.NotFoundResponse(c, auth.ErrNotFound.Error())
	case err != nil:
		return errors.Wrap(err, "unable to get token")
	}

	if err = h.tokens.DeleteToken(ctx, token.ID); err != nil {
		return errors.Wrap(err, "unable to delete token")
	}

	return c.NoContent(http.StatusNoContent)
}
