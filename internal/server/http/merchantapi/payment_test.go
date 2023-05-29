package merchantapi_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	paramMerchantID = "merchantId"
	paramPaymentID  = "paymentId"
	paramAddressID  = "addressId"

	queryParamBalanceID = "balanceId"
	queryParamType      = "type"
)

//nolint:funlen
func TestPaymentRoutes(t *testing.T) {
	const (
		paymentsRoute = "/api/dashboard/v1/merchant/:merchantId/payment"
		paymentRoute  = "/api/dashboard/v1/merchant/:merchantId/payment/:paymentId"
	)

	tc := test.NewIntegrationTest(t)

	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")

	// ARRANGE
	// Given a user
	user, token := tc.Must.CreateSampleUser(t)

	// And a merchant
	mt, _ := tc.Must.CreateMerchant(t, user.ID)

	t.Run("List payments", func(t *testing.T) {
		// And many payments
		var paymentIDs []uuid.UUID
		for i := 0; i < 50; i++ {
			price, err := money.FiatFromFloat64(money.USD, 95.4)
			require.NoError(t, err)

			pt, err := tc.Services.Payment.CreatePayment(tc.Context, mt.ID, payment.CreatePaymentProps{
				MerchantOrderUUID: uuid.New(),
				Money:             price,
			})
			require.NoError(t, err)

			paymentIDs = append(paymentIDs, pt.MerchantOrderUUID)
		}

		// With several withdrawals
		var withdrawalsIDs []uuid.UUID
		for i := 0; i < 5; i++ {
			pt, err := tc.Repository.CreatePayment(tc.Context, repository.CreatePaymentParams{
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
				Type:              string(payment.TypeWithdrawal),
				Status:            string(payment.StatusPending),
				MerchantID:        mt.ID,
				MerchantOrderUuid: uuid.New(),
				Price:             repository.MoneyToNumeric(lo.Must(ethUSDT.MakeAmount("123_000_000"))),
				Decimals:          int32(ethUSDT.Decimals),
				Currency:          ethUSDT.Ticker,
				RedirectUrl:       "https://site.com",
				Metadata:          pgtype.JSONB{Status: pgtype.Null},
			})
			require.NoError(t, err)

			withdrawalsIDs = append(withdrawalsIDs, pt.MerchantOrderUuid)
		}

		makeRequest := func(limit, cursor string, reserve bool, filterByType string) *test.Response {
			return tc.Client.
				GET().
				Path(paymentsRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Query(common.ParamQueryLimit, limit).
				Query(common.ParamQueryCursor, cursor).
				Query(common.ParamQueryReserveOrder, strconv.FormatBool(reserve)).
				Query(queryParamType, filterByType).
				Do()
		}

		for i, tt := range []struct {
			limit          string
			cursor         string
			reverseOrder   bool
			filterByType   string
			assertResponse func(t *testing.T, body model.PaymentsPagination)
		}{
			{
				limit:        "50",
				cursor:       "",
				reverseOrder: false,
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(50), body.Limit)
					assert.Equal(t, withdrawalsIDs[0].String(), body.Cursor)
					assert.Equal(t, 50, len(body.Results))
					assert.Equal(t, paymentIDs[0].String(), body.Results[0].ID)
				},
			},
			{
				// Default limit (30)
				limit:        "",
				cursor:       "",
				reverseOrder: false,
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, paymentIDs[30].String(), body.Cursor)
					assert.Equal(t, 30, len(body.Results))

					res := makeRequest("30", body.Cursor, false, "")

					// Check that second page contains 20 items and cursor equals to ""
					assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

					var body2 model.PaymentsPagination
					assert.NoError(t, res.JSON(&body2))
					assert.Equal(t, 25, len(body2.Results))
					assert.Equal(t, "", body2.Cursor)
				},
			},
			{
				limit:        "55",
				cursor:       "",
				reverseOrder: true,
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(55), body.Limit)
					assert.Equal(t, "", body.Cursor)
					assert.Equal(t, 55, len(body.Results))
					assert.Equal(t, withdrawalsIDs[4].String(), body.Results[0].ID)
				},
			},
			{
				// Default limit (30)
				limit:        "",
				cursor:       "",
				reverseOrder: true,
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, withdrawalsIDs[4].String(), body.Results[0].ID)
					assert.Equal(t, paymentIDs[24].String(), body.Cursor)
					assert.Equal(t, 30, len(body.Results))

					res := makeRequest("30", body.Cursor, true, "")

					// Check that second page contains 25 items and cursor equals to ""
					assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

					var body2 model.PaymentsPagination
					assert.NoError(t, res.JSON(&body2))
					assert.Equal(t, 25, len(body2.Results))
					assert.Equal(t, "", body2.Cursor)
					assert.Equal(t, paymentIDs[24].String(), body2.Results[0].ID)
				},
			},
			{
				// Filter by payments only
				limit:        "",
				cursor:       "",
				reverseOrder: true,
				filterByType: "payment",
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, paymentIDs[49].String(), body.Results[0].ID)
				},
			},
			{
				// Filter by withdrawals only
				limit:        "",
				cursor:       "",
				reverseOrder: true,
				filterByType: "withdrawal",
				assertResponse: func(t *testing.T, body model.PaymentsPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, withdrawalsIDs[4].String(), body.Results[0].ID)
					assert.Len(t, body.Results, len(withdrawalsIDs))
				},
			},
		} {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				// ACT
				// Paginate payments
				res := makeRequest(tt.limit, tt.cursor, tt.reverseOrder, tt.filterByType)

				// ASSERT
				assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

				var body model.PaymentsPagination
				assert.NoError(t, res.JSON(&body))
				tt.assertResponse(t, body)
			})
		}
	})

	t.Run("Get payment", func(t *testing.T) {
		t.Run("Returns validation error", func(t *testing.T) {
			// ACT
			res := tc.Client.
				GET().
				WithToken(token).
				Path(paymentRoute).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramPaymentID, "abc").
				Do()

			// ASSERT
			assert.Equal(t, res.StatusCode(), http.StatusBadRequest)
			assert.Contains(t, res.String(), "validation_error")
		})

		t.Run("Not found", func(t *testing.T) {
			res := tc.Client.
				GET().
				Path(paymentRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramPaymentID, uuid.New().String()).
				Do()

			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		})

		t.Run("Returns payment", func(t *testing.T) {
			// ARRANGE
			// Given a payment
			p := tc.CreateSamplePayment(t, mt.ID)

			res := tc.Client.
				GET().
				Path(paymentRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramPaymentID, p.MerchantOrderUUID.String()).
				Do()

			assert.Equal(t, http.StatusOK, res.StatusCode())

			var body model.Payment
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, p.MerchantOrderUUID.String(), body.ID)
		})
	})

	t.Run("CreatePayment", func(t *testing.T) {
		t.Run("Returns validation errors", func(t *testing.T) {
			// ARRANGE
			testCases := map[string]model.CreatePaymentRequest{
				"price in body is required": {
					Currency:    money.USD.String(),
					ID:          strfmt.UUID(uuid.New().String()),
					Price:       0,
					RedirectURL: util.Ptr("https://site.com"),
				},
				"price in body should be greater than or equal": {
					Currency:    money.USD.String(),
					ID:          strfmt.UUID(uuid.New().String()),
					Price:       -1,
					RedirectURL: util.Ptr("https://site.com"),
				},
				"currency in body should be one of": {
					Currency:    "RUB",
					ID:          strfmt.UUID(uuid.New().String()),
					Price:       1,
					RedirectURL: util.Ptr("https://site.com"),
				},
				"id in body must be of type uuid": {
					Currency:    money.USD.String(),
					ID:          "abc123",
					Price:       1,
					RedirectURL: util.Ptr("https://site.com"),
				},
				"invalid redirect url": {
					Currency:    money.USD.String(),
					ID:          strfmt.UUID(uuid.New().String()),
					Price:       1,
					RedirectURL: util.Ptr("https:///////site.com"),
				},
				"HTTPS": {
					Currency:    money.USD.String(),
					ID:          strfmt.UUID(uuid.New().String()),
					Price:       1,
					RedirectURL: util.Ptr("http://site.com"),
				},
			}

			for errorContains, req := range testCases {
				// ACT
				res := tc.Client.
					POST().
					WithToken(token).
					Path(paymentsRoute).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				assert.Contains(t, res.String(), "validation_error")
				assert.Contains(t, res.String(), errorContains)
			}
		})

		t.Run("Creates payment", func(t *testing.T) {
			// ARRANGE
			orderID := "order#123"
			description := "description test"
			req := model.CreatePaymentRequest{
				ID:          strfmt.UUID(uuid.New().String()),
				OrderID:     &orderID,
				Currency:    money.USD.String(),
				Price:       14.41,
				Description: &description,
				RedirectURL: util.Ptr("https://site.com"),
			}

			// ACT
			res := tc.Client.
				POST().
				WithToken(token).
				Path(paymentsRoute).
				Param(paramMerchantID, mt.UUID.String()).
				JSON(&req).
				Do()

			// ASSERT
			var body model.Payment

			assert.Equal(t, res.StatusCode(), http.StatusCreated)
			assert.NoError(t, res.JSON(&body))

			assert.NotEmpty(t, body.ID)
			assert.NotEmpty(t, body.PaymentURL)

			assert.Equal(t, body.Currency, money.USD.String())
			assert.Equal(t, "14.41", body.Price)

			assert.Equal(t, body.OrderID, req.OrderID)
			assert.Equal(t, body.Description, req.Description)
			assert.Equal(t, body.RedirectURL, *req.RedirectURL)

			assert.Equal(t, body.Status, payment.StatusPending.String())
			assert.Equal(t, body.Type, payment.TypePayment.String())
			assert.Equal(t, false, body.IsTest)

			t.Run("Prevents duplicate creation", func(t *testing.T) {
				// ACT
				res := tc.Client.
					POST().
					WithToken(token).
					Path(paymentsRoute).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				assert.Contains(t, res.String(), "payment already exists")
			})
		})

		t.Run("Creates test payment with default redirect url", func(t *testing.T) {
			// ARRANGE
			orderID := "order#123"
			description := "description test"
			req := model.CreatePaymentRequest{
				ID:          strfmt.UUID(uuid.New().String()),
				OrderID:     &orderID,
				Currency:    money.USD.String(),
				Price:       14.41,
				Description: &description,
				//RedirectURL: util.Ptr("https://site.com"),
				IsTest: true,
			}

			// ACT
			res := tc.Client.
				POST().
				WithToken(token).
				Path(paymentsRoute).
				Param(paramMerchantID, mt.UUID.String()).
				JSON(&req).
				Do()

			// ASSERT
			var body model.Payment

			assert.Equal(t, http.StatusCreated, res.StatusCode())
			assert.NoError(t, res.JSON(&body))

			assert.NotEmpty(t, body.ID)
			assert.NotEmpty(t, body.PaymentURL)

			assert.Equal(t, body.Currency, money.USD.String())
			assert.Equal(t, "14.41", body.Price)

			assert.Equal(t, body.OrderID, req.OrderID)
			assert.Equal(t, body.Description, req.Description)
			assert.Equal(t, mt.Website, body.RedirectURL)
			assert.Equal(t, true, body.IsTest)

			assert.Equal(t, payment.StatusPending.String(), body.Status)
			assert.Equal(t, payment.TypePayment.String(), body.Type)
		})
	})
}
