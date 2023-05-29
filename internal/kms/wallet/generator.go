package wallet

import (
	"sync"

	"github.com/pkg/errors"
)

type Provider interface {
	Generate() *Wallet
	GetBlockchain() Blockchain
	ValidateAddress(address string) bool
}

type Generator struct {
	mu        sync.RWMutex
	providers map[Blockchain]Provider
}

func NewGenerator() *Generator {
	return &Generator{
		mu:        sync.RWMutex{},
		providers: make(map[Blockchain]Provider),
	}
}

func (g *Generator) AddProvider(provider Provider) *Generator {
	g.mu.Lock()
	g.providers[provider.GetBlockchain()] = provider
	g.mu.Unlock()

	return g
}

func (g *Generator) CreateWallet(blockchain Blockchain) (*Wallet, error) {
	if !blockchain.IsValid() {
		return nil, ErrUnknownBlockchain
	}

	var selectedProvider Provider

	g.mu.RLock()
	if p, ok := g.providers[blockchain]; ok {
		selectedProvider = p
	}
	g.mu.RUnlock()

	if selectedProvider == nil {
		return nil, errors.Wrap(ErrUnknownBlockchain, "provider not found")
	}

	return selectedProvider.Generate(), nil
}
