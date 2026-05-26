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