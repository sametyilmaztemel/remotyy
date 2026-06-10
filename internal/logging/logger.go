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
				return fmt.Sprintf("[%-5s]", i)
			},
			FormatMessage: func(i interface{}) string {
				return fmt.Sprintf("%s", i)
			},
			PartsOrder: []string{"time", "level", "caller", "message"},
		}
		writers = append(writers, console)
	}

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		writers = append(writers, f)
	}

	multi := io.MultiWriter(writers...)
	base := zerolog.New(multi).Level(level).With().Timestamp().Caller().Stack().Logger()

	audit, err := NewAuditLogger(logFile)
	if err != nil {
		return nil, fmt.Errorf("init audit log: %w", err)
	}

	return &Logger{Logger: base, Audit: audit}, nil
}

// AuditLogEntry represents a security-relevant event.
type AuditLogEntry struct {
	Time    time.Time `json:"time"`
	Event   string    `json:"event"`
	PeerID  string    `json:"peer_id,omitempty"`
	RoomID  string    `json:"room_id,omitempty"`
	Remote  string    `json:"remote,omitempty"`
	Success bool      `json:"success"`