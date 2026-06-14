// Package cmd is the cobra-based CLI for HOM3.
package cmd

import (
	"fmt"
	"os"

	"github.com/imaustink/hatui/internal/client"
	"github.com/imaustink/hatui/internal/config"
	"github.com/imaustink/hatui/internal/view"
	"github.com/spf13/cobra"
)

var (
	flagURL   string
	flagToken string
	flagName  string
)

var rootCmd = &cobra.Command{
	Use:   "hom3",
	Short: "Terminal UI for Home Assistant",
	Long: `HOM3 — a k9s-inspired terminal dashboard for Home Assistant.

Run without subcommands to launch the interactive TUI.
Use subcommands for one-off CLI operations.`,
	// Launch TUI when no subcommand is given
	RunE: func(cmd *cobra.Command, args []string) error {
		homes, activeIdx, err := config.Load(flagURL, flagToken, flagName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "No configuration found. Set HASS_URL and HASS_TOKEN or create ~/.config/hom3/config.json")
			os.Exit(1)
		}
		c := client.New()
		a := view.NewApp(c, homes, activeIdx)
		return a.Start()
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "Home Assistant URL (e.g. http://homeassistant.local:8123)")
	rootCmd.PersistentFlags().StringVar(&flagToken, "token", "", "Long-lived access token")
	rootCmd.PersistentFlags().StringVar(&flagName, "name", "", "Display name for this home")
}
