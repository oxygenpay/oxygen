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

	redirect, state := h.googleAuth.RedirectURLWithState()

	setSession := map[string]any{middleware.SessionStateKey: state}
	if err := h.persistSession(c, "google", setSession); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirect)
}

func (h *Handler) GetCallback(c echo.Context) error {
	ctx := c.Request().Context()

	if person := middleware.ResolveUser(c); person != nil {
		return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
	}

	query := c.Request().URL.Query()

	expectedState, stateExists := middleware.ResolveSessionOAuthState(c)
	switch {
	case !stateExists:
		return common.ValidationErrorResponse(c, "Missing OAuth state")
	case expectedState != query.Get("state"):
		return common.ValidationErrorResponse(c, "OAuth state mismatch")
	}

	googleUser, err := h.googleAuth.ResolveUser(ctx, query.Get("code"))
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

	setSession := map[string]any{middleware.UserIDContextKey: person.ID}
	if err := h.persistSession(c, "google", setSession); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
}
