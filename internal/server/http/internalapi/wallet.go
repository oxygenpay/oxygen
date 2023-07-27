package internalapi

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	admin "github.com/oxygenpay/oxygen/pkg/api-admin/v1/model"
	"github.com/pkg/errors"
)

const defaultPaginationLimit = int32(30)
const maxPaginationLimit = 100

const paramWalletID = "walletID"

var errInvalidID = errors.New("id should be INT or UUID")

func (h *Handler) CreateWallet(c echo.Context) error {
	ctx := c.Request().Context()

	req := &admin.CreateWalletRequest{}
	if isValid := common.BindAndValidateRequest(c, req); !isValid {
		return nil
	}

	wt, err := h.wallet.Create(ctx, kms.Blockchain(*req.Blockchain), wallet.TypeInbound)
	if err != nil {
		return errors.Wrap(err, "wallet.Create error")
	}

	return c.JSON(http.StatusOK, &admin.Wallet{
		Address:       wt.Address,
		Blockchain:    admin.Blockchain(wt.Blockchain),
		CreatedAtUnix: wt.CreatedAt.Unix(),
		ID:            wt.ID,
		UUID:          wt.UUID.String(),
	})
}

func (h *Handler) GetWallet(c echo.Context) error {
	ctx := c.Request().Context()

	id := c.Param(paramWalletID)
	if id == "" {
		return common.ValidationErrorResponse(c, errInvalidID)
	}

	w, err := h.getWallet(ctx, id)

	switch {
	case errors.Is(err, errInvalidID):
		return common.ValidationErrorResponse(c, errInvalidID)
	case errors.Is(err, wallet.ErrNotFound):
		return common.NotFoundResponse(c, "wallet not found")
	case err != nil:
		return errors.Wrap(err, "unable to get wallet")
	}

	return c.JSON(http.StatusOK, walletToResponse(w))
}

func (h *Handler) CalculateTransactionFee(c echo.Context) error {
	ctx := c.Request().Context()

	req := &admin.EstimateFeesRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	currency, err := h.blockchain.GetCurrencyByTicker(req.Currency)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	baseCurrency, err := h.blockchain.GetNativeCoin(currency.Blockchain)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	fee, err := h.blockchain.CalculateFee(ctx, baseCurrency, currency, req.IsTest)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	// 4. Choose format according to blockchain
	response := func(v any, err error) error {
		if err != nil {
			return common.ErrorResponse(c, err.Error())
		}

		return c.JSON(http.StatusOK, v)
	}

	switch kms.Blockchain(currency.Blockchain) {
	case kms.ETH:
		return response(fee.ToEthFee())
	case kms.MATIC:
		return response(fee.ToMaticFee())
	case kms.BSC:
		return response(fee.ToBSCFee())
	case kms.TRON:
		return response(fee.ToTronFee())
	}

	return common.ErrorResponse(c, "unknown error")
}

func (h *Handler) BroadcastTransaction(c echo.Context) error {
	ctx := c.Request().Context()

	req := &admin.BroadcastTransactionRequest{}
	if !common.BindAndValidateRequest(c, req) {
		return nil
	}

	txHashID, err := h.blockchain.BroadcastTransaction(ctx, money.Blockchain(req.Blockchain), req.Hex, req.IsTest)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &admin.ErrorResponse{
			Message: err.Error(),
			Status:  "broadcast_error",
		})
	}

	return c.JSON(http.StatusOK, &admin.BroadcastTransactionResponse{
		TransactionHashID: txHashID,
	})
}

func (h *Handler) GetTransactionReceipt(c echo.Context) error {
	ctx := c.Request().Context()

	blockchain := money.Blockchain(c.QueryParam("blockchain"))
	if blockchain == "" {
		return common.ValidationErrorItemResponse(c, "blockchain", "required")
	}

	transactionID := c.QueryParam("txId")
	if blockchain == "" {
		return common.ValidationErrorItemResponse(c, "txId", "required")
	}

	var isTest bool
	if isTestRaw := c.QueryParam("isTest"); isTestRaw != "" {
		b, err := strconv.ParseBool(isTestRaw)
		if err != nil {
			return common.ValidationErrorItemResponse(c, "isTest", "invalid value")
		}
		isTest = b
	}

	receipt, err := h.blockchain.GetTransactionReceipt(ctx, blockchain, transactionID, isTest)
	if err != nil {
		return common.ErrorResponse(c, err.Error())
	}

	return c.JSON(http.StatusOK, &admin.TransactionReceiptResponse{
		Blockchain:      receipt.Blockchain.String(),
		TransactionHash: receipt.Hash,
		Nonce:           int64(receipt.Nonce),

		Recipient: receipt.Recipient,
		Sender:    receipt.Sender,

		NetworkFee:          receipt.NetworkFee.StringRaw(),
		NetworkFeeFormatted: receipt.NetworkFee.String(),

		Confirmations: receipt.Confirmations,
		IsConfirmed:   receipt.IsConfirmed,

		Success: receipt.Success,
		IsTest:  receipt.IsTest,
	})
}

func (h *Handler) BulkCreateWallets(c echo.Context) error {
	req := &admin.BulkCreateWalletsRequest{}
	if isValid := common.BindAndValidateRequest(c, req); !isValid {
		return nil
	}

	ctx := c.Request().Context()

	wallets, err := h.wallet.BulkCreateWallets(ctx, kms.Blockchain(*req.Blockchain), req.Amount)
	if err != nil {
		return err
	}

	results := make([]*admin.Wallet, len(wallets))
	for i, w := range wallets {
		results[i] = walletToResponse(w)
	}

	return c.JSON(http.StatusCreated, admin.WalletList{Results: results})
}

func (h *Handler) ListWallets(c echo.Context) error {
	start := c.QueryParam("start")
	limit := c.QueryParam("limit")
	blockchain := kms.Blockchain(c.QueryParam("blockchain"))

	var err error

	startID := 1
	if start != "" {
		startID, err = strconv.Atoi(start)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &admin.ErrorResponse{
				Errors:  nil,
				Message: "Invalid query param: start",
				Status:  "validation_error",
			})
		}
	}

	paginationLimit := defaultPaginationLimit
	if limit != "" {
		l, errParse := strconv.ParseInt(limit, 10, 32)
		if errParse != nil {
			return c.JSON(http.StatusBadRequest, &admin.ErrorResponse{
				Errors:  nil,
				Message: "Invalid query param: limit",
				Status:  "validation_error",
			})
		}

		paginationLimit = int32(l)
	}

	invalid := startID < 1 || paginationLimit > maxPaginationLimit ||
		(blockchain.IsSpecified() && !blockchain.IsValid())

	if invalid {
		return c.JSON(http.StatusBadRequest, &admin.ErrorResponse{
			Errors:  nil,
			Message: "Invalid query parameters",
			Status:  "validation_error",
		})
	}

	ctx := c.Request().Context()
	wallets, nextPageID, err := h.wallet.List(ctx, wallet.Pagination{
		Start:              int64(startID),
		Limit:              paginationLimit,
		FilterByBlockchain: blockchain,
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, &admin.ErrorResponse{
			Errors:  nil,
			Message: err.Error(),
			Status:  "internal_errors",
		})
	}

	results := make([]*admin.Wallet, len(wallets))
	for i, w := range wallets {
		results[i] = walletToResponse(w)
	}

	return c.JSON(http.StatusOK, admin.WalletList{
		NextPageID: nextPageID,
		Results:    results,
	})
}

func (h *Handler) getWallet(ctx context.Context, id string) (*wallet.Wallet, error) {
	walletID, err := strconv.Atoi(id)
	if err != nil {
		walletUUID, err := uuid.Parse(id)
		if err != nil {
			return nil, errInvalidID
		}

		return h.wallet.GetByUUID(ctx, walletUUID)
	}

	return h.wallet.GetByID(ctx, int64(walletID))
}

func walletToResponse(w *wallet.Wallet) *admin.Wallet {
	return &admin.Wallet{
		Address:       w.Address,
		Blockchain:    admin.Blockchain(w.Blockchain),
		CreatedAtUnix: w.CreatedAt.Unix(),
		ID:            w.ID,
		UUID:          w.UUID.String(),
	}
}
