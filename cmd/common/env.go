// Package common provides common utilities for CLIs
package common

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func InitLogger(level slog.Level, json bool) {
	if json {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	}
}

func MustParseEnv[T any](key string, parser func(string) (T, error), defaultValue T) T {
	value, exists := os.LookupEnv(key)
	if !exists || value == "" {
		return defaultValue
	}

	parsed, err := parser(value)
	if err != nil {
		panic(fmt.Errorf("failed to parse environment variable %q: %w", key, err))
	}

	return parsed
}

func LoadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close file", "err", err)
		}
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if err = os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	return scanner.Err()
}
