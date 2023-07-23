package tatum

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
)

func (p *Provider) EthereumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, p.rpcPath("v3/blockchain/node/ETH", isTest))
}

func (p *Provider) MaticRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, p.rpcPath("v3/blockchain/node/MATIC", isTest))
}

func (p *Provider) BinanceSmartChainRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, p.rpcPath("v3/blockchain/node/BSC", isTest))
}

func (p *Provider) rpcPath(path string, isTest bool) string {
	url := fmt.Sprintf("%s/%s/%s", p.config.BasePath, path, p.config.APIKey)
	if !isTest {
		return url
	}

	url = fmt.Sprintf("%s/%s/%s", p.config.BasePath, path, p.config.TestAPIKey)

	if strings.HasSuffix(path, "ETH") {
		url += "?testnetType=" + EthTestnet
	}

	return url
}
