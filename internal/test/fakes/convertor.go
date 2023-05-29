package fakes

import (
	"context"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
)

// ConvertorProxy represents proxy for real implementation of blockchain.Convertor that uses
// fake tatum http server that can we mocked as well.
type ConvertorProxy struct {
	conv blockchain.Convertor
}

func newConvertorProxy(conv blockchain.Convertor) *ConvertorProxy {
	return &ConvertorProxy{conv}
}

func (c *ConvertorProxy) GetExchangeRate(ctx context.Context, from, to string) (blockchain.ExchangeRate, error) {
	return c.conv.GetExchangeRate(ctx, from, to)
}

func (c *ConvertorProxy) Convert(ctx context.Context, from, to, amount string) (blockchain.Conversion, error) {
	return c.conv.Convert(ctx, from, to, amount)
}

func (c *ConvertorProxy) FiatToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (blockchain.Conversion, error) {
	return c.conv.FiatToFiat(ctx, from, to)
}

func (c *ConvertorProxy) FiatToCrypto(ctx context.Context, from money.Money, to money.CryptoCurrency) (blockchain.Conversion, error) {
	return c.conv.FiatToCrypto(ctx, from, to)
}

func (c *ConvertorProxy) CryptoToFiat(ctx context.Context, from money.Money, to money.FiatCurrency) (blockchain.Conversion, error) {
	return c.conv.CryptoToFiat(ctx, from, to)
}
