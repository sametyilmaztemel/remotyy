package cmd

import (
	"context"
	"os"
	gosignal "os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/signal"
	"github.com/spf13/cobra"
)

var signalCmd = &cobra.Command{
	Use:   "signal",
	Short: "Start the signaling server",
	Long: `Start the WebSocket signaling server for WebRTC negotiation.
The signaling server is a blind relay — it coordinates connections
but never sees terminal or screen data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := globalCfg.Signal

		// CLI flags override config file
		if v, _ := cmd.Flags().GetInt("port"); v != 0 {
			cfg.Port = v
		}
		if v, _ := cmd.Flags().GetString("host"); v != "" {
			cfg.Host = v
		}
		if v, _ := cmd.Flags().GetBool("dev"); v {
			cfg.DevMode = v
		}
		if v, _ := cmd.Flags().GetBool("tls"); v {
			cfg.TLS.Enabled = v
		}
		if v, _ := cmd.Flags().GetString("tls-cert"); v != "" {
			cfg.TLS.CertFile = v
		}
		if v, _ := cmd.Flags().GetString("tls-key"); v != "" {
			cfg.TLS.KeyFile = v
		}
		if v, _ := cmd.Flags().GetString("web-dir"); v != "" {
			cfg.WebDir = v
		}

		// Env overrides
		if authToken := os.Getenv("REMOTTY_AUTH_TOKEN"); authToken != "" {
			cfg.AuthToken = authToken
		}

		server := signal.NewServer(cfg, logger)

		ctx, stop := gosignal.NotifyContext(context.Background(),
			syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		log.Info().
			Str("version", config.Version).
			Str("addr", cfg.Addr()).
			Bool("tls", cfg.TLS.Enabled).
			Bool("dev_mode", cfg.DevMode).
			Msg("Signaling server starting")

		return server.Start(ctx)
	},
}

func init() {
	rootCmd.AddCommand(signalCmd)
	signalCmd.Flags().IntP("port", "p", 9000, "Signaling server port")
	signalCmd.Flags().StringP("host", "H", "0.0.0.0", "Bind address")
	signalCmd.Flags().Bool("dev", false, "Developer mode (no auth)")
	signalCmd.Flags().Bool("tls", false, "Enable TLS")
	signalCmd.Flags().String("tls-cert", "", "TLS certificate file")
	signalCmd.Flags().String("tls-key", "", "TLS key file")
	signalCmd.Flags().String("web-dir", "", "Web UI directory to serve (e.g. ./web/dist)")
}
