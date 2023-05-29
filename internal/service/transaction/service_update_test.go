package transaction_test

import (
	"strconv"
	"testing"

	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Update(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	getBalance := func(t *testing.T, entityType string, entityID int64, tx *transaction.Transaction) repository.Balance {
		b, err := tc.Repository.GetBalanceByFilter(tc.Context, repository.GetBalanceByFilterParams{
			EntityType: entityType,
			EntityID:   entityID,
			NetworkID:  tx.NetworkID(),
			Currency:   tx.Currency.Ticker,
		})
		assert.NoError(t, err)

		return b
	}

	// PRE SETUP
	// Resolve currencies
	currencyUSDT, err := tc.Services.Blockchain.GetCurrencyByTicker("ETH_USDT")
	require.NoError(t, err)

	currencyETH, err := tc.Services.Blockchain.GetCurrencyByTicker("ETH")
	require.NoError(t, err)

	// Mock network fee
	networkFee := money.MustCryptoFromRaw(currencyETH.Ticker, "1", currencyETH.Decimals)

	// Create merchants
	mt1, _ := tc.Must.CreateMerchant(t, 1)
	mt2, _ := tc.Must.CreateMerchant(t, 1)

	// Create wallets
	wallet1 := tc.Must.CreateWallet(t, "ETH", "0x111", "pub-key", wallet.TypeInbound)
	wallet2 := tc.Must.CreateWallet(t, "ETH", "0x222", "pub-key", wallet.TypeInbound)

	for testCaseIndex, testCase := range []struct {
		merchantID int64
		txCreate   transaction.CreateTransaction
		txConfirm  transaction.ConfirmTransaction
		assert     func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money)
		after      func(t *testing.T)
	}{
		// wallet N1, merchant N1, USDT
		{
			merchantID: mt1.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        1,
				RecipientWallet: wallet1,
				Currency:        currencyUSDT,
				Amount:          createCrypto(t, currencyUSDT, "1_000_000"),
				ServiceFee:      createCrypto(t, currencyUSDT, "10_000"),
				USDAmount:       createUSD(t, 1),
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyUSDT, "1_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyUSDT, "1_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyUSDT, "990_000"), merchant)
			},
		},
		// wallet N1, merchant N2, USDT
		{
			merchantID: mt2.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        2,
				RecipientWallet: wallet1,
				Currency:        currencyUSDT,
				Amount:          createCrypto(t, currencyUSDT, "1_000_000"),
				ServiceFee:      createCrypto(t, currencyUSDT, "10_000"),
				USDAmount:       createUSD(t, 1),
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyUSDT, "1_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyUSDT, "2_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyUSDT, "990_000"), merchant)
			},
		},
		// wallet N2, merchant N2, USDT
		{
			merchantID: mt2.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        3,
				RecipientWallet: wallet2,
				Currency:        currencyUSDT,
				Amount:          createCrypto(t, currencyUSDT, "1_000_000"),
				ServiceFee:      createCrypto(t, currencyUSDT, "10_000"),
				USDAmount:       createUSD(t, 1),
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyUSDT, "1_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyUSDT, "1_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyUSDT, "1_980_000"), merchant)
			},
		},
		// wallet N1, merchant N1, ETH
		{
			merchantID: mt1.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        4,
				RecipientWallet: wallet1,
				Currency:        currencyETH,
				Amount:          createCrypto(t, currencyETH, "1_000_000_000"),
				ServiceFee:      createCrypto(t, currencyETH, "10_000_000"),
				USDAmount:       createUSD(t, 1),
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyETH, "1_000_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyETH, "1_000_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyETH, "990_000_000"), merchant)
			},
		},
		// wallet N1, merchant N1, ETH & IsTest == true
		{
			merchantID: mt1.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        5,
				RecipientWallet: wallet1,
				Currency:        currencyETH,
				Amount:          createCrypto(t, currencyETH, "1_000_000_000"),
				ServiceFee:      createCrypto(t, currencyETH, "10_000_000"),
				USDAmount:       createUSD(t, 1),
				IsTest:          true,
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyETH, "1_000_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyETH, "1_000_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyETH, "990_000_000"), merchant)
			},
		},
		// wallet N1, merchant N1, ETH & IsTest == true.
		// Fact amount is LESS than expected.
		{
			merchantID: mt1.ID,
			txCreate: transaction.CreateTransaction{
				Type:            transaction.TypeIncoming,
				EntityID:        6,
				RecipientWallet: wallet1,
				Currency:        currencyETH,
				Amount:          createCrypto(t, currencyETH, "1_000_000_000"),
				ServiceFee:      createCrypto(t, currencyETH, "0_010_000_000"),
				USDAmount:       createUSD(t, 2),
				IsTest:          true,
			},
			txConfirm: transaction.ConfirmTransaction{
				Status:          transaction.StatusCompleted,
				SenderAddress:   "0x124",
				TransactionHash: "0xfa123",
				NetworkFee:      networkFee,
				FactAmount:      createCrypto(t, currencyETH, "500_000_000"),
			},
			assert: func(t *testing.T, tx *transaction.Transaction, wallet, merchant money.Money) {
				moneyEqual(t, createCrypto(t, currencyETH, "1_500_000_000"), wallet)
				moneyEqual(t, createCrypto(t, currencyETH, "1_480_000_000"), merchant)
			},
		},
	} {
		name := strconv.Itoa(testCaseIndex + 1)
		t.Run(name, func(t *testing.T) {
			// ASSERT
			// Given a transaction
			tx, err := tc.Services.Transaction.Create(tc.Context, testCase.merchantID, testCase.txCreate)
			require.NoError(t, err)

			// ACT
			// Confirm transaction
			tx, err = tc.Services.Transaction.Confirm(tc.Context, testCase.merchantID, tx.ID, testCase.txConfirm)

			// ASSERT
			assert.NoError(t, err)

			// Perform balance assertions
			walletBalance := getBalance(t, "wallet", testCase.txCreate.RecipientWallet.ID, tx)
			merchantBalance := getBalance(t, "merchant", testCase.merchantID, tx)

			testCase.assert(t, tx, toMoney(walletBalance), toMoney(merchantBalance))
		})
	}
}

func createCrypto(t *testing.T, c money.CryptoCurrency, value string) money.Money {
	m, err := money.CryptoFromRaw(c.Ticker, value, c.Decimals)
	require.NoError(t, err)

	return m
}

func createUSD(t *testing.T, amount float64) money.Money {
	m, err := money.FiatFromFloat64(money.USD, amount)
	require.NoError(t, err)

	return m
}

func toMoney(b repository.Balance) money.Money {
	m, err := repository.NumericToMoney(b.Amount, money.Crypto, b.Currency, int64(b.Decimals))
	if err != nil {
		panic(err)
	}

	return m
}

func moneyEqual(t *testing.T, a, b money.Money) {
	assert.True(t, a.CompatibleTo(b))
	assert.Equal(t, a.StringRaw(), b.StringRaw())
}
