package api_test

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const paramWalletID = "walletId"

//nolint:funlen
func TestHandlerRoutes(t *testing.T) {
	const (
		walletRoute              = "/api/kms/v1/wallet/:walletId"
		ethereumTransactionRoute = "/api/kms/v1/wallet/:walletId/transaction/eth"
		polygonTransactionRoute  = "/api/kms/v1/wallet/:walletId/transaction/matic"
		bscTransactionRoute      = "/api/kms/v1/wallet/:walletId/transaction/bsc"
		tronTransactionRoute     = "/api/kms/v1/wallet/:walletId/transaction/tron"
	)

	tc := test.NewIntegrationTest(t)

	createWallet := func(bc wallet.Blockchain) *wallet.Wallet {
		w, err := tc.KMS.Service.CreateWallet(tc.Context, bc)
		require.NoError(t, err)

		return w
	}

	t.Run("GetWallet", func(t *testing.T) {
		for _, bc := range wallet.ListBlockchains() {
			t.Run(fmt.Sprintf("Returns wallet for %s", bc), func(t *testing.T) {
				// ARRANGE
				// Given a wallet
				w, err := tc.KMS.Service.CreateWallet(tc.Context, bc)
				require.NoError(t, err)

				// ACT
				res := tc.Client.
					GET().
					Path(walletRoute).
					Param(paramWalletID, w.UUID.String()).
					Do()

				// ASSERT
				var body model.Wallet

				assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())
				assert.NoError(t, res.JSON(&body))
				assert.Equal(t, w.UUID.String(), body.ID)
				assert.Equal(t, w.Blockchain.String(), string(body.Blockchain))
				assert.Equal(t, w.Address, body.Address)
			})
		}

		t.Run("Not found", func(t *testing.T) {
			// ACT
			res := tc.Client.
				GET().
				Path(walletRoute).
				Param(paramWalletID, uuid.New().String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "not found")
		})
	})

	t.Run("CreateEthereumTransaction", func(t *testing.T) {
		const usdtContract = "0xdac17f958d2ee523a2206206994597c13d831ec7"

		for testCaseIndex, testCase := range []struct {
			wallet *wallet.Wallet
			req    model.CreateEthereumTransactionRequest
			assert func(t *testing.T, res *test.Response)
		}{
			{
				wallet: createWallet(wallet.ETH),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.EthereumTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				wallet: createWallet(wallet.ETH),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "token",
					Amount:            "123",
					ContractAddress:   usdtContract,
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         5,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.EthereumTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				// blockchain mismatch
				wallet: createWallet(wallet.MATIC),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				},
			},
		} {
			t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
				// ACT
				res := tc.Client.
					POST().
					Path(ethereumTransactionRoute).
					Param(paramWalletID, testCase.wallet.UUID.String()).
					JSON(&testCase.req).
					Do()

				// ASSERT
				testCase.assert(t, res)
			})
		}
	})

	t.Run("CreateMaticTransaction", func(t *testing.T) {
		const usdtContract = "0xdac17f958d2ee523a2206206994597c13d831ec7"

		for testCaseIndex, testCase := range []struct {
			wallet *wallet.Wallet
			req    model.CreateEthereumTransactionRequest
			assert func(t *testing.T, res *test.Response)
		}{
			{
				wallet: createWallet(wallet.MATIC),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.EthereumTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				wallet: createWallet(wallet.MATIC),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "token",
					Amount:            "123",
					ContractAddress:   usdtContract,
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         5,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.EthereumTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				// blockchain mismatch
				wallet: createWallet(wallet.ETH),
				req: model.CreateEthereumTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				},
			},
		} {
			t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
				// ACT
				res := tc.Client.
					POST().
					Path(polygonTransactionRoute).
					Param(paramWalletID, testCase.wallet.UUID.String()).
					JSON(&testCase.req).
					Do()

				// ASSERT
				testCase.assert(t, res)
			})
		}
	})

	t.Run("CreateBSCTransaction", func(t *testing.T) {
		const usdtContract = "0xdac17f958d2ee523a2206206994597c13d831ec7"

		for testCaseIndex, testCase := range []struct {
			wallet *wallet.Wallet
			req    model.CreateBSCTransactionRequest
			assert func(t *testing.T, res *test.Response)
		}{
			{
				wallet: createWallet(wallet.BSC),
				req: model.CreateBSCTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.BSCTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				wallet: createWallet(wallet.BSC),
				req: model.CreateBSCTransactionRequest{
					AssetType:         "token",
					Amount:            "123",
					ContractAddress:   usdtContract,
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         5,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.BSCTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.RawTransaction)
				},
			},
			{
				// blockchain mismatch
				wallet: createWallet(wallet.ETH),
				req: model.CreateBSCTransactionRequest{
					AssetType:         "coin",
					Amount:            "123",
					Gas:               1,
					MaxFeePerGas:      "123",
					MaxPriorityPerGas: "456",
					NetworkID:         1,
					Nonce:             util.Ptr(int64(0)),
					Recipient:         "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
				},
				assert: func(t *testing.T, res *test.Response) {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				},
			},
		} {
			t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
				// ACT
				res := tc.Client.
					POST().
					Path(bscTransactionRoute).
					Param(paramWalletID, testCase.wallet.UUID.String()).
					JSON(&testCase.req).
					Do()

				// ASSERT
				testCase.assert(t, res)
			})
		}
	})

	t.Run("CreateTronTransaction", func(t *testing.T) {
		const usdtContract = "TBnt7Wzvd226i24r95pE82MZpHba63ehQY"

		for testCaseIndex, testCase := range []struct {
			wallet *wallet.Wallet
			req    model.CreateTronTransactionRequest
			setup  func(req model.CreateTronTransactionRequest, w *wallet.Wallet)
			assert func(t *testing.T, res *test.Response)
		}{
			{
				wallet: createWallet(wallet.TRON),
				req: model.CreateTronTransactionRequest{
					AssetType: "coin",
					Amount:    "123",
					Recipient: "TTYxentT3sf8XHbtHGyWX2uDgdadE9uYSL",
				},
				setup: func(req model.CreateTronTransactionRequest, w *wallet.Wallet) {
					tc.Providers.TrongridMock.SetupCreateTransaction(
						trongrid.TransactionRequest{
							OwnerAddress: w.Address,
							ToAddress:    req.Recipient,
							Amount:       123,
							Visible:      true,
						},
						trongrid.Transaction{
							TxID:       "abc123",
							RawDataHex: "a9059cbb",
							Visible:    true,
						},
					)
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.TronTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.Signature[0])
				},
			},
			{
				wallet: createWallet(wallet.TRON),
				req: model.CreateTronTransactionRequest{
					Amount:          "123",
					AssetType:       "token",
					ContractAddress: usdtContract,
					FeeLimit:        123,
					Recipient:       "TTYxentT3sf8XHbtHGyWX2uDgdadE9uYSL",
				},
				setup: func(req model.CreateTronTransactionRequest, w *wallet.Wallet) {
					tc.Providers.TrongridMock.SetupTriggerSmartContract(
						trongrid.ContractCallRequest{
							OwnerAddress:     w.Address,
							ContractAddress:  req.ContractAddress,
							FunctionSelector: "transfer(address,uint256)",
							Parameter:        "static-mock",
							FeeLimit:         123,
							Visible:          true,
						},
						trongrid.Transaction{
							TxID:       "abc123",
							RawDataHex: "a9059cbb",
							Visible:    true,
						},
					)
				},
				assert: func(t *testing.T, res *test.Response) {
					var body model.TronTransaction

					assert.Equal(t, http.StatusCreated, res.StatusCode(), res.String())
					assert.NoError(t, res.JSON(&body))
					assert.NotEmpty(t, body.Signature[0])
				},
			},
			{
				// blockchain mismatch
				wallet: createWallet(wallet.ETH),
				req: model.CreateTronTransactionRequest{
					Amount:          "123",
					AssetType:       "token",
					ContractAddress: usdtContract,
					FeeLimit:        123,
					Recipient:       "TTYxentT3sf8XHbtHGyWX2uDgdadE9uYSL",
				},
				assert: func(t *testing.T, res *test.Response) {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode(), res.String())
				},
			},
		} {
			t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
				// ARRANGE
				if testCase.setup != nil {
					testCase.setup(testCase.req, testCase.wallet)
				}

				// ACT
				res := tc.Client.
					POST().
					Path(tronTransactionRoute).
					Param(paramWalletID, testCase.wallet.UUID.String()).
					JSON(&testCase.req).
					Do()

				// ASSERT
				testCase.assert(t, res)
			})
		}
	})
}
