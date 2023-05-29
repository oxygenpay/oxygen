package merchantapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-dashboard/v1/model"
)

func (h *Handler) ListBalances(c echo.Context) error {
	ctx := c.Request().Context()
	mt := middleware.ResolveMerchant(c)

	balances, err := h.wallets.ListBalances(ctx, wallet.EntityTypeMerchant, mt.ID, true)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, &model.MerchantBalanceList{
		Results: util.MapSlice(balances, h.balanceToResponse),
	})
}

func (h *Handler) balanceToResponse(b *wallet.Balance) *model.MerchantBalance {
	currency, _ := h.blockchain.GetCurrencyByTicker(b.Currency)
	minWithdrawal, _ := h.blockchain.GetMinimalWithdrawalByTicker(currency.Ticker)

	isTest := b.NetworkID != currency.NetworkID

	usdAmount := "0"
	if !isTest && b.UsdAmount != nil {
		usdAmount = b.UsdAmount.String()
	}

	return &model.MerchantBalance{
		ID:                         b.UUID.String(),
		Blockchain:                 currency.Blockchain.String(),
		BlockchainName:             currency.BlockchainName,
		IsTest:                     isTest,
		Name:                       currency.DisplayName(),
		Currency:                   currency.Name,
		Ticker:                     currency.Ticker,
		Amount:                     b.Amount.String(),
		UsdAmount:                  usdAmount,
		MinimalWithdrawalAmountUSD: minWithdrawal.String(),
	}
}
