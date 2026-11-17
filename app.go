// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

// Package hirevec implements core server
package hirevec

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultLogLevel = slog.LevelWarn
)

func ParseUserFullName(value string) (string, error) {
	name, err := NormalizeAndValidateText(value, "full_name", DefaultUserFullNameMinLength, DefaultUserFullNameMaxLength, true)
	if err != nil {
		return "", fmt.Errorf("failed to parse full_name: %w", err)
	}
	return name, nil
}

func ParseEmbeddingsDim(value string) (uint32, error) {
	parsedValue, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse embeddings dim: %w", err)
	}
	return uint32(parsedValue), nil
}

func ParseStringList(value string) ([]string, error) {
	result := make([]string, 0, strings.Count(value, ",")+1)
	for part := range strings.SplitSeq(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("list is empty")
	}

	return result, nil
}

type AppConfig struct {
	LogLevel slog.Level
	LogJSON  bool

	ServerURL           *url.URL
	RequestReadTimeout  time.Duration
	RequestWriteTimeout time.Duration
	GracePeriod         time.Duration
	RateLimitingEnabled bool

	LlamaCppURL             *url.URL
	LlamaCppAPIKeyFile      string
	LlamaCppRerankerModel   string
	LlamaCppEmbeddingsModel string
	LlamaCppEmbeddingsDim   uint32

	EmbeddingsJobFrequency      time.Duration
	RecommendationsJobFrequency time.Duration

	APIKeyFile string

	WebAuthnRPDisplayName string
	WebAuthnRPID          string
	WebAuthnRPOrigins     []string

	SymmetricKeyFile  string
	AsymmetricKeyFile string

	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	TrustedProxy bool
}

func RunApp(c AppConfig) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stop()

	vault, err := NewVault(ctx, VaultConfig{
		ServerURL:         c.ServerURL,
		SymmetricKeyPath:  c.SymmetricKeyFile,
		AsymmetricKeyPath: c.AsymmetricKeyFile,
	})
	if err != nil {
		return fmt.Errorf("failed to init vault: %w", err)
	}

	store, err := NewStore(StoreConfig{LlamaCppEmbeddingsDim: c.LlamaCppEmbeddingsDim})
	if err != nil {
		return fmt.Errorf("failed to init store: %w", err)
	}

	useLlamaCpp := false
	llamaCppAPIKey := ""
	if c.LlamaCppAPIKeyFile != "" {
		data, err := os.ReadFile(c.LlamaCppAPIKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read llama-cpp server API key: %w", err)
		}

		llamaCppAPIKey = strings.TrimSpace(string(data))

		useLlamaCpp = c.LlamaCppURL.String() != "" && llamaCppAPIKey != ""
		if !useLlamaCpp {
			slog.Warn("refusing to use llama-cpp server due to missing API key")
		}
	}

	apiKey := ""
	if c.APIKeyFile != "" {
		data, err := os.ReadFile(c.APIKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}

		apiKey = strings.TrimSpace(string(data))
		if apiKey == "" {
			slog.Warn("refusing to use empty API key")
		}
	}

	return RunAPI(
		ctx,
		APIConfig{
			ServerURL:                   c.ServerURL,
			RequestReadTimeout:          c.RequestReadTimeout,
			RequestWriteTimeout:         c.RequestWriteTimeout,
			GracePeriod:                 c.GracePeriod,
			RateLimitingEnabled:         c.RateLimitingEnabled,
			WebAuthnRPDisplayName:       c.WebAuthnRPDisplayName,
			WebAuthnRPID:                c.WebAuthnRPID,
			WebAuthnRPOrigins:           c.WebAuthnRPOrigins,
			LlamaCppURL:                 c.LlamaCppURL.String(),
			LlamaCppAPIKey:              llamaCppAPIKey,
			LlamaCppRerankerModel:       c.LlamaCppRerankerModel,
			LlamaCppEmbeddingsDim:       c.LlamaCppEmbeddingsDim,
			LlamaCppEmbeddingsModel:     c.LlamaCppEmbeddingsModel,
			UseEmbeddings:               useLlamaCpp,
			UseReranker:                 useLlamaCpp,
			TLSEnabled:                  c.TLSEnabled,
			TLSCertFile:                 c.TLSCertFile,
			TLSKeyFile:                  c.TLSKeyFile,
			TrustedProxy:                c.TrustedProxy,
			APIKey:                      apiKey,
			EmbeddingsJobFrequency:      c.EmbeddingsJobFrequency,
			RecommendationsJobFrequency: c.RecommendationsJobFrequency,
		},
		store,
		vault,
	)
}
