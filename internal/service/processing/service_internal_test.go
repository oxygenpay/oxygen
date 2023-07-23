package processing_test

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	kmswallet "github.com/oxygenpay/oxygen/pkg/api-kms/v1/client/wallet"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
//goland:noinspection GoBoolExpressions
func TestService_BatchCreateInternalTransfers(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	ctx := tc.Context

	// Setup currencies
	eth := tc.Must.GetCurrency(t, "ETH")
	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")

	tron := tc.Must.GetCurrency(t, "TRON")
	tronUSDT := tc.Must.GetCurrency(t, "TRON_USDT")

	bnb := tc.Must.GetCurrency(t, "BNB")

	// Mock tx fees
	tc.Fakes.SetupAllFees(t, tc.Services.Blockchain)

	// Mock exchange rates
	tc.Providers.TatumMock.SetupRates("ETH", money.USD, 1600)
	tc.Providers.TatumMock.SetupRates("ETH_USDT", money.USD, 1)
	tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.066)
	tc.Providers.TatumMock.SetupRates("BNB", money.USD, 240)

	t.Run("Creates transactions", func(t *testing.T) {
		isTest := false

		t.Run("Creates ETH transaction", func(t *testing.T) {
			// ARRANGE
			// Given outbound ETH balance
			tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(eth, "0", isTest))

			// Given an inbound balance with 0.5 ETH
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "500_000_000_000_000_000", isTest))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Empty(t, result.TotalErrors)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)
			assert.NotEqual(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})

		t.Run("Creates ETH_USDT transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given outbound ETH_USDT balance
			tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(ethUSDT, "0", isTest))

			// Given an inbound balance with 100 ETH_USDT
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(ethUSDT, "100_000_000", isTest))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(ethUSDT.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Empty(t, result.TotalErrors)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)

			// For tokens, we should transfer 100% of crypto
			assert.Equal(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})

		t.Run("Creates TRON transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given outbound TRON balance
			tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeOutbound, withBalance(tron, "0", isTest))

			// Given an inbound balance with 1000 TRX
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, withBalance(tron, "1000_000_000", isTest))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateTronTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(tron.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Empty(t, result.TotalErrors)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)
			assert.NotEqual(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})

		t.Run("Creates TRON_USDT transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given outbound TRON balance
			tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeOutbound, withBalance(tronUSDT, "0", isTest))

			// Given an inbound balance with 100 USDT
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeInbound, withBalance(tronUSDT, "100_000_000", isTest))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateTronTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(tronUSDT.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Empty(t, result.TotalErrors)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)

			// For tokens, we should transfer 100% of crypto
			assert.Equal(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})

		t.Run("Creates BNB transaction", func(t *testing.T) {
			// ARRANGE
			// Given outbound BNB balance
			tc.Must.CreateWalletWithBalance(t, "BSC", wallet.TypeOutbound, withBalance(bnb, "0", isTest))

			// Given an inbound balance with 0.5 BNB
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "BSC", wallet.TypeInbound, withBalance(bnb, "500_000_000_000_000_000", isTest))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateBSCTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(bnb.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Empty(t, result.TotalErrors)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)
			assert.NotEqual(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})
	})

	t.Run("Tolerates errors", func(t *testing.T) {
		isTest := false

		t.Run("Creates 2 ETH transaction, one has failed", func(t *testing.T) {
			tc.Clear.Wallets(t)

			// ARRANGE
			// Given outbound ETH balance
			tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(eth, "0", isTest))

			// Given an inbound balance #1 with 0.5 ETH
			w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "500_000_000_000_000_000", isTest))

			// Given an inbound TEST balance #2 with 0.5 ETH
			b2 := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, w1.ID,
				withBalance(eth, "500_000_000_000_000_000", isTest),
				withTestnet(eth),
			)

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			// And mocked ethereum transaction creation & broadcast
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, false, txHashID, nil)

			// ACT
			// Create internal transfer
			result, err := tc.Services.Processing.BatchCreateInternalTransfers(
				tc.Context, []*wallet.Balance{b1, b2},
			)

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 1)
			assert.Len(t, result.RollbackedTransactionIDs, 0)
			assert.Equal(t, int64(1), result.TotalErrors)
			assert.Equal(t, 1, len(result.UnhandledErrors))

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, 0, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created
			assert.NotNil(t, tx)
			assert.Equal(t, w1.ID, *tx.SenderWalletID)
			assert.Equal(t, w1.Address, *tx.SenderAddress)
			assert.Equal(t, b1.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.True(t, tx.ServiceFee.IsZero())
			assert.Nil(t, tx.NetworkFee)
			assert.NotEqual(t, tx.Amount, b1.Amount)

			// Get fresh wallet from DB
			wt, err := tc.Services.Wallet.GetByID(tc.Context, *tx.SenderWalletID)
			require.NoError(t, err)

			// check pending tx counter
			assert.Equal(t, int64(1), wt.PendingMainnetTransactions)

			// Get fresh balance from DB
			b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
			require.NoError(t, err)

			// check that balance has decremented
			assert.True(t, b1Fresh.Amount.LessThan(b1.Amount), b1Fresh.Amount.String())
		})
	})

	t.Run("Fails", func(t *testing.T) {
		tc.Clear.Wallets(t)

		isTest := false

		t.Run("Validation error", func(t *testing.T) {
			// SETUP
			_, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(eth, "123", isTest))

			for testCaseIndex, testCase := range []struct {
				errContains string
				balances    func() []*wallet.Balance
			}{
				{
					errContains: "balance is empty",
					balances: func() []*wallet.Balance {
						_, emptyBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "0", isTest))
						return []*wallet.Balance{emptyBalance}
					},
				},
				{
					errContains: "wallet is not inbound",
					balances:    func() []*wallet.Balance { return []*wallet.Balance{outboundBalance} },
				},
				{
					errContains: "insufficient amount",
					balances: func() []*wallet.Balance {
						_, emptyBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "1", isTest))

						return []*wallet.Balance{emptyBalance}
					},
				},
				{
					errContains: "balances contain duplicates",
					balances: func() []*wallet.Balance {
						return []*wallet.Balance{outboundBalance, outboundBalance}
					},
				},
			} {
				t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
					// ARRANGE
					// Given balances
					input := testCase.balances()

					// ACT
					// Transfer money
					result, err := tc.Services.Processing.BatchCreateInternalTransfers(ctx, input)
					assert.Nil(t, result)

					// ASSERT
					// Check that error contain string
					if testCase.errContains != "" {
						assert.ErrorContains(t, err, testCase.errContains)
					}
				})
			}
		})

		t.Run("Logic error", func(t *testing.T) {
			t.Run("Handles transaction signature failure", func(t *testing.T) {
				tc.Clear.Wallets(t)

				// ARRANGE
				// Given outbound ETH balance
				tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(eth, "0", isTest))

				// Given an inbound balance with 0.5 ETH
				w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "500_000_000_000_000_000", isTest))

				// And mocked errors response from KMS tx signing
				tc.Providers.KMS.
					On("CreateEthereumTransaction", mock.Anything).
					Return(nil, errors.New("sign error"))

				// ACT
				// Create internal transfer
				result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

				// ASSERT
				// Check that there were to txs created, and we received 1 error counter
				assert.NoError(t, err)
				assert.Len(t, result.CreatedTransactions, 0)
				assert.Empty(t, result.RollbackedTransactionIDs)
				assert.Equal(t, int64(1), result.TotalErrors)

				// Get fresh wallet from DB
				wt, err := tc.Services.Wallet.GetByID(tc.Context, w1.ID)
				require.NoError(t, err)

				// check pending tx counter wasn't changed
				assert.Equal(t, int64(0), wt.PendingMainnetTransactions)

				// Get fresh balance from DB
				b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
				require.NoError(t, err)

				// check that balance wasn't changed
				assert.True(t, b1Fresh.Amount.Equals(b1.Amount))
			})

			t.Run("Handles broadcast failure", func(t *testing.T) {
				isTest := false
				tc.Clear.Wallets(t)

				// ARRANGE
				// Given outbound ETH balance
				tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(eth, "0", isTest))

				// Given an inbound balance with 0.5 ETH
				w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(eth, "500_000_000_000_000_000", isTest))

				// And response from KMS
				const rawTxHash = "0x1234567"
				tc.SetupCreateEthereumTransactionWildcard(rawTxHash)

				// And ERROR (!) response from blockchain
				tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxHash, false, "", blockchain.ErrInsufficientFunds)

				// ACT
				// Create internal transfer
				result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

				// ASSERT
				// Check that tx was created but after failed broadcast it reverted
				assert.NoError(t, err)
				assert.Len(t, result.CreatedTransactions, 0)
				assert.Len(t, result.RollbackedTransactionIDs, 1)
				assert.Equal(t, int64(1), result.TotalErrors)

				// Get fresh wallet from DB
				wt, err := tc.Services.Wallet.GetByID(tc.Context, w1.ID)
				require.NoError(t, err)

				// check pending tx counter wasn't changed
				assert.Equal(t, int64(0), wt.PendingMainnetTransactions)

				// Get fresh balance from DB
				b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
				require.NoError(t, err)

				// check that balance wasn't changed
				assert.True(t, b1Fresh.Amount.Equals(b1.Amount))

				// get failed transaction
				tx, err := tc.Services.Transaction.GetByID(ctx, 0, result.RollbackedTransactionIDs[0])
				require.NoError(t, err)

				assert.Equal(t, transaction.StatusCancelled, tx.Status)
				assert.Equal(t, w1.ID, *tx.SenderWalletID)
				assert.Equal(t, w1.Address, *tx.SenderAddress)
				assert.Equal(t, b1.Currency, tx.Amount.Ticker())
				assert.Nil(t, tx.HashID)
				assert.Nil(t, tx.NetworkFee)
				assert.Contains(t, tx.MetaData["comment"], "internal transfer rollback")
			})

			t.Run("Handles concurrent balance decrement", func(t *testing.T) {
				isTest := false
				tc.Clear.Wallets(t)

				// ARRANGE
				// Given outbound ETH_USDT balance
				tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withBalance(ethUSDT, "0", isTest))

				// Given an inbound balance with 100 ETH_USDT
				w1, b1 := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeInbound, withBalance(ethUSDT, "100_000_000", isTest))

				// And mocked ethereum transaction creation that takes some time ...
				// while under unclear circumstances someone decrements balance concurrently
				stealBalance := func(_ mock.Arguments) {
					_, err := tc.Services.Wallet.UpdateBalanceByID(ctx, b1.ID, wallet.UpdateBalanceByIDQuery{
						Operation: wallet.OperationDecrement,
						Amount:    money.MustCryptoFromRaw(ethUSDT.Ticker, "90_000_000", ethUSDT.Decimals),
						Comment:   "I stole your money!",
					})
					require.NoError(t, err)
				}

				tc.Providers.KMS.
					On("CreateEthereumTransaction", mock.Anything).
					Run(stealBalance).
					Return(&kmswallet.CreateEthereumTransactionCreated{
						Payload: &kmsmodel.EthereumTransaction{RawTransaction: "0x123456"},
					}, nil)

				// ACT
				// Create internal transfer
				result, err := tc.Services.Processing.BatchCreateInternalTransfers(tc.Context, []*wallet.Balance{b1})

				// ASSERT
				// Check that tx was created but after failed broadcast it reverted
				assert.NoError(t, err)
				assert.Len(t, result.CreatedTransactions, 0)
				assert.Len(t, result.RollbackedTransactionIDs, 1)
				assert.Equal(t, int64(1), result.TotalErrors)

				// Get fresh wallet from DB
				wt, err := tc.Services.Wallet.GetByID(tc.Context, w1.ID)
				require.NoError(t, err)

				// check pending tx counter wasn't changed
				assert.Equal(t, int64(0), wt.PendingMainnetTransactions)

				// Get fresh balance from DB
				b1Fresh, err := tc.Services.Wallet.GetBalanceByUUID(tc.Context, wallet.EntityTypeWallet, wt.ID, b1.UUID)
				require.NoError(t, err)

				// check that balance not equals to 100 - 90 = 10 USDT
				assert.Equal(t, "10", b1Fresh.Amount.String())

				// get failed transaction
				tx, err := tc.Services.Transaction.GetByID(ctx, 0, result.RollbackedTransactionIDs[0])
				require.NoError(t, err)

				assert.Equal(t, transaction.StatusCancelled, tx.Status)
				assert.Equal(t, w1.ID, *tx.SenderWalletID)
				assert.Equal(t, w1.Address, *tx.SenderAddress)
				assert.Equal(t, b1.Currency, tx.Amount.Ticker())
				assert.Nil(t, tx.HashID)
				assert.Nil(t, tx.NetworkFee)
				assert.Contains(t, tx.MetaData["comment"], "internal transfer rollback")
			})
		})
	})
}

func withBalance(currency money.CryptoCurrency, value string, isTest bool) func(*repository.CreateBalanceParams) {
	return test.WithBalanceFromCurrency(currency, value, isTest)
}

//nolint:funlen
//goland:noinspection GoBoolExpressions
func TestService_BatchCheckInternalTransfers(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	ctx := tc.Context

	eth := tc.Must.GetCurrency(t, "ETH")
	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")
	ethNetworkFee := lo.Must(eth.MakeAmount("1000"))

	tron := tc.Must.GetCurrency(t, "TRON")
	tronUSDT := tc.Must.GetCurrency(t, "TRON_USDT")
	tronNetworkFee := lo.Must(tron.MakeAmount("1000"))

	bnb := tc.Must.GetCurrency(t, "BNB")
	bnbNetworkFee := lo.Must(bnb.MakeAmount("2000"))

	createTransfer := func(
		sender, recipient *wallet.Wallet,
		senderBalance *wallet.Balance,
		currency money.CryptoCurrency,
		amount money.Money,
		isTest bool,
	) (*transaction.Transaction, *wallet.Balance) {
		require.Equal(t, wallet.TypeInbound, sender.Type)
		require.Equal(t, wallet.TypeOutbound, recipient.Type)

		usdAmount, err := money.FiatFromFloat64(money.USD, 1)
		require.NoError(t, err)

		_, err = tc.Services.Wallet.IncrementPendingTransaction(ctx, sender.ID, isTest)
		require.NoError(t, err)

		tx, err := tc.Services.Transaction.Create(ctx, 0, transaction.CreateTransaction{
			Type:            transaction.TypeInternal,
			SenderWallet:    sender,
			RecipientWallet: recipient,
			Currency:        currency,
			Amount:          amount,
			USDAmount:       usdAmount,
			IsTest:          isTest,
		})
		require.NoError(t, err)

		b, err := tc.Services.Wallet.UpdateBalanceByID(ctx, senderBalance.ID, wallet.UpdateBalanceByIDQuery{
			Operation: wallet.OperationDecrement,
			Amount:    amount,
		})
		require.NoError(t, err)

		txHash := fmt.Sprintf("0x0123-abc-tx-%d", tx.ID)

		err = tc.Services.Transaction.UpdateTransactionHash(ctx, transaction.SystemMerchantID, tx.ID, txHash)
		require.NoError(t, err)

		tx.HashID = &txHash

		return tx, b
	}

	t.Run("Confirms ETH transfer", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given INBOUND wallet with ETH balance
		withEth1 := test.WithBalanceFromCurrency(eth, "500_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeInbound, withEth1)

		// And OUTBOUND wallet with zero balance
		withEth2 := test.WithBalanceFromCurrency(eth, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeOutbound, withEth2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(eth.Ticker, "300_000_000", eth.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, eth, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "200000000", balanceIn.Amount.StringRaw())

		// And mocked network fee
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    ethNetworkFee,
			Success:       true,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is successful
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.TypeInternal, tx.Type)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, amount, tx.Amount)
		assert.Equal(t, amount, *tx.FactAmount)
		assert.Equal(t, ethNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender balance equals to 500_000_000 - 300_000_000 - network fee (1000)
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "199999000", balanceIn.Amount.StringRaw())

		// Check that recipient balance equals to $amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amount.StringRaw(), balanceOut.Amount.StringRaw())
	})

	t.Run("Confirms ETH_USDT transfer", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given inbound wallet with ETH_USDT balance
		withUSDT1 := test.WithBalanceFromCurrency(ethUSDT, "100_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeInbound, withUSDT1)

		withETH := test.WithBalanceFromCurrency(eth, "2000", isTest)
		balanceInCoin := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, wtIn.ID, withETH)

		// And outbound wallet with zero ETH_USDT balance
		withUSDT2 := test.WithBalanceFromCurrency(ethUSDT, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeOutbound, withUSDT2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(ethUSDT.Ticker, "50_000_000", ethUSDT.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, ethUSDT, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "50000000", balanceIn.Amount.StringRaw())

		// And mocked transaction receipt
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    ethNetworkFee,
			Success:       true,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is successful
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.TypeInternal, tx.Type)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, ethUSDT, tx.Currency)
		assert.Equal(t, amount, tx.Amount)
		assert.Equal(t, amount, *tx.FactAmount)
		assert.Equal(t, ethNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender ETH_USDT balance equals to 100_000_000 - 50_000_000
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "50000000", balanceIn.Amount.StringRaw())

		// Check that recipient USDT balance equals to $amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amount.StringRaw(), balanceOut.Amount.StringRaw())

		// Check that ETH balance was decremented by networkFee (2000 - 1000)
		balanceInCoin, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceInCoin.UUID)
		require.NoError(t, err)
		assert.Equal(t, "1000", balanceInCoin.Amount.StringRaw())
	})

	t.Run("Confirms TRON transfer", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given inbound wallet with ETH balance
		withTRON1 := test.WithBalanceFromCurrency(tron, "500_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeInbound, withTRON1)

		// And outbound wallet with zero balance
		withTRON2 := test.WithBalanceFromCurrency(tron, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeOutbound, withTRON2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(tron.Ticker, "300_000_000", tron.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, tron, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "200000000", balanceIn.Amount.StringRaw())

		// And mocked transaction receipt
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    tron.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    tronNetworkFee,
			Success:       true,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(tron.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is successful
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.TypeInternal, tx.Type)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, amount, tx.Amount)
		assert.Equal(t, amount, *tx.FactAmount)
		assert.Equal(t, tronNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender balance equals to 500_000_000 - 300_000_000 - network fee (1000)
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "199999000", balanceIn.Amount.StringRaw())

		// Check that recipient balance equals to $amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amount.StringRaw(), balanceOut.Amount.StringRaw())
	})

	t.Run("Confirms TRON_USDT transfer", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given inbound wallet with TRON_USDT balance
		withUSDT1 := test.WithBalanceFromCurrency(tronUSDT, "100_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeInbound, withUSDT1)

		withTRON := test.WithBalanceFromCurrency(tron, "2000", isTest)
		balanceInCoin := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, wtIn.ID, withTRON)

		// And outbound wallet with zero TRON_USDT balance
		withUSDT2 := test.WithBalanceFromCurrency(tronUSDT, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeOutbound, withUSDT2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(tronUSDT.Ticker, "50_000_000", tronUSDT.Decimals)
		tx, _ := createTransfer(wtIn, wtOut, balanceIn, tronUSDT, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "50000000", balanceIn.Amount.StringRaw())

		// And mocked transaction receipt
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    tronUSDT.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    tronNetworkFee,
			Success:       true,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(tronUSDT.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is successful
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.TypeInternal, tx.Type)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, tronUSDT, tx.Currency)
		assert.Equal(t, amount, tx.Amount)
		assert.Equal(t, amount, *tx.FactAmount)
		assert.Equal(t, tronNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender TRON_USDT balance equals to 100_000_000 - 50_000_000
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "50000000", balanceIn.Amount.StringRaw())

		// Check that recipient USDT balance equals to $amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amount.StringRaw(), balanceOut.Amount.StringRaw())

		// Check that ETH balance was decremented by networkFee (2000 - 1000)
		balanceInCoin, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceInCoin.UUID)
		require.NoError(t, err)
		assert.Equal(t, "1000", balanceInCoin.Amount.StringRaw())
	})

	t.Run("Confirms BNB transfer", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given INBOUND wallet with BNB balance
		withBNB1 := test.WithBalanceFromCurrency(bnb, "500_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, bnb.Blockchain.String(), wallet.TypeInbound, withBNB1)

		// And OUTBOUND wallet with zero balance
		withBNB2 := test.WithBalanceFromCurrency(bnb, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, bnb.Blockchain.String(), wallet.TypeOutbound, withBNB2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(bnb.Ticker, "300_000_000", bnb.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, bnb, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "200000000", balanceIn.Amount.StringRaw())

		// And mocked network fee
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    bnb.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    bnbNetworkFee,
			Success:       true,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(bnb.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is successful
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.TypeInternal, tx.Type)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, amount, tx.Amount)
		assert.Equal(t, amount, *tx.FactAmount)
		assert.Equal(t, bnbNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender balance equals to 500_000_000 - 300_000_000 - network fee (2000)
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "199998000", balanceIn.Amount.StringRaw())

		// Check that recipient balance equals to $amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amount.StringRaw(), balanceOut.Amount.StringRaw())
	})

	t.Run("Transaction is not confirmed yet", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given INBOUND wallet with ETH balance
		withEth1 := test.WithBalanceFromCurrency(eth, "500_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeInbound, withEth1)

		// And OUTBOUND wallet with zero balance
		withEth2 := test.WithBalanceFromCurrency(eth, "0", isTest)
		wtOut, _ := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeOutbound, withEth2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(eth.Ticker, "300_000_000", eth.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, eth, amount, isTest)

		// And decremented sender balance
		balanceIn, err := tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		require.Equal(t, "200000000", balanceIn.Amount.StringRaw())

		// (!) And mocked transaction receipt that is not confirmed yet
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    ethNetworkFee,
			Success:       true,
			Confirmations: 1,
			IsConfirmed:   false,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx wasn't changed
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusPending, tx.Status)
	})

	t.Run("Reverts failed ETH transaction", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given INBOUND wallet with ETH balance
		withEth1 := test.WithBalanceFromCurrency(eth, "500_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeInbound, withEth1)

		// And OUTBOUND wallet with zero balance
		withEth2 := test.WithBalanceFromCurrency(eth, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, eth.Ticker, wallet.TypeOutbound, withEth2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(eth.Ticker, "300_000_000", eth.Decimals)

		tx, _ := createTransfer(wtIn, wtOut, balanceIn, eth, amount, isTest)

		// And a mocked receipt that states that a transaction is failed
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    ethNetworkFee,
			Success:       false,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err := tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is failed
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.StatusFailed, tx.Status)
		assert.Equal(t, amount, tx.Amount)
		assert.Nil(t, tx.FactAmount)
		assert.Equal(t, ethNetworkFee, *tx.NetworkFee)
		assert.Equal(t, wtIn.ID, *tx.SenderWalletID)
		assert.Equal(t, wtOut.ID, *tx.RecipientWalletID)

		// Check that sender balance equals to 500_000_000 - network fee (1000)
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, "499999000", balanceIn.Amount.StringRaw())

		// Check that recipient balance stays the same
		amountOutBefore := balanceOut.Amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amountOutBefore, balanceOut.Amount)
	})

	t.Run("Reverts failed TRON_USDT transaction", func(t *testing.T) {
		// ARRANGE
		tc.Clear.Wallets(t)
		isTest := false

		// Given inbound wallet with TRON_USDT balance
		withUSDT1 := test.WithBalanceFromCurrency(tronUSDT, "100_000_000", isTest)
		wtIn, balanceIn := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeInbound, withUSDT1)

		withTRON := test.WithBalanceFromCurrency(tron, "2000", isTest)
		balanceInCoin := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, wtIn.ID, withTRON)

		// And outbound wallet with zero TRON_USDT balance
		withUSDT2 := test.WithBalanceFromCurrency(tronUSDT, "0", isTest)
		wtOut, balanceOut := tc.Must.CreateWalletWithBalance(t, tron.Ticker, wallet.TypeOutbound, withUSDT2)

		// And created internal transfer
		amount := money.MustCryptoFromRaw(tronUSDT.Ticker, "50_000_000", tronUSDT.Decimals)
		tx, _ := createTransfer(wtIn, wtOut, balanceIn, tronUSDT, amount, isTest)

		// And a mocked receipt that states that a tx is failed
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    tronUSDT.Blockchain,
			IsTest:        tx.IsTest,
			Sender:        wtIn.Address,
			Recipient:     wtOut.Address,
			Hash:          *tx.HashID,
			NetworkFee:    tronNetworkFee,
			Success:       false,
			Confirmations: 5,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(tronUSDT.Blockchain, *tx.HashID, tx.IsTest, receipt, nil)

		// ACT
		err := tc.Services.Processing.BatchCheckInternalTransfers(ctx, []int64{tx.ID})

		// ASSERT
		assert.NoError(t, err)

		// Check that tx is failed
		tx, err = tc.Services.Transaction.GetByID(ctx, 0, tx.ID)
		require.NoError(t, err)

		assert.Equal(t, transaction.StatusFailed, tx.Status)
		assert.Equal(t, amount, tx.Amount)
		assert.Nil(t, tx.FactAmount)
		assert.Equal(t, tronNetworkFee, *tx.NetworkFee)

		// Check that sender TRON_USDT hasn't changed
		balanceInBefore := balanceIn.Amount
		balanceIn, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceIn.UUID)
		require.NoError(t, err)
		assert.Equal(t, balanceInBefore.String(), balanceIn.Amount.String())

		// Check that TRON balance was decremented by networkFee (2000 - 1000)
		balanceInCoin, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtIn.ID, balanceInCoin.UUID)
		require.NoError(t, err)
		assert.Equal(t, "1000", balanceInCoin.Amount.StringRaw())

		// Check that recipient USDT balance hasn't changed
		amountOutBefore := balanceOut.Amount
		balanceOut, err = tc.Services.Wallet.GetBalanceByUUID(ctx, wallet.EntityTypeWallet, wtOut.ID, balanceOut.UUID)
		require.NoError(t, err)
		assert.Equal(t, amountOutBefore, balanceOut.Amount)
	})
}

func withTestnet(currency money.CryptoCurrency) func(*repository.CreateBalanceParams) {
	return func(p *repository.CreateBalanceParams) {
		p.NetworkID = currency.TestNetworkID
	}
}
