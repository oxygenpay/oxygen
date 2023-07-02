package wallet_test

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/test/fakes"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTronProvider_Generate(t *testing.T) {
	const (
		mockAddress    = "TCNkawTmcQgYSU8nP8cHswT1QPjharxJr7"
		mockPubKey     = "0x041b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f70beaf8f588b541507fed6a642c5ab42dfdf8120a7f639de5122d47a69a8e8d1"
		mockPrivateKey = "0x0101010101010101010101010101010101010101010101010101010101010101"
	)

	p := &wallet.TronProvider{
		Blockchain:   wallet.TRON,
		CryptoReader: &fakeReader{},
	}

	t.Run("Mock_GenerationSuccessful", func(t *testing.T) {
		w := p.Generate()

		assert.Equal(t, mockAddress, w.Address)
		assert.Equal(t, mockPubKey, w.PublicKey)
		assert.Equal(t, mockPrivateKey, w.PrivateKey)
	})

	t.Run("Mock_PrivateKeyAsStringToPublicKey", func(t *testing.T) {
		w := p.Generate()

		key, err := crypto.HexToECDSA(w.PrivateKey[2:])
		require.NoError(t, err)

		publicKey := hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))
		assert.Equal(t, publicKey, w.PublicKey)
	})

	t.Run("Real_GenerationSuccessful", func(t *testing.T) {
		p := &wallet.TronProvider{
			Blockchain:   wallet.TRON,
			CryptoReader: cryptorand.Reader,
		}

		w := p.Generate()

		key, err := crypto.HexToECDSA(w.PrivateKey[2:])
		require.NoError(t, err)

		publicKey := hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))
		assert.Equal(t, publicKey, w.PublicKey)
	})

	t.Run("Base58ToHex", func(t *testing.T) {
		p := &wallet.TronProvider{
			Blockchain:   wallet.TRON,
			CryptoReader: cryptorand.Reader,
		}

		for _, tc := range []struct {
			addrBase58 string
			addrHex    string
		}{
			{
				addrBase58: "TBREsCfBdPyD612xZnwvGPux7osbXvtzLh",
				addrHex:    "410fe47f49fd91f0edb7fa2b94a3c45d9c2231709c",
			},
			{
				addrBase58: "TLrPF5aTqtqLxFo1Y69jpQvZ2T4ydkX9pC",
				addrHex:    "41775f057150b85086f456aac88fd52ba8e35db282",
			},
			{
				addrBase58: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
				addrHex:    "41a614f803b6fd780986a42c78ec9c7f77e6ded13c",
			},
			{
				addrBase58: "THPvaUhoh2Qn2y9THCZML3H815hhFhn5YC",
				addrHex:    "41517591d35d313bf6a5e33098284502b045e2bc08",
			},
		} {
			actual, err := p.Base58ToHexAddress(tc.addrBase58)
			assert.NoError(t, err)
			assert.Equal(t, tc.addrHex, actual)
		}
	})
}

func TestTronProvider_ValidateAddress(t *testing.T) {
	p := &wallet.TronProvider{}

	for _, tc := range []struct {
		addr          string
		expectInvalid bool
	}{
		{addr: "TW9Lv3KUDwqHThtPNfudA8ZwLNwqiCUpwD"},
		{addr: "TCy1P7aRqvcgrY2BwbrV4aZ8ujfUCsy8kr"},
		{addr: "TCy1P7aRqvcgrY2BwbrV4aZ8ujfUCsy8krA", expectInvalid: true},
		{addr: "TCy1P7aRqvcgrY2BwbrV4aZ8ujfUCsy8kI", expectInvalid: true},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			assert.Equal(t, !tc.expectInvalid, p.ValidateAddress(tc.addr))
		})
	}
}

func TestTronProvider_NewTransaction(t *testing.T) {
	// ARRANGE
	// Given some constants
	const (
		addressRecipient = "TTYxentT3sf8XHbtHGyWX2uDgdadE9uYSL"
		usdtContract     = "TBnt7Wzvd226i24r95pE82MZpHba63ehQY"
		rawData          = `{"hello": "world"}`
		rawDataHex       = "a9059cbb"
	)

	// And mocked trongrid provider
	provider, trongridMock := fakes.NewTrongrid(util.Ptr(zerolog.Nop()))

	// And TronProvider
	p := &wallet.TronProvider{
		Blockchain:   wallet.TRON,
		CryptoReader: &fakeReader{},
		Trongrid:     provider,
	}

	// And generated wallet
	w := p.Generate()

	for testCaseIndex, testCase := range []struct {
		req    wallet.TronTransactionParams
		error  error
		setup  func(t *testing.T, tg *fakes.Trongrid, req wallet.TronTransactionParams)
		assert func(t *testing.T, req wallet.TronTransactionParams, tx wallet.TronTransaction)
	}{
		// Success coin tx
		{
			req: wallet.TronTransactionParams{
				Type:      wallet.Coin,
				Recipient: addressRecipient,
				Amount:    "100000",
			},
			setup: func(t *testing.T, tg *fakes.Trongrid, req wallet.TronTransactionParams) {
				tg.SetupCreateTransaction(
					trongrid.TransactionRequest{
						OwnerAddress: w.Address,
						ToAddress:    req.Recipient,
						Amount:       100000,
						Visible:      true,
					},
					trongrid.Transaction{
						TxID:       "abc123",
						RawDataHex: rawDataHex,
						RawData:    json.RawMessage(rawData),
						Visible:    true,
					},
				)
			},
			assert: func(t *testing.T, req wallet.TronTransactionParams, tx wallet.TronTransaction) {
				assert.Empty(t, tx.Error)
				assert.Len(t, tx.Signature, 1)
				assert.NotEmpty(t, tx.Signature[0])
			},
		},
		// Fail coin tx
		{
			req: wallet.TronTransactionParams{
				Type:      wallet.Coin,
				Recipient: "abc",
				Amount:    "100000",
			},
			error: wallet.ErrInvalidAddress,
		},
		{
			req: wallet.TronTransactionParams{
				Type:      wallet.Coin,
				Recipient: addressRecipient,
				Amount:    "-100000",
			},
			error: wallet.ErrInvalidAmount,
		},
		// Success token tx
		{
			req: wallet.TronTransactionParams{
				Type:            wallet.Token,
				Recipient:       addressRecipient,
				ContractAddress: usdtContract,
				Amount:          "20000",
				FeeLimit:        10000,
			},
			setup: func(t *testing.T, tg *fakes.Trongrid, req wallet.TronTransactionParams) {
				tg.SetupTriggerSmartContract(
					trongrid.ContractCallRequest{
						OwnerAddress:     w.Address,
						ContractAddress:  req.ContractAddress,
						FunctionSelector: "transfer(address,uint256)",
						Parameter:        "static-mock",
						FeeLimit:         req.FeeLimit,
						Visible:          true,
					},
					trongrid.Transaction{
						TxID:       "abc123",
						RawDataHex: rawDataHex,
						RawData:    json.RawMessage(rawData),
						Visible:    true,
					},
				)
			},
			assert: func(t *testing.T, req wallet.TronTransactionParams, tx wallet.TronTransaction) {
				assert.Empty(t, tx.Error)
				assert.Len(t, tx.Signature, 1)
				assert.NotEmpty(t, tx.Signature[0])
			},
		},
		// Fail token tx
		{
			req: wallet.TronTransactionParams{
				Type:      wallet.Token,
				Recipient: "abc",
				Amount:    "100000",
			},
			error: wallet.ErrInvalidAddress,
		},
		{
			req: wallet.TronTransactionParams{
				Type:      wallet.Token,
				Recipient: addressRecipient,
				Amount:    "100000",
			},
			error: wallet.ErrInvalidContractAddress,
		},
		{
			req: wallet.TronTransactionParams{
				Type:            wallet.Token,
				Recipient:       addressRecipient,
				Amount:          "100000",
				ContractAddress: usdtContract,
				FeeLimit:        0,
			},
			error: wallet.ErrInvalidGasSettings,
		},
	} {
		t.Run(strconv.Itoa(testCaseIndex), func(t *testing.T) {
			// ARRANGE
			if testCase.setup != nil {
				testCase.setup(t, trongridMock, testCase.req)
			}

			// ACT
			// Create (generate) transaction
			tx, err := p.NewTransaction(context.Background(), w, testCase.req)

			// ASSERT
			if testCase.error != nil {
				assert.ErrorIs(t, err, testCase.error)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, tx)

			if testCase.assert != nil {
				testCase.assert(t, testCase.req, tx)
			}
		})
	}
}
