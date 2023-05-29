package bus

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type sampleMessage struct {
	PaymentID int64
}

func TestPubSub_Publish(t *testing.T) {
	// ARRANGE
	ctx := context.Background()
	logger := zerolog.New(os.Stdout)

	const topic = Topic("test")

	t.Run("Works well", func(t *testing.T) {
		// Given a bus
		bus := NewPubSub(ctx, true, &logger)

		// And actual values map
		actualMap := map[int64]int64{}
		mu := &sync.Mutex{}

		// And sample subscriber that takes message and
		// increments payment's counter by payment id
		err := bus.Subscribe(topic, func(ctx context.Context, message Message) error {
			sample, err := Bind[sampleMessage](message)
			if err != nil {
				return err
			}

			mu.Lock()
			defer mu.Unlock()

			actualMap[sample.PaymentID]++

			return nil
		})

		require.NoError(t, err)

		// ACT
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 10}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 10}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 10}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 64}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 64}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 2}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 3}))

		require.NoError(t, bus.Shutdown())

		// ASSERT
		expectedMap := map[int64]int64{
			10: 3,
			64: 2,
			2:  1,
			3:  1,
		}

		assert.Equal(t, expectedMap, actualMap)
	})

	t.Run("Handles panics", func(t *testing.T) {
		// Given a bus
		bus := NewPubSub(ctx, true, &logger)

		var b atomic.Bool

		// And sample subscriber that returns a panic
		err := bus.Subscribe(topic, func(_ context.Context, message Message) error {
			sample, err := Bind[sampleMessage](message)
			if err != nil {
				return err
			}

			if sample.PaymentID == 1 {
				panic("omg!")
			} else {
				b.Store(true)
			}

			return nil
		})

		require.NoError(t, err)

		// ACT
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 1}))
		require.NoError(t, bus.Publish(topic, sampleMessage{PaymentID: 2}))

		time.Sleep(time.Millisecond * 100)

		// ASSERT
		assert.True(t, b.Load())
		assert.NoError(t, bus.Shutdown())
	})
}
