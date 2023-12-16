package merchantapi_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
)

const (
	merchantRoute         = "/api/dashboard/v1/merchant/:merchantId"
	webhookRoute          = "/api/dashboard/v1/merchant/:merchantId/webhook"
	supportedMethodsRoute = "/api/dashboard/v1/merchant/:merchantId/supported-method"
)

func TestMerchantRoutes(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	user, token := tc.Must.CreateUser(t, auth.GoogleUser{Name: "John", Email: "john@gmai.com"})

	t.Run("WebhookRoute", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, user.ID)

		// ACT 1
		// Get merchant
		res := tc.Client.
			GET().
			Path(merchantRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		// ASSERT
		output := &model.Merchant{}

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(output))
		assert.Empty(t, output.WebhookSettings)

		// ACT 2
		// Set webhook data
		req := &model.WebhookSettings{Secret: "abc", URL: "https://site.com"}
		res = tc.Client.
			PUT().
			Path(webhookRoute).
			WithToken(token).
			JSON(req).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusNoContent, res.StatusCode())

		// ACT 3
		// Check that data was updated
		res = tc.Client.
			GET().
			Path(merchantRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(output))
		assert.Equal(t, req, output.WebhookSettings)

		// ACT 4
		// Send invalid request
		req = &model.WebhookSettings{Secret: "abc", URL: "invalid url"}
		res = tc.Client.
			PUT().
			Path(webhookRoute).
			WithToken(token).
			JSON(req).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		assert.Contains(t, res.String(), "url is invalid")
	})

	t.Run("SupportedMethodRoute", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, user.ID)

		// And blockchain currencies
		allCurrencies := tc.Services.Blockchain.ListSupportedCurrencies(false)

		// ACT 1
		// Get merchant
		res := tc.Client.
			GET().
			Path(merchantRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		// ASSERT
		output := &model.Merchant{}

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(output))
		assert.Len(t, output.SupportedPaymentMethods, len(allCurrencies))

		// Check that all methods are enabled (sc.Enabled == false is not present in the response)
		assert.NotContains(t, util.MapSlice(
			output.SupportedPaymentMethods,
			func(sc *model.SupportedPaymentMethod) bool { return sc.Enabled },
		), false)

		// ACT 2
		// Set available payment methods
		req := &model.UpdateSupportedPaymentMethodsRequest{SupportedPaymentMethods: []string{"ETH", "ETH_USDT"}}
		res = tc.Client.
			PUT().
			Path(supportedMethodsRoute).
			WithToken(token).
			JSON(req).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusNoContent, res.StatusCode())

		// ACT 3
		// Check that data was updated
		res = tc.Client.
			GET().
			Path(merchantRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(output))
		assert.Len(t, output.SupportedPaymentMethods, len(allCurrencies))

		// Check that only 2 enabled currencies are present in the response
		assert.Equal(t,
			[]bool{true, true},
			util.FilterSlice(
				util.MapSlice(
					output.SupportedPaymentMethods,
					func(sc *model.SupportedPaymentMethod) bool { return sc.Enabled },
				),
				func(b bool) bool { return b },
			),
		)

		t.Run("Fails", func(t *testing.T) {
			for i, testCase := range []model.UpdateSupportedPaymentMethodsRequest{
				// len == 0
				{SupportedPaymentMethods: nil},
				// unknown ticker
				{SupportedPaymentMethods: []string{"ABC"}},
				// duplicates
				{SupportedPaymentMethods: []string{"ETH", "ETH"}},
			} {
				t.Run(strconv.Itoa(i+1), func(t *testing.T) {
					// ACT
					// Send invalid request
					res := tc.Client.
						PUT().
						Path(webhookRoute).
						WithToken(token).
						JSON(&testCase).
						Param(paramMerchantID, mt.UUID.String()).
						Do()

					assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				})
			}
		})
	})

	t.Run("UpdateMerchant", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// And request
		req := &model.UpdateMerchantRequest{
			Name:    "Name1",
			Website: "https://site.com",
		}

		// ACT
		// Update merchant info
		res := tc.Client.
			PUT().
			Path(merchantRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			JSON(req).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusNoContent, res.StatusCode())

		mt, err := tc.Services.Merchants.GetByID(tc.Context, mt.ID, false)
		assert.NoError(t, err)

		assert.Equal(t, "Name1", mt.Name)
		assert.Equal(t, "https://site.com", mt.Website)

		t.Run("Validation error", func(t *testing.T) {
			// ARRANGE
			// Given an invalid website
			req := &model.UpdateMerchantRequest{
				Name:    "Name1",
				Website: "website1",
			}

			// ACT
			// Update merchant info
			res := tc.Client.
				PUT().
				Path(merchantRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				JSON(req).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "website should be a valid URL")
		})
	})
}
