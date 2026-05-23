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

		// Env overrides
		if env := os.Getenv("REMOTTY_SIGNAL_URL"); env != "" && cfg.SignalURL == "" {
			cfg.SignalURL = env
		}
		if env := os.Getenv("REMOTTY_MASTER_PASSWORD"); env != "" && cfg.MasterPassword == "" {
			cfg.MasterPassword = env
		}

		// Positional arg = host ID
		if len(args) > 0 {
			cfg.HostID = args[0]
		}

		c, err := client.NewClient(cfg, log.Logger)
		if err != nil {
			return err
		}

		// List mode (no host specified)
		if cfg.HostID == "" {
			hosts, err := c.ListHosts()
			if err != nil {
				return fmt.Errorf("list hosts: %w", err)
			}
			if len(hosts) == 0 {
				fmt.Println("No hosts available. Start a host with: remotty host")
				return nil
			}
			fmt.Println("\nAvailable hosts:")
			for _, h := range hosts {
				fmt.Printf("  %s — %s/%s [%s]\n",
					h.Name, h.Platform, h.Arch, joinStrings(h.Features, ", "))
			}
			return nil
		}

		ctx, stop := gosignal.NotifyContext(context.Background(),
			syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		return c.ConnectInteractive(ctx)
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringP("signal", "s", "ws://localhost:9000", "Signaling server URL")
	connectCmd.Flags().StringP("password", "p", "", "Master password")
}

func joinStrings(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}
