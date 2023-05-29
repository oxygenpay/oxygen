package tatum

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
)

func (p *Provider) EthereumRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	const path = "v3/blockchain/node/ETH"

	url := fmt.Sprintf("%s/%s/%s", p.config.BasePath, path, p.config.APIKey)
	if isTest {
		url = fmt.Sprintf("%s/%s/%s?testnetType=%s", p.config.BasePath, path, p.config.TestAPIKey, EthTestnet)
	}

	return ethclient.DialContext(ctx, url)
}

func (p *Provider) MaticRPC(ctx context.Context, isTest bool) (*ethclient.Client, error) {
	const path = "v3/blockchain/node/MATIC"

	url := fmt.Sprintf("%s/%s/%s", p.config.BasePath, path, p.config.APIKey)
	if isTest {
		url = fmt.Sprintf("%s/%s/%s", p.config.BasePath, path, p.config.TestAPIKey)
	}

	return ethclient.DialContext(ctx, url)
}
