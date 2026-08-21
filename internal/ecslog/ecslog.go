// Package ecslog builds a structured JSON logger whose output follows the
// Elastic Common Schema (ECS): fields are renamed to @timestamp, log.level,
// and message, and every record carries ecs.version.
package ecslog

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const ecsVersion = "1.6.0"

// ParseLevel maps a case-insensitive level name (debug, info, warn/warning,
// error) to its slog.Level.
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q: want debug, info, warn, or error", name)
	}
}

// New builds an ECS-structured JSON logger writing to stdout, enabled for
// level and above.
func New(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttr,
	})
	return slog.New(handler).With("ecs.version", ecsVersion)
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		a.Key = "@timestamp"
	case slog.LevelKey:
		a.Key = "log.level"
		a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}

// Err renders err as ECS's error.message field, for consistent error
// logging across packages (e.g. logger.Error("elasticsearch query failed", ecslog.Err(err))).
func Err(err error) slog.Attr {
	return slog.Group("error", "message", err.Error())
}
