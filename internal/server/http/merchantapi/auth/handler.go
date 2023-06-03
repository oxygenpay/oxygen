package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// Handler user session auth handler. Uses Google OAuth.
type Handler struct {
	googleAuth *auth.GoogleOAuthManager
	users      *user.Service
	logger     *zerolog.Logger
}

func NewHandler(googleAuth *auth.GoogleOAuthManager, users *user.Service, logger *zerolog.Logger) *Handler {
	log := logger.With().Str("channel", "auth_handler").Logger()

	return &Handler{
		googleAuth: googleAuth,
		users:      users,
		logger:     &log,
	}
}

// GetCookie get csrf cookie & header in this response
func (h *Handler) GetCookie(c echo.Context) error {
	tokenRaw := c.Get("csrf")
	token, ok := tokenRaw.(string)
	if !ok {
		return common.ErrorResponse(c, "internal_error")
	}

	c.Response().Header().Set(echo.HeaderXCSRFToken, token)
	c.Response().Header().Set(echo.HeaderAccessControlExposeHeaders, middleware.CSRFTokenHeader)

	return c.NoContent(http.StatusNoContent)
}

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

	userSession := middleware.ResolveSession(c)
	userSession.Values["user_id"] = person.ID
	if err := userSession.Save(c.Request(), c.Response()); err != nil {
		h.logger.Error().Err(err).Msg("unable to persist user session")
		return common.ErrorResponse(c, "internal error")
	}

	return c.Redirect(http.StatusTemporaryRedirect, h.googleAuth.GetAuthenticatedRedirectURL())
}

func (h *Handler) GetMe(c echo.Context) error {
	person := middleware.ResolveUser(c)

	return c.JSON(http.StatusOK, &model.User{
		UUID:            person.UUID.String(),
		Email:           person.Email,
		Name:            person.Name,
		ProfileImageURL: person.ProfileImageURL,
	})
}

func (h *Handler) PostLogout(c echo.Context) error {
	userSession := middleware.ResolveSession(c)
	userSession.Values["user_id"] = nil
	if err := userSession.Save(c.Request(), c.Response()); err != nil {
		h.logger.Error().Err(err).Msg("unable to persist user session")
	}

	return c.NoContent(http.StatusNoContent)
}
