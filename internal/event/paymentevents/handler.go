package paymentevents

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/slack"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/internal/webhook"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Handler struct {
	merchants       *merchant.Service
	processing      *processing.Service
	payments        *payment.Service
	slackWebhookURL string
	logger          *zerolog.Logger
}

func New(
	merchants *merchant.Service,
	processingService *processing.Service,
	payments *payment.Service,
	slackWebhookURL string,
	logger *zerolog.Logger,
) *Handler {
	log := logger.With().Str("channel", "payment_events_consumer").Logger()

	return &Handler{
		merchants:       merchants,
		processing:      processingService,
		payments:        payments,
		slackWebhookURL: slackWebhookURL,
		logger:          &log,
	}
}

func (h *Handler) Consumers() map[bus.Topic][]bus.Consumer {
	return map[bus.Topic][]bus.Consumer{
		bus.TopicPaymentStatusUpdate: {
			h.ProcessPaymentStatusUpdate,
			h.SendSuccessfulPaymentNotification,
		},
		bus.TopicWithdrawals: {h.ProcessWithdrawals},
	}
}

type PaymentWebhook struct {
	ID     string `json:"id"`
	Status string `json:"status"`

	CustomerEmail string `json:"customerEmail"`

	SelectedBlockchain string `json:"selectedBlockchain"`
	SelectedCurrency   string `json:"selectedCurrency"`

	IsTest bool `json:"isTest"`

	LinkID *string `json:"paymentLinkId"`
}

func (h *Handler) ProcessPaymentStatusUpdate(ctx context.Context, message bus.Message) error {
	req, err := bus.Bind[bus.PaymentStatusUpdateEvent](message)
	if err != nil {
		return err
	}

	mt, err := h.merchants.GetByID(ctx, req.MerchantID, false)
	if err != nil {
		return errors.Wrap(err, "unable to get merchant")
	}

	webhookURL := mt.Settings().WebhookURL()
	signatureSecret := mt.Settings().WebhookSignatureSecret()

	if webhookURL == "" {
		h.logger.Warn().
			Int64("merchant_id", req.MerchantID).Int64("payment_id", req.PaymentID).
			Msg("webhook not set; skipping sending")

		return nil
	}

	p, err := h.processing.GetDetailedPayment(ctx, req.MerchantID, req.PaymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	// omit "locked" event
	if p.Payment.Status == payment.StatusLocked {
		return nil
	}

	wh := PaymentWebhook{
		ID:     p.Payment.MerchantOrderUUID.String(),
		Status: p.Payment.Status.String(),
		IsTest: p.Payment.IsTest,
	}
	if p.Customer != nil {
		wh.CustomerEmail = p.Customer.Email
	}
	if p.PaymentMethod != nil {
		wh.SelectedBlockchain = p.PaymentMethod.Currency.Blockchain.String()
		wh.SelectedCurrency = p.PaymentMethod.Currency.Ticker
	}
	if p.Payment.LinkID() != 0 {
		link, err := h.payments.GetPaymentLinkByID(ctx, mt.ID, p.Payment.LinkID())
		if err != nil {
			return errors.Wrap(err, "unable to get payment link")
		}

		wh.LinkID = util.Ptr(link.PublicID.String())
	}

	if err := webhook.Send(ctx, webhookURL, signatureSecret, wh); err != nil {
		h.logger.Warn().Err(err).
			Int64("merchant_id", req.MerchantID).
			Int64("payment_id", req.PaymentID).
			Interface("webhook", wh).
			Str("webhook_url", webhookURL).
			Msg("unable to send webhook")

		// todo some visual alert for merchant

		return nil
	}

	if err := h.payments.SetWebhookTimestamp(ctx, req.MerchantID, req.PaymentID, time.Now()); err != nil {
		return errors.Wrap(err, "unable to set webhook timestamp")
	}

	h.logger.Info().
		Int64("merchant_id", req.MerchantID).
		Int64("payment_id", req.PaymentID).
		Str("webhook_url", webhookURL).
		Msg("sent webhook to merchant")

	return nil
}

func (h *Handler) ProcessWithdrawals(ctx context.Context, message bus.Message) error {
	req, err := bus.Bind[bus.WithdrawalCreatedEvent](message)
	if err != nil {
		return err
	}

	h.logger.Info().
		Int64("merchant_id", req.MerchantID).
		Int64("payment_id", req.PaymentID).
		Msg("incoming withdrawal request")

	_, err = h.processing.BatchCreateWithdrawals(ctx, []int64{req.PaymentID})
	if err != nil {
		return errors.Wrap(err, "unable to process withdrawal creation")
	}

	return nil
}

func (h *Handler) SendSuccessfulPaymentNotification(ctx context.Context, message bus.Message) error {
	req, err := bus.Bind[bus.PaymentStatusUpdateEvent](message)
	if err != nil {
		return err
	}

	p, err := h.processing.GetDetailedPayment(ctx, req.MerchantID, req.PaymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	if p.Payment.Status != payment.StatusSuccess {
		// skip
		return nil
	}

	content := fmt.Sprintf(
		"[%s] 💰processed payment #%d: %s %s for merchant %q id#%d (isTest=%t) ",
		extractHost(p.Payment.PaymentURL),
		p.Payment.ID,
		p.Payment.Price.String(),
		p.Payment.Price.Ticker(),
		p.Merchant.Name,
		p.Merchant.ID,
		p.Payment.IsTest,
	)

	return h.sendSlackMessage(content)
}

func (h *Handler) sendSlackMessage(messages ...string) error {
	if h.slackWebhookURL == "" {
		h.logger.Warn().Msg("Skipping slack notification send due to empty webhook url")
		return nil
	}

	return slack.SendWebhook(h.slackWebhookURL, messages...)
}

func extractHost(u string) string {
	link, _ := url.Parse(u)

	return link.Host
}
