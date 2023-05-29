package money

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FiatCurrencies(t *testing.T) {
	testCases := []struct {
		ticker       FiatCurrency
		value        float64
		expValue     float64
		expString    string
		expRawString string
		error        bool
	}{
		{ticker: USD, value: 33, expValue: 33, expString: "33", expRawString: "3300"},
		{ticker: USD, value: 33.1, expValue: 33.1, expString: "33.10", expRawString: "3310"},
		{ticker: USD, value: 33.01, expValue: 33.01, expString: "33.01", expRawString: "3301"},
		{ticker: USD, value: 33.001, expValue: 33, expString: "33", expRawString: "3300"},
		{ticker: USD, value: 33.99, expValue: 33.99, expString: "33.99", expRawString: "3399"},
		{ticker: USD, value: 33.999, expValue: 33.99, expString: "33.99", expRawString: "3399"},

		{ticker: USD, value: .50, expValue: 0.50, expString: "0.50", expRawString: "50"},
		{ticker: USD, value: .05, expValue: 0.05, expString: "0.05", expRawString: "5"},
		{ticker: USD, value: .005, error: true},

		{ticker: EUR, value: 49.90, expValue: 49.90, expString: "49.90", expRawString: "4990"},
		{ticker: EUR, value: 49_123.90, expValue: 49_123.90, expString: "49123.90", expRawString: "4912390"},
		{ticker: EUR, value: 956_789.999, expValue: 956_789.99, expString: "956789.99", expRawString: "95678999"},
		{ticker: EUR, value: 10_000_000.20, error: true},

		{ticker: EUR, value: 123_456.789, expValue: 123_456.78, expString: "123456.78", expRawString: "123_456_78"},
	}

	for _, tc := range testCases {
		testName := fmt.Sprintf("%s/%.3f", tc.ticker, tc.value)

		t.Run(testName, func(t *testing.T) {
			m, err := FiatFromFloat64(tc.ticker, tc.value)
			if tc.error {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			rawString := clear(tc.expRawString)

			assert.Equal(t, Fiat, m.Type())
			assert.Equal(t, string(tc.ticker), m.Ticker())
			assert.Equal(t, FiatDecimals, m.Decimals())
			assert.Equal(t, tc.expString, m.String())
			assert.Equal(t, rawString, m.StringRaw())

			mFloatValue, err := m.FiatToFloat64()
			assert.NoError(t, err)
			assert.Equal(t, tc.expValue, mFloatValue)

			newM, err := New(m.Type(), m.Ticker(), m.StringRaw(), m.Decimals())
			assert.NoError(t, err)

			assert.Equal(t, newM.Type(), m.Type())
			assert.Equal(t, newM.Ticker(), m.Ticker())
			assert.Equal(t, newM.Decimals(), m.Decimals())
			assert.Equal(t, newM.String(), m.String())
			assert.Equal(t, newM.StringRaw(), m.StringRaw())
		})
	}
}

func Test_CryptoCurrencies(t *testing.T) {
	testCases := []struct {
		ticker    string
		decimals  int64
		value     string
		expString string
		error     bool
	}{
		{ticker: "BTC", decimals: 8, value: "1", expString: "0.000_000_01"},
		{ticker: "BTC", decimals: 8, value: "1123", expString: "0.000_011_23"},
		{ticker: "BTC", decimals: 8, value: "12345678", expString: "0.123_456_78"},
		{ticker: "BTC", decimals: 8, value: "1_0000_0000", expString: "1"},
		{ticker: "BTC", decimals: 8, value: "10_0000_0000", expString: "10"},
		{ticker: "BTC", decimals: 8, value: "10_1230_0000", expString: "10.123"},
		{ticker: "BTC", decimals: 8, value: "10_1230_0001", expString: "10.123_000_01"},
		{ticker: "BTC", decimals: 8, value: "10_1230_0010", expString: "10.123_000_1"},
		{ticker: "BTC", decimals: 8, value: "1000_0000_0001", expString: "1000.00000001"},

		{ticker: "BTC", decimals: 8, value: "abc", error: true},

		{ticker: "ETH", decimals: 18, value: "1", expString: "0.000_000_000_000_000_001"},
		{ticker: "ETH", decimals: 18, value: "1_631_387", expString: "0.000_000_000_001_631_387"},
		{ticker: "ETH", decimals: 18, value: "31_631_387", expString: "0.000_000_000_031_631_387"},
		{ticker: "ETH", decimals: 18, value: "30_196_008_522", expString: "0.000_000_030_196_008_522"},

		{ticker: "ETH", decimals: 18, value: "123_456__000_000_000_031_631_000", expString: "123_456.000_000_000_031_631"},

		{ticker: "MATIC", decimals: 18, value: "118__746_301_720_649_360_000", expString: "118.746_301_720_649_36"},
	}

	for _, tc := range testCases {
		testName := fmt.Sprintf("%s/%s", tc.ticker, tc.value)

		t.Run(testName, func(t *testing.T) {
			value := clear(tc.value)
			expString := clear(tc.expString)

			m, err := CryptoFromRaw(tc.ticker, value, tc.decimals)
			if tc.error {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			assert.Equal(t, Crypto, m.Type())
			assert.Equal(t, tc.ticker, m.Ticker())
			assert.Equal(t, tc.decimals, m.Decimals())
			assert.Equal(t, expString, m.String())
			assert.Equal(t, value, m.StringRaw())

			// Check that creation from float string e.g. "0.002" will be parsed as expected
			m2, err := CryptoFromStringFloat(tc.ticker, expString, tc.decimals)
			assert.NoError(t, err)
			assert.Equal(t, m.StringRaw(), m2.StringRaw())
		})
	}
}

func TestMoney_MultiplyFloat64(t *testing.T) {
	testcases := []struct {
		from       Money
		multiplier float64
		expected   Money
		error      bool
	}{
		{
			from:       mustCreateCrypto("123000", 3),
			multiplier: 2,
			expected:   mustCreateCrypto("246000", 3),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 1.01,
			expected:   mustCreateCrypto("1010", 2),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 0.1,
			expected:   mustCreateCrypto("100", 2),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 0.1,
			expected:   mustCreateCrypto("100", 2),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 0,
			error:      true,
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: -0.1,
			error:      true,
		},
		{
			from:       mustCreateCrypto("100_222_444", 18),
			multiplier: 0.015,
			expected:   mustCreateCrypto("1_503_336", 18),
		},
	}

	for _, tc := range testcases {
		name := fmt.Sprintf("%s/%s/%f", tc.from.Ticker(), tc.from.String(), tc.multiplier)

		t.Run(name, func(t *testing.T) {
			actual, err := tc.from.MultiplyFloat64(tc.multiplier)

			if tc.error {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tc.expected.StringRaw(), actual.StringRaw())
		})
	}
}

func TestMoney_MultiplyInt64(t *testing.T) {
	testcases := []struct {
		from       Money
		multiplier int64
		expected   Money
		error      bool
	}{
		{
			from:       mustCreateCrypto("123000", 3),
			multiplier: 2,
			expected:   mustCreateCrypto("246000", 3),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 1,
			expected:   mustCreateCrypto("1000", 2),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 3,
			expected:   mustCreateCrypto("3000", 2),
		},
		{
			// real eth tx example: from == tx.GasUsed()
			from:       mustCreateCrypto("51470730463", 18),
			multiplier: 21000,
			expected:   mustCreateCrypto("1_080_885_339_723_000", 18),
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: 0,
			error:      true,
		},
		{
			from:       mustCreateCrypto("1000", 2),
			multiplier: -1,
			error:      true,
		},
	}

	for _, tc := range testcases {
		name := fmt.Sprintf("%s/%s/%d", tc.from.Ticker(), tc.from.String(), tc.multiplier)

		t.Run(name, func(t *testing.T) {
			actual, err := tc.from.MultiplyInt64(tc.multiplier)

			if tc.error {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tc.expected.StringRaw(), actual.StringRaw())
		})
	}
}

func TestMoney_AddSub(t *testing.T) {
	for i, tc := range []struct {
		a, b, sum, sub Money
		expectError    bool
	}{
		{
			a:   mustCreateCrypto("123", 3),
			b:   mustCreateCrypto("123", 3),
			sum: mustCreateCrypto("246", 3),
			sub: mustCreateCrypto("0", 3),
		},
		{
			a:   mustCreateCrypto("15_000_000", 18),
			b:   mustCreateCrypto("150_000", 18),
			sum: mustCreateCrypto("15_150_000", 18),
			sub: mustCreateCrypto("14_850_000", 18),
		},
		{
			a:           mustCreateCrypto("123", 3),
			b:           mustCreateCrypto("123", 4),
			expectError: true,
		},
		{
			// restrict negative results
			a:           mustCreateCrypto("123", 3),
			b:           mustCreateCrypto("124", 4),
			expectError: true,
		},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			sum, errSum := tc.a.Add(tc.b)
			sub, errSub := tc.a.Sub(tc.b)

			if tc.expectError {
				assert.Error(t, errSum)
				assert.Error(t, errSub)
				return
			}

			assert.Equal(t, tc.sum, sum)
			assert.Equal(t, tc.sub, sub)
		})
	}
}

func TestMoney_Compare(t *testing.T) {
	for i, tc := range []struct {
		a, b                          Money
		lessThan, equals, greaterThan bool
	}{
		{a: mustCreateCrypto("123", 18), b: mustCreateCrypto("123", 19)}, // not compatible
		{a: mustCreateCrypto("123", 18), b: mustCreateCrypto("123", 18), equals: true},
		{a: mustCreateCrypto("123", 18), b: mustCreateCrypto("1234", 18), lessThan: true},
		{a: mustCreateCrypto("1234", 18), b: mustCreateCrypto("123", 18), greaterThan: true},
		{a: mustCreateCrypto("123_000_000_000_000_001", 18), b: mustCreateCrypto("123_000_000_000_000_001", 18), equals: true},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert.Equal(t, tc.equals, tc.a.Equals(tc.b))
			assert.Equal(t, tc.greaterThan, tc.a.GreaterThan(tc.b))
			assert.Equal(t, tc.lessThan, tc.a.LessThan(tc.b))
		})
	}
}

func TestCryptoToFiat(t *testing.T) {
	for i, tc := range []struct {
		crypto       Money
		exchangeRate float64
		expectedFiat Money
	}{
		{
			crypto:       mustCreateCrypto("100", 2),
			exchangeRate: 1,
			expectedFiat: mustCreateUSD("100"),
		},
		{
			crypto:       mustCreateCrypto("1000000", 6),
			exchangeRate: 1,
			expectedFiat: mustCreateUSD("100"),
		},
		{
			crypto:       mustCreateCrypto("1000000", 6),
			exchangeRate: 0.5,
			expectedFiat: mustCreateUSD("50"),
		},
		{
			crypto:       mustCreateCrypto("2000000", 6),
			exchangeRate: 1300,
			expectedFiat: mustCreateUSD("2600_00"),
		},
		{
			crypto:       mustCreateCrypto("1_000_000_000_000_000_000", 18),
			exchangeRate: 1300,
			expectedFiat: mustCreateUSD("1300_00"),
		},
		{
			crypto:       mustCreateCrypto("1", 6),
			exchangeRate: 1,
			expectedFiat: mustCreateUSD("0"),
		},
	} {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			actual, err := CryptoToFiat(tc.crypto, USD, tc.exchangeRate)
			assert.NoError(t, err)
			assert.Equal(t, actual, tc.expectedFiat)
		})
	}
}

func mustCreateCrypto(value string, decimals int64) Money {
	m, err := New(Crypto, "ticker", value, decimals)
	if err != nil {
		panic(err)
	}

	return m
}

func mustCreateUSD(value string) Money {
	m, err := New(Fiat, USD.String(), value, 2)
	if err != nil {
		panic(err)
	}

	return m
}
