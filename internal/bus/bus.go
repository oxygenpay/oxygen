package bus

import (
	"context"
	"encoding/json"

	evbus "github.com/asaskevich/EventBus"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// PubSub simple pub-sub implementation. Works locally using in-memory store.
// No persistence, no semaphores. Should do the job until we switch to a real bus (e.g. Kafka).
type PubSub struct {
	ctx    context.Context
	bus    evbus.Bus
	async  bool
	logger *zerolog.Logger
}

type Topic string
type Message []byte
type Consumer func(ctx context.Context, message Message) error

type Handler interface {
	Consumers() map[Topic][]Consumer
}

type Publisher interface {
	Publish(topic Topic, message any) error
}

func NewPubSub(ctx context.Context, async bool, logger *zerolog.Logger) *PubSub {
	log := logger.With().Str("channel", "event_bus").Logger()

	return &PubSub{
		ctx:    ctx,
		bus:    evbus.New(),
		async:  async,
		logger: &log,
	}
}

func (p *PubSub) RegisterHandler(h Handler) error {
	for topic, consumers := range h.Consumers() {
		for _, c := range consumers {
			if err := p.Subscribe(topic, c); err != nil {
				return errors.Wrapf(err, "unable to subscibe to topic %q", topic)
			}
		}
	}

	return nil
}

func (p *PubSub) Subscribe(topic Topic, fn Consumer) error {
	sTopic := string(topic)

	wrapper := func(ctx context.Context, message Message) {
		defer func() {
			if err := recover(); err != nil {
				p.logger.Error().Interface("panic", err).
					Str("topic", sTopic).Bytes("message", message).
					Msg("consumer panic")
			}
		}()

		if err := fn(ctx, message); err != nil {
			p.logger.Error().Err(err).
				Str("topic", sTopic).Bytes("message", message).
				Msg("consumer failed")
		}
	}

	if p.async {
		return p.bus.SubscribeAsync(sTopic, wrapper, false)
	}

	return p.bus.Subscribe(sTopic, wrapper)
}

func (p *PubSub) Publish(topic Topic, message any) error {
	raw, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if !p.bus.HasCallback(string(topic)) {
		p.logger.Warn().
			Str("topic", string(topic)).
			Bytes("message", raw).
			Msg("topic has no subscribers")
	}

	p.bus.Publish(string(topic), p.ctx, raw)

	return nil
}

func (p *PubSub) Shutdown() error {
	p.logger.Info().Msg("Shutting down event listener")
	p.bus.WaitAsync()

	return nil
}

func Bind[T comparable](input Message) (T, error) {
	var value T
	if err := json.Unmarshal(input, &value); err != nil {
		return value, errors.Wrapf(err, "unable to bind message to %+v", value)
	}

	var empty T
	if value == empty {
		return value, errors.New("message is empty")
	}

	return value, nil
}
