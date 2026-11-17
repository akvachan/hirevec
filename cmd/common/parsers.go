package common

import (
	"errors"
	"log/slog"
	"strings"
)

var ErrUnknownLogLevel = errors.New("unknown log level")

func ParseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "info":
		return slog.LevelInfo, nil
	case "error":
		return slog.LevelError, nil
	case "warn":
		return slog.LevelWarn, nil
	case "debug":
		return slog.LevelDebug, nil
	default:
		return -1, ErrUnknownLogLevel
	}
}
