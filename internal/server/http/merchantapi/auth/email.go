package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

func (h *Handler) PostLogin(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.LoginRequest
	if !common.BindAndValidateRequest(c, &req) {
		return nil
	}

	// already logged in
	if u := middleware.ResolveUser(c); u != nil {
		return c.NoContent(http.StatusNoContent)
	}

	person, err := h.users.GetByEmailWithPasswordCheck(ctx, req.Email.String(), req.Password)
	switch {
	case errors.Is(err, user.ErrNotFound), errors.Is(err, user.ErrWrongPassword):
		return common.ValidationErrorItemResponse(c, "email", "User with provided email or password not found")
	case err != nil:
		return errors.Wrap(err, "unable to resolve user")
	}

	setSession := map[string]any{middleware.UserIDContextKey: person.ID}
	if err := h.persistSession(c, "email", setSession); err != nil {
		return common.ErrorResponse(c, "internal error")
	}

	return c.NoContent(http.StatusNoContent)
}
