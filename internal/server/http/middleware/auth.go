package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

// nolint gosec
const (
	TokenHeader     = "X-O2PAY-TOKEN"
	CSRFTokenHeader = "X-CSRF-TOKEN"

	UserContextKey        = "user"
	UserIDContextKey      = "user_id"
	IsTokenAuthContextKey = "token_auth"
	MerchantContextKey    = "merchant"

	SessionStateKey = "session_state"

	ParamMerchantID = "merchantId"
)

// ResolvesUserBySession attaches user to echo.Context if possible
func ResolvesUserBySession(users *user.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userSession := ResolveSession(c)
			userID, ok := userSession.Values[UserIDContextKey].(int64)
			if !ok {
				return next(c)
			}

			if person, err := users.GetByID(c.Request().Context(), userID); err == nil {
				c.Set(UserContextKey, person)
			}

			return next(c)
		}
	}
}

// ResolvesUserByToken attaches user to echo.Context
// if user still isn't set by session
func ResolvesUserByToken(tokens *auth.TokenAuthManager, users *user.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// if user already resolved by other middleware -> skipp
			person := ResolveUser(c)
			if person != nil {
				return next(c)
			}

			value := c.Request().Header.Get(TokenHeader)
			if value == "" {
				return next(c)
			}

			ctx := c.Request().Context()

			token, err := tokens.GetToken(ctx, auth.TokenTypeUser, value)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, &model.ErrorResponse{
					Message: "Invalid api token",
					Status:  "token_error",
				})
			}

			if person, err = users.GetByID(ctx, token.EntityID); err == nil {
				c.Set(UserContextKey, person)
				c.Set(IsTokenAuthContextKey, true)
			}

			return next(c)
		}
	}
}

// ResolvesMerchantByUUID. Middleware tries to bind merchant from request to echo.Context
// if uuid is invalid or merchant not found, no error occurs.
// Warning: user with middleware only after ResolvesUserBySession or ResolvesUserByToken
func ResolvesMerchantByUUID(merchants *merchant.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// if merchant already resolved by other middleware -> skipp
			m := ResolveMerchant(c)
			if m != nil {
				return next(c)
			}

			merchantUUID, err := uuid.Parse(c.Param(ParamMerchantID))
			if err != nil {
				return next(c)
			}

			person := ResolveUser(c)
			if person == nil {
				return next(c)
			}

			m, err = merchants.GetByUUIDAndCreatorID(
				c.Request().Context(),
				merchantUUID,
				person.ID,
				false,
			)

			if err == nil && m != nil {
				c.Set(MerchantContextKey, m)
			}

			return next(c)
		}
	}
}

// ResolvesMerchantByToken attaches merchant to echo.Context. Returns 400 if auth token not provided
func ResolvesMerchantByToken(tokens *auth.TokenAuthManager, merchants *merchant.Service) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// if merchant already resolved by other middleware -> skipp
			mt := ResolveMerchant(c)
			if mt != nil {
				return next(c)
			}

			value := c.Request().Header.Get(TokenHeader)
			if value == "" {
				return c.JSON(http.StatusUnauthorized, &model.ErrorResponse{
					Message: "Unauthorized",
					Status:  "token_error",
				})
			}

			ctx := c.Request().Context()

			token, err := tokens.GetToken(ctx, auth.TokenTypeMerchant, value)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, &model.ErrorResponse{
					Message: "Invalid api token",
					Status:  "token_error",
				})
			}

			if mt, err = merchants.GetByID(ctx, token.EntityID, false); err == nil {
				c.Set(MerchantContextKey, mt)
				c.Set(IsTokenAuthContextKey, true)
			}

			return next(c)
		}
	}
}

// GuardsUsers validates that user attached to echo.Context otherwise returns '401 Unauthorized'.
func GuardsUsers() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if ResolveUser(c) == nil {
				return c.JSON(http.StatusUnauthorized, &model.ErrorResponse{
					Errors:  nil,
					Message: "Authentication required",
					Status:  "unauthorized",
				})
			}

			return next(c)
		}
	}
}

// GuardsMerchants validate that user's merchant is attached to echo.Context or returns 400 bad request
func GuardsMerchants() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			mt := ResolveMerchant(c)

			if mt == nil {
				return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
					Errors:  nil,
					Message: "Merchant not found",
					Status:  "not_found",
				})
			}

			// if resolved merchant by token does not match to provided URL
			// Example: GET /merchant/{merchant1} -H 'Token: merchant2-token'
			merchantID := c.Param(ParamMerchantID)
			if merchantID != "" && merchantID != mt.UUID.String() {
				return c.JSON(http.StatusBadRequest, &model.ErrorResponse{
					Errors:  nil,
					Message: "Invalid merchant",
					Status:  "invalid_merchant",
				})
			}

			return next(c)
		}
	}
}

func ResolveSession(c echo.Context) *sessions.Session {
	sessionOptions, ok := c.Get(sessionOptionsKey).(sessions.Options)
	if !ok {
		return nil
	}

	userSession, _ := session.Get("session", c)
	userSession.Options = &sessionOptions

	return userSession
}

func ResolveSessionOAuthState(c echo.Context) (string, bool) {
	s := ResolveSession(c)

	raw, ok := s.Values[SessionStateKey]
	if !ok {
		return "", false
	}

	state, ok := raw.(string)
	if !ok {
		return "", false
	}

	return state, true
}

func ResolveUser(c echo.Context) *user.User {
	personRaw := c.Get(UserContextKey)
	person, ok := personRaw.(*user.User)
	if !ok || person == nil || person.ID == 0 {
		return nil
	}

	return person
}

func ResolveMerchant(c echo.Context) *merchant.Merchant {
	raw := c.Get(MerchantContextKey)
	m, ok := raw.(*merchant.Merchant)
	if !ok || m == nil || m.ID == 0 {
		return nil
	}

	return m
}
