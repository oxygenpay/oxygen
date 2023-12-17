package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/oxygenpay/oxygen/internal/config"
	"github.com/spf13/cobra"
)

var (
	Commit        = "none"
	Version       = "none"
	EmbedFrontend = false
	configPath    = "config.yml"
	skipConfig    = false
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "oxygen",
	Short: "OxygenPay",
	Long:  "OxygenPay: Accept crypto payments. Free and source-available crypto payment gateway",
}

var envHelp = &cobra.Command{
	Use:   "env",
	Short: "Outputs available ENV variables",
	Run:   envCommand,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// resolveConfig or exit with error
func resolveConfig() *config.Config {
	cfg, err := config.New(Commit, Version, configPath, skipConfig, EmbedFrontend)
	if err != nil {
		fmt.Printf("unable to initialize config: %s\n", err.Error())
		os.Exit(1)
	}

	if skipConfig {
		fmt.Println("Skipped file-based configuration, using only ENV")
	}

	return cfg
}

func envCommand(_ *cobra.Command, _ []string) {
	if err := config.PrintUsage(os.Stdout); err != nil {
		fmt.Println(err.Error())
	}
}

// nolint gochecknoinits
func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "oxygen.yml", "path to yml config")
	rootCmd.PersistentFlags().BoolVar(&skipConfig, "skip-config", false, "skips config and uses ENV only")

	rootCmd.AddCommand(serveWebCommand)
	rootCmd.AddCommand(serverKMSCommand)
	rootCmd.AddCommand(runSchedulerCommand)
	rootCmd.AddCommand(allInOneCommand)
	rootCmd.AddCommand(envHelp)

	rootCmd.AddCommand(migrateCommand)
	migrateCommand.PersistentFlags().StringVar(&migrateSelectedCommand, "command", "status", "migration command")

	rootCmd.AddCommand(createUserCommand)
	createUserCommand.PersistentFlags().BoolVar(&overridePassword, "override-password", false, "overrides password if user already exists")

	rootCmd.AddCommand(listWalletsCommand)
	rootCmd.AddCommand(listBalancesCommand)

	topupBalanceSetup(topupBalanceCommand)
	rootCmd.AddCommand(topupBalanceCommand)

	rand.Seed(time.Now().Unix())
}
