package merchantapi_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:funlen
func TestAddressRoutes(t *testing.T) {
	const (
		addressesRoute = "/api/dashboard/v1/merchant/:merchantId/address"
		addressRoute   = "/api/dashboard/v1/merchant/:merchantId/address/:addressId"
	)

	tc := test.NewIntegrationTest(t)

	user, token := tc.Must.CreateSampleUser(t)

	t.Run("ListMerchantAddresses", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, user.ID)

		// And several addresses
		a1, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A1",
			Blockchain: kmswallet.ETH,
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})

		require.NoError(t, err)

		a2, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A2",
			Blockchain: kmswallet.MATIC,
			Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
		})
		require.NoError(t, err)

		a3, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A3",
			Blockchain: kmswallet.BTC,
			Address:    "bc1q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
		})
		require.NoError(t, err)

		a4, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A4",
			Blockchain: kmswallet.TRON,
			Address:    "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE",
		})
		require.NoError(t, err)

		// ACT
		// List addresses
		res := tc.Client.
			GET().
			Path(addressesRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusOK, res.StatusCode(), res.String())

		var body model.MerchantAddressList
		assert.NoError(t, res.JSON(&body))
		assert.Equal(t, a4.UUID.String(), body.Results[0].ID)
		assert.Equal(t, "Tron", body.Results[0].BlockchainName)
		assert.Equal(t, a3.UUID.String(), body.Results[1].ID)
		assert.Equal(t, a2.UUID.String(), body.Results[2].ID)
		assert.Equal(t, a1.UUID.String(), body.Results[3].ID)
	})

	t.Run("CreateMerchantAddress", func(t *testing.T) {
		for testCaseIndex, testCase := range []struct {
			req         model.CreateMerchantAddressRequest
			setup       func(t *testing.T, mtID int64)
			expectError bool
		}{
			{
				req: model.CreateMerchantAddressRequest{
					Address:    "bc1q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
					Blockchain: string(kmswallet.BTC),
					Name:       "A1",
				},
			},
			{
				req: model.CreateMerchantAddressRequest{
					Address:    "0x690b9a9e9aa1c9db991c7721a92d351db4fac990",
					Blockchain: string(kmswallet.ETH),
					Name:       "A2",
				},
			},
			{
				req: model.CreateMerchantAddressRequest{
					Name:       "A3",
					Blockchain: string(kmswallet.MATIC),
					Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
				},
			},
			{
				req: model.CreateMerchantAddressRequest{
					Name:       "A3",
					Blockchain: string(kmswallet.BSC),
					Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
				},
			},
			{
				req: model.CreateMerchantAddressRequest{
					Name:       "A4",
					Blockchain: string(kmswallet.TRON),
					Address:    "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE",
				},
			},
			{
				req: model.CreateMerchantAddressRequest{
					Name:       "A5",
					Blockchain: string(kmswallet.TRON),
					Address:    "0x95222290dd7278aa3ddd389cc1e1d165cc4bafe5",
				},
				expectError: true,
			},
			{
				req: model.CreateMerchantAddressRequest{
					Address:    "91q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
					Blockchain: string(kmswallet.BTC),
					Name:       "A6",
				},
				expectError: true,
			},
			{
				// prevents creation of existing addresses
				setup: func(t *testing.T, mtID int64) {
					_, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mtID, merchant.CreateMerchantAddressParams{
						Name:       "A7",
						Blockchain: kmswallet.TRON,
						Address:    "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE",
					})
					require.NoError(t, err)
				},
				req: model.CreateMerchantAddressRequest{
					Name:       "A7 one more time",
					Blockchain: string(kmswallet.TRON),
					Address:    "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE",
				},
				expectError: true,
			},
			{
				// prevents creation of addresses that match system wallets
				setup: func(t *testing.T, mtID int64) {
					tc.Must.CreateWallet(t, "TRON", "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE", "0x123456", wallet.TypeInbound)
				},
				req: model.CreateMerchantAddressRequest{
					Name:       "A8",
					Blockchain: string(kmswallet.TRON),
					Address:    "THdMRUhrqfnbH4juqVgHivhM6gYZEiXEAE",
				},
				expectError: true,
			},
		} {
			t.Run(strconv.Itoa(testCaseIndex+1), func(t *testing.T) {
				// ARRANGE
				// Given a merchant
				mt, _ := tc.Must.CreateMerchant(t, user.ID)

				// And setup logic
				if testCase.setup != nil {
					testCase.setup(t, mt.ID)
				}

				// ACT
				// Create address
				res := tc.Client.
					POST().
					Path(addressesRoute).
					WithToken(token).
					Param(paramMerchantID, mt.UUID.String()).
					JSON(&testCase.req).
					Do()

				// ASSERT
				if testCase.expectError {
					assert.Equal(t, http.StatusBadRequest, res.StatusCode())
					return
				}

				var body model.MerchantAddress

				assert.Equal(t, http.StatusCreated, res.StatusCode())
				assert.NoError(t, res.JSON(&body))

				assert.Equal(t, testCase.req.Blockchain, body.Blockchain)
				assert.Equal(t, testCase.req.Address, body.Address)
				assert.NotEqual(t, uuid.Nil.String(), body.ID)

				_, err := tc.Services.Merchants.GetMerchantAddressByUUID(tc.Context, mt.ID, uuid.MustParse(body.ID))
				assert.NoError(t, err)
			})
		}
	})

	t.Run("GetMerchantAddress", func(t *testing.T) {
		t.Run("Returns address", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			// And an address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "A1",
				Blockchain: kmswallet.BTC,
				Address:    "bc1q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
			})
			require.NoError(t, err)

			// ACT
			// Get address
			res := tc.Client.
				GET().
				Path(addressRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramAddressID, addr.UUID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode())

			var body model.MerchantAddress
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, addr.UUID.String(), body.ID)
			assert.Equal(t, addr.Name, body.Name)
			assert.Equal(t, string(addr.Blockchain), body.Blockchain)
			assert.Equal(t, addr.BlockchainName, body.BlockchainName)
			assert.Equal(t, addr.Address, body.Address)
		})

		t.Run("Not found", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			// Get address
			res := tc.Client.
				GET().
				Path(addressRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramAddressID, uuid.New().String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		})
	})

	t.Run("UpdateMerchantAddress", func(t *testing.T) {
		t.Run("Updates address", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			// And an address
			addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
				Name:       "A1",
				Blockchain: kmswallet.BTC,
				Address:    "bc1q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
			})
			require.NoError(t, err)

			req := model.UpdateMerchantAddressRequest{Name: "A2"}

			// ACT
			// Update address
			res := tc.Client.
				PUT().
				Path(addressRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramAddressID, addr.UUID.String()).
				JSON(&req).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusOK, res.StatusCode())

			var body model.MerchantAddress
			assert.NoError(t, res.JSON(&body))
			assert.Equal(t, addr.UUID.String(), body.ID)
			assert.Equal(t, req.Name, body.Name)
			assert.Equal(t, string(addr.Blockchain), body.Blockchain)
			assert.Equal(t, addr.Address, body.Address)
		})

		t.Run("Not found", func(t *testing.T) {
			// ARRANGE
			// Given a merchant
			mt, _ := tc.Must.CreateMerchant(t, user.ID)

			req := model.UpdateMerchantAddressRequest{Name: "A1"}

			// ACT
			// Update the address
			res := tc.Client.
				PUT().
				Path(addressRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramAddressID, uuid.New().String()).
				JSON(&req).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
		})
	})

	t.Run("DeleteMerchantAddress", func(t *testing.T) {
		// ARRANGE
		// Given a merchant
		mt, _ := tc.Must.CreateMerchant(t, user.ID)

		// And an address
		addr, err := tc.Services.Merchants.CreateMerchantAddress(tc.Context, mt.ID, merchant.CreateMerchantAddressParams{
			Name:       "A1",
			Blockchain: kmswallet.BTC,
			Address:    "bc1q43ugfc3tawhzvxgjrycq0papwmndlc7quzy96j",
		})
		require.NoError(t, err)

		// ACT
		// Delete the address
		res := tc.Client.
			DELETE().
			Path(addressRoute).
			WithToken(token).
			Param(paramMerchantID, mt.UUID.String()).
			Param(paramAddressID, addr.UUID.String()).
			Do()

		// ASSERT
		assert.Equal(t, http.StatusNoContent, res.StatusCode())

		_, err = tc.Services.Merchants.GetMerchantAddressByUUID(tc.Context, mt.ID, addr.UUID)
		assert.ErrorIs(t, err, merchant.ErrAddressNotFound)

		t.Run("Not found", func(t *testing.T) {
			// ACT
			// Try to delete one more time
			res := tc.Client.
				DELETE().
				Path(addressRoute).
				WithToken(token).
				Param(paramMerchantID, mt.UUID.String()).
				Param(paramAddressID, addr.UUID.String()).
				Do()

			// ASSERT
			assert.Equal(t, http.StatusBadRequest, res.StatusCode())
			assert.Contains(t, res.String(), "not found")
		})
	})
}
