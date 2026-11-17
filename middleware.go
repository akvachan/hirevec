// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"math"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

type Visitor struct {
	Limiter  *rate.Limiter
	LastSeen atomic.Int64
}

type LimiterKey struct {
	IP       string
	Method   string
	Endpoint string
}

type RateLimiter struct {
	Visitors map[LimiterKey]*Visitor
	Mu       sync.RWMutex
}

func (rl *RateLimiter) GetLimiter(key LimiterKey, limit rate.Limit, burst int) *rate.Limiter {
	rl.Mu.RLock()
	visitor, exists := rl.Visitors[key]
	rl.Mu.RUnlock()

	if !exists {
		rl.Mu.Lock()

		visitor, exists = rl.Visitors[key]
		if !exists {
			visitor = &Visitor{
				Limiter: rate.NewLimiter(limit, burst),
			}
			rl.Visitors[key] = visitor
		}

		rl.Mu.Unlock()
	}

	visitor.LastSeen.Store(time.Now().Unix())

	return visitor.Limiter
}

func (rl *RateLimiter) Allow(limiter *rate.Limiter) (bool, time.Duration) {
	reservation := limiter.Reserve()

	if !reservation.OK() {
		return false, 0
	}

	delay := reservation.Delay()

	if delay > 0 {
		reservation.Cancel()
		return false, delay
	}

	return true, 0
}

func (a *API) MiddlewareRateLimit(rateLimit rate.Limit, burst int) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			endpoint := r.Pattern
			if endpoint == "" {
				endpoint = r.URL.Path
			}

			limiter := a.RateLimiter.GetLimiter(
				LimiterKey{
					IP:       a.ClientIP(r),
					Method:   r.Method,
					Endpoint: endpoint,
				},
				rateLimit,
				burst,
			)
			allowed, retryAfter := a.RateLimiter.Allow(limiter)

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(max(int(math.Ceil(retryAfter.Seconds())), 1)))
				JSON(w, Problem{
					Type:   ProblemTypeRateLimitExceeded,
					Detail: "rate limit exceeded",
				}, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

func (a *API) MiddlewareSecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
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
