package blockchain

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/antihax/optional"
	"github.com/jellydator/ttlcache/v3"
	"github.com/oxygenpay/oxygen/internal/money"
	client "github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/pkg/errors"
)

type Convertor interface {
	GetExchangeRate(ctx context.Context, from, to string) (ExchangeRate, error)
	Convert(ctx context.Context, from, to, amount string) (Conversion, error)
	FiatToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (Conversion, error)
	FiatToCrypto(ctx context.Context, from money.Money, to money.CryptoCurrency) (Conversion, error)
	CryptoToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (Conversion, error)
}

type ExchangeRate struct {
	From         string
	To           string
	Rate         float64
	CalculatedAt time.Time
}

type ConversionType string

const (
	ConversionTypeFiatToFiat   ConversionType = "fiatToFiat"
	ConversionTypeFiatToCrypto ConversionType = "fiatToCrypto"
	ConversionTypeCryptoToFiat ConversionType = "cryptoToFiat"
)

type Conversion struct {
	Type ConversionType
	Rate float64
	From money.Money
	To   money.Money
}

func (s *Service) GetExchangeRate(ctx context.Context, from, to string) (ExchangeRate, error) {
	if from == "" || to == "" {
		return ExchangeRate{}, ErrValidation
	}

	// noop
	if from == to {
		return ExchangeRate{
			Rate:         1,
			From:         from,
			To:           to,
			CalculatedAt: time.Now(),
		}, nil
	}

	convType, err := determineConversionType(from, to)
	if err != nil {
		return ExchangeRate{}, err
	}

	var (
		rate float64
		at   time.Time
	)

	switch convType {
	case ConversionTypeFiatToFiat, ConversionTypeCryptoToFiat:
		rate, at, err = s.getExchangeRate(ctx, NormalizeTicker(to), NormalizeTicker(from))
	case ConversionTypeFiatToCrypto:
		// Tatum does not support USD to ETH, that's why we need to calculate ETH to USD and reverse it
		rate, at, err = s.getExchangeRate(ctx, NormalizeTicker(from), NormalizeTicker(to))
		if err == nil {
			rate = 1 / rate
		}
	default:
		return ExchangeRate{}, errors.Errorf("unsupported conversion type %q", convType)
	}

	if err != nil {
		return ExchangeRate{}, err
	}

	return ExchangeRate{
		From:         from,
		To:           to,
		Rate:         rate,
		CalculatedAt: at,
	}, nil
}

// Convert Converts currencies according to automatically resolved ConversionType. This method parses amount as float64,
// please don't use it internally as output would contain huge error rate when dealing with 18 eth decimals.
// Suitable for API responses.
//
//nolint:gocyclo
func (s *Service) Convert(ctx context.Context, from, to, amount string) (Conversion, error) {
	switch {
	case from == "":
		return Conversion{}, errors.Wrap(ErrValidation, "from is required")
	case to == "":
		return Conversion{}, errors.Wrap(ErrValidation, "to is required")
	case amount == "":
		return Conversion{}, errors.Wrap(ErrValidation, "amount is required")
	}

	from, to = strings.ToUpper(from), strings.ToUpper(to)

	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil || amountFloat <= 0 {
		return Conversion{}, errors.Wrap(ErrValidation, "invalid amount")
	}

	convType, err := determineConversionType(from, to)
	if err != nil {
		return Conversion{}, errors.Wrap(ErrValidation, err.Error())
	}

	switch convType {
	case ConversionTypeFiatToFiat:
		fromMoney, err := money.FiatFromFloat64(money.FiatCurrency(from), amountFloat)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to make selected fiat money")
		}

		toCurrency, err := money.MakeFiatCurrency(to)
		if err != nil {
			return Conversion{}, errors.Wrap(err, "unable to resolve desired fiat currency")
		}

		return s.FiatToFiat(ctx, fromMoney, toCurrency)
	case ConversionTypeFiatToCrypto:
		fromMoney, err := money.FiatFromFloat64(money.FiatCurrency(from), amountFloat)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to make selected fiat money")
		}

		toCurrency, err := s.GetCurrencyByTicker(to)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to resolve desired currency")
		}

		return s.FiatToCrypto(ctx, fromMoney, toCurrency)
	case ConversionTypeCryptoToFiat:
		fromCurrency, err := s.GetCurrencyByTicker(from)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to resolve selected crypto currency")
		}

		fromMoney, err := money.CryptoFromFloat64(fromCurrency.Ticker, amountFloat, fromCurrency.Decimals)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to resolve selected crypto currency")
		}

		toCurrency, err := money.MakeFiatCurrency(to)
		if err != nil {
			return Conversion{}, errors.Wrap(ErrValidation, "unable to resolve desired fiat currency")
		}

		return s.CryptoToFiat(ctx, fromMoney, toCurrency)
	}

	return Conversion{}, errors.Wrap(ErrValidation, "unsupported conversion")
}

func (s *Service) FiatToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (Conversion, error) {
	if from.Type() != money.Fiat {
		return Conversion{}, errors.Wrapf(ErrValidation, "%s is not fiat", from.Ticker())
	}

	rate, err := s.GetExchangeRate(ctx, from.Ticker(), to.String())
	if err != nil {
		return Conversion{}, err
	}

	toValue, err := from.MultiplyFloat64(rate.Rate)
	if err != nil {
		return Conversion{}, err
	}

	toMoney, err := money.New(money.Fiat, to.String(), toValue.StringRaw(), money.FiatDecimals)
	if err != nil {
		return Conversion{}, err
	}

	return Conversion{
		Type: ConversionTypeFiatToFiat,
		From: from,
		To:   toMoney,
		Rate: rate.Rate,
	}, nil
}

func (s *Service) FiatToCrypto(ctx context.Context, from money.Money, to money.CryptoCurrency) (Conversion, error) {
	if from.Type() != money.Fiat {
		return Conversion{}, errors.Wrapf(ErrValidation, "%s is not fiat", from.Ticker())
	}
	if from.IsZero() {
		return Conversion{}, errors.Wrapf(ErrValidation, "%s is zero", from.Ticker())
	}

	rate, err := s.GetExchangeRate(ctx, from.Ticker(), to.Ticker)
	if err != nil {
		return Conversion{}, err
	}

	// Example: "$123 to ETH". How it works:
	//  - Create "1 ETH"
	//  - Multiply it by 123 -> "123 ETH"
	//  - Multiply by exchange rate of 1/1800
	//  - 1 * 123 * 1/1800 = 123/1800 = 0.0683 ETH
	//
	// This approach is taken because cryptocurrencies have more decimals that USD/EUR, so if we'd multiply USD by
	// exchange rate (that can be <1), we would get a huge error rate due to rounding.
	cryptoMoney, err := money.CryptoFromFloat64(to.Ticker, 1, to.Decimals)
	if err != nil {
		return Conversion{}, err
	}

	fiat, err := from.FiatToFloat64()
	if err != nil {
		return Conversion{}, err
	}

	cryptoMoney, err = cryptoMoney.MultiplyFloat64(fiat)
	if err != nil {
		return Conversion{}, err
	}

	cryptoMoney, err = cryptoMoney.MultiplyFloat64(rate.Rate)
	if err != nil {
		return Conversion{}, err
	}

	return Conversion{
		Type: ConversionTypeFiatToCrypto,
		Rate: rate.Rate,
		From: from,
		To:   cryptoMoney,
	}, nil
}

func (s *Service) CryptoToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (Conversion, error) {
	if from.Type() != money.Crypto {
		return Conversion{}, errors.Wrapf(ErrValidation, "%s is not crypto", from.Ticker())
	}

	rate, err := s.GetExchangeRate(ctx, from.Ticker(), to.String())
	if err != nil {
		return Conversion{}, err
	}

	fiatMoney, err := money.CryptoToFiat(from, to, rate.Rate)
	if err != nil {
		return Conversion{}, err
	}

	return Conversion{
		Type: ConversionTypeCryptoToFiat,
		Rate: rate.Rate,
		From: from,
		To:   fiatMoney,
	}, nil
}

// getExchangeRate. Example: is 1 ETH = $1500, then semantics are following:
// getExchangeRate(ctx, "USD", "ETH") (1500, time.Time, nil)
func (s *Service) getExchangeRate(ctx context.Context, desired, selected string) (float64, time.Time, error) {
	res, err := s.getTatumExchangeRate(ctx, desired, selected)
	if err != nil {
		return 0, time.Time{}, errors.Wrapf(err, "unable to get exchange rate of %q / %q", desired, selected)
	}

	rate, err := strconv.ParseFloat(res.Value, 64)
	if err != nil {
		return 0, time.Time{}, err
	}

	return rate, time.UnixMilli(int64(res.Timestamp)), nil
}

func tatumRateCacheKey(desired, selected string) string {
	return fmt.Sprintf("%s/%s", selected, desired)
}

func (s *Service) getTatumExchangeRate(ctx context.Context, desired, selected string) (client.ExchangeRate, error) {
	key := tatumRateCacheKey(desired, selected)

	if s.ratesCache != nil {
		if hit := s.ratesCache.Get(key); hit != nil {
			return hit.Value(), nil
		}
	}

	opts := &client.ExchangeRateApiGetExchangeRateOpts{BasePair: optional.NewString(desired)}
	res, _, err := s.providers.Tatum.Main().ExchangeRateApi.GetExchangeRate(ctx, selected, opts)
	if err != nil {
		return client.ExchangeRate{}, errors.Wrapf(err, "unable to get exchange rate of %q / %q", desired, selected)
	}

	if s.ratesCache != nil {
		s.ratesCache.Set(key, res, ttlcache.DefaultTTL)
	}

	return res, err
}

func determineConversionType(from, to string) (ConversionType, error) {
	var fromIsFiat, toIsFiat bool

	if _, err := money.MakeFiatCurrency(from); err == nil {
		fromIsFiat = true
	}

	if _, err := money.MakeFiatCurrency(to); err == nil {
		toIsFiat = true
	}

	switch {
	case fromIsFiat && toIsFiat:
		return ConversionTypeFiatToFiat, nil
	case fromIsFiat && !toIsFiat:
		return ConversionTypeFiatToCrypto, nil
	case !fromIsFiat && toIsFiat:
		return ConversionTypeCryptoToFiat, nil
	}

	return "", errors.Errorf("unsupported conversion type: %q to %q", from, to)
}

// e.g. ETH_USDT -> USDT
var normalizations = map[string]string{
	"_USDT": "USDT",
	"_USDC": "USDC",
	"_BUSD": "BUSD",
}

// NormalizeTicker normalizes fiat / crypto ticker for further usage in external services (e.g. Tatum).
func NormalizeTicker(ticker string) string {
	ticker = strings.ToUpper(ticker)

	for substr, replaced := range normalizations {
		if strings.Contains(ticker, substr) {
			return replaced
		}
	}

	return ticker
}
