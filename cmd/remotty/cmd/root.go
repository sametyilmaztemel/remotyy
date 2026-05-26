package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/sametyilmaztemel/remotty/internal/config"
	"github.com/sametyilmaztemel/remotty/internal/logging"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
	logFile   string
	globalCfg *config.Config
	logger    *logging.Logger
)

var rootCmd = &cobra.Command{
	Use:   "remotty",
	Short: "Remote terminal & screen access via WebRTC",
	Long: `remotty — open-source remote access for your machines.
