package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/pkg/errors"
)

func (h *Handler) GetRedirect(c echo.Context) error {
	if person := middleware.ResolveUser(c); person != nil {
		return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
	}

	return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.RedirectURL())
}

func (h *Handler) GetCallback(c echo.Context) error {
	if person := middleware.ResolveUser(c); person != nil {
		return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
	}

	ctx := c.Request().Context()

	code := c.Request().URL.Query().Get("code")
	googleUser, err := h.googleAuth.ResolveUser(ctx, code)
	if err != nil {
		msg := "unable to resolve googleUser"
		h.logger.Error().Err(err).Msg(msg)
		return c.JSON(http.StatusInternalServerError, msg)
	}

	// check that user exists
	person, err := h.users.ResolveWithGoogle(ctx, googleUser)

	switch {
	case errors.Is(err, user.ErrRestricted):
		return common.ValidationErrorResponse(c, "Registration is available by whitelists only")
	case err != nil:
		return errors.Wrap(err, "unable to resolve google user")
	}

	if err := h.persistSessionUserID(c, person.ID, "google"); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
}
