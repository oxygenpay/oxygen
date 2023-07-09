package blockchain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertor(t *testing.T) {
	ctx := context.Background()
	conv, tatum := newConvertor(false)

	tatum.SetupRates(money.EUR.String(), money.USD, 1.10)
	tatum.SetupRates(money.USD.String(), money.EUR, 0.91)
	tatum.SetupRates("ETH", money.USD, 1000)
	tatum.SetupRates("ETH_USDT", money.USD, 1)
	tatum.SetupRates("MATIC_USDC", money.USD, 1)
	tatum.SetupRates("TRON", money.USD, 0.07)

	eth := lo.Must(conv.GetCurrencyByTicker("ETH"))
	ethUSDT := lo.Must(conv.GetCurrencyByTicker("ETH_USDT"))
	ethUSDC := lo.Must(conv.GetCurrencyByTicker("ETH_USDC"))

	for _, tt := range []struct {
		from        string
		to          string
		amount      string
		expectError bool
		expected    blockchain.Conversion
	}{
		{
			from:   "USD",
			to:     "USD",
			amount: "10",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToFiat,
				Rate: 1,
				From: lo.Must(money.USD.MakeAmount("1000")),
				To:   lo.Must(money.USD.MakeAmount("1000")),
			},
		},
		{
			from:   "EUR",
			to:     "USD",
			amount: "100",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToFiat,
				Rate: 1.10,
				From: lo.Must(money.EUR.MakeAmount("10000")),
				To:   lo.Must(money.USD.MakeAmount("11000")),
			},
		},
		{
			from:   "USD",
			to:     "EUR",
			amount: "100",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToFiat,
				Rate: 0.91,
				From: lo.Must(money.USD.MakeAmount("10000")),
				To:   lo.Must(money.EUR.MakeAmount("9100")),
			},
		},
		{
			from:   "ETH",
			to:     "USD",
			amount: "1",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeCryptoToFiat,
				Rate: 1000,
				From: lo.Must(eth.MakeAmount("1_000_000_000_000_000_000")),
				To:   lo.Must(money.USD.MakeAmount("1000_00")),
			},
		},
		{
			from:   "USD",
			to:     "ETH",
			amount: "500",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToCrypto,
				Rate: 0.001,
				From: lo.Must(money.USD.MakeAmount("500_00")),
				To:   lo.Must(eth.MakeAmount("500_000_000_000_000_010")), // error rate :)
			},
		},
		{
			from:   "USD",
			to:     "ETH_USDT",
			amount: "500",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToCrypto,
				Rate: 1,
				From: lo.Must(money.USD.MakeAmount("500_00")),
				To:   lo.Must(ethUSDT.MakeAmount("500_000_000")),
			},
		},
		{
			from:   "USD",
			to:     "ETH_USDC",
			amount: "200",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToCrypto,
				Rate: 1,
				From: lo.Must(money.USD.MakeAmount("200_00")),
				To:   lo.Must(ethUSDC.MakeAmount("200_000_000")),
			},
		},
		{
			// case-insensitive
			from:   "usd",
			to:     "eth_usdt",
			amount: "500",
			expected: blockchain.Conversion{
				Type: blockchain.ConversionTypeFiatToCrypto,
				Rate: 1,
				From: lo.Must(money.USD.MakeAmount("500_00")),
				To:   lo.Must(ethUSDT.MakeAmount("500_000_000")),
			},
		},
		{from: "ETH", to: "TRON", amount: "1", expectError: true},
		{from: "ETH", to: "USD", amount: "0", expectError: true},
		{from: "USD", to: "ETH", amount: "0", expectError: true},
		{from: "a", to: "b", amount: "0", expectError: true},
	} {
		t.Run(fmt.Sprintf("%s/%s", tt.from, tt.to), func(t *testing.T) {
			actual, err := conv.Convert(ctx, tt.from, tt.to, tt.amount)
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}

	t.Run("Without cache", func(t *testing.T) {
		// ACT
		rate1, err := conv.GetExchangeRate(ctx, "USD", "ETH")
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 10)

		rate2, err := conv.GetExchangeRate(ctx, "USD", "ETH")
		require.NoError(t, err)

		// ASSERT
		assert.NotEqual(t, rate1.CalculatedAt, rate2.CalculatedAt)
	})

	t.Run("With cache", func(t *testing.T) {
		// ARRANGE
		conv, tatum := newConvertor(true)
		tatum.SetupRates("ETH", money.USD, 1000)

		// ACT
		rate1, err := conv.GetExchangeRate(ctx, "USD", "ETH")
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 10)

		rate2, err := conv.GetExchangeRate(ctx, "USD", "ETH")
		require.NoError(t, err)

		// ASSERT
		assert.Equal(t, rate1, rate2)
	})
}

func newConvertor(enableCache bool) (*blockchain.Service, *test.TatumMock) {
	currencies := blockchain.NewCurrencies()
	if err := blockchain.DefaultSetup(currencies); err != nil {
		panic(err.Error())
	}

	logger := zerolog.Nop()
	tatumAPI, mock := test.NewTatum(nil, &logger)

	bc := blockchain.New(
		currencies,
		blockchain.Providers{Tatum: tatumAPI},
		enableCache,
		&logger,
	)

	return bc, mock
}
