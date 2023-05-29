package fakes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type Broadcaster struct {
	t          *testing.T
	mu         sync.RWMutex
	broadcasts map[string]lo.Tuple2[string, error]
	receipts   map[string]lo.Tuple2[*blockchain.TransactionReceipt, error]
}

func newBroadcaster(t *testing.T) *Broadcaster {
	return &Broadcaster{
		t:          t,
		broadcasts: make(map[string]lo.Tuple2[string, error]),
		receipts:   map[string]lo.Tuple2[*blockchain.TransactionReceipt, error]{},
	}
}

func (m *Broadcaster) BroadcastTransaction(
	_ context.Context,
	chain money.Blockchain,
	raw string,
	isTest bool,
) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.broadcastKey(chain, raw, isTest)

	result, exists := m.broadcasts[key]
	if !exists {
		return "", errors.New("unexpected call of (*BroadcasterMock).BroadcastTransaction with args " + key)
	}

	return result.A, result.B
}

func (m *Broadcaster) GetTransactionReceipt(
	_ context.Context, chain money.Blockchain, txID string, isTest bool,
) (*blockchain.TransactionReceipt, error) {
	key := m.receiptKey(chain, txID, isTest)

	res, exists := m.receipts[key]
	if !exists {
		return nil, errors.New("unexpected call of (*BroadcasterMock).GetTransactionReceipt with args " + key)
	}

	return res.A, res.B
}

func (m *Broadcaster) SetupBroadcastTransaction(
	chain money.Blockchain,
	rawTransaction string,
	isTest bool,
	txHash string,
	err error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rawTX := rawTransaction
	if chain == kms.TRON.ToMoneyBlockchain() {
		b, _ := json.Marshal(kmsmodel.TronTransaction{RawDataHex: rawTX})
		rawTX = string(b)
	}

	m.broadcasts[m.broadcastKey(chain, rawTX, isTest)] = lo.T2(txHash, err)
}

func (m *Broadcaster) SetupGetTransactionReceipt(
	chain money.Blockchain,
	txID string,
	isTest bool,
	receipt *blockchain.TransactionReceipt,
	err error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.receipts[m.receiptKey(chain, txID, isTest)] = lo.T2(receipt, err)
}

func (m *Broadcaster) broadcastKey(chain money.Blockchain, raw string, isTest bool) string {
	return fmt.Sprintf("%s/%s/%t", chain.String(), raw, isTest)
}

func (m *Broadcaster) receiptKey(chain money.Blockchain, txID string, isTest bool) string {
	return fmt.Sprintf("%s/%s/%t", chain.String(), txID, isTest)
}
