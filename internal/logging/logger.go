// Package logging provides centralized logging configuration for remotty.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

// Logger wraps zerolog.Logger with remotty-specific enhancements.
type Logger struct {
	zerolog.Logger
	Audit *AuditLogger
}

// Init creates and configures the application logger.
func Init(level zerolog.Level, format, logFile string) (*Logger, error) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.DurationFieldUnit = time.Millisecond

	var writers []io.Writer

	if format == "console" || logFile == "" {
		console := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			FormatLevel: func(i interface{}) string {
				if i == nil {
					return ""
				}