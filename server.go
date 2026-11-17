// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

type APIConfig struct {
	ServerURL           *url.URL
	RequestReadTimeout  time.Duration
	RequestWriteTimeout time.Duration
	GracePeriod         time.Duration
	RateLimitingEnabled bool

	LlamaCppURL             string
	LlamaCppAPIKey          string // TODO: Make this a secret value
	LlamaCppEmbeddingsModel string
	LlamaCppRerankerModel   string
	LlamaCppEmbeddingsDim   uint32

	EmbeddingsJobFrequency      time.Duration
	RecommendationsJobFrequency time.Duration
	UseEmbeddings               bool
	UseReranker                 bool

	WebAuthnRPDisplayName string
	WebAuthnRPID          string
	WebAuthnRPOrigins     []string

	APIKey       string // TODO: Make this a secret value
	TLSCertFile  string
	TLSKeyFile   string
	TLSEnabled   bool
	TrustedProxy bool
}

type API struct {
	ServerURL           *url.URL
	APIKey              string // TODO: Make this a secret value
	TLSEnabled          bool
	RateLimitingEnabled bool
	RateLimiter         *RateLimiter
	TrustedProxy        bool
	Store               Store
	Vault               Vault
	Server              http.Server
	Mux                 *http.ServeMux
	WebAuthn            *webauthn.WebAuthn
}

const (
	DefaultRequestReadTimeout  = 1000 * time.Millisecond
	DefaultRequestWriteTimeout = 1000 * time.Millisecond
	DefaultGracePeriod         = 5000 * time.Millisecond
)

var (
	ErrFailedShutdownServer = errors.New("failed to shutdown server")
	ErrBearerTokenRequired  = errors.New("bearer token is required")
	ErrInvalidAccessToken   = errors.New("invalid access token")
)

func NewAPI(ctx context.Context, c APIConfig, s Store, v Vault) (*API, error) {
	slog.Debug("initializing server")

	rpDisplayName := c.WebAuthnRPDisplayName
	if rpDisplayName == "" {
		rpDisplayName = "Hirevec"
	}

	rpID := c.WebAuthnRPID
	if rpID == "" {
		rpID = c.ServerURL.Hostname()
	}

	rpOrigins := c.WebAuthnRPOrigins
	if len(rpOrigins) == 0 {
		scheme := c.ServerURL.Scheme
		if scheme == "" {
			scheme = "http"
			if c.TLSEnabled {
				scheme = "https"
			}
		}
		rpOrigins = []string{fmt.Sprintf("%s://%s", scheme, c.ServerURL.Host)}
	}

	config := &webauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
	}

	wan, err := webauthn.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebAuthn config: %w", err)
	}

	api := API{
		ServerURL:           c.ServerURL,
		APIKey:              c.APIKey,
		TLSEnabled:          c.TLSEnabled,
		TrustedProxy:        c.TrustedProxy,
		RateLimitingEnabled: c.RateLimitingEnabled,
		Store:               s,
		Vault:               v,
		Mux:                 http.NewServeMux(),
		WebAuthn:            wan,
	}

	api.Server = http.Server{
		Addr:         c.ServerURL.Host,
		ReadTimeout:  c.RequestReadTimeout,
		WriteTimeout: c.RequestWriteTimeout,
		ErrorLog:     slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		Handler:      api.Mux,
	}

	if c.RateLimitingEnabled {
		api.RateLimiter = &RateLimiter{Visitors: make(map[LimiterKey]*Visitor)}
	}

	api.RegisterRoutes()

	return &api, nil
}

func RunAPI(ctx context.Context, c APIConfig, s Store, v Vault) error {
	api, err := NewAPI(ctx, c, s, v)
	if err != nil {
		return fmt.Errorf("failed to create new API: %w", err)
	}

	slog.Debug("creating listener", "addr", api.Server.Addr)
	listener, err := net.Listen("tcp", api.Server.Addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	slog.Debug("starting server", "addr", api.Server.Addr)
	errCh := make(chan error, 1)

	go func() {
		if c.TLSEnabled {
			if err := api.Server.ServeTLS(listener, c.TLSCertFile, c.TLSKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("failed to serve with TLS", "err", err)
				errCh <- err
			}
		} else {
			slog.Warn("server is running without TLS, channels are compromised, enable TLS in production")
			if err := api.Server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("failed to serve", "err", err)
				errCh <- err
			}
		}
	}()
	slog.Info("server ready", "addr", api.Server.Addr)

	var rerankURL, embeddingsURL string
	rerankURL, err = url.JoinPath(c.LlamaCppURL, "v1/rerank")
	if err != nil {
		return fmt.Errorf("failed to form rerank URL: %w", err)
	}

	embeddingsURL, err = url.JoinPath(c.LlamaCppURL, "v1/embeddings")
	if err != nil {
		return fmt.Errorf("failed to form embeddings URL: %w", err)
	}

	aiConfig := AIConfig{
		UseEmbeddings:           c.UseEmbeddings,
		UseReranker:             c.UseReranker,
		LlamaCppRerankURL:       rerankURL,
		LlamaCppEmbeddingsURL:   embeddingsURL,
		LlamaCppAPIKey:          c.LlamaCppAPIKey,
		LlamaCppEmbeddingsModel: c.LlamaCppEmbeddingsModel,
		LlamaCppEmbeddingsDim:   c.LlamaCppEmbeddingsDim,
		LlamaCppRerankerModel:   c.LlamaCppRerankerModel,
	}

	if c.UseEmbeddings {
		slog.Debug("using embeddings")
		go func() {
			for range time.Tick(c.EmbeddingsJobFrequency) {
				if err := api.RunEmbeddingsJob(aiConfig); err != nil {
					slog.Error("failed to run embeddings job", "err", err)
				}
			}
		}()
	}

	go func() {
		for range time.Tick(c.RecommendationsJobFrequency) {
			if err := api.RunRecommendationsJob(aiConfig); err != nil {
				slog.Error("failed to run recommendations job", "err", err)
			}
		}
	}()

	go func() {
		for range time.Tick(DefaultWebAuthnCleanupJobFrequency) {
			if err := api.RunWebAuthnCleanupJob(); err != nil {
				slog.Error("failed to run webauthn cleanup job", "err", err)
			}
		}
	}()

	if c.RateLimitingEnabled {
		go func() {
			for range time.Tick(DefaultVisitorsCleanupJobFrequency) {
				api.RunCleanupVisitors()
			}
		}()
	}

	if err := api.WaitAndShutdown(ctx, errCh, c.GracePeriod); err != nil {
		return fmt.Errorf("failed to wait and shutdown: %w", err)
	}

	return nil
}

func (a *API) WaitAndShutdown(ctx context.Context, errCh chan error, gracePeriod time.Duration) error {
	select {
	case <-ctx.Done():
		slog.Debug("shutdown signal received", "err", ctx.Err())
	case err := <-errCh:
		slog.Debug("shutdown signal received", "err", err)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, gracePeriod)
	defer cancel()

	slog.Debug("starting graceful shutdown", "timeout", gracePeriod)
	if err := a.Server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to gracefully shutdown, forcing close", "err", err)
		if err := a.Server.Close(); err != nil {
			slog.Error("failed to force close server", "err", err)
		}
		return ErrFailedShutdownServer
	}
	slog.Info("server shutdown")

	return nil
}
