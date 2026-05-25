package cmd

import (
	"context"
	"fmt"
	"os"
	gosignal "os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/host"
	"github.com/sametyilmaztemel/remotty/internal/qr"
	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Start the host daemon",
	Long: `Start the remotty host daemon on this machine.
Connects to signaling server and waits for client connections.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg.Host

		// CLI flags override config file
		signalFlag, _ := cmd.Flags().GetString("signal")
		nameFlag, _ := cmd.Flags().GetString("name")
		if signalFlag != "" {
			cfg.SignalURL = signalFlag
		}
		if nameFlag != "" {
			cfg.Name = nameFlag
		}
		if v, _ := cmd.Flags().GetString("master-password"); v != "" {
			cfg.MasterPassword = v
		}
		if v, _ := cmd.Flags().GetBool("qr"); v {
			cfg.ShowQR = true
		}

		// Env overrides (lowest priority)
		if env := os.Getenv("REMOTTY_SIGNAL_URL"); env != "" && cfg.SignalURL == "" {
			cfg.SignalURL = env
		}
		if env := os.Getenv("REMOTTY_MASTER_PASSWORD"); env != "" && cfg.MasterPassword == "" {
			cfg.MasterPassword = env
		}

		daemon, err := host.NewDaemon(cfg, log.Logger)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create host daemon")
			return err
		}

		if cfg.ShowQR {
			cfg.OnRegistered = func(peerID string) {
				qrArt, url, err := qr.Generate(qr.PairingURL{
					Version:  1,
					Signal:   cfg.SignalURL,
					HostID:   peerID,
					HostName: cfg.Name,
				})
				if err != nil {
					log.Error().Err(err).Msg("Failed to generate QR code")
					return
				}
				fmt.Println("\n" + qrArt)
				fmt.Println("📱 Scan this QR code with your phone camera")
				fmt.Println("   Or open this URL:")
				fmt.Println("   " + url)
				fmt.Println()
			}
		}

		ctx, stop := gosignal.NotifyContext(context.Background(),
			syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		log.Info().
			Str("version", config.Version).
			Str("name", cfg.Name).
			Str("signal", cfg.SignalURL).
			Strs("features", cfg.Features).
			Bool("has_master_pw", cfg.MasterPassword != "" || cfg.MasterHash != "").
			Msg("Host daemon starting")

		return daemon.Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(hostCmd)
	hostCmd.Flags().StringP("signal", "s", "ws://localhost:9000", "Signaling server URL")
	hostCmd.Flags().StringP("name", "n", "", "Host display name")
	hostCmd.Flags().StringP("master-password", "m", "", "Master password")
	hostCmd.Flags().Bool("qr", false, "Show QR code for zero-config phone pairing")
}
