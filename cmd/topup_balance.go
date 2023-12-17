package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/oxygenpay/oxygen/internal/app"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var topupBalanceCommand = &cobra.Command{
	Use:   "topup-balance",
	Short: "Topup Merchant's balance using 'system' funds",
	Run:   topupBalance,
}

var topupBalanceArgs = struct {
	MerchantID *int64
	Ticker     *string
	Amount     *string
	Comment    *string
	IsTest     *bool
}{
	MerchantID: util.Ptr(int64(0)),
	Ticker:     util.Ptr(""),
	Amount:     util.Ptr(""),
	Comment:    util.Ptr(""),
	IsTest:     util.Ptr(false),
}

func topupBalance(_ *cobra.Command, _ []string) {
	var (
		ctx               = context.Background()
		cfg               = resolveConfig()
		service           = app.New(ctx, cfg)
		blockchainService = service.Locator().BlockchainService()
		merchantService   = service.Locator().MerchantService()
		walletService     = service.Locator().WalletService()
		processingService = service.Locator().ProcessingService()
		logger            = service.Logger()
		exit              = func(err error, message string) { logger.Fatal().Err(err).Msg(message) }
	)

	// 1. Get input
	currency, err := blockchainService.GetCurrencyByTicker(*topupBalanceArgs.Ticker)
	if err != nil {
		exit(err, "invalid ticker")
	}

	amount, err := money.CryptoFromStringFloat(currency.Ticker, *topupBalanceArgs.Amount, currency.Decimals)
	if err != nil {
		exit(err, "invalid amount")
	}

	merchant, err := merchantService.GetByID(ctx, *topupBalanceArgs.MerchantID, false)
	if err != nil {
		exit(err, "invalid merchant id")
	}

	if *topupBalanceArgs.Comment == "" {
		exit(nil, "comment should not be empty")
	}

	isTest := *topupBalanceArgs.IsTest
	comment := *topupBalanceArgs.Comment

	// 2. Locate system balance
	balances, err := walletService.ListAllBalances(ctx, wallet.ListAllBalancesOpts{WithSystemBalances: true})
	if err != nil {
		exit(err, "unable to list balances")
	}

	systemBalance, found := lo.Find(balances[wallet.EntityTypeSystem], func(b *wallet.Balance) bool {
		tickerMatches := b.Currency == currency.Ticker
		networkMatches := b.NetworkID == currency.ChooseNetwork(isTest)

		return tickerMatches && networkMatches
	})

	if !found {
		exit(err, "unable to locate system balance")
	}

	logger.Info().
		Str("amount", amount.String()).
		Str("currency", currency.Ticker).
		Str("merchant.name", merchant.Name).
		Int64("merchant.id", merchant.ID).
		Str("merchant.uuid", merchant.UUID.String()).
		Str("system_balance", systemBalance.Amount.String()).
		Msg("Performing internal topup from the system balance")

	// 3. Confirm
	if !confirm("Are you sure you want to continue?") {
		logger.Info().Msg("Aborting.")
		return
	}

	// 4. Perform topup
	logger.Info().Msg("Sending...")

	input := processing.TopupInput{
		Currency: currency,
		Amount:   amount,
		Comment:  comment,
		IsTest:   isTest,
	}

	out, err := processingService.TopupMerchantFromSystem(ctx, merchant.ID, input)
	if err != nil {
		exit(err, "unable to topup the balance")
	}

	logger.
		Info().
		Int64("payment.id", out.Payment.ID).
		Int64("tx.id", out.Transaction.ID).
		Str("tx.usd_amount", out.Transaction.USDAmount.String()).
		Str("merchant.balance", out.MerchantBalance.Amount.String()).
		Msg("Done")
}

func topupBalanceSetup(cmd *cobra.Command) {
	f := cmd.Flags()

	f.Int64Var(topupBalanceArgs.MerchantID, "merchant-id", 0, "Merchant ID")
	f.StringVar(topupBalanceArgs.Ticker, "ticker", "", "Ticker")
	f.StringVar(topupBalanceArgs.Amount, "amount", "0", "Amount")
	f.StringVar(topupBalanceArgs.Comment, "comment", "", "Comment")
	f.BoolVar(topupBalanceArgs.IsTest, "is-test", false, "Test balance")

	for _, name := range []string{"merchant-id", "ticker", "amount", "comment"} {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(name + ": " + err.Error())
		}
	}
}

func confirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/n): ", message)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes"
}
