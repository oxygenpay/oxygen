package merchantapi_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
func TestWithdrawalRoutes(t *testing.T) {
	const (
		withdrawalsRoute   = "/api/dashboard/v1/merchant/:merchantId/withdrawal"
		withdrawalFeeRoute = "/api/dashboard/v1/merchant/:merchantId/withdrawal-fee"
	)

	tc := test.NewIntegrationTest(t)

	eth := tc.Must.GetCurrency(t, "ETH")
	matic := tc.Must.GetCurrency(t, "MATIC")

	user, token := tc.Must.CreateSampleUser(t)

	tc.Providers.TatumMock.SetupRates(eth.Ticker, money.USD, 1300)
	tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, false, lo.Must(money.USD.MakeAmount("300")))
	tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, true, lo.Must(money.USD.MakeAmount("300")))

	tc.Providers.TatumMock.SetupRates(matic.Ticker, money.USD, 1300)
	tc.Fakes.SetupCalculateWithdrawalFeeUSD(matic, matic, false, lo.Must(money.USD.MakeAmount("50")))
	tc.Fakes.SetupCalculateWithdrawalFeeUSD(matic, matic, true, lo.Must(money.USD.MakeAmount("50")))

	t.Run("CreateWithdrawal", func(t *testing.T) {
		t.Run("Happy path: creates withdrawal", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			// And address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "A1",
				Blockchain: "ETH",
				Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
			})
			require.NoError(t, err)

			// And specified withdrawal amount
			balanceETH := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
			amountETH := lo.Must(eth.MakeAmount("490_000_000_000_000_000"))

			// And balance
			withETH := test.WithBalanceFromCurrency(eth, balanceETH.StringRaw(), false)
			balance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withETH)

			// And a withdrawal request
			req := model.CreateWithdrawalRequest{
				AddressID: addr.UUID.String(),
				BalanceID: balance.UUID.String(),
				Amount:    amountETH.String(),
			}

			// ACT
			// Send request
			res := tc.Client.
				POST().
				Path(withdrawalsRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				JSON(&req).
				Do()

			// ASSERT
			var body model.Withdrawal

			// Check that response is expected
			assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
			assert.NoError(t, res.JSON(&body))
			assert.NotEqual(t, uuid.Nil.String(), body.PaymentID)

			publicID := uuid.MustParse(body.PaymentID)

			// Check payment
			r, err := tc.Services.Payment.GetByMerchantOrderIDWithRelations(tc.Context, mt.ID, publicID)
			assert.NoError(t, err)
			assert.Equal(t, payment.TypeWithdrawal, r.Payment.Type)
			assert.Equal(t, payment.StatusPending, r.Payment.Status)
			assert.Equal(t, amountETH, r.Payment.Price)
			assert.False(t, r.Payment.IsTest)

			// Check metadata
			assert.Equal(t, balance.ID, r.Balance.ID)
			assert.Equal(t, addr.ID, r.Address.ID)
		})

		t.Run("Fails", func(t *testing.T) {
			t.Run("Invalid request", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				req := model.CreateWithdrawalRequest{
					AddressID: "abc",
					BalanceID: "xyz",
					Amount:    "-1",
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Len(t, body.Errors, 3)
				assert.Equal(t, "balanceId", body.Errors[0].Field)
				assert.Equal(t, "addressId", body.Errors[1].Field)
				assert.Equal(t, "amount", body.Errors[2].Field)
			})

			t.Run("Address not found", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				req := model.CreateWithdrawalRequest{
					AddressID: uuid.New().String(),
					BalanceID: uuid.New().String(),
					Amount:    "0.123",
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, "addressId", body.Errors[0].Field)
				assert.Contains(t, body.Errors[0].Message, "not found")
			})

			t.Run("Balance not found", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
					Name:       "A1",
					Blockchain: "ETH",
					Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				})
				require.NoError(t, err)

				// And a request
				req := model.CreateWithdrawalRequest{
					AddressID: addr.UUID.String(),
					BalanceID: uuid.New().String(),
					Amount:    "0.123",
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, "balanceId", body.Errors[0].Field)
				assert.Contains(t, body.Errors[0].Message, "not found")
			})

			t.Run("Address and balance mismatch", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
					Name:       "A1",
					Blockchain: "MATIC",
					Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				})
				require.NoError(t, err)

				// And specified withdrawal amount
				amountETH := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
				require.NoError(t, err)

				// And balance
				withETH := test.WithBalanceFromCurrency(eth, amountETH.StringRaw(), false)
				balance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withETH)

				// And a withdrawal request
				req := model.CreateWithdrawalRequest{
					AddressID: addr.UUID.String(),
					BalanceID: balance.UUID.String(),
					Amount:    amountETH.String(),
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, "balanceId", body.Errors[0].Field)
				assert.Contains(t, body.Errors[0].Message, "balance does not match to address")
			})

			t.Run("Amount is too small", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
					Name:       "A1",
					Blockchain: "ETH",
					Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				})
				require.NoError(t, err)

				// And specified balance amount
				balanceETH := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
				require.NoError(t, err)

				// And balance
				withETH := test.WithBalanceFromCurrency(eth, balanceETH.StringRaw(), false)
				balance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withETH)

				// And a withdrawal request
				req := model.CreateWithdrawalRequest{
					AddressID: addr.UUID.String(),
					BalanceID: balance.UUID.String(),
					Amount:    "0.0005",
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, "amount", body.Errors[0].Field)
				assert.Contains(t, body.Errors[0].Message, "withdrawal amount is too small")
			})

			t.Run("Amount is too high", func(t *testing.T) {
				// ARRANGE
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// Given a request
				addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
					Name:       "A1",
					Blockchain: "MATIC",
					Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac991",
				})
				require.NoError(t, err)

				// And specified balance amount
				balanceMATIC := lo.Must(matic.MakeAmount("500_000_000_000_000_000"))
				require.NoError(t, err)

				// And balance
				withMATIC := test.WithBalanceFromCurrency(matic, balanceMATIC.StringRaw(), false)
				balance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withMATIC)

				// And a withdrawal request
				req := model.CreateWithdrawalRequest{
					AddressID: addr.UUID.String(),
					BalanceID: balance.UUID.String(),
					Amount:    "0.50001",
				}

				// ACT
				// Send request
				res := tc.Client.
					POST().
					Path(withdrawalsRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&req).
					Do()

				// ASSERT
				var body model.ErrorResponse
				assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, "amount", body.Errors[0].Field)
				assert.Contains(t, body.Errors[0].Message, "not enough funds")
			})
		})
	})

	t.Run("GetWithdrawalFee", func(t *testing.T) {
		asset := func(ticker string) money.CryptoCurrency {
			c, err := tc.Services.Blockchain.GetCurrencyByTicker(ticker)
			require.NoError(t, err)

			return c
		}

		usd := func(f float64) money.Money {
			return lo.Must(money.FiatFromFloat64(money.USD, f))
		}

		makeBalance := func(currency money.CryptoCurrency, isTest bool, usdFee money.Money) func(int64) *wallet.Balance {
			return func(merchantID int64) *wallet.Balance {
				b, err := tc.Services.Wallet.EnsureBalance(
					tc.Context,
					wallet.EntityTypeMerchant,
					merchantID,
					currency,
					isTest,
				)
				require.NoError(t, err)

				baseCurrency, err := tc.Services.Blockchain.GetNativeCoin(currency.Blockchain)
				require.NoError(t, err)

				tc.Providers.TatumMock.SetupRates(currency.Ticker, money.USD, 2)
				tc.Fakes.SetupCalculateWithdrawalFeeUSD(baseCurrency, currency, isTest, usdFee)

				return b
			}
		}

		t.Run("Returns fee", func(t *testing.T) {
			for testCaseIndex, testCase := range []struct {
				balance           func(merchantID int64) *wallet.Balance
				expectedFeeUSD    string
				expectedFeeCrypto string
			}{
				{
					balance:           makeBalance(asset("ETH"), false, usd(6)),
					expectedFeeUSD:    "6",
					expectedFeeCrypto: "3",
				},
				{
					balance:           makeBalance(asset("ETH_USDT"), false, usd(12)),
					expectedFeeUSD:    "12",
					expectedFeeCrypto: "6",
				},
				{
					balance:           makeBalance(asset("MATIC"), false, usd(1)),
					expectedFeeUSD:    "1",
					expectedFeeCrypto: "0.5",
				},
				{
					balance:           makeBalance(asset("TRON"), false, usd(1.5)),
					expectedFeeUSD:    "1.50",
					expectedFeeCrypto: "0.75",
				},
				{
					balance:           makeBalance(asset("TRON_USDT"), false, usd(3.65)),
					expectedFeeUSD:    "3.65",
					expectedFeeCrypto: "1.824999",
				},
				{
					balance:           makeBalance(asset("ETH"), false, usd(0.01)),
					expectedFeeUSD:    "0.01",
					expectedFeeCrypto: "0.005",
				},
				{
					balance:           makeBalance(asset("BNB"), false, usd(0.02)),
					expectedFeeUSD:    "0.02",
					expectedFeeCrypto: "0.01",
				},
				{
					// in testnets money "cost" $0
					balance:           makeBalance(asset("TRON"), true, usd(1.5)),
					expectedFeeUSD:    "0",
					expectedFeeCrypto: "0.75",
				},
			} {
				t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
					// ARRANGE
					// Given a merchant
					mt, _ := tc.Must.CreateMerchant(t, user.ID)

					// And balance
					balance := testCase.balance(mt.ID)

					// ACT
					res := tc.Client.
						GET().
						Path(withdrawalFeeRoute).
						WithToken(token).
						Param(paramMerchantID, mt.UUID.String()).
						Query(queryParamBalanceID, balance.UUID.String()).
						Do()

					// ASSERT
					var body model.WithdrawalFee

					assert.Equal(t, http.StatusOK, res.StatusCode())
					assert.NoError(t, res.JSON(&body))

					assert.Equal(t, balance.Network, body.Blockchain)
					assert.Equal(t, balance.Currency, body.Currency)
					assert.Equal(t, testCase.expectedFeeUSD, body.UsdFee)
					assert.Equal(t, testCase.expectedFeeCrypto, body.CurrencyFee)
				})
			}
		})

		t.Run("Balance not found", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			// ACT
			res := tc.Client.
				GET().
				Path(withdrawalFeeRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Query(queryParamBalanceID, uuid.New().String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "balance not found")
		})
	})
}
