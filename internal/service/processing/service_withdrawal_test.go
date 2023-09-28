package processing_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
//goland:noinspection ALL
func TestService_BatchCreateWithdrawals(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	ctx := tc.Context

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
	tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.08)
	tc.Providers.TatumMock.SetupRates("BNB", money.USD, 240)

	t.Run("Creates transactions", func(t *testing.T) {
		t.Run("Creates ETH transaction", func(t *testing.T) {
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With ETH address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(eth.Blockchain),
				Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
			})
			require.NoError(t, err)

			// And ETH balance
			withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal
			amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of 1 ETH
			withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))
			serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, eth)).To

			const (
				rawTxData               = "0x123456"
				txHashID                = "0xffffff"
				expectedMerchantBalance = "975" + "00000000000000"  // 0.6 ETH - 0.5 ETH - $4 = 0.1 ETH - $4: = 0.1 ETH - 2500*10^12 wei
				expectedWalletBalance   = "5" + "00000000000000000" // 0.5 ETH: 1 ETH - 0.5 ETH
			)

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

			// Given mocked ETH transaction creation & broadcast
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Empty(t, result.TotalErrors)
			assert.Empty(t, result.UnhandledErrors)
			assert.Len(t, result.CreatedTransactions, 1)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created and properties are correct
			assert.NotNil(t, tx)
			assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
			assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
			assert.Equal(t, addr.Address, tx.RecipientAddress)
			assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
			assert.Equal(t, withdrawal.Price, tx.Amount)
			assert.Nil(t, tx.NetworkFee)

			// Get fresh merchant balance and check balance
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

			// Check outbound wallet's balance
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())

			// Check withdrawal
			withdrawal, err = tc.Services.Payment.GetByID(ctx, mt.ID, withdrawal.ID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusInProgress, withdrawal.Status)
		})

		t.Run("Creates ETH_USDT transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With ETH address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(eth.Blockchain),
				Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
			})
			require.NoError(t, err)

			// And ETH_USDT balance ($100)
			withUSDT1 := test.WithBalanceFromCurrency(ethUSDT, "100_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withUSDT1)

			// Given withdrawal
			amount := lo.Must(ethUSDT.MakeAmount("50_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of $150 (USDT)
			withUSDT2 := test.WithBalanceFromCurrency(ethUSDT, "150_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withUSDT2)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 6))
			serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, ethUSDT)).To

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, ethUSDT, isTest, serviceFeeUSD)

			const (
				rawTxData               = "0x123456"
				txHashID                = "0xffffff"
				expectedMerchantBalance = "44000000"  // $44: $100 - $50 - $6 (service fee)
				expectedWalletBalance   = "100000000" // $100: $150 - $50
			)

			// Given mocked ETH transaction creation & broadcast
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Empty(t, result.TotalErrors)
			assert.Empty(t, result.UnhandledErrors)
			assert.Len(t, result.CreatedTransactions, 1)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created and properties are correct
			assert.NotNil(t, tx)
			assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
			assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
			assert.Equal(t, addr.Address, tx.RecipientAddress)
			assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
			assert.Equal(t, withdrawal.Price, tx.Amount)
			assert.Nil(t, tx.NetworkFee)

			// Get fresh merchant balance and check balance
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

			// Check outbound wallet's balance
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())

			// Check withdrawal
			withdrawal, err = tc.Services.Payment.GetByID(ctx, mt.ID, withdrawal.ID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusInProgress, withdrawal.Status)
		})

		t.Run("Creates TRON transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With TRON address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(tron.Blockchain),
				Address:    "TEAqQj9GcdK7EpGyGRM8PQb2oFTcecazF1",
			})
			require.NoError(t, err)

			// And TRON balance (150 trx)
			withBalance := test.WithBalanceFromCurrency(tron, "150_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal (125 trx)
			amount := lo.Must(tron.MakeAmount("125_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of 200 trx
			withTRX := test.WithBalanceFromCurrency(tron, "200_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeOutbound, withTRX)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 1))
			serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, tron)).To

			const (
				rawTxData               = "0x123456"
				txHashID                = "0xffffff"
				expectedMerchantBalance = "12500000" // 150 trx - 125 rtx - $1 = 25 trx - 12.5 trx = 12.5 trx
				expectedWalletBalance   = "75000000" // 200 trx - 125 trx = 75 trx
			)

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(tron, tron, isTest, serviceFeeUSD)

			// Given mocked TRON transaction creation & broadcast
			tc.SetupCreateTronTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(tron.Blockchain, rawTxData, isTest, txHashID, nil)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Empty(t, result.TotalErrors)
			assert.Empty(t, result.UnhandledErrors)
			assert.Len(t, result.CreatedTransactions, 1)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created and properties are correct
			assert.NotNil(t, tx)
			assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
			assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
			assert.Equal(t, addr.Address, tx.RecipientAddress)
			assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
			assert.Equal(t, withdrawal.Price, tx.Amount)
			assert.Nil(t, tx.NetworkFee)

			// Get fresh merchant balance and check balance
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

			// Check outbound wallet's balance
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())

			// Check withdrawal
			withdrawal, err = tc.Services.Payment.GetByID(ctx, mt.ID, withdrawal.ID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusInProgress, withdrawal.Status)
		})

		t.Run("Creates TRON_USDT transaction", func(t *testing.T) {
			tc.Clear.Wallets(t)
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With TRON address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(tron.Blockchain),
				Address:    "TEAqQj9GcdK7EpGyGRM8PQb2oFTcecazF1",
			})
			require.NoError(t, err)

			// And USDT balance ($150)
			withBalance := test.WithBalanceFromCurrency(tronUSDT, "150_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal ($50)
			amount := lo.Must(tronUSDT.MakeAmount("50_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of $150
			withUSDT := test.WithBalanceFromCurrency(tronUSDT, "150_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "TRON", wallet.TypeOutbound, withUSDT)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 2))
			serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, tronUSDT)).To

			const (
				rawTxData               = "0x123456"
				txHashID                = "0xffffff"
				expectedMerchantBalance = "98000000"  // 150 USDT - 50 USDT - $2 = 98 USDT
				expectedWalletBalance   = "100000000" // 150 - 50 USDT = 100 USDT
			)

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(tron, tronUSDT, isTest, serviceFeeUSD)

			// Given mocked TRON transaction creation & broadcast
			tc.SetupCreateTronTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(tron.Blockchain, rawTxData, isTest, txHashID, nil)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Empty(t, result.TotalErrors)
			assert.Empty(t, result.UnhandledErrors)
			assert.Len(t, result.CreatedTransactions, 1)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created and properties are correct
			assert.NotNil(t, tx)
			assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
			assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
			assert.Equal(t, addr.Address, tx.RecipientAddress)
			assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
			assert.Equal(t, withdrawal.Price, tx.Amount)
			assert.Nil(t, tx.NetworkFee)

			// Get fresh merchant balance and check balance
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

			// Check outbound wallet's balance
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())

			// Check withdrawal
			withdrawal, err = tc.Services.Payment.GetByID(ctx, mt.ID, withdrawal.ID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusInProgress, withdrawal.Status)
		})

		t.Run("Creates BNB transaction", func(t *testing.T) {
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With BSC address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "Bob's Address",
				Blockchain: kmswallet.Blockchain(bnb.Blockchain),
				Address:    "0x95222290dd7278003ddd389cc1e1d165cc4bafe0",
			})
			require.NoError(t, err)

			// And BNB 0.5 balance
			withBalance := test.WithBalanceFromCurrency(bnb, "500_000_000_000_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal
			amount := lo.Must(bnb.MakeAmount("400_000_000_000_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of 1 BNB
			withETH := test.WithBalanceFromCurrency(bnb, "1_000_000_000_000_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 3))
			serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, bnb)).To

			const (
				rawTxData               = "0x123456"
				txHashID                = "0xffffff"
				expectedMerchantBalance = "875" + "00000000000001"  // 0.5 BNB - 0.4 BNB - $3 = 0.1 BNB - $3: = 0.1 BNB - 0.0125 BNB
				expectedWalletBalance   = "6" + "00000000000000000" // 0.6 BNB: 1 BNB - 0.4 BNB
			)

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(bnb, bnb, isTest, serviceFeeUSD)

			// Given mocked ETH transaction creation & broadcast
			tc.SetupCreateBSCTransactionWildcard(rawTxData)
			tc.Fakes.SetupBroadcastTransaction(bnb.Blockchain, rawTxData, isTest, txHashID, nil)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Empty(t, result.TotalErrors)
			assert.Empty(t, result.UnhandledErrors)
			assert.Len(t, result.CreatedTransactions, 1)

			// Get fresh transaction from DB
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
			require.NoError(t, err)

			// Check that tx was created and properties are correct
			assert.NotNil(t, tx)
			assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
			assert.Equal(t, transaction.StatusPending, tx.Status)
			assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
			assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
			assert.Equal(t, addr.Address, tx.RecipientAddress)
			assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
			assert.Equal(t, txHashID, *tx.HashID)
			assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
			assert.Equal(t, withdrawal.Price, tx.Amount)
			assert.Nil(t, tx.NetworkFee)

			// Get fresh merchant balance and check balance
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

			// Check outbound wallet's balance
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())

			// Check withdrawal
			withdrawal, err = tc.Services.Payment.GetByID(ctx, mt.ID, withdrawal.ID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusInProgress, withdrawal.Status)
		})
	})

	t.Run("Creates 2 ETH transactions, one fails due to insufficient balance", func(t *testing.T) {
		tc.Clear.Wallets(t)
		isTest := false

		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With ETH address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "John's Address",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And ETH balance
		withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

		// Given withdrawal N1
		amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
		withdrawal1, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// And the same withdrawal N2
		withdrawal2, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of 1 ETH
		withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
		outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))
		serviceFeeCrypto := lo.Must(tc.Services.Blockchain.FiatToCrypto(ctx, serviceFeeUSD, eth)).To

		const (
			rawTxData               = "0x123456"
			txHashID                = "0xffffff"
			expectedMerchantBalance = "975" + "00000000000000"  // 0.6 ETH - 0.5 ETH - $4 = 0.1 ETH - $4: = 0.1 ETH - 2500*10^12 wei
			expectedWalletBalance   = "5" + "00000000000000000" // 0.5 ETH: 1 ETH - 0.5 ETH
		)

		tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

		// Given mocked ETH transaction creation & broadcast
		tc.SetupCreateEthereumTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

		// ACT
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal1.ID, withdrawal2.ID})

		// ASSERT
		// 1 tx should succeed, another one should fail due to insufficient balance
		assert.NoError(t, err)
		assert.Equal(t, int64(1), result.TotalErrors)
		assert.Empty(t, result.UnhandledErrors)
		assert.Len(t, result.CreatedTransactions, 1)

		// Here we can't really predict whether rollback contains tx or not.
		// Due to concurrenct processing tx might not be even created because balanceCovers(...) validation fails.
		if len(result.RollbackedTransactionIDs) > 0 {
			failedTX := lo.Must(tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.RollbackedTransactionIDs[0]))
			require.Equal(t, transaction.StatusCancelled, failedTX.Status)
		}

		// Get fresh transaction from DB [success]
		tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.CreatedTransactions[0].ID)
		require.NoError(t, err)

		// Check that tx was created and properties are correct
		assert.NotNil(t, tx)
		assert.Equal(t, transaction.TypeWithdrawal, tx.Type)
		assert.Equal(t, transaction.StatusPending, tx.Status)
		assert.Equal(t, outboundWallet.ID, *tx.SenderWalletID)
		assert.Equal(t, outboundWallet.Address, *tx.SenderAddress)
		assert.Equal(t, addr.Address, tx.RecipientAddress)
		assert.Equal(t, outboundBalance.Currency, tx.Amount.Ticker())
		assert.Equal(t, txHashID, *tx.HashID)
		assert.Equal(t, serviceFeeCrypto, tx.ServiceFee)
		assert.Equal(t, withdrawal1.Price, tx.Amount)
		assert.Nil(t, tx.NetworkFee)

		// Get fresh merchant balance and check balance
		merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
		require.NoError(t, err)
		assert.Equal(t, expectedMerchantBalance, merchantBalance.Amount.StringRaw())

		// Check outbound wallet's balance
		outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
		require.NoError(t, err)
		assert.Equal(t, expectedWalletBalance, outboundBalance.Amount.StringRaw())
	})

	t.Run("Validation error", func(t *testing.T) {
		tc.Clear.Wallets(t)
		mt, _ := tc.Must.CreateMerchant(t, 1)

		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance(eth, "123_000_000_000_000_000", false))
		merchantAddress, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A1",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		makeWithdrawal := func(amount money.Money, meta payment.Metadata) *payment.Payment {
			entry, err := tc.Repository.CreatePayment(ctx, repository.CreatePaymentParams{
				PublicID:          uuid.New(),
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
				Type:              string(payment.TypeWithdrawal),
				Status:            string(payment.StatusPending),
				MerchantID:        mt.ID,
				MerchantOrderUuid: uuid.New(),
				Price:             repository.MoneyToNumeric(amount),
				Decimals:          int32(amount.Decimals()),
				Currency:          amount.Ticker(),
				Metadata:          meta.ToJSONB(),
				IsTest:            false,
			})
			require.NoError(t, err)

			pt, err := tc.Services.Payment.GetByID(ctx, mt.ID, entry.ID)
			require.NoError(t, err)

			return pt
		}

		for _, tt := range []struct {
			name        string
			assert      func(t *testing.T, in []*payment.Payment, result *processing.TransferResult, err error)
			withdrawals func() []*payment.Payment
		}{
			{
				name: "payment is not withdrawal",
				withdrawals: func() []*payment.Payment {
					return []*payment.Payment{tc.CreateSamplePayment(t, mt.ID)}
				},
				assert: func(t *testing.T, _ []*payment.Payment, _ *processing.TransferResult, err error) {
					assert.ErrorContains(t, err, `withdrawals filter mismatch for status "pending"`)
				},
			},
			{
				name: "status is not pending",
				withdrawals: func() []*payment.Payment {
					withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
						BalanceID: merchantBalance.UUID,
						AddressID: merchantAddress.UUID,
						AmountRaw: "0.1",
					})
					require.NoError(t, err)

					_, err = tc.Services.Payment.Update(ctx, mt.ID, withdrawal.ID, payment.UpdateProps{Status: payment.StatusInProgress})
					require.NoError(t, err)

					return []*payment.Payment{withdrawal}
				},
				assert: func(t *testing.T, _ []*payment.Payment, _ *processing.TransferResult, err error) {
					assert.ErrorContains(t, err, `withdrawals filter mismatch for status "pending"`)
				},
			},
			{
				name: "invalid address id",
				withdrawals: func() []*payment.Payment {
					return []*payment.Payment{
						makeWithdrawal(
							lo.Must(eth.MakeAmount("123_456")),
							payment.Metadata{payment.MetaBalanceID: strconv.Itoa(int(merchantBalance.ID))},
						),
					}
				},
				assert: func(t *testing.T, in []*payment.Payment, result *processing.TransferResult, err error) {
					assert.NoError(t, err)
					assert.Equal(t, int64(1), result.TotalErrors)
					assert.ErrorContains(t, result.UnhandledErrors[0], "withdrawal is invalid, marked as failed")

					pt, _ := tc.Services.Payment.GetByID(ctx, in[0].MerchantID, in[0].ID)
					require.NoError(t, err)
					assert.Equal(t, payment.StatusFailed, pt.Status)
				},
			},
			{
				name: "invalid balance id",
				withdrawals: func() []*payment.Payment {
					return []*payment.Payment{
						makeWithdrawal(
							lo.Must(eth.MakeAmount("123_456")),
							payment.Metadata{payment.MetaAddressID: strconv.Itoa(int(merchantAddress.ID))},
						),
					}
				},
				assert: func(t *testing.T, in []*payment.Payment, result *processing.TransferResult, err error) {
					assert.NoError(t, err)
					assert.Equal(t, int64(1), result.TotalErrors)
					assert.ErrorContains(t, result.UnhandledErrors[0], "withdrawal is invalid, marked as failed")

					pt, _ := tc.Services.Payment.GetByID(ctx, in[0].MerchantID, in[0].ID)
					require.NoError(t, err)
					assert.Equal(t, payment.StatusFailed, pt.Status)
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				// ARRANGE
				// Given balances
				input := tt.withdrawals()

				// ACT
				// Create withdrawals
				ids := util.MapSlice(input, func(p *payment.Payment) int64 { return p.ID })
				result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, ids)

				// ASSERT
				tt.assert(t, input, result, err)
			})
		}
	})

	t.Run("Logic error", func(t *testing.T) {
		tc.Clear.Wallets(t)

		t.Run("Handles transaction signature failure", func(t *testing.T) {
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With ETH address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(eth.Blockchain),
				Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
			})
			require.NoError(t, err)

			// And ETH balance
			withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal
			amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of 1 ETH
			withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))
			tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

			// And mocked errors response from KMS tx signing
			tc.Providers.KMS.
				On("CreateEthereumTransaction", mock.Anything).
				Return(nil, errors.New("sign error"))

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 0)
			assert.Empty(t, result.RollbackedTransactionIDs)
			assert.Equal(t, int64(1), result.TotalErrors)

			// Get fresh merchant balance and check that balance didn't change
			expectedMerchantBalanceAmount := merchantBalance.Amount
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalanceAmount, merchantBalance.Amount)

			// Get fresh wallet balance and check that balance didn't change
			expectedWalletBalanceAmount := outboundBalance.Amount
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalanceAmount, outboundBalance.Amount)

			// Check that payment is still pending
			pt, err := tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusPending, pt.Status)
		})

		t.Run("Handles broadcast failure", func(t *testing.T) {
			tc.Clear.Wallets(t)
			isTest := false

			// ARRANGE
			// Given merchant
			mt, _ := tc.Must.CreateMerchant(t, 1)

			// With ETH address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "John's Address",
				Blockchain: kmswallet.Blockchain(eth.Blockchain),
				Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
			})
			require.NoError(t, err)

			// And ETH balance
			withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
			merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

			// Given withdrawal
			amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
			withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
				BalanceID: merchantBalance.UUID,
				AddressID: addr.UUID,
				AmountRaw: amount.String(),
			})
			require.NoError(t, err)

			// Given OUTBOUND wallet with balance of 1 ETH
			withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
			outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

			// Given service fee mock for withdrawal
			serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))

			const (
				rawTxData = "0x123456"
				txHashID  = "0xffffff"
			)

			tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

			// And response from KMS
			tc.SetupCreateEthereumTransactionWildcard(rawTxData)

			// And ERROR (!) response from blockchain
			tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, "", blockchain.ErrInsufficientFunds)

			// ACT
			result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})

			// ASSERT
			assert.NoError(t, err)
			assert.Len(t, result.CreatedTransactions, 0)
			assert.Len(t, result.RollbackedTransactionIDs, 1)
			assert.Equal(t, int64(1), result.TotalErrors)

			// Get fresh transaction from DB and check that it was canceled
			tx, err := tc.Services.Transaction.GetByID(tc.Context, mt.ID, result.RollbackedTransactionIDs[0])
			require.NoError(t, err)
			assert.Equal(t, transaction.StatusCancelled, tx.Status)

			// Get fresh merchant balance and check that balance didn't change
			expectedMerchantBalanceAmount := merchantBalance.Amount
			merchantBalance, err = tc.Services.Wallet.GetMerchantBalanceByUUID(ctx, mt.ID, merchantBalance.UUID)
			require.NoError(t, err)
			assert.Equal(t, expectedMerchantBalanceAmount, merchantBalance.Amount)

			// Get fresh wallet balance and check that balance didn't change
			expectedWalletBalanceAmount := outboundBalance.Amount
			outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
			require.NoError(t, err)
			assert.Equal(t, expectedWalletBalanceAmount, outboundBalance.Amount)

			// Check that payment was canceled
			pt, err := tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
			require.NoError(t, err)
			assert.Equal(t, payment.StatusFailed, pt.Status)
		})
	})
}

//nolint:funlen
func TestService_BatchCheckWithdrawals(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	ctx := tc.Context

	eth := tc.Must.GetCurrency(t, "ETH")
	ethUSDT := tc.Must.GetCurrency(t, "ETH_USDT")
	bnb := tc.Must.GetCurrency(t, "BNB")

	// Mock tx fees
	tc.Fakes.SetupAllFees(t, tc.Services.Blockchain)

	// Mock exchange rates
	tc.Providers.TatumMock.SetupRates("ETH", money.USD, 1600)
	tc.Providers.TatumMock.SetupRates("ETH_USDT", money.USD, 1)
	tc.Providers.TatumMock.SetupRates("TRON", money.USD, 0.08)
	tc.Providers.TatumMock.SetupRates("BNB", money.USD, 240)

	t.Run("Confirms ETH transaction", func(t *testing.T) {
		isTest := false

		// ARRANGE
		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With ETH address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "John's Address",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And ETH balance
		withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

		// Given withdrawal
		amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
		withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of 1 ETH
		withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
		outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))
		tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

		const (
			rawTxData = "0x123456"
			txHashID  = "0xffffff"
		)

		// Given mocked ETH transaction creation & broadcast
		tc.SetupCreateEthereumTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

		// Given successful tx creation & broadcasting
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})
		require.NoError(t, err)
		require.Len(t, result.CreatedTransactions, 1)

		txID := result.CreatedTransactions[0].ID

		// ... time goes by ...

		// Given transaction receipt
		networkFee := lo.Must(eth.MakeAmount("2000"))
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        isTest,
			Sender:        outboundWallet.Address,
			Recipient:     addr.Address,
			Hash:          txHashID,
			Nonce:         0,
			NetworkFee:    networkFee,
			Success:       true,
			Confirmations: 10,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, txHashID, isTest, receipt, nil)

		// ACT
		// Check for withdrawal progress
		err = tc.Services.Processing.BatchCheckWithdrawals(ctx, []int64{txID})

		// ASSERT
		assert.NoError(t, err)

		// Check transaction
		tx, err := tc.Services.Transaction.GetByID(ctx, mt.ID, txID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, networkFee, *tx.NetworkFee)

		// Check outbound wallet & balance
		outboundWallet, err = tc.Services.Wallet.GetByID(ctx, outboundWallet.ID)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), outboundWallet.PendingMainnetTransactions)

		// Check that outbound balance was decremented by tx amount and network fee
		outboundAmountBefore := outboundBalance.Amount
		outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
		assert.NoError(t, err)
		assert.Equal(
			t,
			outboundAmountBefore,
			lo.Must(lo.Must(outboundBalance.Amount.Add(tx.Amount)).Add(receipt.NetworkFee)),
		)

		// Check withdrawal
		withdrawal, err = tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
		assert.NoError(t, err)
		assert.Equal(t, payment.StatusSuccess, withdrawal.Status)

		// Extra assertion from merchant's perspective
		related, err := tc.Services.Payment.GetByMerchantOrderIDWithRelations(ctx, mt.ID, withdrawal.MerchantOrderUUID)
		assert.NoError(t, err)
		assert.Equal(t, tx.ID, related.Transaction.ID)
		assert.Equal(t, merchantBalance.ID, related.Balance.ID)
		assert.Equal(t, addr.ID, related.Address.ID)
	})

	t.Run("Confirms ETH_USDT transaction", func(t *testing.T) {
		tc.Clear.Wallets(t)
		isTest := false

		// ARRANGE
		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With ETH address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "John's Address",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And ETH_USDT balance ($100)
		withUSDT1 := test.WithBalanceFromCurrency(ethUSDT, "100_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withUSDT1)

		// Given withdrawal
		amount := lo.Must(ethUSDT.MakeAmount("50_000_000"))
		withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of $150 (USDT) and 0.001 ETH
		withUSDT2 := test.WithBalanceFromCurrency(ethUSDT, "150_000_000", isTest)
		withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000", isTest)
		outboundWallet, outboundBalanceUSDT := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withUSDT2)
		outboundBalanceETH := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, outboundWallet.ID, withETH)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 6))
		tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, ethUSDT, isTest, serviceFeeUSD)

		const (
			rawTxData = "0x123456"
			txHashID  = "0xffffff"
		)

		// Given mocked ETH transaction creation & broadcast
		tc.SetupCreateEthereumTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

		// Given successful tx creation & broadcasting
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})
		require.NoError(t, err)
		require.Len(t, result.CreatedTransactions, 1)

		txID := result.CreatedTransactions[0].ID

		// ... time goes by ...

		// Given transaction receipt
		networkFee := lo.Must(eth.MakeAmount("4000"))
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        isTest,
			Sender:        outboundWallet.Address,
			Recipient:     addr.Address,
			Hash:          txHashID,
			Nonce:         0,
			NetworkFee:    networkFee,
			Success:       true,
			Confirmations: 10,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, txHashID, isTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckWithdrawals(ctx, []int64{txID})

		// ASSERT
		assert.NoError(t, err)

		// Check transaction
		tx, err := tc.Services.Transaction.GetByID(ctx, mt.ID, txID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, networkFee, *tx.NetworkFee)

		// Check outbound wallet & balance
		outboundWallet, err = tc.Services.Wallet.GetByID(ctx, outboundWallet.ID)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), outboundWallet.PendingMainnetTransactions)

		// Check that outbound balance in USDT was decremented by tx amount
		// and ETH balance was decremented by network fee
		outboundAmountUSDTBefore := outboundBalanceUSDT.Amount
		outboundBalanceUSDT, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalanceUSDT.ID)
		assert.NoError(t, err)
		assert.Equal(t, outboundAmountUSDTBefore, lo.Must(outboundBalanceUSDT.Amount.Add(tx.Amount)))

		outboundAmountETHBefore := outboundBalanceETH.Amount
		outboundBalanceUSDT, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalanceETH.ID)
		assert.NoError(t, err)
		assert.Equal(t, outboundAmountETHBefore, lo.Must(outboundBalanceUSDT.Amount.Add(networkFee)))

		// Check withdrawal
		withdrawal, err = tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
		assert.NoError(t, err)
		assert.Equal(t, payment.StatusSuccess, withdrawal.Status)
	})

	t.Run("Confirms BNB transaction", func(t *testing.T) {
		isTest := false

		// ARRANGE
		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With BNB address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "Bob's Address",
			Blockchain: kmswallet.Blockchain(bnb.Blockchain),
			Address:    "0x85222290dd7278ff3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And BNB balance
		withBalance := test.WithBalanceFromCurrency(bnb, "600_000_000_000_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

		// Given withdrawal
		amount := lo.Must(bnb.MakeAmount("500_000_000_000_000_000"))
		withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of 1 BNB
		withBNB := test.WithBalanceFromCurrency(bnb, "1_000_000_000_000_000_000", isTest)
		outboundWallet, outboundBalance := tc.Must.CreateWalletWithBalance(t, "BSC", wallet.TypeOutbound, withBNB)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 3))
		tc.Fakes.SetupCalculateWithdrawalFeeUSD(bnb, bnb, isTest, serviceFeeUSD)

		const (
			rawTxData = "0x123456"
			txHashID  = "0xffffff"
		)

		// Given mocked BNB transaction creation & broadcast
		tc.SetupCreateBSCTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(bnb.Blockchain, rawTxData, isTest, txHashID, nil)

		// Given successful tx creation & broadcasting
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})
		require.NoError(t, err)
		require.Len(t, result.CreatedTransactions, 1)

		txID := result.CreatedTransactions[0].ID

		// ... time goes by ...

		// Given transaction receipt
		networkFee := lo.Must(bnb.MakeAmount("1000"))
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    bnb.Blockchain,
			IsTest:        isTest,
			Sender:        outboundWallet.Address,
			Recipient:     addr.Address,
			Hash:          txHashID,
			Nonce:         0,
			NetworkFee:    networkFee,
			Success:       true,
			Confirmations: 10,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(bnb.Blockchain, txHashID, isTest, receipt, nil)

		// ACT
		// Check for withdrawal progress
		err = tc.Services.Processing.BatchCheckWithdrawals(ctx, []int64{txID})

		// ASSERT
		assert.NoError(t, err)

		// Check transaction
		tx, err := tc.Services.Transaction.GetByID(ctx, mt.ID, txID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusCompleted, tx.Status)
		assert.Equal(t, networkFee, *tx.NetworkFee)

		// Check outbound wallet & balance
		outboundWallet, err = tc.Services.Wallet.GetByID(ctx, outboundWallet.ID)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), outboundWallet.PendingMainnetTransactions)

		// Check that outbound balance was decremented by tx amount and network fee
		outboundAmountBefore := outboundBalance.Amount
		outboundBalance, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalance.ID)
		assert.NoError(t, err)
		assert.Equal(
			t,
			outboundAmountBefore,
			lo.Must(lo.Must(outboundBalance.Amount.Add(tx.Amount)).Add(receipt.NetworkFee)),
		)

		// Check withdrawal
		withdrawal, err = tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
		assert.NoError(t, err)
		assert.Equal(t, payment.StatusSuccess, withdrawal.Status)

		// Extra assertion from merchant's perspective
		related, err := tc.Services.Payment.GetByMerchantOrderIDWithRelations(ctx, mt.ID, withdrawal.MerchantOrderUUID)
		assert.NoError(t, err)
		assert.Equal(t, tx.ID, related.Transaction.ID)
		assert.Equal(t, merchantBalance.ID, related.Balance.ID)
		assert.Equal(t, addr.ID, related.Address.ID)
	})

	t.Run("Transaction is not confirmed yet", func(t *testing.T) {
		tc.Clear.Wallets(t)

		isTest := false

		// ARRANGE
		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With ETH address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "John's Address",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And ETH balance
		withBalance := test.WithBalanceFromCurrency(eth, "600_000_000_000_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withBalance)

		// Given withdrawal
		amount := lo.Must(eth.MakeAmount("500_000_000_000_000_000"))
		withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of 1 ETH
		withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000_000", isTest)
		outboundWallet, _ := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withETH)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 4))
		tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, eth, isTest, serviceFeeUSD)

		const (
			rawTxData = "0x123456"
			txHashID  = "0xffffff"
		)

		// Given mocked ETH transaction creation & broadcast
		tc.SetupCreateEthereumTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

		// Given successful tx creation & broadcasting
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})
		require.NoError(t, err)
		require.Len(t, result.CreatedTransactions, 1)

		txID := result.CreatedTransactions[0].ID

		// ... time goes by ...

		// Given transaction receipt
		networkFee := lo.Must(eth.MakeAmount("2000"))
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        isTest,
			Sender:        outboundWallet.Address,
			Recipient:     addr.Address,
			Hash:          txHashID,
			Nonce:         0,
			NetworkFee:    networkFee,
			Success:       true,
			Confirmations: 3,
			IsConfirmed:   false,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, txHashID, isTest, receipt, nil)

		// ACT
		// Check for withdrawal progress
		err = tc.Services.Processing.BatchCheckWithdrawals(ctx, []int64{txID})

		// ASSERT
		assert.NoError(t, err)

		// Check transaction
		tx, err := tc.Services.Transaction.GetByID(ctx, mt.ID, txID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusPending, tx.Status)
	})

	t.Run("Handles ETH_USDT transaction failure", func(t *testing.T) {
		tc.Clear.Wallets(t)
		isTest := false

		// ARRANGE
		// Given merchant
		mt, _ := tc.Must.CreateMerchant(t, 1)

		// With ETH address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "John's Address",
			Blockchain: kmswallet.Blockchain(eth.Blockchain),
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		// And ETH_USDT balance ($100)
		withUSDT1 := test.WithBalanceFromCurrency(ethUSDT, "100_000_000", isTest)
		merchantBalance := tc.Must.CreateBalance(t, wallet.EntityTypeMerchant, mt.ID, withUSDT1)

		// Given withdrawal
		amount := lo.Must(ethUSDT.MakeAmount("50_000_000"))
		withdrawal, err := tc.Services.Payment.CreateWithdrawal(ctx, mt.ID, payment.CreateWithdrawalProps{
			BalanceID: merchantBalance.UUID,
			AddressID: addr.UUID,
			AmountRaw: amount.String(),
		})
		require.NoError(t, err)

		// Given OUTBOUND wallet with balance of $150 (USDT) and 0.001 ETH
		withUSDT2 := test.WithBalanceFromCurrency(ethUSDT, "150_000_000", isTest)
		withETH := test.WithBalanceFromCurrency(eth, "1_000_000_000_000_000", isTest)
		outboundWallet, outboundBalanceUSDT := tc.Must.CreateWalletWithBalance(t, "ETH", wallet.TypeOutbound, withUSDT2)
		outboundBalanceETH := tc.Must.CreateBalance(t, wallet.EntityTypeWallet, outboundWallet.ID, withETH)

		// Given service fee mock for withdrawal
		serviceFeeUSD := lo.Must(money.FiatFromFloat64(money.USD, 6))
		tc.Fakes.SetupCalculateWithdrawalFeeUSD(eth, ethUSDT, isTest, serviceFeeUSD)

		const (
			rawTxData = "0x123456"
			txHashID  = "0xffffff"
		)

		// Given mocked ETH transaction creation & broadcast
		tc.SetupCreateEthereumTransactionWildcard(rawTxData)
		tc.Fakes.SetupBroadcastTransaction(eth.Blockchain, rawTxData, isTest, txHashID, nil)

		// Given successful tx creation & broadcasting
		result, err := tc.Services.Processing.BatchCreateWithdrawals(ctx, []int64{withdrawal.ID})
		require.NoError(t, err)
		require.Len(t, result.CreatedTransactions, 1)

		txID := result.CreatedTransactions[0].ID

		// ... time goes by ...

		// Given transaction receipt
		networkFee := lo.Must(eth.MakeAmount("4000"))
		receipt := &blockchain.TransactionReceipt{
			Blockchain:    eth.Blockchain,
			IsTest:        isTest,
			Sender:        outboundWallet.Address,
			Recipient:     addr.Address,
			Hash:          txHashID,
			Nonce:         0,
			NetworkFee:    networkFee,
			Success:       false,
			Confirmations: 10,
			IsConfirmed:   true,
		}

		tc.Fakes.SetupGetTransactionReceipt(eth.Blockchain, txHashID, isTest, receipt, nil)

		// ACT
		err = tc.Services.Processing.BatchCheckWithdrawals(ctx, []int64{txID})

		// ASSERT
		assert.NoError(t, err)

		// Check transaction
		tx, err := tc.Services.Transaction.GetByID(ctx, mt.ID, txID)
		assert.NoError(t, err)
		assert.Equal(t, transaction.StatusFailed, tx.Status)
		assert.Equal(t, networkFee, *tx.NetworkFee)

		// Check outbound wallet & balance
		outboundWallet, err = tc.Services.Wallet.GetByID(ctx, outboundWallet.ID)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), outboundWallet.PendingMainnetTransactions)

		// Check that outbound balance in USDT wasn't decremented
		// and ETH balance was decremented by network fee
		outboundAmountUSDTBefore := outboundBalanceUSDT.Amount
		outboundBalanceUSDT, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalanceUSDT.ID)
		assert.NoError(t, err)
		assert.Equal(t, outboundAmountUSDTBefore, outboundBalanceUSDT.Amount)

		outboundAmountETHBefore := outboundBalanceETH.Amount
		outboundBalanceETH, err = tc.Services.Wallet.GetBalanceByID(ctx, wallet.EntityTypeWallet, outboundWallet.ID, outboundBalanceETH.ID)
		assert.NoError(t, err)
		assert.Equal(t, outboundAmountETHBefore, lo.Must(outboundBalanceETH.Amount.Add(networkFee)))

		// Check withdrawal
		withdrawal, err = tc.Services.Payment.GetByPublicID(ctx, withdrawal.PublicID)
		assert.NoError(t, err)
		assert.Equal(t, payment.StatusFailed, withdrawal.Status)
	})
}
