package merchantapi_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomerRoutes(t *testing.T) {
	const (
		customersRoute  = "/api/dashboard/v1/merchant/:merchantId/customer"
		customerRoute   = "/api/dashboard/v1/merchant/:merchantId/customer/:customerId"
		paramMerchantID = "merchantId"
		paramCustomerID = "customerId"
	)

	tc := test.NewIntegrationTest(t)

	// ARRANGE
	// Given a user
	user, token := tc.Must.CreateSampleUser(t)

	// And a merchant
	mt, _ := tc.Must.CreateMerchant(t, user.ID)

	t.Run("List customers", func(t *testing.T) {
		// And many customers
		var customerIDs []uuid.UUID
		for i := 0; i < 50; i++ {
			c := tc.Must.CreateCustomer(t, mt.ID, fmt.Sprintf("test-%d@me.com", i+1))

			customerIDs = append(customerIDs, c.UUID)
		}

		makeRequest := func(limit, cursor string, reserve bool) *test.Response {
			return tc.Client.
				GET().
				Path(customersRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Query(common.ParamQueryLimit, limit).
				Query(common.ParamQueryCursor, cursor).
				Query(common.ParamQueryReserveOrder, strconv.FormatBool(reserve)).
				Do()
		}

		for i, tt := range []struct {
			limit          string
			cursor         string
			reverseOrder   bool
			filterByType   string
			assertResponse func(t *testing.T, body model.CustomersPagination)
		}{
			{
				limit:        "49",
				cursor:       "",
				reverseOrder: false,
				assertResponse: func(t *testing.T, body model.CustomersPagination) {
					assert.Equal(t, int64(49), body.Limit)
					assert.Equal(t, customerIDs[49].String(), body.Cursor)
					assert.Equal(t, 49, len(body.Results))
					assert.Equal(t, customerIDs[0].String(), body.Results[0].ID)
				},
			},
			{
				// Default limit (30)
				limit:        "",
				cursor:       "",
				reverseOrder: false,
				assertResponse: func(t *testing.T, body model.CustomersPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, customerIDs[30].String(), body.Cursor)
					assert.Equal(t, 30, len(body.Results))

					res := makeRequest("30", body.Cursor, false)

					// Check that second page contains 20 items and cursor equals to ""
					assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

					var body2 model.CustomersPagination
					assert.NoError(t, res.JSON(&body2))
					assert.Equal(t, 20, len(body2.Results))
					assert.Equal(t, "", body2.Cursor)
				},
			},
			{
				limit:        "55",
				cursor:       "",
				reverseOrder: true,
				assertResponse: func(t *testing.T, body model.CustomersPagination) {
					assert.Equal(t, int64(55), body.Limit)
					assert.Equal(t, "", body.Cursor)
					assert.Equal(t, 50, len(body.Results))
					assert.Equal(t, customerIDs[49].String(), body.Results[0].ID)
				},
			},
			{
				// Default limit (30)
				limit:        "",
				cursor:       "",
				reverseOrder: true,
				assertResponse: func(t *testing.T, body model.CustomersPagination) {
					assert.Equal(t, int64(30), body.Limit)
					assert.Equal(t, customerIDs[49].String(), body.Results[0].ID)
					assert.Equal(t, customerIDs[19].String(), body.Cursor)
					assert.Equal(t, 30, len(body.Results))

					res := makeRequest("30", body.Cursor, true)

					// Check that second page contains 25 items and cursor equals to ""
					assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

					var body2 model.CustomersPagination
					assert.NoError(t, res.JSON(&body2))
					assert.Equal(t, 20, len(body2.Results))
					assert.Equal(t, "", body2.Cursor)
					assert.Equal(t, customerIDs[19].String(), body2.Results[0].ID)
				},
			},
		} {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				// ACT
				res := makeRequest(tt.limit, tt.cursor, tt.reverseOrder)

				// ASSERT
				assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

				var body model.CustomersPagination
				assert.NoError(t, res.JSON(&body))
				tt.assertResponse(t, body)
			})
		}
	})

	t.Run("Get customer", func(t *testing.T) {
		makeRequest := func(id uuid.UUID) *test.Response {
			return tc.Client.
				GET().
				Path(customerRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramCustomerID, id.String()).
				Do()
		}

		t.Run("Not found", func(t *testing.T) {
			// ACT
			res := makeRequest(uuid.New())

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		})

		t.Run("Empty details", func(t *testing.T) {
			// ASSERT
			c := tc.Must.CreateCustomer(t, mt.ID, "test@me.com")

			// ACT
			res := makeRequest(c.UUID)

			// ASSERT
			var body model.Customer
			assert.Equal(t, http.StatusOK, res.StatusCode())
			assert.NoError(t, res.JSON(&body))

			assert.Equal(t, c.Email, body.Email)
			assert.Equal(t, int64(0), body.Details.SuccessfulPayments)
			assert.Empty(t, body.Details.Payments)
		})

		t.Run("Full of details", func(t *testing.T) {
			// ASSERT
			// Given customer
			c := tc.Must.CreateCustomer(t, mt.ID, "test2@me.com")

			assign := func(pt *payment.Payment) {
				_, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, pt, c.Email)
				require.NoError(t, err)
			}

			// With several payments attached
			pt1 := tc.CreateSamplePayment(t, mt.ID)
			pt2 := tc.CreateSamplePayment(t, mt.ID)
			pt3 := tc.CreateSamplePayment(t, mt.ID)

			// And one successful payment
			_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
				ID:         pt3.ID,
				MerchantID: mt.ID,
				Status:     payment.StatusSuccess.String(),
				UpdatedAt:  time.Now(),
			})
			require.NoError(t, err)

			assign(pt1)
			assign(pt2)
			assign(pt3)

			// ACT
			res := makeRequest(c.UUID)

			// ASSERT
			var body model.Customer
			assert.Equal(t, http.StatusOK, res.StatusCode())
			assert.NoError(t, res.JSON(&body))

			assert.Equal(t, c.Email, body.Email)
			assert.Equal(t, int64(1), body.Details.SuccessfulPayments)

			require.Len(t, body.Details.Payments, 3)
			require.Equal(t, body.Details.Payments[0].ID, pt3.MerchantOrderUUID.String())
			require.Equal(t, body.Details.Payments[1].ID, pt2.MerchantOrderUUID.String())
			require.Equal(t, body.Details.Payments[2].ID, pt1.MerchantOrderUUID.String())
		})
	})
}
