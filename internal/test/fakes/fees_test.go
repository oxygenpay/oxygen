package fakes

import (
	"context"
	"testing"
	"time"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/stretchr/testify/assert"
)

func TestFeeCalculatorMock_CalculateFee(t *testing.T) {
	eth := money.CryptoCurrency{
		Ticker:   "ETH",
		Name:     "Ethereum",
		Decimals: 18,
	}

	usdt := money.CryptoCurrency{
		Ticker:   "ETH_USDT",
		Name:     "USDT",
		Decimals: 6,
	}

	now := time.Now().UTC()
	ctx := context.Background()

	m := newFeeCalculator(t)

	t.Run("Returns fee", func(t *testing.T) {
		// ARRANGE
		expected := blockchain.NewFee(eth, now, false, blockchain.EthFee{
			TotalCostUSD: "123",
		})

		m.SetupCalculateFee(eth, usdt, false, expected)

		// ACT
		actual, err := m.CalculateFee(ctx, eth, usdt, false)

		// ASSERT
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Not set", func(t *testing.T) {
		// ACT
		_, err := m.CalculateFee(ctx, eth, usdt, true)

		// ASSERT
		assert.Error(t, err)
	})
}
