package processing_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
func TestService_BatchCheckIncomingTransactions(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	// SETUP
	// Given a merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	// And several currencies
	eth := tc.Must.GetCurrency(t, "ETH")
	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")
	tron := tc.Must.GetCurrency(t, "TRON")
	bnb := tc.Must.GetCurrency(t, "BNB")
	bscUSDT := tc.Must.GetCurrency(t, "BSC_USDT")

	// Given shortcut for imitating incoming tx
	incomingTX := func(fiat money.FiatCurrency, price float64, crypto money.CryptoCurrency, isTest bool) *transaction.Transaction {
		// 0. create setup inbound wallet
		tc.Must.CreateWallet(t, crypto.Blockchain.String(), "0x123-inbound", "0x-pub-key", wallet.TypeInbound)

		// 1. create & lock payment
		merchantOrderID := uuid.New().String()

		amount, err := money.FiatFromFloat64(fiat, price)
		require.NoError(t, err)

		p, err := tc.Services.Payment.CreatePayment(tc.Context, mt.ID, payment.CreatePaymentProps{
			MerchantOrderUUID: uuid.New(),
			MerchantOrderID:   &merchantOrderID,
			Money:             amount,
			IsTest:            isTest,
		})

		require.NoError(t, err)

		_, err = tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, "user@me.com")
		require.NoError(t, err)

		// Assume conversion rate is 1 / 1
		tc.Providers.TatumMock.SetupRates(crypto.Ticker, money.FiatCurrency(p.Price.Ticker()), 1)

		// Assume that EUR is 1.1 USD
		tc.Providers.TatumMock.SetupRates(money.USD.String(), money.EUR, 0.91)
		tc.Providers.TatumMock.SetupRates(money.EUR.String(), money.USD, 1)

		method, err := tc.Services.Processing.SetPaymentMethod(tc.Context, p, crypto.Ticker)
		require.NoError(t, err)

		err = tc.Services.Processing.LockPaymentOptions(tc.Context, mt.ID, p.ID)
		require.NoError(t, err)

		// 2. Get tx
		tx, err := tc.Services.Transaction.GetByID(tc.Context, p.MerchantID, method.TransactionID)
		require.NoError(t, err)

		return tx
	}

	// Given a shortcut for imitating tx webhook processing
	whReceived := func(tx *transaction.Transaction, txHash string, factAmount money.Money, status transaction.Status) *transaction.Transaction {
		tx, err := tc.Services.Transaction.Receive(tc.Context, tx.MerchantID, tx.ID, transaction.ReceiveTransaction{
			Status:          status,
			SenderAddress:   fmt.Sprintf("0x123-sender-%d", time.Now().Unix()),
			TransactionHash: txHash,
			FactAmount:      factAmount,
			MetaData:        tx.MetaData,
		})
		require.NoError(t, err)

		if tx.Status == transaction.StatusInProgress {
			_, err = tc.Services.Payment.Update(tc.Context, mt.ID, tx.EntityID, payment.UpdateProps{Status: payment.StatusInProgress})
			require.NoError(t, err)
		}

		return tx
	}

	// Given a shortcut for receipt mocking
	makeReceipt := func(confirmations int64, isConfirmed, isSuccess bool) func(*transaction.Transaction) *blockchain.TransactionReceipt {
		return func(tx *transaction.Transaction) *blockchain.TransactionReceipt {
			coin := tc.Must.GetBlockchainCoin(t, tx.Currency.Blockchain)

			return &blockchain.TransactionReceipt{
				Blockchain:    tx.Currency.Blockchain,
				IsTest:        tx.IsTest,
				Sender:        *tx.SenderAddress,
				Recipient:     tx.RecipientAddress,
				Hash:          *tx.HashID,
				NetworkFee:    lo.Must(coin.MakeAmount("1000")),
				Success:       isSuccess,
				Confirmations: confirmations,
				IsConfirmed:   isConfirmed,
			}
		}
	}

	// Given a shortcut for loading wallet's and merchant's balances
	loadBalances := func(t *testing.T, tx *transaction.Transaction) (*wallet.Balance, *wallet.Balance) {
		ticker := tx.Currency.Ticker
		networkID := tx.NetworkID()

		walletBalance, err := tc.Services.Wallet.GetWalletsBalance(tc.Context, *tx.RecipientWalletID, ticker, networkID)
		require.NoError(t, err)

		merchantBalance, err := tc.Services.Wallet.GetMerchantBalance(tc.Context, mt.ID, ticker, networkID)
		require.NoError(t, err)

		return walletBalance, merchantBalance
	}

	assertUpdateStatusEventSent := func(t *testing.T, sent bool) {
		calls := tc.Fakes.GetBusCalls()
		if sent {
			require.Len(t, calls, 1)
			assert.Equal(t, bus.TopicPaymentStatusUpdate, calls[0].A)
		} else {
			require.Empty(t, calls)
		}
	}

	for _, testCase := range []struct {
		name        string
		isTest      bool
		transaction func(isTest bool) *transaction.Transaction
		receipt     func(tx *transaction.Transaction) *blockchain.TransactionReceipt
		assert      func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment)
	}{
		{
			name: "success ETH",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, eth, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "100", wtBalance.Amount.String())
				assert.Equal(t, "98.500000000000000056", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name:   "success ETH_USDT (testnet)",
			isTest: true,
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, ethUSDT, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "100", wtBalance.Amount.String())
				assert.Equal(t, "98.500001", mtBalance.Amount.String())
				assert.Equal(t, ethUSDT.TestNetworkID, wtBalance.NetworkID)
				assert.Equal(t, ethUSDT.TestNetworkID, mtBalance.NetworkID)

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "success BNB",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, bnb, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "100", wtBalance.Amount.String())
				assert.Equal(t, "98.500000000000000056", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name:   "success BNB (testnet)",
			isTest: true,
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 50, bnb, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("50_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "50", wtBalance.Amount.String())
				assert.Equal(t, "49.250000000000000028", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "success BSC USDT",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, bscUSDT, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())
				assert.Equal(t, "BNB", tx.NetworkFee.Ticker())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "100", wtBalance.Amount.String())
				assert.Equal(t, "98.500000000000000056", mtBalance.Amount.String())
				assert.Equal(t, "BSC_USDT", mtBalance.Currency)
				assert.Equal(t, "BSC", wtBalance.Network)

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "success TRON: network fee is not zero",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.EUR, 50, tron, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("50_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(5, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())
				assert.False(t, tx.NetworkFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "50", wtBalance.Amount.String())
				assert.Equal(t, "49.250001", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "success TRON: zero network fee",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.EUR, 50, tron, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("50_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: func(tx *transaction.Transaction) *blockchain.TransactionReceipt {
				receipt := makeReceipt(5, true, true)(tx)
				receipt.NetworkFee = lo.Must(tron.MakeAmount("0"))

				return receipt
			},
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.Equal(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())
				assert.True(t, tx.NetworkFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "50", wtBalance.Amount.String())
				assert.Equal(t, "49.250001", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "success ETH: customer paid more that expected",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, eth, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("120_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusSuccess, pt.Status)

				assert.Equal(t, transaction.StatusCompleted, tx.Status)
				assert.NotEqual(t, tx.Amount, *tx.FactAmount)
				assert.False(t, tx.ServiceFee.IsZero())

				wtBalance, mtBalance := loadBalances(t, tx)

				assert.Equal(t, "120", wtBalance.Amount.String())
				assert.Equal(t, "98.500000000000000056", mtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "error ETH: customer paid less than expected",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, eth, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("90_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgressInvalid)
			},
			receipt: makeReceipt(10, true, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusFailed, pt.Status)

				assert.Equal(t, transaction.StatusCompletedInvalid, tx.Status)
				assert.NotEqual(t, tx.Amount, *tx.FactAmount)
				assert.True(t, tx.ServiceFee.IsZero())

				ticker := tx.Currency.Ticker
				networkID := tx.NetworkID()

				wtBalance, err := tc.Services.Wallet.GetWalletsBalance(tc.Context, *tx.RecipientWalletID, ticker, networkID)
				require.NoError(t, err)

				assert.Equal(t, "90", wtBalance.Amount.String())

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
		{
			name: "ETH transaction is not confirmed yet",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, eth, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(1, false, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)
				assert.Equal(t, transaction.StatusInProgress, tx.Status)

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, false)
			},
		},
		{
			name: "BNB transaction is not confirmed yet",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, bnb, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(1, false, true),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusInProgress, pt.Status)
				assert.Equal(t, transaction.StatusInProgress, tx.Status)

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, false)
			},
		},
		{
			name: "transaction reverted by blockchain",
			transaction: func(isTest bool) *transaction.Transaction {
				tx := incomingTX(money.USD, 100, eth, isTest)
				factAmount := lo.Must(tx.Currency.MakeAmount("100_000_000_000_000_000_000"))

				return whReceived(tx, "0x123-hash-abc", factAmount, transaction.StatusInProgress)
			},
			receipt: makeReceipt(10, true, false),
			assert: func(t *testing.T, tx *transaction.Transaction, pt *payment.Payment) {
				assert.Equal(t, payment.StatusFailed, pt.Status)
				assert.Equal(t, transaction.StatusFailed, tx.Status)

				tc.AssertTableRows(t, "wallet_locks", 0)

				assertUpdateStatusEventSent(t, true)
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tc.Clear.Wallets(t)

			// ARRANGE
			// Given a transaction
			tx := testCase.transaction(testCase.isTest)

			// And cleared bus calls
			tc.Fakes.Bus.Clear()

			// And mocked tx receipt from blockchain
			receipt := testCase.receipt(tx)

			// And mocked transaction receipt
			tc.Fakes.SetupGetTransactionReceipt(tx.Currency.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

			// ACT
			err := tc.Services.Processing.BatchCheckIncomingTransactions(tc.Context, []int64{tx.ID})

			// ASSERT
			assert.NoError(t, err)

			// Load fresh tx
			tx, err = tc.Services.Transaction.GetByID(tc.Context, tx.MerchantID, tx.ID)
			assert.NoError(t, err)

			// Load fresh payment
			pt, err := tc.Services.Payment.GetByID(tc.Context, tx.MerchantID, tx.EntityID)
			assert.NoError(t, err)

			testCase.assert(t, tx, pt)
		})
	}

	t.Run("unexpected transactions", func(t *testing.T) {
		t.Run("Confirms unexpected transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given an incoming ETH_USDT 'in progress' transaction
			tx := incomingTX(money.USD, 100, ethUSDT, false)

			// Given locked wallet for that tx
			lockedWallet, err := tc.Services.Wallet.GetByID(tc.Context, *tx.RecipientWalletID)
			require.NoError(t, err)

			// Given webhook that represents unexpected incoming tx for locked wallet
			networkID := eth.ChooseNetwork(tx.IsTest)
			wh := webhook(lockedWallet.Address, "0x123456789", "ETH", "native", "2")

			tc.Providers.TatumMock.SetupRates(wh.Asset, money.USD, 1)
			require.NoError(t, tc.Services.Processing.ProcessIncomingWebhook(
				tc.Context,
				lockedWallet.UUID,
				networkID,
				*wh,
			))

			unexpectedTX, err := tc.Services.Transaction.GetByHash(tc.Context, networkID, wh.TransactionID)
			require.NoError(t, err)

			// Given mocked receipt for unexpected tx
			receipt := makeReceipt(10, true, true)(unexpectedTX)
			tc.Fakes.SetupGetTransactionReceipt(
				eth.Blockchain,
				*unexpectedTX.HashID,
				tx.IsTest,
				receipt,
				nil,
			)

			// ACT
			err = tc.Services.Processing.BatchCheckIncomingTransactions(tc.Context, []int64{unexpectedTX.ID})

			// ASSERT
			assert.NoError(t, err)

			// Check that tx & payment are untouched
			tx, err = tc.Services.Transaction.GetByID(tc.Context, tx.MerchantID, tx.ID)
			assert.NoError(t, err)

			pt, err := tc.Services.Payment.GetByID(tc.Context, tx.MerchantID, tx.EntityID)
			assert.NoError(t, err)

			// expected tx is untouched
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, payment.StatusLocked, pt.Status)

			// unexpected tx is confirmed
			unexpectedTX, err = tc.Services.Transaction.GetByID(tc.Context, transaction.MerchantIDWildcard, unexpectedTX.ID)
			assert.NoError(t, err)
			assert.Equal(t, transaction.StatusCompleted, unexpectedTX.Status)
			assert.Equal(t, wh.Amount, unexpectedTX.Amount.String())
			assert.Equal(t, unexpectedTX.Amount, *unexpectedTX.FactAmount)

			// wallet's balance is incremented
			wtBalance, err := tc.Services.Wallet.GetWalletsBalance(
				tc.Context,
				lockedWallet.ID,
				unexpectedTX.Currency.Ticker,
				unexpectedTX.NetworkID(),
			)
			require.NoError(t, err)
			assert.Equal(t, wh.Amount, wtBalance.Amount.String())
		})

		t.Run("Cancels unexpected transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given an incoming ETH_USDT 'in progress' transaction
			tx := incomingTX(money.USD, 100, ethUSDT, false)

			// Given locked wallet for that tx
			lockedWallet, err := tc.Services.Wallet.GetByID(tc.Context, *tx.RecipientWalletID)
			require.NoError(t, err)

			// Given webhook that represents unexpected incoming tx for locked wallet
			networkID := eth.ChooseNetwork(tx.IsTest)
			wh := webhook(lockedWallet.Address, "0x1234567890", "ETH", "native", "2")

			tc.Providers.TatumMock.SetupRates(wh.Asset, money.USD, 1)
			require.NoError(t, tc.Services.Processing.ProcessIncomingWebhook(
				tc.Context,
				lockedWallet.UUID,
				networkID,
				*wh,
			))

			unexpectedTX, err := tc.Services.Transaction.GetByHash(tc.Context, networkID, wh.TransactionID)
			require.NoError(t, err)

			// Given mocked receipt for unexpected tx
			receipt := makeReceipt(10, true, false)(unexpectedTX)
			tc.Fakes.SetupGetTransactionReceipt(
				eth.Blockchain,
				*unexpectedTX.HashID,
				tx.IsTest,
				receipt,
				nil,
			)

			// ACT
			err = tc.Services.Processing.BatchCheckIncomingTransactions(tc.Context, []int64{unexpectedTX.ID})

			// ASSERT
			assert.NoError(t, err)

			// Check that tx & payment are untouched
			tx, err = tc.Services.Transaction.GetByID(tc.Context, tx.MerchantID, tx.ID)
			assert.NoError(t, err)

			pt, err := tc.Services.Payment.GetByID(tc.Context, tx.MerchantID, tx.EntityID)
			assert.NoError(t, err)

			// expected tx is untouched
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, payment.StatusLocked, pt.Status)

			// unexpected tx is failed
			unexpectedTX, err = tc.Services.Transaction.GetByID(tc.Context, transaction.MerchantIDWildcard, unexpectedTX.ID)
			assert.NoError(t, err)
			assert.Equal(t, transaction.StatusFailed, unexpectedTX.Status)
		})
	})
}

func TestService_BatchExpirePayments(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	// ARRANGE
	// Given a merchant
	mt, _ := tc.Must.CreateMerchant(t, 1)

	// And several currencies
	eth := tc.Must.GetCurrency(t, "ETH")
	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")
	tron := tc.Must.GetCurrency(t, "TRON")

	// Given a shortcut for mocking incoming payment
	selectCurrency := func(p *payment.Payment, crypto money.CryptoCurrency, emulateCurrencySwitch, lock bool) {
		// 0. create setup inbound wallet
		tc.Must.CreateWallet(t, crypto.Blockchain.String(), test.RandomAddress, "0x-pub-key", wallet.TypeInbound)

		_, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, "user@me.com")
		require.NoError(t, err)

		// Assume conversion rate is 1 / 1
		tc.Providers.TatumMock.SetupRates(money.USD.String(), money.EUR, 1)

		// emulate "user has switched currencies several times"
		if emulateCurrencySwitch {
			for _, ticker := range []string{"ETH", "MATIC", "TRON"} {
				tc.Must.CreateWallet(t, ticker, test.RandomAddress, "0x-pub-key", wallet.TypeInbound)
				tc.Providers.TatumMock.SetupRates(ticker, money.FiatCurrency(p.Price.Ticker()), 1)
				_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, ticker)
				require.NoError(t, err)
			}
		}

		tc.Providers.TatumMock.SetupRates(crypto.Ticker, money.FiatCurrency(p.Price.Ticker()), 1)
		_, err = tc.Services.Processing.SetPaymentMethod(tc.Context, p, crypto.Ticker)
		require.NoError(t, err)

		if lock {
			err = tc.Services.Processing.LockPaymentOptions(tc.Context, mt.ID, p.ID)
			require.NoError(t, err)
		}
	}

	incomingPayment := func(fiat money.FiatCurrency, price float64, crypto money.CryptoCurrency, isTest bool) *payment.Payment {
		// 1. create & lock payment
		merchantOrderID := uuid.New().String()

		amount, err := money.FiatFromFloat64(fiat, price)
		require.NoError(t, err)

		p, err := tc.Services.Payment.CreatePayment(tc.Context, mt.ID, payment.CreatePaymentProps{
			MerchantOrderUUID: uuid.New(),
			MerchantOrderID:   &merchantOrderID,
			Money:             amount,
			IsTest:            isTest,
		})

		require.NoError(t, err)

		selectCurrency(p, crypto, true, true)

		return p
	}

	// Given a shortcut for setting expiration timestamp
	setExpiration := func(pt *payment.Payment, expiresAt time.Time) {
		_, err := tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
			ID:           pt.ID,
			MerchantID:   pt.MerchantID,
			Status:       pt.Status.String(),
			UpdatedAt:    time.Now(),
			ExpiresAt:    repository.TimeToNullable(expiresAt),
			SetExpiresAt: true,
		})
		require.NoError(t, err)
	}

	// Given a shortcut for altering created_at
	alterCreatedAt := func(dur time.Duration) func(*repository.CreatePaymentParams) {
		return func(create *repository.CreatePaymentParams) {
			create.CreatedAt = time.Now().Add(dur)
		}
	}

	// Given shortcut for loading fresh payment
	fresh := func(pt *payment.Payment) *payment.Payment {
		ptNew, err := tc.Services.Payment.GetByID(tc.Context, pt.MerchantID, pt.ID)
		require.NoError(t, err)

		return ptNew
	}

	loadTx := func(pt *payment.Payment) *transaction.Transaction {
		tx, err := tc.Services.Transaction.GetLatestByPaymentID(tc.Context, pt.ID)
		require.NoError(t, err)

		return tx
	}

	// Given several payments
	pt1 := tc.CreateSamplePayment(t, mt.ID)
	pt2 := incomingPayment(money.USD, 50, eth, false)    // should expire
	pt3 := incomingPayment(money.USD, 50, ethUSDT, true) // should expire
	pt4 := incomingPayment(money.USD, 100, tron, true)

	pt5Raw := tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-time.Hour))
	pt5, err := tc.Services.Payment.GetByID(tc.Context, pt5Raw.MerchantID, pt5Raw.ID)
	require.NoError(t, err)

	// Payment N6 emulates a situation when a customer selected currency, but left the payment "unlocked"
	pt6Raw := tc.CreateRawPayment(t, mt.ID, alterCreatedAt(-payment.ExpirationPeriodForNotLocked)) // should expire
	pt6, err := tc.Services.Payment.GetByID(tc.Context, pt6Raw.MerchantID, pt6Raw.ID)
	require.NoError(t, err)
	selectCurrency(pt6, eth, true, false)

	// Check that wallets are locked properly
	tc.AssertTableRows(t, "wallet_locks", 3+1)

	// And some of them are outdated
	setExpiration(pt2, time.Now())
	setExpiration(pt3, time.Now().Add(-time.Minute))

	// Given clear bus events
	tc.Fakes.Bus.Clear()

	// ACT
	err = tc.Services.Processing.BatchExpirePayments(tc.Context, []int64{pt2.ID, pt3.ID, pt6.ID})

	// ASSERT
	assert.NoError(t, err)

	// Check payments statuses
	assert.Equal(t, payment.StatusPending, fresh(pt1).Status)
	assert.Equal(t, payment.StatusFailed, fresh(pt2).Status)
	assert.Equal(t, payment.StatusFailed, fresh(pt3).Status)
	assert.Equal(t, payment.StatusLocked, fresh(pt4).Status)
	assert.Equal(t, payment.StatusPending, fresh(pt5).Status)
	assert.Equal(t, payment.StatusFailed, fresh(pt6).Status)

	// Check tx statuses
	assert.Equal(t, transaction.StatusCancelled, loadTx(pt2).Status)
	assert.Equal(t, transaction.StatusCancelled, loadTx(pt3).Status)
	assert.Equal(t, transaction.StatusPending, loadTx(pt4).Status)
	assert.Equal(t, transaction.StatusCancelled, loadTx(pt6).Status)

	tc.AssertTableRows(t, "wallet_locks", 1)

	// Check that webhooks were fired
	require.Len(t, tc.Fakes.GetBusCalls(), 3)
	assert.ElementsMatch(t,
		[]int64{pt2.ID, pt3.ID, pt6.ID},
		[]int64{
			tc.Fakes.GetBusCalls()[0].B.(bus.PaymentStatusUpdateEvent).PaymentID,
			tc.Fakes.GetBusCalls()[1].B.(bus.PaymentStatusUpdateEvent).PaymentID,
			tc.Fakes.GetBusCalls()[2].B.(bus.PaymentStatusUpdateEvent).PaymentID,
		},
	)
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
