package blockchain_test

import (
	"testing"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestNew(_ *testing.T) {
	// todo returns sorted list
	// todo returns sorted by blockchain
	// todo add token to two different blockchain
}

func TestCreatePaymentLink(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	const (
		evmAddr  = "0xc2132d05d31c914a87c6611c10748aeb04b58e8f"
		tronAddr = "TVEaDaTKJZ2RsQUWREWykouuHak9scyZaf"
	)

	for _, tt := range []struct {
		address  string
		currency string
		amount   string
		isTest   bool
		expected string
	}{
		{
			address:  evmAddr,
			currency: "ETH",
			amount:   "123",
			isTest:   false,
			expected: "ethereum:0xc2132d05d31c914a87c6611c10748aeb04b58e8f@1?value=123",
		},
		{
			address:  evmAddr,
			currency: "ETH",
			amount:   "123",
			isTest:   true,
			expected: "ethereum:0xc2132d05d31c914a87c6611c10748aeb04b58e8f@5?value=123",
		},
		{
			address:  evmAddr,
			currency: "ETH_USDT",
			amount:   "333",
			isTest:   false,
			expected: "ethereum:0xdac17f958d2ee523a2206206994597c13d831ec7@1/transfer?address=0xc2132d05d31c914a87c6611c10748aeb04b58e8f&uint256=333",
		},
		{
			address:  evmAddr,
			currency: "ETH_USDT",
			amount:   "333",
			isTest:   true,
			expected: "ethereum:0xdac17f958d2ee523a2206206994597c13d831ec7@5/transfer?address=0xc2132d05d31c914a87c6611c10748aeb04b58e8f&uint256=333",
		},
		{
			address:  evmAddr,
			currency: "MATIC",
			amount:   "123",
			isTest:   true,
			expected: "ethereum:0xc2132d05d31c914a87c6611c10748aeb04b58e8f@80001?value=123",
		},
		{
			address:  evmAddr,
			currency: "MATIC_USDT",
			amount:   "333",
			isTest:   false,
			expected: "ethereum:0xc2132d05d31c914a87c6611c10748aeb04b58e8f@137/transfer?address=0xc2132d05d31c914a87c6611c10748aeb04b58e8f&uint256=333",
		},
		{
			address:  tronAddr,
			currency: "TRON",
			amount:   "444",
			isTest:   false,
			expected: "tron:TVEaDaTKJZ2RsQUWREWykouuHak9scyZaf?amount=0.000444",
		},
		{
			address:  tronAddr,
			currency: "TRON_USDT",
			amount:   "444",
			isTest:   true,
			expected: "tron:TVEaDaTKJZ2RsQUWREWykouuHak9scyZaf?amount=0.000444",
		},
	} {
		t.Run(tt.expected, func(t *testing.T) {
			// ARRANGE
			currency := tc.Must.GetCurrency(t, tt.currency)
			amount := lo.Must(currency.MakeAmount(tt.amount))

			// ACT
			actual, err := blockchain.CreatePaymentLink(tt.address, currency, amount, tt.isTest)

			// ASSERT
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestExplorerTXLink(t *testing.T) {
	eth := money.Blockchain("ETH")
	matic := money.Blockchain("MATIC")
	tron := money.Blockchain("TRON")

	for _, tt := range []struct {
		blockchain  money.Blockchain
		networkID   string
		expectError bool
		expected    string
	}{
		{blockchain: eth, networkID: "1", expected: "https://etherscan.io/tx/0x123"},
		{blockchain: eth, networkID: "5", expected: "https://goerli.etherscan.io/tx/0x123"},
		{blockchain: matic, networkID: "137", expected: "https://polygonscan.com/tx/0x123"},
		{blockchain: matic, networkID: "80001", expected: "https://mumbai.polygonscan.com/tx/0x123"},
		{blockchain: tron, networkID: "mainnet", expected: "https://tronscan.org/#/transaction/0x123"},
		{blockchain: tron, networkID: "testnet", expected: "https://shasta.tronscan.org/#/transaction/0x123"},
		{blockchain: "abc", networkID: "1", expectError: true},
		{blockchain: matic, networkID: "1", expectError: true},
		{blockchain: tron, networkID: "1", expectError: true},
	} {
		t.Run(tt.expected, func(t *testing.T) {
			actual, err := blockchain.CreateExplorerTXLink(tt.blockchain, tt.networkID, "0x123")

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
