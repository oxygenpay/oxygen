package processing_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetPaymentMethod for that assertion-intensive test we need separate test context
//
//nolint:funlen
func TestSetPaymentMethod(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	t.Run("Payment is not editable anymore", func(t *testing.T) {
		// ARRANGE
		mt, _ := tc.Must.CreateMerchant(t, 1)
		ticker := "ABC"

		p := tc.CreateSamplePayment(t, mt.ID)

		_, err := tc.Services.Payment.Update(tc.Context, p.MerchantID, p.ID, payment.UpdateProps{
			Status: payment.StatusInProgress,
		})
		require.NoError(t, err)

		// ACT
		_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)

		// ASSERT
		assert.Error(t, err)
	})

	t.Run("Changes payment methods", func(t *testing.T) {
		// ARRANGE
		// Given a payments
		const (
			ethAddress      = "0x123"
			ethPubKey       = "eth-pub-key-goes-here"
			ethMainnetSubID = "eth_mainnet_sub_1"
			ethTestnetSubID = "eth_testnet_sub_1"

			maticAddress      = "0x999"
			maticPubKey       = "matic-pub-key-goes-here"
			maticMainnetSubID = "matic_mainnet_sub_1"
			maticTestnetSubID = "matic_testnet_sub_1"
		)

		mt, _ := tc.Must.CreateMerchant(t, 1)
		merchantID := mt.ID

		// Setup exchange rates
		tc.Providers.TatumMock.SetupRates("ETH", money.USD, 1)
		tc.Providers.TatumMock.SetupRates("ETH_USDT", money.USD, 1)
		tc.Providers.TatumMock.SetupRates("TRON_USDT", money.USD, 1)
		tc.Providers.TatumMock.SetupRates("MATIC", money.USD, 1)
		tc.Providers.TatumMock.SetupRates("ETH_USDT", money.EUR, 1)

		// Assume 1 EUR = $1.1
		tc.Providers.TatumMock.SetupRates(money.USD.String(), money.EUR, 0.91)
		tc.Providers.TatumMock.SetupRates(money.EUR.String(), money.USD, 1.1)

		p1 := tc.CreatePayment(t, merchantID, money.USD, 35.50)
		p2 := tc.CreatePayment(t, merchantID, money.EUR, 180)
		p3, _ := tc.Services.Payment.CreatePayment(tc.Context, merchantID, payment.CreatePaymentProps{
			MerchantOrderUUID: uuid.New(),
			Money:             p1.Price,
			IsTest:            true,
		})

		var transactionIDs, walletIDs []int64

		for testCaseIndex, testCase := range []struct {
			ticker              string
			payment             *payment.Payment
			setupMocks          func(ticker, blockchain string)
			expectedServiceFee  string
			expectedCryptoPrice string
			assertTransactions  func(t *testing.T, ids []int64)
			assertWallets       func(t *testing.T, ids []int64)
		}{
			{
				payment: p1,
				ticker:  "ETH",
				// $35.50 * 1 * 1.5% = 0.5325 -> "532499999999999980" :)
				expectedServiceFee: "532_499_999_999_999_980",
				// $35.50 * 1  = 35.50
				expectedCryptoPrice: "35_500_000_000_000_000_000",
				setupMocks: func(ticker, blockchain string) {
					tc.Providers.TatumMock.SetupSubscription(blockchain, ethAddress, false, ethMainnetSubID)
					tc.Providers.TatumMock.SetupSubscription(blockchain, ethAddress, true, ethTestnetSubID)
					tc.SetupCreateWallet(blockchain, ethAddress, ethPubKey)
				},
			},
			{
				payment:             p1,
				ticker:              "ETH_USDT",
				expectedServiceFee:  "532_499",
				expectedCryptoPrice: "35_500_000",
				setupMocks: func(ticker, blockchain string) {
					// no new wallet creation
				},
			},
			{
				payment: p1,
				ticker:  "MATIC",
				// $35.50 * 1 * 1.5% = 0.5325 -> "532499999999999980" :)
				expectedServiceFee: "532_499_999_999_999_980",
				// $35.50 * 1  = 35.50
				expectedCryptoPrice: "35_500_000_000_000_000_000",
				setupMocks: func(ticker, blockchain string) {
					tc.Providers.TatumMock.SetupSubscription(blockchain, maticAddress, false, maticMainnetSubID)
					tc.Providers.TatumMock.SetupSubscription(blockchain, maticAddress, true, maticTestnetSubID)
					tc.SetupCreateWallet(blockchain, maticAddress, maticPubKey)
				},
			},
			{
				payment: p1,
				ticker:  "ETH",
				// $35.50 * 1 * 1.5% = 0.5325 -> "532499999999999980" :)
				expectedServiceFee: "532_499_999_999_999_980",
				// $35.50 * 1  = 35.50
				expectedCryptoPrice: "35_500_000_000_000_000_000",
				setupMocks: func(ticker, blockchain string) {
					// no new wallet creation
					tc.Providers.TatumMock.SetupRates(ticker, money.USD, 1)
				},
				assertTransactions: func(t *testing.T, ids []int64) {
					currentTX := ids[int64(len(ids)-1)]

					// All previous tx expect current one should be canceled.
					for _, id := range ids {
						tx, err := tc.Services.Transaction.GetByID(tc.Context, merchantID, id)
						assert.NoError(t, err)

						if id == currentTX {
							assert.Equal(t, transaction.StatusPending, tx.Status)
						} else {
							assert.Equal(t, transaction.StatusCancelled, tx.Status)
						}
					}

					// Check rows in the DB
					tc.AssertTableRowsByMerchant(t, merchantID, "payments", 3)
					tc.AssertTableRowsByMerchant(t, merchantID, "transactions", 4)
				},
				assertWallets: func(t *testing.T, ids []int64) {
					// Important!
					// first, second, and fourth wallets should be the same wallet.
					assert.Equal(t, ids[0], ids[1], ids)
					assert.Equal(t, ids[0], ids[3], ids)

					tc.AssertTableRows(t, "wallets", 2)
					tc.AssertTableRowsByMerchant(t, merchantID, "wallet_locks", 1)
				},
			},
			{
				payment:             p2,
				ticker:              "ETH_USDT",
				expectedServiceFee:  "2_699_999",
				expectedCryptoPrice: "180_000_000",
				setupMocks: func(ticker, blockchain string) {
					// no new wallet creation
				},
			},
			{
				payment: p2,
				ticker:  "MATIC",
				// eur 180 * 1 * 1.5% = "2_699_999_999_999_999_900" :)
				expectedServiceFee:  "2_699_999_999_999_999_900",
				expectedCryptoPrice: "180_000_000_000_000_000_000",
				setupMocks: func(ticker, blockchain string) {
					// no new wallet creation
					tc.Providers.TatumMock.SetupRates(ticker, money.EUR, 1)
					tc.Providers.TatumMock.SetupRates(money.USD.String(), money.EUR, 1)
				},
				assertWallets: func(t *testing.T, ids []int64) {
					tc.AssertTableRows(t, "wallets", 2)
					tc.AssertTableRowsByMerchant(t, merchantID, "wallet_locks", 2)
				},
			},
			// Check that .IsTest behaves as expected. As we use test network for ETH,
			// we have one free wallet for that -> no wallet creation needed.
			{
				payment: p3,
				ticker:  "ETH",
				// $35.50 * 1 * 1.5% = 0.5325 -> "532499999999999980" :)
				expectedServiceFee: "532_499_999_999_999_980",
				// $35.50 * 1  = 35.50
				expectedCryptoPrice: "35_500_000_000_000_000_000",
				setupMocks: func(ticker, blockchain string) {
					// no new wallet creation
					tc.Providers.TatumMock.SetupSubscription(blockchain, ethAddress, false, ethMainnetSubID)
					tc.Providers.TatumMock.SetupSubscription(blockchain, ethAddress, true, ethTestnetSubID)
				},
				// So, at the end we should have 2 wallets: ETH and MATIC.
				// ETH wallet should be subscribed to both mainnet and testnet,
				// while MATIC - to both as well because we should always subscribe to mainnet&testnet
				assertWallets: func(t *testing.T, ids []int64) {
					ethWallet, err := tc.Services.Wallet.GetByID(tc.Context, ids[0])
					require.NoError(t, err)
					assert.Equal(t, kms.ETH, ethWallet.Blockchain)
					assert.Equal(t, ethMainnetSubID, ethWallet.TatumSubscription.MainnetSubscriptionID)
					assert.Equal(t, ethTestnetSubID, ethWallet.TatumSubscription.TestnetSubscriptionID)

					maticWallet, err := tc.Services.Wallet.GetByID(tc.Context, ids[2])
					require.NoError(t, err)
					assert.Equal(t, kms.MATIC, maticWallet.Blockchain)
					assert.Equal(t, maticMainnetSubID, maticWallet.TatumSubscription.MainnetSubscriptionID)
					assert.Equal(t, maticTestnetSubID, maticWallet.TatumSubscription.TestnetSubscriptionID)
				},
			},
		} {
			t.Run(fmt.Sprintf("payment/%d/%s", testCaseIndex+1, testCase.ticker), func(t *testing.T) {
				// Given a currency
				currency := tc.Must.GetCurrency(t, testCase.ticker)

				// And mocked dependency services
				if testCase.setupMocks != nil {
					testCase.setupMocks(currency.Ticker, currency.Blockchain.String())
				}

				// ACT
				// Change payment method
				paymentMethod, errPayment := tc.Services.Processing.SetPaymentMethod(tc.Context, testCase.payment, testCase.ticker)
				require.NoError(t, errPayment)

				// Resolve payment's transaction
				tx, errTX := tc.Services.Transaction.GetLatestByPaymentID(tc.Context, testCase.payment.ID)
				require.NoError(t, errTX)

				// Resolve transaction's wallet
				require.NotNil(t, tx.RecipientWalletID)
				txWallet, errWallet := tc.Services.Wallet.GetByID(tc.Context, *tx.RecipientWalletID)
				require.NoError(t, errWallet)

				// ASSERT
				// Operation is successful
				assert.Equal(t, testCase.ticker, paymentMethod.Currency.Ticker)

				// Transaction is created
				assert.Equal(t, transaction.StatusPending, tx.Status)
				assert.Equal(t, transaction.TypeIncoming, tx.Type)
				assert.Equal(t, merchantID, tx.MerchantID)
				if testCase.payment.Price.Ticker() == money.USD.String() {
					assert.Equal(t, testCase.payment.Price, tx.USDAmount)
				}

				// Transaction's blockchain and tickers are valid
				assert.Equal(t, currency.Blockchain, tx.Currency.Blockchain)
				assert.Equal(t, currency.Ticker, tx.Currency.Ticker)
				assert.Equal(t, currency.Ticker, tx.Amount.Ticker())

				// Transaction follows payment's test predicate
				assert.Equal(t, testCase.payment.IsTest, tx.IsTest)
				if !testCase.payment.IsTest {
					assert.Equal(t, currency.NetworkID, tx.Currency.NetworkID)
				} else {
					assert.Equal(t, currency.TestNetworkID, tx.Currency.TestNetworkID)
				}

				// Crypto price and fee are valid
				expectedCryptoPrice := lo.Must(currency.MakeAmount(testCase.expectedCryptoPrice))
				expectedServiceFee := lo.Must(currency.MakeAmount(testCase.expectedServiceFee))

				assert.Equal(t, expectedCryptoPrice.StringRaw(), tx.Amount.StringRaw())
				assert.Equal(t, expectedServiceFee.StringRaw(), tx.ServiceFee.StringRaw())

				// Extra transactions' assertion is successful
				transactionIDs = append(transactionIDs, tx.ID)
				if testCase.assertTransactions != nil {
					testCase.assertTransactions(t, transactionIDs)
				}

				// Wallet is valid
				assert.Equal(t, tx.Currency.Blockchain, txWallet.Blockchain.ToMoneyBlockchain())
				assert.Equal(t, currency.Blockchain, txWallet.Blockchain.ToMoneyBlockchain())

				// Extra wallets' assertion is successful
				walletIDs = append(walletIDs, txWallet.ID)
				if testCase.assertWallets != nil {
					testCase.assertWallets(t, walletIDs)
				}
			})
		}

		t.Run("Tolerates concurrent calls", func(t *testing.T) {
			tc.Clear.Wallets(t)
			tc.Clear.Table(t, "transactions")

			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// Given a payment
			pt := tc.CreatePayment(t, mt.ID, money.USD, 100)

			// ACT change payment method concurrently 5 times
			var wg sync.WaitGroup
			const iterations = 5

			wg.Add(3 * iterations)
			for i := 0; i < iterations; i++ {
				tc.SetupCreateWalletWithSubscription("ETH", test.RandomAddress, "0x123-"+strconv.Itoa(i))
				tc.SetupCreateWalletWithSubscription("TRON", test.RandomAddress, "0x124-"+strconv.Itoa(i))
				tc.SetupCreateWalletWithSubscription("MATIC", test.RandomAddress, "0x125-"+strconv.Itoa(i))

				go func() {
					_, err := tc.Services.Processing.SetPaymentMethod(tc.Context, pt, "ETH")
					wg.Done()
					assert.NoError(t, err, "ETH")
				}()

				go func() {
					_, err := tc.Services.Processing.SetPaymentMethod(tc.Context, pt, "TRON_USDT")
					wg.Done()
					assert.NoError(t, err, "ETH_USDT")
				}()

				go func() {
					_, err := tc.Services.Processing.SetPaymentMethod(tc.Context, pt, "MATIC")
					wg.Done()
					assert.NoError(t, err, "MATIC")
				}()
			}

			wg.Wait()

			// ASSERT
			// List all transactions for the payment.
			txs, err := tc.Services.Transaction.ListByFilter(tc.Context, transaction.Filter{}, 100)
			assert.NoError(t, err)

			statuses := map[transaction.Status]int{}
			for _, tx := range txs {
				statuses[tx.Status]++

				assert.Equal(t, mt.ID, tx.MerchantID)
				assert.Equal(t, transaction.TypeIncoming, tx.Type)
			}

			// Only one tx should be pending and all others should be canceled
			assert.Equal(t, 1, statuses[transaction.StatusPending])
			assert.Equal(t, len(txs)-1, statuses[transaction.StatusCancelled])

			// Wallet locks should contain only 1 lock
			tc.AssertTableRows(t, "wallet_locks", 1)

			// Wallets table should only have 3 wallets: ETH, TRON, MATIC
			tc.AssertTableRows(t, "wallets", 3)
		})
	})
}
