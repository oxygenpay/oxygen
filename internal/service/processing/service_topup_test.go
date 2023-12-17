package processing_test

import (
	"testing"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_TopupMerchantFromSystem(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	// ARRANGE
	// Given a currency
	tron := tc.Must.GetCurrency(t, "TRON")
	tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.1)

	// Given several wallets with balances
	// in total we should have 100 trx => $10 on mainnet and 50 trx => $5 on testnet
	_, b1 := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, test.WithBalanceFromCurrency(tron, "50_000_000", false))
	_, b2 := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeOutbound, test.WithBalanceFromCurrency(tron, "50_000_000", false))
	_, b3 := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, test.WithBalanceFromCurrency(tron, "50_000_000", true))

	// Given a function that ensures wallets balances are untouched
	ensureWalletBalances := func(t *testing.T) {
		for _, b := range []*wallet.Balance{b1, b2, b3} {
			fresh, err := tc.Services.Wallet.GetBalanceByID(tc.Context, wallet.EntityTypeWallet, b.EntityID, b.ID)
			require.NoError(t, err)
			assert.Equal(t, "50", fresh.Amount.String())
		}
	}

	// Given a merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	for _, tt := range []struct {
		name       string
		in         processing.TopupInput
		assert     func(t *testing.T, out processing.TopupOut)
		expectsErr string
	}{
		{
			name: "success: merchant balance does not exist yet in the db yet",
			in: processing.TopupInput{
				Currency: tron,
				Amount:   lo.Must(tron.MakeAmount("10_000_000")),
				Comment:  "hello world",
				IsTest:   false,
			},
			assert: func(t *testing.T, out processing.TopupOut) {
				assert.Equal(t, "1", out.Transaction.USDAmount.String())
				assert.Equal(t, "TRON", out.MerchantBalance.Amount.Ticker())
				assert.Equal(t, "10", out.MerchantBalance.Amount.String())
			},
		},
		{
			name: "success: merchant balance exists",
			in: processing.TopupInput{
				Currency: tron,
				Amount:   lo.Must(tron.MakeAmount("50_000_000")),
				Comment:  "hello world",
				IsTest:   false,
			},
			assert: func(t *testing.T, out processing.TopupOut) {
				assert.Equal(t, "5", out.Transaction.USDAmount.String())
				assert.Equal(t, "TRON", out.MerchantBalance.Amount.Ticker())
				assert.Equal(t, "60", out.MerchantBalance.Amount.String())
			},
		},
		{
			name: "success: works for test balances",
			in: processing.TopupInput{
				Currency: tron,
				Amount:   lo.Must(tron.MakeAmount("20_000_000")),
				Comment:  "hello world",
				IsTest:   true,
			},
			assert: func(t *testing.T, out processing.TopupOut) {
				assert.Equal(t, "2", out.Transaction.USDAmount.String())
				assert.Equal(t, "TRON", out.MerchantBalance.Amount.Ticker())
				assert.Equal(t, "20", out.MerchantBalance.Amount.String())
			},
		},
		{
			name: "fail: system balance is negative",
			in: processing.TopupInput{
				Currency: tron,
				Amount:   lo.Must(tron.MakeAmount("50_000_000")),
				Comment:  "one more time",
				IsTest:   false,
			},
			expectsErr: "system balance has no sufficient funds",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// ACT
			out, err := tc.Services.Processing.TopupMerchantFromSystem(tc.Context, mt.ID, tt.in)

			// ASSERT
			// Check that regardless of outcome, wallet balances remain the same
			ensureWalletBalances(t)

			// optionally expect an error
			if tt.expectsErr != "" {
				assert.ErrorContains(t, err, tt.expectsErr)
				return
			}

			assert.NoError(t, err)

			// check payment props
			assert.NotNil(t, out.Payment)
			assert.Equal(t, payment.StatusSuccess, out.Payment.Status)
			assert.Equal(t, payment.TypePayment, out.Payment.Type)
			assert.Contains(t, *out.Payment.Description, tt.in.Comment)
			assert.Equal(t, tt.in.IsTest, out.Payment.IsTest)

			// check tx props
			assert.NotNil(t, out.Transaction)
			assert.Equal(t, mt.ID, out.Transaction.MerchantID)
			assert.Equal(t, out.Payment.ID, out.Transaction.EntityID)

			assert.Equal(t, transaction.TypeVirtual, out.Transaction.Type)
			assert.Equal(t, transaction.StatusCompleted, out.Transaction.Status)

			assert.Empty(t, out.Transaction.SenderAddress)
			assert.Empty(t, out.Transaction.SenderWalletID)
			assert.Empty(t, out.Transaction.RecipientAddress)
			assert.Empty(t, out.Transaction.RecipientWalletID)
			assert.Empty(t, out.Transaction.HashID)

			assert.Equal(t, tt.in.Currency.Ticker, out.Transaction.Currency.Ticker)
			assert.Equal(t, tt.in.Amount.String(), out.Transaction.Amount.String())
			assert.Equal(t, tt.in.Amount.String(), out.Transaction.FactAmount.String())

			assert.True(t, out.Transaction.NetworkFee.IsZero())
			assert.True(t, out.Transaction.ServiceFee.IsZero())

			assert.Equal(t, tt.in.IsTest, out.Transaction.IsTest)

			// check balance props
			assert.NotNil(t, out.MerchantBalance)

			if tt.assert != nil {
				tt.assert(t, out)
			}
		})
	}
}
