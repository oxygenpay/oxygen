package merchantapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestBalanceRoutes(t *testing.T) {
	const (
		balancesRoute = "/api/dashboard/v1/merchant/:merchantId/balance"
	)

	tc := test.NewIntegrationTest(t)

	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")
	eth := tc.Must.GetCurrency(t, "ETH")
	matic := tc.Must.GetCurrency(t, "MATIC")

	tc.Providers.TatumMock.SetupRates(ethUSDT.Ticker, money.USD, 1)
	tc.Providers.TatumMock.SetupRates(eth.Ticker, money.USD, 1800)
	tc.Providers.TatumMock.SetupRates(matic.Ticker, money.USD, 1)

	// Given a user
	user, token := tc.Must.CreateUser(t, auth.GoogleUser{
		Name:          "John",
		Email:         "john@gmail.com",
		EmailVerified: true,
		Locale:        "ru",
	})

	t.Run("ListBalances", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, user.ID)

		withSmallUSDTBalance := test.WithBalanceFromCurrency(ethUSDT, "10", false)
		withEthBalance := test.WithBalanceFromCurrency(eth, "500_000_000_000_000_000", false)
		withMaticBalance := test.WithBalanceFromCurrency(matic, "1_500_000_000_000_000_000", true)

		// And merchant wallets
		b1 := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withSmallUSDTBalance)
		b2 := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withEthBalance)
		b3 := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withMaticBalance)

		// ACT
		// Get list of balances
		res := tc.Client.
			GET().
			Path(balancesRoute).
			Param(paramMerchantID, mt.UUID.String()).
			WithCSRF().
			WithToken(token).
			Do()

		// ASSERT
		var body model.MerchantBalanceList

		assert.Equal(t, http.StatusOK, res.StatusCode())
		assert.NoError(t, res.JSON(&body))

		assert.NotEqual(t, uuid.Nil.String(), body.Results[0].ID)
		assert.Equal(t, b3.UUID.String(), body.Results[0].ID)
		assert.Equal(t, "1.5", body.Results[0].Amount)
		assert.Equal(t, "MATIC", body.Results[0].Ticker)
		assert.Equal(t, "MATIC", body.Results[0].Currency)
		assert.Equal(t, "MATIC", body.Results[0].Blockchain)
		assert.Equal(t, "Polygon", body.Results[0].BlockchainName)
		assert.Equal(t, "MATIC (Polygon)", body.Results[0].Name)
		assert.Equal(t, "10", body.Results[0].MinimalWithdrawalAmountUSD)
		assert.Equal(t, "0", body.Results[0].UsdAmount) // test balances are always displayed as "0"
		assert.True(t, body.Results[0].IsTest)

		assert.NotEqual(t, uuid.Nil.String(), body.Results[1].ID)
		assert.Equal(t, b2.UUID.String(), body.Results[1].ID)
		assert.Equal(t, "0.5", body.Results[1].Amount)
		assert.Equal(t, "ETH", body.Results[1].Ticker)
		assert.Equal(t, "ETH", body.Results[1].Currency)
		assert.Equal(t, "ETH", body.Results[1].Blockchain)
		assert.Equal(t, "Ethereum", body.Results[1].BlockchainName)
		assert.Equal(t, "ETH (Ethereum)", body.Results[1].Name)
		assert.Equal(t, "40", body.Results[1].MinimalWithdrawalAmountUSD)
		assert.Equal(t, "900", body.Results[1].UsdAmount)
		assert.False(t, body.Results[1].IsTest)

		assert.NotEqual(t, uuid.Nil.String(), body.Results[2].ID)
		assert.Equal(t, b1.UUID.String(), body.Results[2].ID)
		assert.Equal(t, "0.00001", body.Results[2].Amount)
		assert.Equal(t, "ETH_USDT", body.Results[2].Ticker)
		assert.Equal(t, "USDT", body.Results[2].Currency)
		assert.Equal(t, "ETH", body.Results[2].Blockchain)
		assert.Equal(t, "Ethereum", body.Results[2].BlockchainName)
		assert.Equal(t, "USDT (Ethereum)", body.Results[2].Name)
		assert.Equal(t, "40", body.Results[2].MinimalWithdrawalAmountUSD)
		assert.Equal(t, "0", body.Results[2].UsdAmount) // < $0.01 is 0
		assert.False(t, body.Results[2].IsTest)
	})
}
