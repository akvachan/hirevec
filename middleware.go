// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

func (a *API) MiddlewareSecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if a.TLSEnabled {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	}
}

func (a *API) ClientIP(r *http.Request) string {
	if a.TrustedProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if _, client, found := strings.Cut(forwarded, ","); found {
				return strings.TrimSpace(client)
			}
			return strings.TrimSpace(forwarded)
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func (a *API) MiddlewareAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")

		if apiKey == "" {
			JSON(w, Problem{
				Type:   ProblemTypeAPIKeyRequired,
				Detail: "API key is required",
				Source: &ProblemSource{Header: "X-API-Key"},
			}, http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(a.APIKey)) != 1 {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidAPIKey,
				Detail: "invalid API key",
				Source: &ProblemSource{Header: "X-API-Key"},
			}, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func (a *API) MiddlewareAuthorization(roles map[Role]bool) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			bearer, found := strings.CutPrefix(authHeader, "Bearer ")
			if !found || bearer == "" {
				w.Header().Set("WWW-Authenticate", "Bearer")
				JSON(w, Problem{
					Type:   ProblemTypeBearerTokenRequired,
					Detail: ErrBearerTokenRequired.Error(),
					Source: &ProblemSource{Header: "Authorization"},
				}, http.StatusUnauthorized)
				return
			}

			claims, err := a.Vault.ParseAccessToken(bearer)
			if err != nil {
				w.Header().Set("WWW-Authenticate", "Bearer")
				JSON(w, Problem{
					Type:   ProblemTypeInvalidAccessToken,
					Detail: ErrInvalidAccessToken.Error(),
					Source: &ProblemSource{Header: "Authorization"},
				}, http.StatusUnauthorized)
				return
			}

			if len(roles) > 0 {
				authorized := false
				for role := range claims.Roles {
					if roles[role] {
						authorized = true
						break
					}
				}
				if !authorized {
					required := make([]string, 0, len(roles))
					for role := range roles {
						required = append(required, string(role))
					}
					slices.Sort(required)
					JSON(w, Problem{
						Type:   ProblemTypeMissingRequiredRole,
						Detail: "missing one of required roles: " + strings.Join(required, ", "),
					}, http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ContextKeyClaims, claims)))
		}
	}
}

func (a *API) MiddlewareLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		responseWriter := &ResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(responseWriter, r)
		slog.Info(
			"request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", responseWriter.status,
			"duration", time.Since(start),
		)
	}
}

func (a *API) MiddlewareMaxBytesLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1_000_000)
		next.ServeHTTP(w, r)
	}
}

func (a *API) MiddlewareChain(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	wrapped := handler
	for _, middleware := range slices.Backward(middlewares) {
		wrapped = middleware(wrapped)
	}
	return wrapped
}

func (a *API) MiddlewarePanicRecovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error(
					"recovered from panic",
					"panic", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}
}
