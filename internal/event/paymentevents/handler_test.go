package paymentevents_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/event/paymentevents"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*test.IntegrationTest, *paymentevents.Handler, *[]string) {
	tc := test.NewIntegrationTest(t)

	var responses []string

	okResponder := func(writer http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		responses = append(responses, string(raw))

		writer.WriteHeader(http.StatusOK)
	}

	handler := paymentevents.New(
		tc.Services.Merchants,
		tc.Services.Processing,
		tc.Services.Payment,
		httptest.NewServer(http.HandlerFunc(okResponder)).URL,
		tc.Logger,
	)

	return tc, handler, &responses
}

func TestHandler_ProcessPaymentStatusUpdate(t *testing.T) {
	tc, handler, responses := setup(t)

	const merchantID = 1

	// ARRANGE
	// Given a mocked merchant server
	var actualWebhook paymentevents.PaymentWebhook
	srv := assertServer(t, func(t *testing.T, writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)

		assertBind(t, request, &actualWebhook)
	})

	// And a merchant
	mt, err := tc.Services.Merchants.Create(tc.Context, merchantID, "my-site", "my-site.com", merchant.Settings{
		merchant.PropertySignatureSecret: "abc",
		merchant.PropertyWebhookURL:      srv.URL,
	})

	require.NoError(t, err)
	require.Equal(t, "abc", mt.Settings().WebhookSignatureSecret())
	require.Equal(t, srv.URL, mt.Settings().WebhookURL())

	// ... and a payment make from a link
	// This payment represents the most extended webhook case with payment link
	link, err := tc.Services.Payment.CreatePaymentLink(tc.Context, mt.ID, payment.CreateLinkProps{
		Name:          "test link",
		Price:         lo.Must(money.USD.MakeAmount("5000")),
		SuccessAction: payment.SuccessActionRedirect,
		RedirectURL:   util.Ptr("https://site.com"),
	})
	require.NoError(t, err)

	p, err := tc.Services.Payment.CreatePaymentFromLink(tc.Context, link)
	require.NoError(t, err)

	// And a transaction
	tx := tc.Must.CreateTransaction(t, merchantID, func(p *transaction.CreateTransaction) {
		p.RecipientWallet = tc.Must.CreateWallet(t, "ETH", "0x123", "pub-key", wallet.TypeInbound)
	})

	// And a customer
	person, err := tc.Services.Payment.AssignCustomerByEmail(tc.Context, p, "test@me.com")
	require.NoError(t, err)

	// And the payment mocked as "successful"
	_, err = tc.Repository.UpdatePayment(tc.Context, repository.UpdatePaymentParams{
		ID:         p.ID,
		MerchantID: p.MerchantID,
		Status:     payment.StatusSuccess.String(),
		UpdatedAt:  time.Now(),
	})
	require.NoError(t, err)

	// ACT
	msg := marshal(bus.PaymentStatusUpdateEvent{
		MerchantID: p.MerchantID,
		PaymentID:  p.ID,
	})

	assert.NoError(t, handler.ProcessPaymentStatusUpdate(tc.Context, msg))
	assert.NoError(t, handler.SendSuccessfulPaymentNotification(tc.Context, msg))

	// ASSERT
	expectedWebhook := paymentevents.PaymentWebhook{
		ID:                 p.MerchantOrderUUID.String(),
		Status:             payment.StatusSuccess.String(),
		CustomerEmail:      person.Email,
		SelectedBlockchain: tx.Currency.Blockchain.String(),
		SelectedCurrency:   tx.Currency.Ticker,
		LinkID:             util.Ptr(link.PublicID.String()),
		IsTest:             p.IsTest,
	}

	assert.Equal(t, expectedWebhook, actualWebhook)

	// Check that webhook timestamp was updated
	freshPayment, err := tc.Services.Payment.GetByID(tc.Context, merchantID, p.ID)
	assert.NoError(t, err)
	assert.NotNil(t, freshPayment.WebhookSentAt)

	// Check Slack notification
	assert.Len(t, *responses, 1)
	assert.Contains(t, (*responses)[0], "processed payment #1: 50 USD for merchant")
	assert.Contains(t, (*responses)[0], mt.Name)
	assert.Contains(t, (*responses)[0], "isTest=false")
	assert.Contains(t, (*responses)[0], "pay.o2pay.co")
}

func marshal(v any) []byte {
	return lo.Must(json.Marshal(v))
}

func assertBind(t *testing.T, request *http.Request, v any) {
	bytes, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(bytes, v))
}

func assertServer(t *testing.T, handler func(*testing.T, http.ResponseWriter, *http.Request)) *httptest.Server {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		handler(t, writer, request)
	}

	return httptest.NewServer(http.HandlerFunc(fn))
}
