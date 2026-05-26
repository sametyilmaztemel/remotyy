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

Connect to your Mac or Linux server from anywhere via encrypted WebRTC.
Features: terminal, screen sharing, file transfer, clipboard sync.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		initConfig()
		return initLogging()
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "console", "log format (console, json)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "log file path")
}

func initConfig() {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	globalCfg = cfg

	if logLevel != "info" {
		cfg.Logging.Level = logLevel
	}
	if logFormat != "console" {
		cfg.Logging.Format = logFormat
	}
	if logFile != "" {
		cfg.Logging.File = logFile
	}
}

func initLogging() error {
	l, err := logging.Init(globalCfg.Logging.ParseLevel(), globalCfg.Logging.Format, globalCfg.Logging.File)
	if err != nil {
		return fmt.Errorf("init logging: %w", err)
	}
	logger = l
	log.Logger = l.Logger
	return nil
}
