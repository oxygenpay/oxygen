package fakes

import (
	"sync"

	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/samber/lo"
)

type Bus struct {
	mu       sync.RWMutex
	busCalls []lo.Tuple2[bus.Topic, any]
}

func (b *Bus) Publish(topic bus.Topic, message any) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.busCalls = append(b.busCalls, lo.T2(topic, message))

	return nil
}

func (b *Bus) GetBusCalls() []lo.Tuple2[bus.Topic, any] {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]lo.Tuple2[bus.Topic, any], len(b.busCalls))
	copy(result, b.busCalls)

	return result
}

func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.busCalls = nil
}
