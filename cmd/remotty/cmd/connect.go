package cmd

import (
	"context"
	"fmt"
	"os"
	gosignal "os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/client"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect [host-id]",
	Short: "Connect to a remote host",
	Long: `Connect to a remotty host for remote terminal access.
If no host ID is given, lists available hosts.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg.Client

		// CLI flags override config file
		if v, _ := cmd.Flags().GetString("signal"); v != "" {
			cfg.SignalURL = v
		}
		if v, _ := cmd.Flags().GetString("password"); v != "" {
			cfg.MasterPassword = v
		}