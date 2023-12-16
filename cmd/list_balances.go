package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/spf13/cobra"
)

var listBalancesCommand = &cobra.Command{
	Use:   "list-balances",
	Short: "List all balances including system balances",
	Run:   listBalances,
}

func listBalances(_ *cobra.Command, _ []string) {
	var (
		ctx               = context.Background()
		cfg               = resolveConfig()
		service           = app.New(ctx, cfg)
		walletsService    = service.Locator().WalletService()
		blockchainService = service.Locator().BlockchainService()
		logger            = service.Logger()
	)

	opts := wallet.ListAllBalancesOpts{
		WithUSD:            true,
		WithSystemBalances: true,
		HideEmpty:          true,
	}

	balances, err := walletsService.ListAllBalances(ctx, opts)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to list wallets")
	}

	t := tablewriter.NewWriter(os.Stdout)
	defer t.Render()

	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)

	t.SetHeader([]string{"id", "type", "entity Id", "currency", "test", "amount", "usd"})

	add := func(b *wallet.Balance) {
		currency, err := blockchainService.GetCurrencyByTicker(b.Currency)
		if err != nil {
			logger.Error().Err(err)
			return
		}

		t.Append(balanceAsRow(currency, b))
	}

	for _, b := range balances[wallet.EntityTypeMerchant] {
		add(b)
	}
	for _, b := range balances[wallet.EntityTypeWallet] {
		add(b)
	}
	for _, b := range balances[wallet.EntityTypeSystem] {
		add(b)
	}
}

func balanceAsRow(currency money.CryptoCurrency, b *wallet.Balance) []string {
	isTest := b.NetworkID != currency.NetworkID

	line := fmt.Sprintf(
		"%d,%s,%d,%s,%t,%s,%s",
		b.ID,
		b.EntityType,
		b.EntityID,
		b.Currency,
		isTest,
		b.Amount.String(),
		b.UsdAmount.String(),
	)

	return strings.Split(line, ",")
}
