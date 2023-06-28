package wallet_test

import (
	cryptorand "crypto/rand"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEthProvider_Generate(t *testing.T) {
	const (
		mockAddress    = "0x1a642f0E3c3aF545E7AcBD38b07251B3990914F1"
		mockPubKey     = "0x041b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f70beaf8f588b541507fed6a642c5ab42dfdf8120a7f639de5122d47a69a8e8d1"
		mockPrivateKey = "0x0101010101010101010101010101010101010101010101010101010101010101"
	)

	p := &wallet.EthProvider{
		Blockchain:   wallet.ETH,
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

	t.Run("Mock_PrivateKeyAsStringToAddress", func(t *testing.T) {
		w := p.Generate()

		key, err := crypto.HexToECDSA(w.PrivateKey[2:])
		require.NoError(t, err)

		address := crypto.PubkeyToAddress(key.PublicKey).Hex()
		assert.Equal(t, address, w.Address)
	})

	t.Run("Real_GenerationSuccessful", func(t *testing.T) {
		p := &wallet.EthProvider{
			Blockchain:   wallet.ETH,
			CryptoReader: cryptorand.Reader,
		}

		w := p.Generate()

		key, err := crypto.HexToECDSA(w.PrivateKey[2:])
		require.NoError(t, err)

		publicKey := hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))
		address := crypto.PubkeyToAddress(key.PublicKey).Hex()

		assert.Equal(t, w.PublicKey, publicKey)
		assert.Equal(t, w.Address, address)
	})
}

func TestEthProvider_ValidateAddress(t *testing.T) {
	p := &wallet.EthProvider{}

	for _, tc := range []struct {
		addr          string
		expectInvalid bool
	}{
		{addr: "0x95222290DD7278Aa3Ddd389Cc1E1d165CC4BAfe5"},
		{addr: "0xb364e75b1189dcbbf7f0c856456c1ba8e4d6481b"},
		{addr: "1xb364e75b1189dcbbf7f0c856456c1ba8e4d6481b", expectInvalid: true},
		{addr: "xb364e75b1189dcbbf7f0c856456c1ba8e4d6481b", expectInvalid: true},
		{addr: "wwwxb364e75b1189dcbbf7f0c856456c1ba8e4d6481b", expectInvalid: true},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			assert.Equal(t, !tc.expectInvalid, p.ValidateAddress(tc.addr))
		})
	}
}

func TestEthProvider_NewTransaction(t *testing.T) {
	const (
		addressRecipient = "0x816840B298C3A326330236aC1368d3887d27A7Cb"
		usdtContract     = "0xdac17f958d2ee523a2206206994597c13d831ec7"
	)

	p := &wallet.EthProvider{Blockchain: wallet.ETH, CryptoReader: &fakeReader{}}
	w := p.Generate()

	amount1, err := money.CryptoFromStringFloat("ETH", "0.5", 18)
	require.NoError(t, err)

	for testCaseIndex, testCase := range []struct {
		req    wallet.EthTransactionParams
		error  error
		assert func(t *testing.T, req wallet.EthTransactionParams, encoded string)
	}{
		// Success coin tx
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            addressRecipient,
				Amount:               amount1.StringRaw(),
				NetworkID:            1,
				Nonce:                0,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  2,
			},
			assert: func(t *testing.T, req wallet.EthTransactionParams, encoded string) {
				encodedHex, err := hexutil.Decode(encoded)
				assert.NoError(t, err)

				tx := &types.Transaction{}
				assert.NoError(t, tx.UnmarshalBinary(encodedHex))

				assert.Equal(t, uint64(req.Gas), tx.Gas())
				assert.Equal(t, req.Recipient, tx.To().String())
			},
		},
		// Fail coin txs
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            "0xABC",
				Amount:               "123",
				NetworkID:            1,
				Nonce:                -1,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  2,
			},
			error: wallet.ErrInvalidAddress,
		},
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            addressRecipient,
				Amount:               "123",
				NetworkID:            1,
				Nonce:                -1,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  2,
			},
			error: wallet.ErrInvalidNonce,
		},
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            addressRecipient,
				Amount:               "123",
				NetworkID:            1,
				Nonce:                -1,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  0,
			},
			error: wallet.ErrInvalidGasSettings,
		},
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            addressRecipient,
				Amount:               "wtf",
				NetworkID:            1,
				Nonce:                1,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  1,
			},
			error: wallet.ErrInvalidAmount,
		},
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Coin,
				Recipient:            addressRecipient,
				Amount:               "123",
				NetworkID:            0,
				Nonce:                1,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  1,
			},
			error: wallet.ErrInvalidNetwork,
		},
		// Success token tx
		{
			req: wallet.EthTransactionParams{
				Type:                 wallet.Token,
				Recipient:            addressRecipient,
				ContractAddress:      usdtContract,
				Amount:               "100000",
				NetworkID:            1,
				Nonce:                0,
				MaxPriorityFeePerGas: "1000000000",
				MaxFeePerGas:         "24000000000",
				Gas:                  2,
			},
			assert: func(t *testing.T, req wallet.EthTransactionParams, encoded string) {
				encodedHex, err := hexutil.Decode(encoded)
				assert.NoError(t, err)

				tx := &types.Transaction{}
				assert.NoError(t, tx.UnmarshalBinary(encodedHex))

				assert.Equal(t, uint64(req.Gas), tx.Gas())
				assert.Equal(t, big.NewInt(0), tx.Value())
				assert.Equal(t, strings.ToLower(req.ContractAddress), strings.ToLower(tx.To().String()))
			},
		},
	} {
		t.Run(strconv.Itoa(testCaseIndex), func(t *testing.T) {
			// ACT
			encoded, err := p.NewTransaction(w, testCase.req)

			// ASSERT
			if testCase.error != nil {
				assert.ErrorIs(t, err, testCase.error)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, encoded)

			if testCase.assert != nil {
				testCase.assert(t, testCase.req, encoded)
			}
		})
	}
}

type fakeReader struct{}

func (r *fakeReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 1
	}

	return len(p), nil
}
