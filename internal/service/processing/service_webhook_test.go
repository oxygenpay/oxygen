package processing_test

import (
	"testing"

	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestUnknownNetwork(t *testing.T) {
	tc := test.NewIntegrationTest(t)

	// Given a wallet
	wt := tc.Must.CreateWallet(t, "ETH", "0x123", "0x123", wallet.TypeInbound)

	// And a webhook
	wh := processing.TatumWebhook{
		Asset:         "ETH",
		Type:          "native",
		TransactionID: "0x444",
		Address:       "0x123",
		Sender:        "0x-sender",
	}

	err := tc.Services.Processing.ProcessIncomingWebhook(tc.Context, wt.UUID, "333", wh)
	assert.ErrorContains(t, err, "unknown ETH network id \"333\"")
}
