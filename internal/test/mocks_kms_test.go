package test

import (
	"testing"

	"github.com/oxygenpay/oxygen/internal/service/wallet"
	kmswallet "github.com/oxygenpay/oxygen/pkg/api-kms/v1/client/wallet"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/stretchr/testify/assert"
)

func TestIntegrationTest_MockCreateEthereumTransaction(t *testing.T) {
	tc := NewIntegrationTest(t)

	// ARRANGE
	// Given a wallet
	wallet1 := tc.Must.CreateWallet(t, "ETH", "0xabc", "pub-key", wallet.TypeInbound)

	// And expected rawTx
	expectedRawTx := "0xf86e83014b2985048ccb44b182753...eb85a3b7bb5cf3749245e907158e9c8daa033c7ec9362ee890"

	// And mocked eth transaction
	req := kmsmodel.CreateEthereumTransactionRequest{
		Amount:            "999",
		AssetType:         "coin",
		Gas:               2,
		MaxFeePerGas:      "123",
		MaxPriorityPerGas: "456",
		NetworkID:         1,
		Recipient:         "0xabc",
	}

	tc.SetupCreateEthereumTransaction(wallet1.UUID, req, expectedRawTx)

	// ACT
	res, err := tc.Providers.KMS.CreateEthereumTransaction(&kmswallet.CreateEthereumTransactionParams{
		Data:     &req,
		WalletID: wallet1.UUID.String(),
	})

	// ASSERT
	assert.NoError(t, err)
	assert.Equal(t, expectedRawTx, res.Payload.RawTransaction)
}

func TestIntegrationTest_MockCreateMaticTransaction(t *testing.T) {
	tc := NewIntegrationTest(t)

	// ARRANGE
	// Given a wallet
	wallet1 := tc.Must.CreateWallet(t, "MATIC", "0xabc", "pub-key", wallet.TypeInbound)

	// And expected rawTx
	expectedRawTx := "0xf86e83014b2985048ccb44b182753...eb85a3b7bb5cf3749245e907158e9c8daa033c7ec9362ee890"

	// And mocked eth transaction
	req := kmsmodel.CreateMaticTransactionRequest{
		Amount:            "999",
		AssetType:         "coin",
		Gas:               2,
		MaxFeePerGas:      "123",
		MaxPriorityPerGas: "456",
		NetworkID:         1,
		Recipient:         "0xabc",
	}

	tc.SetupCreateMaticTransaction(wallet1.UUID, req, expectedRawTx)

	// ACT
	res, err := tc.Providers.KMS.CreateMaticTransaction(&kmswallet.CreateMaticTransactionParams{
		Data:     &req,
		WalletID: wallet1.UUID.String(),
	})

	// ASSERT
	assert.NoError(t, err)
	assert.Equal(t, expectedRawTx, res.Payload.RawTransaction)
}
