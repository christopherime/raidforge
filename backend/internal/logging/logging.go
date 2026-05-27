// Package logging builds the application's structured logger.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a slog.Logger for the given level (debug|info|warn|error) and
// format (json|text), writing to stderr, and installs it as the default logger.
func New(level, format string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if strings.EqualFold(format, "text") {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}

	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
