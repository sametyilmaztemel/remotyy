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