package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/rs/zerolog"
)

// Handler user session auth handler. Uses Google OAuth.
type Handler struct {
	googleAuth       *auth.GoogleOAuthManager
	users            *user.Service
	enabledProviders []auth.ProviderType
	logger           *zerolog.Logger
}

func NewHandler(
	googleAuth *auth.GoogleOAuthManager,
	users *user.Service,
	enabledProviders []auth.ProviderType,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "auth_handler").Logger()

	return &Handler{
		googleAuth:       googleAuth,
		users:            users,
		enabledProviders: enabledProviders,
		logger:           &log,
	}
}

// GetCookie returns csrf cookie & csrf header in the same response.
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

func (h *Handler) ListAvailableProviders(c echo.Context) error {
	providers := util.MapSlice(h.enabledProviders, func(t auth.ProviderType) *model.Provider {
		return &model.Provider{Name: string(t)}
	})

	return c.JSON(http.StatusOK, &model.AvailableProvidersResponse{Providers: providers})
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

func (h *Handler) persistSession(c echo.Context, source string, values map[string]any) error {
	s := middleware.ResolveSession(c)

	for k, v := range values {
		s.Values[k] = v
	}

	if err := s.Save(c.Request(), c.Response()); err != nil {
		h.logger.Error().Err(err).Str("source", source).Msg("unable to persist user session")
		return err
	}

	return nil
}
