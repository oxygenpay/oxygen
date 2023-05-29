package merchantapi_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

// const paramPaymentLinkID = "paymentLinkId"

func TestPaymentLinkRoutes(t *testing.T) {
	const (
		paymentsLinksRoute = "/api/dashboard/v1/merchant/:merchantId/payment-link"
	)

	tc := test.NewIntegrationTest(t)

	u, token := tc.Must.CreateSampleUser(t)
	mt, _ := tc.Must.CreateMerchant(t, u.ID)

	t.Run("CreatePaymentLink", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			req           model.CreatePaymentLinkRequest
			errorContains string
			assert        func(t *testing.T, link model.PaymentLink)
		}{
			// Success cases
			{
				name: "USD/redirect",
				req: model.CreatePaymentLinkRequest{
					Currency:      "USD",
					Name:          "test",
					Price:         20,
					SuccessAction: string(payment.SuccessActionRedirect),
					RedirectURL:   util.Ptr("https://site.com"),
				},
				assert: func(t *testing.T, link model.PaymentLink) {
					assert.NotEmpty(t, link.RedirectURL)
				},
			},
			{
				name: "EUR/redirect/description",
				req: model.CreatePaymentLinkRequest{
					Currency:      "EUR",
					Name:          "test",
					Price:         30,
					Description:   util.Ptr("description"),
					SuccessAction: string(payment.SuccessActionRedirect),
					RedirectURL:   util.Ptr("https://site.com"),
				},
				assert: func(t *testing.T, link model.PaymentLink) {
					assert.NotEmpty(t, link.RedirectURL)
				},
			},
			{
				name: "EUR/message/description",
				req: model.CreatePaymentLinkRequest{
					Currency:       "EUR",
					Name:           "test",
					Price:          30,
					Description:    util.Ptr("description"),
					SuccessAction:  string(payment.SuccessActionShowMessage),
					SuccessMessage: util.Ptr("message"),
				},
				assert: func(t *testing.T, link model.PaymentLink) {
					assert.Empty(t, link.RedirectURL)
					assert.Equal(t, "message", *link.SuccessMessage)
				},
			},
			// Validation errors
			{
				name: "EUR/message/no-message",
				req: model.CreatePaymentLinkRequest{
					Currency:      "EUR",
					Name:          "test",
					Price:         30,
					Description:   util.Ptr("description"),
					SuccessAction: string(payment.SuccessActionShowMessage),
				},
				errorContains: "successMessage required",
			},
			{
				name: "EUR/no-action",
				req: model.CreatePaymentLinkRequest{
					Currency:    "EUR",
					Name:        "test",
					Price:       30,
					Description: util.Ptr("description"),
				},
				errorContains: "successAction in body is required",
			},
			{
				name: "USD/redirect/invalid url",
				req: model.CreatePaymentLinkRequest{
					Currency:      "USD",
					Name:          "test",
					Price:         20,
					SuccessAction: string(payment.SuccessActionRedirect),
					RedirectURL:   util.Ptr("//site.com"),
				},
				errorContains: "invalid redirect url: scheme should be HTTPS",
			},
			{
				name: "USD/redirect/http",
				req: model.CreatePaymentLinkRequest{
					Currency:      "USD",
					Name:          "test",
					Price:         20,
					SuccessAction: string(payment.SuccessActionRedirect),
					RedirectURL:   util.Ptr("http://site.com"),
				},
				errorContains: "invalid redirect url: scheme should be HTTPS",
			},
			{
				name: "USD/redirect/invalid-price",
				req: model.CreatePaymentLinkRequest{
					Currency:      "USD",
					Name:          "test",
					Price:         -20,
					SuccessAction: string(payment.SuccessActionRedirect),
					RedirectURL:   util.Ptr("https:////site.com"),
				},
				errorContains: "price in body should be greater than or equal to 0.01",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				// ACT
				res := tc.Client.
					POST().
					Path(paymentsLinksRoute).
					WithToken(token).
					WithCSRF().
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&tt.req).
					Do()

				// ASSERT
				if tt.errorContains != "" {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode())
					assert.Contains(t, res.String(), tt.errorContains)
					return
				}

				var body model.PaymentLink

				assert.Equal(t, http.StatusCreated, res.StatusCode())
				assert.NoError(t, res.JSON(&body))

				// check basic stuff
				assert.NotEmpty(t, body.ID)
				assert.NotEmpty(t, body.URL)

				assert.Equal(t, tt.req.Name, body.Name)
				assert.Equal(t, tt.req.Description, body.Description)

				assert.Equal(t, tt.req.Currency, body.Currency)
				assert.Equal(t, tt.req.Price, lo.Must(strconv.ParseFloat(body.Price, 64)))

				assert.Equal(t, tt.req.SuccessAction, body.SuccessAction)

				// extra assertion
				if tt.assert != nil {
					tt.assert(t, body)
				}
			})
		}
	})
}
