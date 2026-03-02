package klogging

import (
	"fmt"
	"log/slog"
	"strings"
)

// Level aliases for backward compatibility and convenience.
const (
	LevelVerbose = slog.LevelDebug - 1
	LevelDebug   = slog.LevelDebug
	LevelInfo    = slog.LevelInfo
	LevelWarn    = slog.LevelWarn
	LevelError   = slog.LevelError
	LevelFatal   = slog.LevelError + 1
)

// ParseLevel parses a log level string.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "verbose", "trace":
		return LevelVerbose
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// LevelString returns the string representation of a level.
func LevelString(level slog.Level) string {
	switch level {
	case LevelVerbose:
		return "VERBOSE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return fmt.Sprintf("LEVEL(%d)", level)
	}
}
