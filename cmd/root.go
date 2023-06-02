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
	Commit     = "none"
	Version    = "none"
	configPath = "config.yml"
	skipConfig = false
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "oxygen",
	Short: "Oxygen service",
	Long:  "Main O2Pay backend service",
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
	cfg, err := config.New(Version, Commit, configPath, skipConfig)
	if err != nil {
		fmt.Printf("unable to initialize config: %s\n", err.Error())
		os.Exit(1)
	}

	if skipConfig {
		fmt.Println("Skipped file-based configuration, using only ENV")
	}

	return cfg
}

func envCommand(c *cobra.Command, _ []string) {
	if err := config.PrintUsage(os.Stdout); err != nil {
		fmt.Println(err.Error())
	}
}

// nolint gochecknoinits
func init() {
	rootCmd.AddCommand(startServerCmd)
	startServerCmd.PersistentFlags().StringVar(&configPath, "config", "oxygen.yml", "--config=oxygen.yml")
	startServerCmd.PersistentFlags().BoolVar(&skipConfig, "skip-config", false, "--skip-config=false")

	rootCmd.AddCommand(kmsServerCmd)
	kmsServerCmd.PersistentFlags().StringVar(&configPath, "config", "oxygen.yml", "--config=oxygen.yml")
	kmsServerCmd.PersistentFlags().BoolVar(&skipConfig, "skip-config", false, "--skip-config=false")

	rootCmd.AddCommand(schedulerCmd)
	schedulerCmd.PersistentFlags().StringVar(&configPath, "config", "oxygen.yml", "--config=oxygen.yml")
	schedulerCmd.PersistentFlags().BoolVar(&skipConfig, "skip-config", false, "--skip-config=false")

	rootCmd.AddCommand(migrateCmd)
	migrateCmd.PersistentFlags().StringVar(&configPath, "config", "oxygen.yml", "--config=oxygen.yml")
	migrateCmd.PersistentFlags().BoolVar(&skipConfig, "skip-config", false, "--skip-config=false")
	migrateCmd.PersistentFlags().StringVar(&migrateSelectedCommand, "command", "status", "--command=status")

	rootCmd.AddCommand(envHelp)

	rand.Seed(time.Now().Unix())
}
