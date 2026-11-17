// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

type Method string

const (
	MethodPost   Method = http.MethodPost
	MethodGet    Method = http.MethodGet
	MethodPatch  Method = http.MethodPatch
	MethodDelete Method = http.MethodDelete
)

type Route string

const (
	RouteCandidate              Route = "/candidates/{id}"
	RouteCandidates             Route = "/candidates"
	RouteWebauthnRegisterStart  Route = "/webauthn/register/start"
	RouteWebauthnRegisterFinish Route = "/webauthn/register/finish"
	RouteWebauthnLoginStart     Route = "/webauthn/login/start"
	RouteWebauthnLoginFinish    Route = "/webauthn/login/finish"
	RouteDocs                   Route = "/docs"
	RouteHealth                 Route = "/health"
	RouteLocation               Route = "/locations/{id}"
	RouteLocations              Route = "/locations"
	RouteMatches                Route = "/matches"
	RouteOpenAPI                Route = "/openapi.yaml"
	RoutePasskeys               Route = "/passkeys"
	RoutePosition               Route = "/positions/{id}"
	RoutePositions              Route = "/positions"
	RouteReactions              Route = "/reactions"
	RouteRecommendation         Route = "/recommendations/{id}"
	RouteRecommendations        Route = "/recommendations"
	RouteRecruiter              Route = "/recruiters/{id}"
	RouteRecruiters             Route = "/recruiters"
	RouteRefresh                Route = "/refresh"
	RouteUser                   Route = "/users/{id}"
	RouteUsers                  Route = "/users"
)

var (
	RateLimitHealth  rate.Limit = 20
	RateLimitStatic  rate.Limit = 20
	RateLimitRead    rate.Limit = 10
	RateLimitWrite   rate.Limit = 2
	RateLimitDelete  rate.Limit = 1
	RateLimitSearch  rate.Limit = 3
	RateLimitAuth    rate.Limit = rate.Every(time.Minute)
	RateLimitRefresh rate.Limit = rate.Every(time.Minute)

	BurstHealth  = 20
	BurstStatic  = 10
	BurstRead    = 20
	BurstWrite   = 5
	BurstDelete  = 2
	BurstSearch  = 10
	BurstAuth    = 5
	BurstRefresh = 5
)

type RouteConfig struct {
	Method    Method
	Handler   http.HandlerFunc
	Route     Route
	Roles     []Role
	RateLimit rate.Limit
	Burst     int
	NoAPIKey  bool
}

func (a *API) PublicRoute(c RouteConfig) {
	handler := a.MiddlewareChain(c.Handler, a.MiddlewareLogging)
	if !c.NoAPIKey && a.APIKey != "" {
		handler = a.MiddlewareChain(handler, a.MiddlewareAPIKey)
	}
	if a.RateLimitingEnabled {
		handler = a.MiddlewareChain(handler, a.MiddlewareRateLimit(c.RateLimit, c.Burst))
	}
	handler = a.MiddlewareChain(
		handler,
		a.MiddlewareLogging,
		a.MiddlewareRateLimit(c.RateLimit, c.Burst),
		a.MiddlewareSecurityHeaders,
		a.MiddlewarePanicRecovery,
		a.MiddlewareMaxBytesLimit,
	)

	a.Mux.Handle(
		fmt.Sprintf("%s %s", c.Method, c.Route),
		handler,
	)
}

func (a *API) PrivateRoute(c RouteConfig) {
	rolesMap := make(map[Role]bool)
	for _, role := range c.Roles {
		rolesMap[role] = true
	}

	handler := a.MiddlewareChain(c.Handler, a.MiddlewareLogging)
	if !c.NoAPIKey && a.APIKey != "" {
		handler = a.MiddlewareChain(handler, a.MiddlewareAPIKey)
	}
	if a.RateLimitingEnabled {
		handler = a.MiddlewareChain(handler, a.MiddlewareRateLimit(c.RateLimit, c.Burst))
	}
	handler = a.MiddlewareChain(
		handler,
		a.MiddlewareLogging,
		a.MiddlewareRateLimit(c.RateLimit, c.Burst),
		a.MiddlewareSecurityHeaders,
		a.MiddlewarePanicRecovery,
		a.MiddlewareMaxBytesLimit,
		a.MiddlewareAuthorization(rolesMap),
	)

	a.Mux.Handle(
		fmt.Sprintf("%s %s", c.Method, c.Route),
		handler,
	)
}

func (a *API) RegisterRoutes() {
	slog.Debug("registering routes")

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteOpenAPI,
		Handler:   a.HandlerOpenAPI,
		RateLimit: RateLimitStatic,
		Burst:     BurstStatic,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteDocs,
		Handler:   a.HandlerDocs,
		RateLimit: RateLimitStatic,
		Burst:     BurstStatic,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteHealth,
		Handler:   a.HandlerHealth,
		RateLimit: RateLimitHealth,
		Burst:     BurstHealth,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RoutePasskeys,
		Handler:   a.HandlerPasskeys,
		RateLimit: RateLimitStatic,
		Burst:     BurstStatic,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteWebauthnRegisterStart,
		Handler:   a.HandlerWebAuthnRegisterStart(),
		RateLimit: RateLimitAuth,
		Burst:     BurstAuth,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteWebauthnRegisterFinish,
		Handler:   a.HandlerWebAuthnRegisterFinish(),
		RateLimit: RateLimitAuth,
		Burst:     BurstAuth,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteWebauthnLoginStart,
		Handler:   a.HandlerWebAuthnLoginStart(),
		RateLimit: RateLimitAuth,
		Burst:     BurstAuth,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteWebauthnLoginFinish,
		Handler:   a.HandlerWebAuthnLoginFinish(),
		RateLimit: RateLimitAuth,
		Burst:     BurstAuth,
		NoAPIKey:  true,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteRefresh,
		Handler:   a.HandleRefresh(),
		RateLimit: RateLimitRefresh,
		Burst:     BurstRefresh,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteUsers,
		Handler:   a.HandlerCreateUser(),
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteUser,
		Handler:   a.HandlerGetUser(),
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPatch,
		Route:     RouteUser,
		Handler:   a.HandlerPatchUser(),
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodDelete,
		Route:     RouteUser,
		Handler:   a.HandlerDeleteUser(),
		RateLimit: RateLimitDelete,
		Burst:     BurstDelete,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteCandidates,
		Handler:   a.HandlerCreateCandidate(),
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteCandidate,
		Handler:   a.HandlerGetCandidate(),
		Roles:     []Role{RoleRecruiter, RoleCandidate},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPatch,
		Route:     RouteCandidate,
		Handler:   a.HandlerPatchCandidate(),
		Roles:     []Role{RoleCandidate},
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodDelete,
		Route:     RouteCandidate,
		Handler:   a.HandlerDeleteCandidate(),
		Roles:     []Role{RoleCandidate},
		RateLimit: RateLimitDelete,
		Burst:     BurstDelete,
	})

	a.PrivateRoute(RouteConfig{
		Method:  MethodPost,
		Route:   RouteRecruiters,
		Handler: a.HandlerCreateRecruiter(),
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteRecruiter,
		Handler:   a.HandlerGetRecruiter(),
		Roles:     []Role{RoleRecruiter, RoleCandidate},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodDelete,
		Route:     RouteRecruiter,
		Handler:   a.HandlerDeleteRecruiter(),
		Roles:     []Role{RoleRecruiter},
		RateLimit: RateLimitDelete,
		Burst:     BurstDelete,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RoutePositions,
		Handler:   a.HandlerCreatePosition(),
		Roles:     []Role{RoleRecruiter},
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RoutePositions,
		Handler:   a.HandlerGetPositions(),
		Roles:     []Role{RoleRecruiter},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RoutePosition,
		Handler:   a.HandlerGetPosition(),
		Roles:     []Role{RoleRecruiter, RoleCandidate},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPatch,
		Route:     RoutePosition,
		Handler:   a.HandlerPatchPosition(),
		Roles:     []Role{RoleRecruiter},
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodDelete,
		Route:     RoutePosition,
		Handler:   a.HandlerDeletePosition(),
		Roles:     []Role{RoleRecruiter},
		RateLimit: RateLimitDelete,
		Burst:     BurstDelete,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteRecommendations,
		Handler:   a.HandlerGetRecommendations(),
		Roles:     []Role{RoleCandidate, RoleRecruiter},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteRecommendation,
		Handler:   a.HandlerGetRecommendation(),
		Roles:     []Role{RoleCandidate, RoleRecruiter},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteReactions,
		Handler:   a.HandlerGetReactions(),
		Roles:     []Role{RoleCandidate, RoleRecruiter},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodPost,
		Route:     RouteReactions,
		Handler:   a.HandlerCreateReaction(),
		Roles:     []Role{RoleCandidate, RoleRecruiter},
		RateLimit: RateLimitWrite,
		Burst:     BurstWrite,
	})

	a.PrivateRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteMatches,
		Handler:   a.HandlerGetMatches(),
		Roles:     []Role{RoleCandidate, RoleRecruiter},
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteLocations,
		Handler:   a.HandlerSearchLocations(),
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})

	a.PublicRoute(RouteConfig{
		Method:    MethodGet,
		Route:     RouteLocation,
		Handler:   a.HandlerGetLocation(),
		RateLimit: RateLimitRead,
		Burst:     BurstRead,
	})
}
