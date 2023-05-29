package paymentapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-payment/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const paramPaymentLinkSlug = "paymentLinkSlug"

const (
	paymentLinkRoute   = "/api/payment/v1/payment-link/:paymentLinkSlug"
	createPaymentRoute = "/api/payment/v1/payment-link/:paymentLinkSlug/payment"
)

func TestPaymentLinkRoutes(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	mt, _ := tc.Must.CreateMerchant(t, 1)

	t.Run("GetLink", func(t *testing.T) {
		// Given a payment link
		link, err := tc.Services.Payment.CreatePaymentLink(tc.Context, mt.ID, payment.CreateLinkProps{
			Name:          "my-link",
			Price:         lo.Must(money.USD.MakeAmount("3000")),
			Description:   util.Ptr("Golang online course"),
			SuccessAction: payment.SuccessActionRedirect,
			RedirectURL:   util.Ptr("https://site.com"),
		})
		require.NoError(t, err)

		// ACT
		res := tc.Client.
			GET().
			Path(paymentLinkRoute).
			WithCSRF().
			Param(paramPaymentLinkSlug, link.Slug).
			Do()

		// ASSERT
		var body model.PaymentLink

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(&body))

		assert.Equal(t, mt.Name, body.MerchantName)
		assert.Equal(t, link.Description, body.Description)
		assert.Equal(t, link.Price.Ticker(), body.Currency)
		assert.Equal(t, lo.Must(link.Price.FiatToFloat64()), body.Price)

		t.Run("NotFound", func(t *testing.T) {
			// ACT
			res := tc.Client.
				GET().
				Path(paymentLinkRoute).
				WithCSRF().
				Param(paramPaymentLinkSlug, "abc").
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		})
	})

	t.Run("CreatePaymentFromLink", func(t *testing.T) {
		mt, _ := tc.Must.CreateMerchant(t, 1)

		var (
			description    = "my description"
			successMessage = "my success message"
			redirectURL    = "https://site.com"
		)

		for _, tt := range []payment.CreateLinkProps{
			{
				Name:          "redirect/with-description",
				Price:         lo.Must(money.USD.MakeAmount("3000")),
				Description:   &description,
				SuccessAction: payment.SuccessActionRedirect,
				RedirectURL:   &redirectURL,
			},
			{
				Name:           "message/no-description",
				Price:          lo.Must(money.EUR.MakeAmount("5000")),
				Description:    &description,
				SuccessAction:  payment.SuccessActionShowMessage,
				SuccessMessage: &successMessage,
			},
		} {
			t.Run(tt.Name, func(t *testing.T) {
				// ARRANGE
				link, err := tc.Services.Payment.CreatePaymentLink(tc.Context, mt.ID, tt)
				require.NoError(t, err)

				// ACT
				res := tc.Client.
					POST().
					Path(createPaymentRoute).
					WithCSRF().
					Param(paramPaymentLinkSlug, link.Slug).
					Do()

				// ASSERT
				var body model.PaymentRedirectInfo

				assert.Equal(t, http.StatusCreated, res.StatusCode())
				assert.NoError(t, res.JSON(&body))

				id := uuid.MustParse(body.ID)

				pt, err := tc.Services.Payment.GetByPublicID(tc.Context, id)
				assert.NoError(t, err)

				// Assert payment
				assert.True(t, tt.Price.Equals(pt.Price))
				assert.NotEmpty(t, pt.MerchantOrderUUID)
				assert.Equal(t, tt.IsTest, pt.IsTest)
				assert.Equal(t, tt.Description, pt.Description)
				assert.Equal(t, tt.SuccessAction, *pt.LinkSuccessAction())
				assert.Equal(t, link.ID, pt.LinkID())

				if tt.SuccessAction == payment.SuccessActionRedirect {
					assert.Equal(t, *tt.RedirectURL, pt.RedirectURL)
				}

				if tt.SuccessAction == payment.SuccessActionShowMessage {
					assert.Equal(t, tt.SuccessMessage, pt.LinkSuccessMessage())
				}

				// sleep to prevent "429 too many requests"
				time.Sleep(time.Second)
			})
		}
	})
}
