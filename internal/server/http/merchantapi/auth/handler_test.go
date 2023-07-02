package auth_test

import (
	"net/http"
	"testing"

	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestNewAuthHandler(t *testing.T) {
	const (
		authGetMeRoute = "/api/dashboard/v1/auth/me"
	)

	tc := test.NewIntegrationTest(t)

	t.Run("GetMe", func(t *testing.T) {
		// ARRANGE
		user, token := tc.Must.CreateUser(t, auth.GoogleUser{
			Name:          "user1",
			Email:         "user@google.com",
			EmailVerified: true,
			Locale:        "ru",
		})

		// ACT
		res := tc.Client.
			GET().
			Path(authGetMeRoute).
			WithToken(token).
			Do()

		// ASSERT
		assert.Equal(t, res.StatusCode(), http.StatusOK)

		var body model.User
		assert.NoError(t, res.JSON(&body))

		assert.Equal(t, user.Name, body.Name)
		assert.Equal(t, user.Email, body.Email)
	})
}
