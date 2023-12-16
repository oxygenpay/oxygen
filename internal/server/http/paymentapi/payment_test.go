package paymentapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/paymentapi"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-payment/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	paymentRoute          = "/api/payment/v1/payment/:paymentId"
	customerRoute         = "/api/payment/v1/payment/:paymentId/customer"
	supportedMethodsRoute = "/api/payment/v1/payment/:paymentId/supported-method"
	methodRoute           = "/api/payment/v1/payment/:paymentId/method"
)

//nolint:funlen
func TestHandlers(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	allCurrencies := tc.Services.Blockchain.ListSupportedCurrencies(false)

	t.Run("GetSupportedMethods", func(t *testing.T) {
		t.Run("Returns list of supported methods", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreateSamplePayment(t, mt.ID)

			// ACT 1
			res := tc.
				GET().
				Path(supportedMethodsRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.SupportedPaymentMethods
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, len(allCurrencies), len(body.AvailableMethods))

			// ACT 2
			// When I update merchants supported methods
			err := tc.Services.Merchants.UpdateSupportedMethods(tc.Context, mt, []string{"ETH"})
			require.NoError(t, err)

			// And request the endpoint again
			res = tc.
				GET().
				Path(supportedMethodsRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			// Then I see only 1 available method
			assert.NoError(t, res.JSON(&body))
			assert.Len(t, body.AvailableMethods, 1)
			assert.Equal(t, "ETH", body.AvailableMethods[0].Ticker)
		})
	})

	t.Run("GetPayment", func(t *testing.T) {
		t.Run("Payment not found", func(t *testing.T) {
			// ARRANGE
			paymentID := uuid.New()

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, paymentID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "not found")
		})

		t.Run("Payment is archived", func(t *testing.T) {
			// ARRANGE
			// Given a payment
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreateSamplePayment(t, mt.ID)

			// That was marked as successful 1h 1m ago
			const window = time.Minute * 61
			_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
				ID:         p.ID,
				MerchantID: mt.ID,
				Status:     string(payment.StatusSuccess),
				UpdatedAt:  time.Now().UTC().Add(-window),
			})
			require.NoError(t, err)

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "archived")
		})

		t.Run("Returns payment", func(t *testing.T) {
			// ARRANGE
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreateSamplePayment(t, mt.ID)

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, body.ID, p.PublicID.String())
			assert.Equal(t, body.Price, 12.34)
			assert.Equal(t, body.Currency, "USD")
			assert.False(t, body.IsLocked)

			// no customer / payment method
			assert.Nil(t, body.Customer)
			assert.Nil(t, body.PaymentMethod)
		})

		t.Run("Returns payment with customer", func(t *testing.T) {
			// ARRANGE
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreateSamplePayment(t, mt.ID)

			email := "me@test.com"

			person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, email)
			require.NoError(t, err)

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))
			require.NotNil(t, body.Customer)
			assert.Equal(t, email, body.Customer.Email)
			assert.Equal(t, person.UUID.String(), body.Customer.ID)
			assert.False(t, body.IsLocked)
		})

		t.Run("Returns payment with paymentMethod", func(t *testing.T) {
			// ARRANGE
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreateSamplePayment(t, mt.ID)
			ticker := "ETH"

			currency, err := tc.Services.Blockchain.GetCurrencyByTicker(ticker)
			require.NoError(t, err)
			require.Equal(t, ticker, currency.Ticker)

			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1300)
			tc.SetupCreateWalletWithSubscription(ticker, "0x123", "eth-pubkey-goes-here")

			method, err := tc.Services.Processing.SetPaymentMethod(tc.Context, p, currency.Ticker)
			require.NoError(t, err)
			require.Equal(t, currency.Ticker, method.Currency.Ticker)

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))
			require.NotNil(t, body.PaymentMethod)
			assert.Equal(t, currency.Ticker, body.PaymentMethod.Ticker)
			assert.Equal(t, currency.DisplayName(), body.PaymentMethod.DisplayName)
			assert.Equal(t, currency.NetworkID, body.PaymentMethod.NetworkID)
			assert.False(t, body.PaymentMethod.IsTest)

			t.Run("Returns updated payment method", func(t *testing.T) {
				// ARRANGE
				ticker := "MATIC"
				currency, err := tc.Services.Blockchain.GetCurrencyByTicker(ticker)
				require.NoError(t, err)
				require.Equal(t, ticker, currency.Ticker)

				tc.Providers.TatumMock.SetupRates(ticker, money.USD, 0.83)
				tc.SetupCreateWalletWithSubscription(ticker, "0x123", "polygon-pubkey-goes-here")

				_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, currency.Ticker)
				require.NoError(t, err)

				// ACT
				res := tc.
					GET().
					Path(paymentRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusOK, res.StatusCode())

				var body2 model.Payment
				assert.NoError(t, res.JSON(&body2))
				require.NotNil(t, body2.PaymentMethod)
				assert.Equal(t, currency.Ticker, body2.PaymentMethod.Ticker)
				assert.Equal(t, currency.DisplayName(), body2.PaymentMethod.DisplayName)
				assert.NotEqual(t, body.PaymentMethod.Ticker, body2.PaymentMethod.Ticker)
				assert.Equal(t, currency.NetworkID, body2.PaymentMethod.NetworkID)
				assert.False(t, body2.PaymentMethod.IsTest)
			})
		})

		t.Run("Returns payment with customer, paymentMethod & paymentInfo", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			email := "me@test.com"
			p := tc.CreateSamplePayment(t, merchantID)

			person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, email)
			require.NoError(t, err)

			ticker := "ETH"
			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
			tc.SetupCreateWalletWithSubscription(ticker, "0x123", "pubkey-goes-here")

			_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)
			require.NoError(t, err)

			err = tc.Services.Processing.LockPaymentOptions(tc.Context, p.MerchantID, p.ID)
			require.NoError(t, err)

			// ACT
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))

			require.NotNil(t, body.Customer)
			assert.Equal(t, email, body.Customer.Email)
			assert.Equal(t, person.UUID.String(), body.Customer.ID)

			require.NotNil(t, body.PaymentMethod)
			require.Equal(t, ticker, body.PaymentMethod.Ticker)

			assert.True(t, body.IsLocked)

			require.NotNil(t, body.PaymentInfo)
			require.NotEmpty(t, body.PaymentInfo.RecipientAddress)
			require.NotEmpty(t, body.PaymentInfo.Amount)
			require.NotEmpty(t, body.PaymentInfo.AmountFormatted)
			require.Equal(t, payment.StatusPending.String(), body.PaymentInfo.Status)
			require.Nil(t, body.PaymentInfo.SuccessAction)
			require.Nil(t, body.PaymentInfo.SuccessURL)
			require.Nil(t, body.PaymentInfo.SuccessMessage)
		})

		t.Run("Returns successful payment", func(t *testing.T) {
			// ARRANGE
			// make sure all wallets deleted
			tc.Clear.Wallets(t)

			// given sample payment
			merchantID := int64(1)
			email := "me@test.com"
			p := tc.CreateSamplePayment(t, merchantID)

			// and assigned customer
			person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, email)
			require.NoError(t, err)

			// and mocked Tatum data
			ticker := "ETH"
			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
			tc.SetupCreateWalletWithSubscription(ticker, "0x123", "pubkey-goes-here")

			// and set payment method
			_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)
			require.NoError(t, err)

			// and locked payment options
			err = tc.Services.Processing.LockPaymentOptions(tc.Context, p.MerchantID, p.ID)
			require.NoError(t, err)

			// and payment that artificially marked as successful
			p, err = tc.Services.Payment.Update(tc.Context, merchantID, p.ID, payment.UpdateProps{
				Status: payment.StatusSuccess,
			})
			require.NoError(t, err)

			// ACT
			// Get payment data
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))

			// Check body
			require.NotNil(t, body.Customer)
			assert.Equal(t, email, body.Customer.Email)
			assert.Equal(t, person.UUID.String(), body.Customer.ID)

			require.NotNil(t, body.PaymentMethod)
			require.Equal(t, ticker, body.PaymentMethod.Ticker)

			assert.True(t, body.IsLocked)

			require.NotNil(t, body.PaymentInfo)
			require.NotEmpty(t, body.PaymentInfo.RecipientAddress)
			require.NotEmpty(t, body.PaymentInfo.Amount)
			require.NotEmpty(t, body.PaymentInfo.AmountFormatted)
			require.Equal(t, payment.StatusSuccess.String(), body.PaymentInfo.Status)

			require.Equal(t, string(payment.SuccessActionRedirect), *body.PaymentInfo.SuccessAction)
			require.Equal(t, p.RedirectURL, *body.PaymentInfo.SuccessURL)
			require.Nil(t, body.PaymentInfo.SuccessMessage)
		})

		t.Run("Returns successful payment created from link with redirect", func(t *testing.T) {
			// ARRANGE
			// make sure all wallets deleted
			tc.Clear.Wallets(t)

			mt, _ := tc.Must.CreateMerchant(t, 1)

			// given sample payment with link
			email := "me@test.com"
			link, err := tc.Services.Payment.CreatePaymentLink(tc.Context, mt.ID, payment.CreateLinkProps{
				Name:          "test link",
				Price:         lo.Must(money.USD.MakeAmount("300")),
				SuccessAction: payment.SuccessActionRedirect,
				RedirectURL:   util.Ptr("https://site.com"),
			})
			require.NoError(t, err)

			p, err := tc.Services.Payment.CreatePaymentFromLink(tc.Context, link)
			require.NoError(t, err)

			// and assigned customer
			person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, email)
			require.NoError(t, err)

			// and mocked Tatum data
			ticker := "ETH"
			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
			tc.SetupCreateWalletWithSubscription(ticker, "0x123", "pubkey-goes-here")

			// and set payment method
			_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)
			require.NoError(t, err)

			// and locked payment options
			err = tc.Services.Processing.LockPaymentOptions(tc.Context, p.MerchantID, p.ID)
			require.NoError(t, err)

			// and payment that artificially marked as successful
			p, err = tc.Services.Payment.Update(tc.Context, mt.ID, p.ID, payment.UpdateProps{
				Status: payment.StatusSuccess,
			})
			require.NoError(t, err)

			// ACT
			// Get payment data
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))

			// Check body
			require.NotNil(t, body.Customer)
			assert.Equal(t, email, body.Customer.Email)
			assert.Equal(t, person.UUID.String(), body.Customer.ID)

			require.NotNil(t, body.PaymentMethod)
			require.Equal(t, ticker, body.PaymentMethod.Ticker)

			assert.True(t, body.IsLocked)

			require.NotNil(t, body.PaymentInfo)
			require.NotEmpty(t, body.PaymentInfo.RecipientAddress)
			require.NotEmpty(t, body.PaymentInfo.Amount)
			require.NotEmpty(t, body.PaymentInfo.AmountFormatted)
			require.Equal(t, payment.StatusSuccess.String(), body.PaymentInfo.Status)

			require.Equal(t, string(payment.SuccessActionRedirect), *body.PaymentInfo.SuccessAction)
			require.Equal(t, p.RedirectURL, *body.PaymentInfo.SuccessURL)
			require.Nil(t, body.PaymentInfo.SuccessMessage)
		})

		t.Run("Returns successful payment created from link with message", func(t *testing.T) {
			// ARRANGE
			// make sure all wallets deleted
			tc.Clear.Wallets(t)

			mt, _ := tc.Must.CreateMerchant(t, 1)

			// given sample payment with link
			email := "me@test.com"
			link, err := tc.Services.Payment.CreatePaymentLink(tc.Context, mt.ID, payment.CreateLinkProps{
				Name:           "test link with message",
				Price:          lo.Must(money.USD.MakeAmount("300")),
				SuccessAction:  payment.SuccessActionShowMessage,
				SuccessMessage: util.Ptr("thank you!"),
			})
			require.NoError(t, err)

			p, err := tc.Services.Payment.CreatePaymentFromLink(tc.Context, link)
			require.NoError(t, err)

			// and assigned customer
			person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, email)
			require.NoError(t, err)

			// and mocked Tatum data
			ticker := "ETH"
			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
			tc.SetupCreateWalletWithSubscription(ticker, "0x123", "pubkey-goes-here")

			// and set payment method
			_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)
			require.NoError(t, err)

			// and locked payment options
			err = tc.Services.Processing.LockPaymentOptions(tc.Context, p.MerchantID, p.ID)
			require.NoError(t, err)

			// and payment that artificially marked as successful
			p, err = tc.Services.Payment.Update(tc.Context, mt.ID, p.ID, payment.UpdateProps{
				Status: payment.StatusSuccess,
			})
			require.NoError(t, err)

			// ACT
			// Get payment data
			res := tc.
				GET().
				Path(paymentRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))

			// Check body
			require.NotNil(t, body.Customer)
			assert.Equal(t, email, body.Customer.Email)
			assert.Equal(t, person.UUID.String(), body.Customer.ID)

			require.NotNil(t, body.PaymentMethod)
			require.Equal(t, ticker, body.PaymentMethod.Ticker)

			assert.True(t, body.IsLocked)

			require.NotNil(t, body.PaymentInfo)
			require.NotEmpty(t, body.PaymentInfo.RecipientAddress)
			require.NotEmpty(t, body.PaymentInfo.Amount)
			require.NotEmpty(t, body.PaymentInfo.AmountFormatted)
			require.Equal(t, payment.StatusSuccess.String(), body.PaymentInfo.Status)

			require.Equal(t, string(payment.SuccessActionShowMessage), *body.PaymentInfo.SuccessAction)
			require.Equal(t, "thank you!", *body.PaymentInfo.SuccessMessage)
			require.Nil(t, body.PaymentInfo.SuccessURL)
		})
	})

	t.Run("ResolveCustomer", func(t *testing.T) {
		t.Run("Payment not found", func(t *testing.T) {
			// ARRANGE
			paymentID := uuid.New()

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(customerRoute).
				Param(paymentapi.ParamPaymentID, paymentID.String()).
				JSON(&model.CreateCustomerRequest{Email: "test@me.com"}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "not found")
		})

		t.Run("Assigns customer to the payment", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			email := "test@me.com"
			p := tc.CreateSamplePayment(t, merchantID)

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(customerRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				JSON(&model.CreateCustomerRequest{Email: email}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusCreated, res.StatusCode())

			var body model.Customer
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, body.Email, email)
			assert.NotEqual(t, body.ID, uuid.Nil.String())

			freshPayment, err := tc.Services.Payment.GetByPublicID(tc.Context, p.PublicID)
			assert.NoError(t, err)

			freshCustomer, err := tc.Services.Payment.GetCustomerByEmail(tc.Context, merchantID, email)
			assert.NoError(t, err)

			assert.Equal(t, *freshPayment.CustomerID, freshCustomer.ID)

			t.Run("Second call does not change customer", func(t *testing.T) {
				// ACT
				res := tc.
					POST().WithCSRF().
					Path(customerRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					JSON(&model.CreateCustomerRequest{Email: "test@me.com"}).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusCreated, res.StatusCode())

				var body2 model.Customer
				assert.NoError(t, res.JSON(&body2))
				assert.Equal(t, body.Email, "test@me.com")
				assert.Equal(t, body.ID, body2.ID)
			})

			t.Run("Creates another customer", func(t *testing.T) {
				// ARRANGE
				newEmail := "new@test.com"

				// ACT
				res := tc.
					POST().WithCSRF().
					Path(customerRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					JSON(&model.CreateCustomerRequest{Email: newEmail}).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusCreated, res.StatusCode())

				var body3 model.Customer
				assert.NoError(t, res.JSON(&body3))
				assert.Equal(t, body.Email, "test@me.com")
				assert.NotEqual(t, body.ID, body3.ID)

				freshPayment, err := tc.Services.Payment.GetByPublicID(tc.Context, p.PublicID)
				assert.NoError(t, err)

				freshCustomer, err := tc.Services.Payment.GetCustomerByEmail(tc.Context, merchantID, newEmail)
				assert.NoError(t, err)

				assert.Equal(t, *freshPayment.CustomerID, freshCustomer.ID)
			})

			t.Run("Returns validation error", func(t *testing.T) {
				// ACT
				res := tc.
					POST().WithCSRF().
					Path(customerRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					JSON(&model.CreateCustomerRequest{Email: "abc123"}).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				assert.Contains(t, res.String(), "invalid email provided")
			})
		})

		t.Run("Assigns the same customer to 3 payments", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			email := "test@me.com"

			// ACT
			for i := 0; i < 3; i++ {
				p := tc.CreateSamplePayment(t, merchantID)

				res := tc.
					POST().WithCSRF().
					Path(customerRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					JSON(&model.CreateCustomerRequest{Email: email}).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusCreated, res.StatusCode())

				var body model.Customer
				assert.NoError(t, res.JSON(&body))

				assert.Equal(t, body.Email, email)
				assert.NotEqual(t, body.ID, uuid.Nil.String())

				freshPayment, err := tc.Services.Payment.GetByPublicID(tc.Context, p.PublicID)
				assert.NoError(t, err)

				freshCustomer, err := tc.Services.Payment.GetCustomerByEmail(tc.Context, merchantID, email)
				assert.NoError(t, err)

				assert.Equal(t, *freshPayment.CustomerID, freshCustomer.ID)
			}
		})

		t.Run("Fails as payment is already locked", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			email := "test@me.com"
			p := tc.CreateSamplePayment(t, merchantID)

			_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
				ID:         p.ID,
				MerchantID: p.MerchantID,
				Status:     payment.StatusInProgress.String(),
				UpdatedAt:  time.Now(),
			})
			require.NoError(t, err)

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(customerRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				JSON(&model.CreateCustomerRequest{Email: email}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), payment.ErrPaymentLocked.Error())
		})
	})

	t.Run("SetPaymentMethod", func(t *testing.T) {
		t.Run("Selected currency does not exists", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			ticker := "ABC"

			p := tc.CreateSamplePayment(t, merchantID)

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(methodRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				JSON(&model.CreatePaymentMethodRequest{Ticker: ticker}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "invalid ticker")
		})

		t.Run("Selected currency is disabled by merchant", func(t *testing.T) {
			// ARRANGE
			blockchain := "ETH"
			ticker := "ETH"

			// Given a merchant
			// With only ETH_USDT supported currency
			mt, _ := tc.Must.CreateMerchant(t, 1)

			err := tc.Services.Merchants.UpdateSupportedMethods(tc.Context, mt, []string{"ETH_USDT"})
			require.NoError(t, err)

			// And a payment
			p := tc.CreateSamplePayment(t, mt.ID)

			// And mocked responses
			tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
			tc.SetupCreateWalletWithSubscription(blockchain, "0x999", "eth-pubkey-goes-here")

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(methodRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				JSON(&model.CreatePaymentMethodRequest{Ticker: ticker}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "invalid ticker")
		})

		t.Run("Successfully sets different payment methods", func(t *testing.T) {
			// ARRANGE
			// Given a payment and no wallets
			mt, _ := tc.Must.CreateMerchant(t, 1)
			p := tc.CreatePayment(t, mt.ID, money.USD, 35.50)
			tc.Clear.Wallets(t)

			for testcaseIndex, testCase := range []struct {
				ticker     string
				setupMocks func(ticker, blockchain string)
			}{
				{
					ticker: "ETH",
					setupMocks: func(ticker, blockchain string) {
						tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
						tc.SetupCreateWalletWithSubscription(blockchain, "0x123", "eth-pubkey-goes-here")
					},
				},
				{
					ticker: "ETH_USDT",
					setupMocks: func(ticker, blockchain string) {
						tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
						tc.SetupCreateWalletWithSubscription(blockchain, "0x456", "eth-pubkey-goes-here")
					},
				},
				{
					ticker: "MATIC",
					setupMocks: func(ticker, blockchain string) {
						tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
						tc.SetupCreateWalletWithSubscription(blockchain, "0x789", "polygon-pubkey-goes-here")
					},
				},
				{
					ticker: "ETH",
					setupMocks: func(ticker, blockchain string) {
						tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
						tc.SetupCreateWalletWithSubscription(blockchain, "0x999", "eth-pubkey-goes-here")
					},
				},
			} {
				t.Run(fmt.Sprintf("%d/%s", testcaseIndex, testCase.ticker), func(t *testing.T) {
					// And a currency
					currency := tc.Must.GetCurrency(t, testCase.ticker)

					// And mocked resources
					testCase.setupMocks(currency.Ticker, currency.Blockchain.String())

					// ACT
					// Change payment method
					res := tc.
						POST().WithCSRF().
						Path(methodRoute).
						Param(paymentapi.ParamPaymentID, p.PublicID.String()).
						JSON(&model.CreatePaymentMethodRequest{Ticker: currency.Ticker}).
						Do()

					// ASSERT
					// Returns response with specified currency ticker
					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())

					var body model.PaymentMethod
					assert.NoError(t, res.JSON(&body))
					assert.Equal(t, testCase.ticker, body.Ticker)
				})
			}
		})

		t.Run("Payment is not editable anymore", func(t *testing.T) {
			// ARRANGE
			merchantID := int64(1)
			ticker := "ABC"

			p := tc.CreateSamplePayment(t, merchantID)

			_, err := tc.Services.Payment.Update(tc.Context, p.MerchantID, p.ID, payment.UpdateProps{
				Status: payment.StatusInProgress,
			})
			require.NoError(t, err)

			// ACT
			res := tc.
				POST().WithCSRF().
				Path(methodRoute).
				Param(paymentapi.ParamPaymentID, p.PublicID.String()).
				JSON(&model.CreatePaymentMethodRequest{Ticker: ticker}).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "payment method is already set and locked")
		})
	})

	t.Run("LockPaymentOptions", func(t *testing.T) {
		tc.Clear.Wallets(t)
		mt, _ := tc.Must.CreateMerchant(t, 1)

		testCases := []struct {
			name     string
			customer string
			ticker   string
			error    bool
		}{
			{name: "no customer, no payment selected", error: true},
			{name: "no ticker selected", customer: "test@me.com", error: true},
			{name: "all good", customer: "test@me.com", ticker: "ETH"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				// ARRANGE
				p := tc.CreateSamplePayment(t, mt.ID)

				if testCase.customer != "" {
					_, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, testCase.customer)
					require.NoError(t, err)
				}

				if testCase.ticker != "" {
					tc.Providers.TatumMock.SetupRates(testCase.ticker, money.USD, 1)
					tc.SetupCreateWalletWithSubscription(testCase.ticker, "0x0f0f0f999", "pubkey-goes-here")

					_, err := tc.Services.Processing.SetPaymentMethod(tc.Context, p, testCase.ticker)
					require.NoError(t, err)
				}

				// ACT
				res := tc.
					PUT().WithCSRF().
					Path(paymentRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					Do()

				// ASSERT
				if testCase.error {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
					return
				}

				assert.Equal(t, http.StatusNoContent, res.StatusCode(), res.String())

				// check that status is updated
				getPaymentRes := tc.GET().Path(paymentRoute).Param(paymentapi.ParamPaymentID, p.PublicID.String()).Do()
				require.Equal(t, http.StatusOK, getPaymentRes.StatusCode())

				var pt model.Payment
				require.NoError(t, getPaymentRes.JSON(&pt))

				assert.Equal(t, payment.StatusPending.String(), pt.PaymentInfo.Status)
				assert.True(t, pt.IsLocked)
				require.Nil(t, pt.PaymentInfo.SuccessURL)
				require.NotEmpty(t, pt.PaymentInfo.ExpiresAt.String())
				require.NotEmpty(t, pt.PaymentInfo.ExpirationDurationMin)
				require.NotEmpty(t, pt.PaymentInfo.PaymentLink)

				// check that second multiple calls work
				res2 := tc.
					PUT().WithCSRF().
					Path(paymentRoute).
					Param(paymentapi.ParamPaymentID, p.PublicID.String()).
					Do()

				assert.Equal(t, http.StatusNoContent, res2.StatusCode())
			})
		}
	})
}
