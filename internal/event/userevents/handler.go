package userevents

import (
	"context"
	"fmt"

	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/internal/slack"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Handler struct {
	env             string
	slackWebhookURL string
	users           *user.Service
	logger          *zerolog.Logger
}

func New(
	env string,
	slackWebhookURL string,
	users *user.Service,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "user_events_consumer").Logger()

	return &Handler{
		env:             env,
		slackWebhookURL: slackWebhookURL,
		users:           users,
		logger:          &log,
	}
}

func (h *Handler) Consumers() map[bus.Topic][]bus.Consumer {
	return map[bus.Topic][]bus.Consumer{
		bus.TopicFormSubmissions: {h.FormSubmitted},
		bus.TopicUserRegistered:  {h.UserRegistered},
	}
}

func (h *Handler) FormSubmitted(ctx context.Context, message bus.Message) error {
	event, err := bus.Bind[bus.FormSubmittedEvent](message)
	if err != nil {
		return err
	}

	u, err := h.users.GetByID(ctx, event.UserID)
	if err != nil {
		return errors.Wrap(err, "unable to get user")
	}

	return h.sendSlackMessage(
		fmt.Sprintf("[%s] New request from user %s (%s):", h.env, u.Email, event.RequestType),
		fmt.Sprintf("Message: %s", event.Message),
	)
}

func (h *Handler) UserRegistered(ctx context.Context, message bus.Message) error {
	event, err := bus.Bind[bus.UserRegisteredEvent](message)
	if err != nil {
		return err
	}

	u, err := h.users.GetByID(ctx, event.UserID)
	if err != nil {
		return errors.Wrap(err, "unable to get user")
	}

	return h.sendSlackMessage(fmt.Sprintf("[%s] 🔥New user registration: %s %s", h.env, u.Name, u.Email))
}

func (h *Handler) sendSlackMessage(messages ...string) error {
	if h.slackWebhookURL == "" {
		h.logger.Warn().Msg("Skipping slack notification send due to empty webhook url")
		return nil
	}

	return slack.SendWebhook(h.slackWebhookURL, messages...)
}
