package auth_test

import (
	"net/http"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
)

const (
	authLoginRoute = "/api/dashboard/v1/auth/login"
)

func TestHandler_PostLogin(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	t.Run("Login successful", func(t *testing.T) {
		// ARRANGE
		// Given a user with email auth
		pass := "qwerty123"
		u, _ := tc.Must.CreateUserViaEmail(t, "test@me.com", pass)

		// Given login request
		req := &model.LoginRequest{Email: strfmt.Email(u.Email), Password: pass}

		// ACT
		// Login
		res := tc.Client.
			POST().
			WithCSRF().
			Path(authLoginRoute).
			JSON(req).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusNoContent, res.StatusCode())
		assert.Contains(t, res.Headers().Values("set-cookie")[1], "session=")
	})

	t.Run("Invalid email or password", func(t *testing.T) {
		// ARRANGE
		// Given a user with email auth
		u, _ := tc.Must.CreateUserViaEmail(t, "test2@me.com", "qwerty123")

		// Given login request
		req := &model.LoginRequest{Email: strfmt.Email(u.Email), Password: "qwerty456"}

		// ACT
		// Login
		res := tc.Client.
			POST().
			WithCSRF().
			Path(authLoginRoute).
			JSON(req).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		assert.Contains(t, res.String(), "User with provided email or password not found")
	})
}
