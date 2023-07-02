package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/spf13/cobra"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

var listWalletsCommand = &cobra.Command{
	Use:   "list-wallets",
	Short: "List wallets in the database",
	Run:   listWallets,
}

func listWallets(_ *cobra.Command, _ []string) {
	var (
		ctx            = context.Background()
		cfg            = resolveConfig()
		service        = app.New(ctx, cfg)
		walletsService = service.Locator().WalletService()
		logger         = service.Logger()
	)

	wallets, _, err := walletsService.List(ctx, wallet.Pagination{Start: 0, Limit: 10000})
	if err != nil {
		logger.Error().Err(err).Msg("Unable to list wallets")
	}

	slices.SortFunc(wallets, func(a, b *wallet.Wallet) bool {
		asserts := []int{
			compare(a.Type, b.Type),
			compare(a.Blockchain, b.Blockchain),
			compare(a.ID, b.ID),
		}

		for _, sign := range asserts {
			if sign == 0 {
				continue
			}
			return sign > 0
		}

		return true
	})

	t := tablewriter.NewWriter(os.Stdout)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeader([]string{"ID", "Type", "Blockchain", "Address"})
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for _, w := range wallets {
		line := fmt.Sprintf("%d,%s,%s,%s", w.ID, w.Type, w.Blockchain, w.Address)
		t.Append(strings.Split(line, ","))
	}

	t.Render()
}

func compare[T constraints.Ordered](a, b T) int {
	switch {
	case a > b:
		return 1
	case a < b:
		return -1
	default:
		return 0
	}
}
