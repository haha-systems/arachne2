// Package daemon implements the process-level Arachne runtime lifecycle.
package daemon

import (
	"fmt"
	"log/slog"
	"time"
)

// Config holds the process-level settings for one Arachne organism daemon.
type Config struct {
	OrganismID      string
	LogLevel        string
	MailboxCapacity int
	EventCapacity   int
	ShutdownTimeout time.Duration
}

// DefaultConfig returns conservative defaults for a local development daemon.
func DefaultConfig() Config {
	return Config{
		OrganismID:      "arachne-local",
		LogLevel:        "info",
		MailboxCapacity: 16,
		EventCapacity:   10000,
		ShutdownTimeout: 5 * time.Second,
	}
}

// Validate checks configuration before the daemon reports readiness.
func (c Config) Validate() error {
	if c.OrganismID == "" {
		return fmt.Errorf("organism ID must not be empty")
	}
	if _, err := ParseLogLevel(c.LogLevel); err != nil {
		return err
	}
	if c.MailboxCapacity < 1 {
		return fmt.Errorf("mailbox capacity must be positive")
	}
	if c.EventCapacity < 1 {
		return fmt.Errorf("event capacity must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	return nil
}

// ParseLogLevel converts the supported configuration spelling to a slog level.
func ParseLogLevel(value string) (slog.Level, error) {
	switch value {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", value)
	}
}
