package mock

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

// ProcessingProxyMock proxies some methods of processing.Service and mocks others
type ProcessingProxyMock struct {
	t                          *testing.T
	service                    *processing.Service
	mu                         sync.RWMutex
	incomingCheckCalls         map[string]error
	internalTransferCalls      map[string]lo.Tuple2[*processing.TransferResult, error]
	internalTransferCheckCalls map[string]error
	withdrawalTransferCalls    map[string]lo.Tuple2[*processing.TransferResult, error]
	withdrawalCheckCalls       map[string]error
	expirationCheckCalls       map[string]error
}

func NewProcessingProxyMock(t *testing.T, service *processing.Service) *ProcessingProxyMock {
	return &ProcessingProxyMock{
		t:                          t,
		service:                    service,
		incomingCheckCalls:         map[string]error{},
		internalTransferCalls:      map[string]lo.Tuple2[*processing.TransferResult, error]{},
		internalTransferCheckCalls: map[string]error{},
		withdrawalTransferCalls:    map[string]lo.Tuple2[*processing.TransferResult, error]{},
		withdrawalCheckCalls:       map[string]error{},
		expirationCheckCalls:       map[string]error{},
	}
}

func (m *ProcessingProxyMock) BatchCheckIncomingTransactions(_ context.Context, transactionIDs []int64) error {
	key := idsKey(transactionIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.incomingCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCheckIncomingTransactions for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) BatchCreateInternalTransfers(
	_ context.Context,
	balances []*wallet.Balance,
) (*processing.TransferResult, error) {
	key := m.transferKey(balances)

	m.mu.RLock()
	defer m.mu.RUnlock()

	res, exists := m.internalTransferCalls[key]
	if !exists {
		return nil, fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCreateInternalTransfers for %q", key)
	}

	return res.A, res.B
}

func (m *ProcessingProxyMock) BatchCheckInternalTransfers(_ context.Context, transactionIDs []int64) error {
	key := idsKey(transactionIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.internalTransferCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCheckInternalTransfers for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) BatchCreateWithdrawals(
	_ context.Context,
	paymentIDs []int64,
) (*processing.TransferResult, error) {
	key := idsKey(paymentIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	res, exists := m.withdrawalTransferCalls[key]
	if !exists {
		return nil, fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCreateWithdrawals for %q", key)
	}

	return res.A, res.B
}

func (m *ProcessingProxyMock) BatchCheckWithdrawals(_ context.Context, transactionIDs []int64) error {
	key := idsKey(transactionIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.withdrawalCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchCheckWithdrawals for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) BatchExpirePayments(_ context.Context, paymentIDs []int64) error {
	key := idsKey(paymentIDs)

	m.mu.RLock()
	defer m.mu.RUnlock()

	err, exists := m.expirationCheckCalls[key]
	if !exists {
		return fmt.Errorf("unexpected call (*ProcessingProxyMock).BatchExpirePayments for %q", key)
	}

	return err
}

func (m *ProcessingProxyMock) SetupBatchCreateInternalTransfers(
	balances []*wallet.Balance,
	result *processing.TransferResult,
	err error,
) {
	key := m.transferKey(balances)
	if result == nil {
		result = &processing.TransferResult{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.internalTransferCalls[key] = lo.T2(result, err)
}

func (m *ProcessingProxyMock) SetupBatchCheckIncomingTransactions(transactionIDs []int64, err error) {
	key := idsKey(transactionIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.incomingCheckCalls[key] = err
}

func (m *ProcessingProxyMock) SetupBatchCheckInternalTransfers(transactionIDs []int64, err error) {
	key := idsKey(transactionIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.internalTransferCheckCalls[key] = err
}

func (m *ProcessingProxyMock) SetupBatchCreateWithdrawals(
	paymentIDs []int64,
	result *processing.TransferResult,
	err error,
) {
	key := idsKey(paymentIDs)
	if result == nil {
		result = &processing.TransferResult{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.withdrawalTransferCalls[key] = lo.T2(result, err)
}

func (m *ProcessingProxyMock) SetupBatchCheckWithdrawals(transactionIDs []int64, err error) {
	key := idsKey(transactionIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.withdrawalCheckCalls[key] = err
}

func (m *ProcessingProxyMock) SetupBatchExpirePayments(paymentsIDs []int64, err error) {
	key := idsKey(paymentsIDs)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.expirationCheckCalls[key] = err
}

func (m *ProcessingProxyMock) EnsureOutboundWallet(ctx context.Context, chain money.Blockchain) (*wallet.Wallet, bool, error) {
	return m.service.EnsureOutboundWallet(ctx, chain)
}

const empty = "[ <empty> ]"

func (m *ProcessingProxyMock) transferKey(balances []*wallet.Balance) string {
	return idsKey(util.MapSlice(
		balances,
		func(b *wallet.Balance) int64 { return b.ID }),
	)
}

func idsKey(ids []int64) string {
	if len(ids) == 0 {
		return empty
	}

	slices.Sort(ids)

	stringInts := util.MapSlice(ids, func(id int64) string { return strconv.Itoa(int(id)) })

	return "[" + strings.Join(stringInts, ", ") + "]"
}
