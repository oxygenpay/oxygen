package wallet_test

import (
	"testing"

	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestNonceMethods(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	w := tc.Must.CreateWallet(t, "ETH", "0x123", "pub-key", wallet.TypeOutbound)

	t.Run("Mainnet", func(t *testing.T) { run(t, tc, w.ID, false) })
	t.Run("Testnet", func(t *testing.T) { run(t, tc, w.ID, true) })
}

func run(t *testing.T, tc *test.IntegrationTest, walletID int64, isTest bool) {
	// ARRANGE
	// Given a wallet
	originalWallet, err := tc.Services.Wallet.GetByID(tc.Context, walletID)
	require.NoError(t, err)

	// That has 5 pending transactions
	var actualNonce []int
	for i := 0; i < 5; i++ {
		nonce, errInc := tc.Services.Wallet.IncrementPendingTransaction(tc.Context, originalWallet.ID, isTest)
		actualNonce = append(actualNonce, nonce)

		require.NoError(t, errInc)
	}

	require.ElementsMatch(t, []int{0, 1, 2, 3, 4}, actualNonce)

	// ACT 1
	// Concurrently confirm 5 tx and create 5 pending tx
	var group errgroup.Group
	for i := 0; i < 5; i++ {
		group.Go(func() error {
			return tc.Services.Wallet.IncrementConfirmedTransaction(tc.Context, originalWallet.ID, isTest)
		})

		group.Go(func() error {
			_, errInc := tc.Services.Wallet.IncrementPendingTransaction(tc.Context, originalWallet.ID, isTest)
			return errInc
		})
	}

	// ASSERT 1
	assert.NoError(t, group.Wait())

	// Check counters
	freshWallet, err := tc.Services.Wallet.GetByID(tc.Context, originalWallet.ID)
	assert.NoError(t, err)

	if isTest {
		// mainnet counters didn't change
		assert.Equal(t, originalWallet.ConfirmedMainnetTransactions, freshWallet.ConfirmedMainnetTransactions)
		assert.Equal(t, originalWallet.PendingMainnetTransactions, freshWallet.PendingMainnetTransactions)

		assert.Equal(t, int64(5), freshWallet.ConfirmedTestnetTransactions)
		assert.Equal(t, int64(5), freshWallet.PendingTestnetTransactions)
	} else {
		assert.Equal(t, int64(5), freshWallet.ConfirmedMainnetTransactions)
		assert.Equal(t, int64(5), freshWallet.PendingMainnetTransactions)

		// testnet counters didn't change
		assert.Equal(t, originalWallet.ConfirmedTestnetTransactions, freshWallet.ConfirmedTestnetTransactions)
		assert.Equal(t, originalWallet.PendingTestnetTransactions, freshWallet.PendingTestnetTransactions)
	}

	// ACT 2
	// Concurrently reject 3 tx and create 5 pending tx
	var errg2 errgroup.Group
	for i := 0; i < 5; i++ {
		errg2.Go(func() error {
			_, errTx := tc.Services.Wallet.IncrementPendingTransaction(tc.Context, originalWallet.ID, isTest)
			return errTx
		})
	}
	for j := 0; j < 3; j++ {
		errg2.Go(func() error {
			return tc.Services.Wallet.DecrementPendingTransaction(tc.Context, originalWallet.ID, isTest)
		})
	}

	// ASSERT 2
	assert.NoError(t, errg2.Wait())

	// Check counters
	freshWallet, err = tc.Services.Wallet.GetByID(tc.Context, originalWallet.ID)
	assert.NoError(t, err)

	if isTest {
		// mainnet counters didn't change
		assert.Equal(t, originalWallet.ConfirmedMainnetTransactions, freshWallet.ConfirmedMainnetTransactions)
		assert.Equal(t, originalWallet.PendingMainnetTransactions, freshWallet.PendingMainnetTransactions)

		assert.Equal(t, int64(5), freshWallet.ConfirmedTestnetTransactions)
		assert.Equal(t, int64(7), freshWallet.PendingTestnetTransactions)
	} else {
		// testnet counters didn't change
		assert.Equal(t, originalWallet.ConfirmedTestnetTransactions, freshWallet.ConfirmedTestnetTransactions)
		assert.Equal(t, originalWallet.PendingTestnetTransactions, freshWallet.PendingTestnetTransactions)

		assert.Equal(t, int64(5), freshWallet.ConfirmedMainnetTransactions)
		assert.Equal(t, int64(7), freshWallet.PendingMainnetTransactions)
	}

	// ACT 3
	nonce, err := tc.Services.Wallet.IncrementPendingTransaction(tc.Context, originalWallet.ID, isTest)

	// ASSERT 3
	assert.NoError(t, err)
	assert.Equal(t, 12, nonce)
}
