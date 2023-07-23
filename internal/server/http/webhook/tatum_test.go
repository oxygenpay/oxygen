package webhook_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	webhookRoute   = "/api/webhook/v1/tatum/:networkId/:walletId"
	paramWalletID  = "walletId"
	paramNetworkID = "networkId"

	EthUsdAddress         = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	BSCUSDTAddress        = "0x55d398326f99059fF775485246999027B3197955"
	TronUsdAddressMainnet = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	TronUsdAddressTestnet = "TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs"

	typeCoin  = "native"
	typeToken = "token"
)

//nolint:funlen
//goland:noinspection GoBoolExpressions
func TestHandler_ReceiveTatum(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	mt, _ := tc.Must.CreateMerchant(t, 1)

	setupPayment := func(currency money.FiatCurrency, price float64) *payment.Payment {
		merchantOrderID := uuid.New().String()

		amount, err := money.FiatFromFloat64(currency, price)
		require.NoError(t, err)

		p, err := tc.Services.Payment.CreatePayment(tc.Context, mt.ID, payment.CreatePaymentProps{
			MerchantOrderUUID: uuid.New(),
			MerchantOrderID:   &merchantOrderID,
			Money:             amount,
			RedirectURL:       util.Ptr("https://example.com/redirect"),
			IsTest:            false,
		})

		require.NoError(t, err)

		_, err = tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, "user@me.com")
		require.NoError(t, err)

		return p
	}

	assertUpdateStatusEventSent := func(t *testing.T, sent bool) {
		calls := tc.Fakes.GetBusCalls()
		if sent {
			assert.Len(t, calls, 1)
			assert.Equal(t, bus.TopicPaymentStatusUpdate, calls[0].A)
		} else {
			assert.Empty(t, calls)
		}
	}

	for _, testCase := range []struct {
		name                string
		selectedCurrency    string
		payment             func() *payment.Payment
		modifyReceipt       func(r *blockchain.TransactionReceipt)
		customWalletIDParam uuid.UUID
		req                 func(*transaction.Transaction, *wallet.Wallet) *processing.TatumWebhook
		expectError         string
		assert              func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction)
	}{
		{
			name:             "success ETH",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX1", "ETH", typeCoin, "50.51")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "50.51", tx.FactAmount.String())
				assert.Equal(t, "0xTX1", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "success ETH_USDT",
			selectedCurrency: "ETH_USDT",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX2", EthUsdAddress, typeToken, "50")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "50", tx.FactAmount.String())
				assert.Equal(t, "0xTX2", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "success ETH: customer paid more that expected",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX3", "ETH", typeCoin, "60")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "60", tx.FactAmount.String())
				assert.Equal(t, "0xTX3", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)
				assert.NotEmpty(t, tx.MetaData[transaction.MetaComment])

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "error ETH: customer paid less than expected",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX4", "ETH", typeCoin, "45")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusLocked, pt.Status)

				assert.Equal(t, transaction.StatusInProgressInvalid, tx.Status)
				assert.Equal(t, "45", tx.FactAmount.String())
				assert.Equal(t, "0xTX4", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)
				assert.NotEmpty(t, tx.MetaData[transaction.MetaErrorReason])

				assertUpdateStatusEventSent(t, false)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "success ETH_USDT: customer paid less than expected but diff is less than $0.01",
			selectedCurrency: "ETH_USDT",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX5", EthUsdAddress, typeToken, "49.99")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "49.99", tx.FactAmount.String())
				assert.Equal(t, "0xTX5", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "error ETH_USDT: customer paid less than expected: diff more than $0.01",
			selectedCurrency: "ETH_USDT",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX6", EthUsdAddress, typeToken, "49.98")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusLocked, pt.Status)

				assert.Equal(t, transaction.StatusInProgressInvalid, tx.Status)
				assert.Equal(t, "49.98", tx.FactAmount.String())
				assert.Equal(t, "0xTX6", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, false)
			},
		},
		{
			name:             "success BSC_USDT",
			selectedCurrency: "BSC_USDT",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX_BSC", BSCUSDTAddress, typeToken, "50")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "50", tx.FactAmount.String())
				assert.Equal(t, "0xTX_BSC", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			// Imitation of Tatum's "weird" webhook when they send ticker instead of contract address
			name:             "success TRON_USDT",
			selectedCurrency: "TRON_USDT",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX7", "USDT_TRON", "trc20", "50")
			},
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)

				assert.Equal(t, transaction.StatusInProgress, tx.Status)
				assert.Equal(t, "50", tx.FactAmount.String())
				assert.Equal(t, "0xTX7", *tx.HashID)
				assert.Equal(t, "0x123sender456", *tx.SenderAddress)

				assertUpdateStatusEventSent(t, true)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "Unknown token",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX7", "0x123-wtf-token", typeToken, "123")
			},
			expectError: "unable to process tatum webhook",
			assert: func(t *testing.T, pt *payment.Payment, tx *transaction.Transaction) {
				assert.Equal(t, payment.StatusLocked, pt.Status)
				assert.Equal(t, transaction.StatusPending, tx.Status)

				assertUpdateStatusEventSent(t, false)

				tc.AssertTableRows(t, "wallet_locks", 0)
			},
		},
		{
			name:             "wallet not found",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX4", "ETH", typeCoin, "123")
			},
			customWalletIDParam: uuid.New(),
			expectError:         "unable to process tatum webhook",
		},
		{
			name:             "provided currency not found",
			selectedCurrency: "ETH",
			payment:          func() *payment.Payment { return setupPayment(money.USD, 50) },
			req: func(tx *transaction.Transaction, wt *wallet.Wallet) *processing.TatumWebhook {
				return webhook(wt.Address, "0xTX5", "0xABC123", typeToken, "123")
			},
			expectError: "unable to process tatum webhook",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// ARRANGE
			// Given a payment
			pt := testCase.payment()

			// And selected payment method
			blockchainName := tc.Must.GetCurrency(t, testCase.selectedCurrency).Blockchain.String()
			tc.SetupCreateWalletWithSubscription(blockchainName, test.RandomAddress, "0x123-pub-key")

			tc.Providers.TatumMock.SetupRates(testCase.selectedCurrency, money.FiatCurrency(pt.Price.Ticker()), 1)

			method, err := tc.Services.Processing.SetPaymentMethod(tc.Context, pt, testCase.selectedCurrency)
			require.NoError(t, err)

			// And successful lock acquired
			err = tc.Services.Processing.LockPaymentOptions(tc.Context, mt.ID, pt.ID)
			require.NoError(t, err)

			// Given fresh payment
			details, err := tc.Services.Processing.GetDetailedPayment(tc.Context, pt.MerchantID, pt.ID)
			require.NoError(t, err)

			pt = details.Payment

			// Given created transaction
			tx, err := tc.Services.Transaction.GetByID(tc.Context, pt.MerchantID, details.PaymentMethod.TransactionID)
			require.NoError(t, err)

			// Given locked wallet
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.RecipientWalletID)
			require.NoError(t, err)

			walletIDParam := wt.UUID.String()
			if testCase.customWalletIDParam != uuid.Nil {
				walletIDParam = testCase.customWalletIDParam.String()
			}

			makeRequest := func() *test.Response {
				return tc.
					POST().
					Path(webhookRoute).
					Param(paramWalletID, walletIDParam).
					Param(paramNetworkID, method.NetworkID).
					JSON(testCase.req(tx, wt)).
					Do()
			}

			// Given cleared bus calls
			tc.Fakes.Bus.Clear()

			// ACT
			res := makeRequest()

			// ASSERT
			// Check that error is expected
			if testCase.expectError != "" {
				assert.Equal(t, http.StatusBadRequest, res.StatusCode())
				assert.Contains(t, res.String(), testCase.expectError)
				return
			}

			// Check that no error occurred
			assert.Equal(t, http.StatusNoContent, res.StatusCode(), res.String())

			// Perform assertions
			freshPayment, err := tc.Services.Payment.GetByID(tc.Context, pt.MerchantID, pt.ID)
			require.NoError(t, err)

			freshTX, err := tc.Services.Transaction.GetByID(tc.Context, pt.MerchantID, method.TransactionID)
			require.NoError(t, err)

			testCase.assert(t, freshPayment, freshTX)

			t.Run("Tolerates duplicate webhooks", func(t *testing.T) {
				// ARRANGE
				// Given processed payment
				pt, err = tc.Services.Payment.GetByID(tc.Context, mt.ID, pt.ID)
				require.NoError(t, err)

				updatedAt := pt.UpdatedAt

				// ACT
				res := makeRequest()

				// ASSERT
				assert.Equal(t, http.StatusNoContent, res.StatusCode())

				// Check that payment wasn't modified
				pt, err = tc.Services.Payment.GetByID(tc.Context, mt.ID, pt.ID)
				assert.NoError(t, err)
				assert.Equal(t, updatedAt, pt.UpdatedAt)
			})
		})
	}

	t.Run("Handles unexpected transactions", func(t *testing.T) {
		// Setup exchange rates
		tc.Providers.TatumMock.SetupRates("ETH", money.USD, 1500)
		tc.Providers.TatumMock.SetupRates("ETH_USD", money.USD, 1)
		tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.07)
		tc.Providers.TatumMock.SetupRates("TRON_USDT", money.USD, 1)

		inboundWallet := func(blockchain, address string) func() *wallet.Wallet {
			return func() *wallet.Wallet {
				return tc.Must.CreateWallet(t, blockchain, address, "0x-pub-key", wallet.TypeInbound)
			}
		}

		outboundWallet := func(blockchain, address string) func() *wallet.Wallet {
			return func() *wallet.Wallet {
				return tc.Must.CreateWallet(t, blockchain, address, "0x-pub-key", wallet.TypeOutbound)
			}
		}

		txIsValid := func(t *testing.T, tx *transaction.Transaction, amount string, cur money.CryptoCurrency, networkID string) {
			// tx is created & completed
			assert.Equal(t, transaction.TypeIncoming, tx.Type)
			assert.Equal(t, transaction.StatusInProgress, tx.Status)
			assert.Equal(t, cur.Ticker, tx.Amount.Ticker())
			assert.Equal(t, networkID, tx.NetworkID())
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)
			assert.Equal(t, tx.Amount.String(), tx.FactAmount.String())
			assert.Equal(t, amount, tx.Amount.String())
			assert.Equal(t, "0x123sender456", *tx.SenderAddress)
			assert.Equal(t, "Unexpected transaction", tx.MetaData[transaction.MetaComment])
		}

		for _, testCase := range []struct {
			name          string
			currency      string
			wallet        func() *wallet.Wallet
			arrange       func(t *testing.T, wt *wallet.Wallet)
			isTest        bool
			modifyReceipt func(r *blockchain.TransactionReceipt)
			req           *processing.TatumWebhook
			assert        func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, c money.CryptoCurrency, networkID string)
			expectError   bool
		}{
			{
				name:     "unexpected tx: inbound wallet",
				currency: "ETH",
				isTest:   false,
				wallet:   inboundWallet("ETH", "0x123"),
				req:      webhook("0x123", "0x123tx-abc", "ETH", typeCoin, "0.1"),
				assert: func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, cur money.CryptoCurrency, networkID string) {
					txIsValid(t, tx, "0.1", cur, networkID)
					assert.Equal(t, "0x123tx-abc", *tx.HashID)

					assertUpdateStatusEventSent(t, false)
				},
			},
			{
				name:     "unexpected tx: inbound wallet: awaits for other currency",
				currency: "ETH",
				isTest:   false,
				wallet:   inboundWallet("ETH", "0x124"),
				arrange: func(t *testing.T, wt *wallet.Wallet) {
					// mock awaiting ETH_USDT incoming tx
					tc.Must.CreateTransaction(t, 1, func(p *transaction.CreateTransaction) {
						p.RecipientWallet = wt
						p.RecipientAddress = ""
					})
				},
				req: webhook("0x124", "0x1234tx-abc", "ETH", typeCoin, "0.2"),
				assert: func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, cur money.CryptoCurrency, networkID string) {
					txIsValid(t, tx, "0.2", cur, networkID)
					assert.Equal(t, "0x1234tx-abc", *tx.HashID)

					assertUpdateStatusEventSent(t, false)
				},
			},
			{
				name:     "unexpected tx: outbound wallet",
				currency: "TRON",
				isTest:   true,
				wallet:   outboundWallet("TRON", "0x123"),
				req:      webhook("0x123", "0x123tx-abc", "TRON", typeCoin, "400"),
				assert: func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, cur money.CryptoCurrency, networkID string) {
					txIsValid(t, tx, "400", cur, networkID)
					assert.Equal(t, "0x123tx-abc", *tx.HashID)

					assertUpdateStatusEventSent(t, false)
				},
			},
			{
				name:     "unexpected tx: outbound wallet: has other pending transactions",
				currency: "TRON_USDT",
				isTest:   true,
				wallet:   outboundWallet("TRON", "0x124"),
				arrange: func(t *testing.T, wt *wallet.Wallet) {
					mtID := transaction.SystemMerchantID
					isTest := true

					tron := tc.Must.GetCurrency(t, "TRON")

					withTRX := test.WithBalanceFromCurrency(tron, "123_000_000", isTest)
					inbound, _ := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, withTRX)

					// imitate another pending tx for outbound wallet
					tc.Must.CreateTransaction(t, mtID, func(p *transaction.CreateTransaction) {
						p.EntityID = 0
						p.SenderWallet = inbound
						p.RecipientWallet = wt
						p.Type = transaction.TypeInternal
						p.Currency = tron
						p.IsTest = isTest
						p.Amount = lo.Must(tron.MakeAmount("100_000_000"))
						p.USDAmount = lo.Must(money.FiatFromFloat64(money.USD, 100))
						p.ServiceFee = money.Money{}
					})
				},
				req: webhook("0x124", "0x124tx-abc", TronUsdAddressTestnet, typeToken, "400"),
				assert: func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, cur money.CryptoCurrency, networkID string) {
					txIsValid(t, tx, "400", cur, networkID)
					assert.Equal(t, "0x124tx-abc", *tx.HashID)

					assertUpdateStatusEventSent(t, false)
				},
			},
			{
				name:     "expected tx: outbound wallet: skips exact match",
				currency: "TRON_USDT",
				isTest:   false,
				wallet:   outboundWallet("TRON", "0x1250"),
				arrange: func(t *testing.T, wt *wallet.Wallet) {
					mtID := transaction.SystemMerchantID
					isTest := false

					tron := tc.Must.GetCurrency(t, "TRON")

					withTRX := test.WithBalanceFromCurrency(tron, "123_000_000", isTest)
					inbound, _ := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, withTRX)

					// pending tx for outbound wallet
					tx := tc.Must.CreateTransaction(t, mtID, func(p *transaction.CreateTransaction) {
						p.EntityID = 0
						p.SenderWallet = inbound
						p.RecipientWallet = wt
						p.Type = transaction.TypeInternal
						p.Currency = tron
						p.IsTest = isTest
						p.Amount = lo.Must(tron.MakeAmount("100_000_000"))
						p.USDAmount = lo.Must(money.FiatFromFloat64(money.USD, 100))
						p.ServiceFee = money.Money{}
					})

					err := tc.Services.Transaction.UpdateTransactionHash(tc.Context, mtID, tx.ID, "0x1250-abc")
					require.NoError(t, err)
				},
				req: webhook("0x1250", "0x1250-abc", TronUsdAddressMainnet, typeToken, "400"),
				assert: func(t *testing.T, wt *wallet.Wallet, tx *transaction.Transaction, cur money.CryptoCurrency, networkID string) {
					// tx is untouched
					assert.Equal(t, transaction.TypeInternal, tx.Type)
					assert.Equal(t, transaction.StatusPending, tx.Status)
					assert.Equal(t, "0x1250-abc", *tx.HashID)

					assertUpdateStatusEventSent(t, false)
				},
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				tc.Clear.Wallets(t)

				// ARRANGE
				// Given a wallet
				wt := testCase.wallet()

				// Given a currency
				currency := tc.Must.GetCurrency(t, testCase.currency)
				networkID := currency.ChooseNetwork(testCase.isTest)

				// Given receipt
				baseCurrency := tc.Must.GetCurrency(t, wt.Blockchain.String())

				// Given custom arrange logic
				if testCase.arrange != nil {
					testCase.arrange(t, wt)
				}

				receipt := &blockchain.TransactionReceipt{
					Blockchain:    wt.Blockchain.ToMoneyBlockchain(),
					IsTest:        testCase.isTest,
					Sender:        testCase.req.Sender,
					Recipient:     wt.Address,
					Hash:          testCase.req.TransactionID,
					NetworkFee:    lo.Must(baseCurrency.MakeAmount("1000")),
					Success:       true,
					Confirmations: 20,
					IsConfirmed:   true,
				}

				if testCase.modifyReceipt != nil {
					testCase.modifyReceipt(receipt)
				}

				tc.Fakes.SetupGetTransactionReceipt(
					wt.Blockchain.ToMoneyBlockchain(),
					testCase.req.TransactionID,
					testCase.isTest,
					receipt,
					nil,
				)

				// ACT
				res := tc.
					POST().
					Path(webhookRoute).
					Param(paramWalletID, wt.UUID.String()).
					Param(paramNetworkID, networkID).
					JSON(testCase.req).
					Do()

				// ASSERT
				if testCase.expectError {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode())
					return
				}

				// Check that tx exists
				assert.Equal(t, http.StatusNoContent, res.StatusCode())

				tx, err := tc.Services.Transaction.GetByHash(tc.Context, networkID, testCase.req.TransactionID)
				assert.NoError(t, err)

				testCase.assert(t, wt, tx, currency, networkID)
			})
		}
	})

	// Description: TRON requires a separate transaction for an address activation of 1 trx.
	// Tatum sends 2 webhooks sequentially (unordered): 'top-up of 1 trx (account activated)', 'incoming tx of X trx'
	t.Run("Handles TRON account activations", func(t *testing.T) {
		tc.Clear.Wallets(t)
		tc.Clear.Table(t, "payments")
		tc.Clear.Table(t, "transactions")

		tron := tc.Must.GetCurrency(t, "TRON")
		currency := tron.Ticker
		networkID := tron.NetworkID

		// Given a payment
		pt := tc.CreatePayment(t, mt.ID, money.USD, 7)

		// With selected customer
		_, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, pt, "test@me.com")
		require.NoError(t, err)

		// And selected TRON currency (with wallet)
		wt := tc.Must.CreateWallet(t, currency, "TRON_ABC", "0x-pub-key", wallet.TypeInbound)
		require.NotNil(t, wt)

		tc.Providers.TatumMock.SetupRates(currency, money.USD, 0.07)
		_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, pt, currency)
		require.NoError(t, err)

		// And "locked" state
		require.NoError(t, tc.Services.Processing.LockPaymentOptions(tc.Context, mt.ID, pt.ID))
		tc.AssertTableRows(t, "wallet_locks", 1)

		// Given 2 webhooks
		webhooks := []*processing.TatumWebhook{
			webhook(wt.Address, "0xabc_1", currency, typeCoin, "0.000001"), // "account activated"
			webhook(wt.Address, "0xabc_2", currency, typeCoin, "100"),      // "incoming payment"
		}

		req := func(wh *processing.TatumWebhook) *test.Response {
			return tc.
				POST().
				Path(webhookRoute).
				Param(paramWalletID, wt.UUID.String()).
				Param(paramNetworkID, networkID).
				JSON(wh).
				Do()
		}

		// ACT
		tc.Fakes.Clear()

		// Receive 2 webhooks
		for _, wh := range webhooks {
			res := req(wh)
			require.Equal(t, http.StatusNoContent, res.StatusCode())
		}

		// ASSERT
		// Unexpected TX is created and marked inProgress even if it was received before expected TX.
		unexpectedTX, err := tc.Services.Transaction.GetByHash(tc.Context, networkID, "0xabc_1")
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusInProgress, unexpectedTX.Status)
		assert.Equal(t, int64(0), unexpectedTX.EntityID)

		// Expected TX is marked inProgress
		expectedTX, err := tc.Services.Transaction.GetByHash(tc.Context, networkID, "0xabc_2")
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusInProgress, expectedTX.Status)
		assert.Equal(t, pt.ID, expectedTX.EntityID)

		// Check events and transactions tables
		require.Len(t, tc.Fakes.GetBusCalls(), 1)
		assert.Equal(t, bus.TopicPaymentStatusUpdate, tc.Fakes.GetBusCalls()[0].A)
		tc.AssertTableRows(t, "transactions", 2)

		// Wallets should have no locks
		tc.AssertTableRows(t, "wallet_locks", 0)
	})
}

// warn: chain and mempool props are omitted
func webhook(toAddress, txID, asset, txType, amount string) *processing.TatumWebhook {
	return &processing.TatumWebhook{
		SubscriptionType: "ADDRESS_TRANSACTION",
		TransactionID:    txID,
		Sender:           "0x123sender456",
		Address:          toAddress,
		Asset:            asset,
		BlockNumber:      1,
		Type:             txType,
		Amount:           strings.ReplaceAll(amount, "_", ""),
	}
}
