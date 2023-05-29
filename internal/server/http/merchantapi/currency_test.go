package merchantapi_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
)

const (
	conversionRoute  = "/api/dashboard/v1/merchant/:merchantId/currency-convert"
	queryParamFrom   = "from"
	queryParamTo     = "to"
	queryParamAmount = "amount"
)

func TestHandler_GetCurrencyConvert(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	tc.Providers.TatumMock.SetupRates(money.EUR.String(), money.USD, 1.10)
	tc.Providers.TatumMock.SetupRates(money.USD.String(), money.EUR, 0.91)
	tc.Providers.TatumMock.SetupRates("ETH", money.USD, 1000)
	tc.Providers.TatumMock.SetupRates("ETH_USDT", money.USD, 1)
	tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.07)

	// Given a user and a merchant
	user, token := tc.Must.CreateUser(t, auth.GoogleUser{Name: "A", Email: "john@gmail.com"})
	mt, _ := tc.Must.CreateMerchant(t, user.ID)

	for _, tt := range []struct {
		from        string
		to          string
		amount      string
		expectError bool
		expected    model.Conversion
	}{
		{
			from:   "USD",
			to:     "USD",
			amount: "10",
			expected: model.Conversion{
				From:            "USD",
				FromType:        "fiat",
				To:              "USD",
				ToType:          "fiat",
				SelectedAmount:  "10",
				ConvertedAmount: "10",
				ExchangeRate:    1,
			},
		},
		{
			from:   "EUR",
			to:     "USD",
			amount: "100",
			expected: model.Conversion{
				From:            "EUR",
				FromType:        "fiat",
				To:              "USD",
				ToType:          "fiat",
				SelectedAmount:  "100",
				ConvertedAmount: "110",
				ExchangeRate:    1.1,
			},
		},
		{
			from:   "USD",
			to:     "EUR",
			amount: "100",
			expected: model.Conversion{
				From:            "USD",
				FromType:        "fiat",
				To:              "EUR",
				ToType:          "fiat",
				SelectedAmount:  "100",
				ConvertedAmount: "91",
				ExchangeRate:    0.91,
			},
		},
		{
			from:   "ETH",
			to:     "USD",
			amount: "2",
			expected: model.Conversion{
				From:            "ETH",
				FromType:        "crypto",
				To:              "USD",
				ToType:          "fiat",
				SelectedAmount:  "2",
				ConvertedAmount: "2000",
				ExchangeRate:    1000,
			},
		},
		{
			from:   "USD",
			to:     "ETH",
			amount: "500",
			expected: model.Conversion{
				From:            "USD",
				FromType:        "fiat",
				To:              "ETH",
				ToType:          "crypto",
				SelectedAmount:  "500",
				ConvertedAmount: "0.50000000000000001",
				ExchangeRate:    0.001,
			},
		},
		{
			from:   "USD",
			to:     "ETH_USDT",
			amount: "500",
			expected: model.Conversion{
				From:            "USD",
				FromType:        "fiat",
				To:              "ETH_USDT",
				ToType:          "crypto",
				SelectedAmount:  "500",
				ConvertedAmount: "500",
				ExchangeRate:    1,
			},
		},
		{
			// case-insensitive
			from:   "usd",
			to:     "eth_usdt",
			amount: "500",
			expected: model.Conversion{
				From:            "USD",
				FromType:        "fiat",
				To:              "ETH_USDT",
				ToType:          "crypto",
				SelectedAmount:  "500",
				ConvertedAmount: "500",
				ExchangeRate:    1,
			},
		},
		{from: "ETH", to: "TRON", amount: "1", expectError: true},
		{from: "ETH", to: "USD", amount: "0", expectError: true},
		{from: "USD", to: "ETH", amount: "0", expectError: true},
		{from: "a", to: "b", amount: "0", expectError: true},
	} {
		t.Run(fmt.Sprintf("%s/%s/%s", tt.from, tt.to, tt.amount), func(t *testing.T) {
			// ACT
			res := tc.Client.
				GET().
				Path(conversionRoute).
				Query(queryParamFrom, tt.from).
				Query(queryParamTo, tt.to).
				Query(queryParamAmount, tt.amount).
				Param(paramMerchantID, mt.UUID.String()).
				WithCSRF().
				WithToken(token).
				Do()

			// ASSERT
			if tt.expectError {
				assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				return
			}

			var body model.Conversion

			assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, tt.expected, body)
		})
	}
}
