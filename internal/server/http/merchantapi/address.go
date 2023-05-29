package merchantapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
	"github.com/pkg/errors"
)

const (
	paramAddressID = "addressId"
)

func (h *Handler) ListMerchantAddresses(c echo.Context) error {
	ctx := c.Request().Context()

	mt := middleware.ResolveMerchant(c)

	addresses, err := h.merchants.ListMerchantAddresses(ctx, mt.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &model.MerchantAddressList{
		Results: util.MapSlice(addresses, addressToResponse),
	})
}

func (h *Handler) GetMerchantAddress(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramAddressID)
	if err != nil {
		return err
	}

	mt := middleware.ResolveMerchant(c)

	address, err := h.merchants.GetMerchantAddressByUUID(ctx, mt.ID, id)

	switch {
	case errors.Is(err, merchant.ErrAddressNotFound):
		return common.NotFoundResponse(c, "address not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, addressToResponse(address))
}

func (h *Handler) CreateMerchantAddress(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateMerchantAddressRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	mt := middleware.ResolveMerchant(c)

	address, err := h.merchants.CreateMerchantAddress(ctx, mt.ID, merchant.CreateMerchantAddressParams{
		Name:       req.Name,
		Blockchain: wallet.Blockchain(req.Blockchain),
		Address:    req.Address,
	})

	switch {
	case errors.Is(err, wallet.ErrInvalidAddress):
		return common.ValidationErrorItemResponse(c, "address", "invalid address provided")
	case errors.Is(err, merchant.ErrAddressAlreadyExists):
		return common.ValidationErrorItemResponse(c, "address", "address already exists")
	case errors.Is(err, merchant.ErrAddressReserved):
		return common.ValidationErrorItemResponse(c, "address", "address is reserved")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusCreated, addressToResponse(address))
}

func (h *Handler) UpdateMerchantAddress(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramAddressID)
	if err != nil {
		return err
	}

	var req model.UpdateMerchantAddressRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	mt := middleware.ResolveMerchant(c)
	address, err := h.merchants.UpdateMerchantAddress(ctx, mt.ID, id, req.Name)

	switch {
	case errors.Is(err, merchant.ErrAddressNotFound):
		return common.NotFoundResponse(c, "address not found")
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, addressToResponse(address))
}

func (h *Handler) DeleteMerchantAddress(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramAddressID)
	if err != nil {
		return err
	}

	mt := middleware.ResolveMerchant(c)

	err = h.merchants.DeleteMerchantAddress(ctx, mt.ID, id)

	switch {
	case errors.Is(err, merchant.ErrAddressNotFound):
		return common.NotFoundResponse(c, "address not found")
	case err != nil:
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func addressToResponse(a *merchant.Address) *model.MerchantAddress {
	return &model.MerchantAddress{
		ID:             a.UUID.String(),
		Name:           a.Name,
		Blockchain:     string(a.Blockchain),
		BlockchainName: a.BlockchainName,
		Address:        a.Address,
	}
}
