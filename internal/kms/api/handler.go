package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
	httpServer "github.com/oxygenpay/oxygen/internal/server/http"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Handler struct {
	wallets *wallet.Service
	logger  *zerolog.Logger
}

const paramWalletID = "walletId"

func SetupRoutes(handler *Handler) httpServer.Opt {
	return func(s *httpServer.Server) {
		kmsAPI := s.Echo().Group("/api/kms/v1")

		kmsAPI.POST("/wallet", handler.Create)
		kmsAPI.GET("/wallet/:walletId", handler.Get)
		kmsAPI.DELETE("/wallet/:walletId", handler.Delete)

		kmsAPI.POST("/wallet/:walletId/transaction/eth", handler.CreateEthereumTransaction)
		kmsAPI.POST("/wallet/:walletId/transaction/matic", handler.CreateMaticTransaction)
		kmsAPI.POST("/wallet/:walletId/transaction/bsc", handler.CreateBSCTransaction)
		kmsAPI.POST("/wallet/:walletId/transaction/tron", handler.CreateTronTransaction)
	}
}

func New(wallets *wallet.Service, logger *zerolog.Logger) *Handler {
	log := logger.With().Str("channel", "kms_handler").Logger()

	return &Handler{
		wallets: wallets,
		logger:  &log,
	}
}

func (h *Handler) Create(c echo.Context) error {
	ctx := c.Request().Context()

	var req model.CreateWalletRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	w, err := h.wallets.CreateWallet(ctx, wallet.Blockchain(req.Blockchain))
	if err != nil {
		return common.ErrorResponse(c, "unable to create wallet")
	}

	return c.JSON(http.StatusCreated, walletToResponse(w))
}

func (h *Handler) Get(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	w, err := h.wallets.GetWallet(ctx, id, false)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	return c.JSON(http.StatusOK, walletToResponse(w))
}

func (h *Handler) Delete(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	err = h.wallets.DeleteWallet(ctx, id)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) CreateEthereumTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	w, err := h.wallets.GetWallet(ctx, id, false)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	var req model.CreateEthereumTransactionRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	raw, err := h.wallets.CreateEthereumTransaction(ctx, w, wallet.EthTransactionParams{
		Type:                 wallet.AssetType(req.AssetType),
		Recipient:            req.Recipient,
		ContractAddress:      req.ContractAddress,
		Amount:               req.Amount,
		NetworkID:            req.NetworkID,
		Nonce:                *req.Nonce,
		MaxPriorityFeePerGas: req.MaxPriorityPerGas,
		MaxFeePerGas:         req.MaxFeePerGas,
		Gas:                  req.Gas,
	})

	if err != nil {
		return transactionCreationFailed(c, err)
	}

	return c.JSON(http.StatusCreated, &model.EthereumTransaction{RawTransaction: raw})
}

func (h *Handler) CreateMaticTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	w, err := h.wallets.GetWallet(ctx, id, false)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	var req model.CreateMaticTransactionRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	raw, err := h.wallets.CreateMaticTransaction(ctx, w, wallet.EthTransactionParams{
		Type:                 wallet.AssetType(req.AssetType),
		Recipient:            req.Recipient,
		ContractAddress:      req.ContractAddress,
		Amount:               req.Amount,
		NetworkID:            req.NetworkID,
		Nonce:                *req.Nonce,
		MaxPriorityFeePerGas: req.MaxPriorityPerGas,
		MaxFeePerGas:         req.MaxFeePerGas,
		Gas:                  req.Gas,
	})

	if err != nil {
		return transactionCreationFailed(c, err)
	}

	return c.JSON(http.StatusCreated, &model.EthereumTransaction{RawTransaction: raw})
}

func (h *Handler) CreateBSCTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	w, err := h.wallets.GetWallet(ctx, id, false)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	var req model.CreateBSCTransactionRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	raw, err := h.wallets.CreateBSCTransaction(ctx, w, wallet.EthTransactionParams{
		Type:                 wallet.AssetType(req.AssetType),
		Recipient:            req.Recipient,
		ContractAddress:      req.ContractAddress,
		Amount:               req.Amount,
		NetworkID:            req.NetworkID,
		Nonce:                *req.Nonce,
		MaxPriorityFeePerGas: req.MaxPriorityPerGas,
		MaxFeePerGas:         req.MaxFeePerGas,
		Gas:                  req.Gas,
	})

	if err != nil {
		return transactionCreationFailed(c, err)
	}

	return c.JSON(http.StatusCreated, &model.EthereumTransaction{RawTransaction: raw})
}

func (h *Handler) CreateTronTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := common.UUID(c, paramWalletID)
	if err != nil {
		return err
	}

	w, err := h.wallets.GetWallet(ctx, id, false)

	switch {
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, wallet.ErrNotFound.Error())
	case err != nil:
		return err
	}

	var req model.CreateTronTransactionRequest
	if valid := common.BindAndValidateRequest(c, &req); !valid {
		return nil
	}

	tx, err := h.wallets.CreateTronTransaction(ctx, w, wallet.TronTransactionParams{
		Type:      wallet.AssetType(req.AssetType),
		Recipient: req.Recipient,
		Amount:    req.Amount,

		ContractAddress: req.ContractAddress,
		FeeLimit:        uint64(req.FeeLimit),

		IsTest: req.IsTest,
	})

	if err != nil {
		return transactionCreationFailed(c, err)
	}

	return c.JSON(http.StatusCreated, &model.TronTransaction{
		TxID:       tx.TxID,
		Visible:    tx.Visible,
		RawData:    tx.RawData,
		RawDataHex: tx.RawDataHex,
		Signature:  tx.Signature,
	})
}

func transactionCreationFailed(c echo.Context, err error) error {
	switch {
	case errors.Is(err, wallet.ErrUnknownBlockchain):
		return common.ValidationErrorResponse(c, err.Error())
	case errors.Is(err, wallet.ErrInvalidAddress):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidAddress)
	case errors.Is(err, wallet.ErrInvalidContractAddress):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidContractAddress)
	case errors.Is(err, wallet.ErrInvalidAmount):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidAmount)
	case errors.Is(err, wallet.ErrInvalidNetwork):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidNetwork)
	case errors.Is(err, wallet.ErrInvalidGasSettings):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidGasSettings)
	case errors.Is(err, wallet.ErrInvalidNonce):
		return common.ValidationErrorResponse(c, wallet.ErrInvalidNonce)
	case errors.Is(err, wallet.ErrTronResponse):
		return common.ValidationErrorResponse(c, wallet.ErrTronResponse)
	case errors.Is(err, wallet.ErrInsufficientBalance):
		return common.ValidationErrorResponse(c, wallet.ErrInsufficientBalance)
	default:
		return err
	}
}

func walletToResponse(w *wallet.Wallet) *model.Wallet {
	return &model.Wallet{
		ID:            w.UUID.String(),
		Blockchain:    model.Blockchain(w.Blockchain),
		CreatedAtUnix: w.CreatedAt.Unix(),
		Address:       w.Address,
		PublicKey:     w.PublicKey,
	}
}
