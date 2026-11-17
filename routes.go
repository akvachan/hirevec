// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"fmt"
	"log/slog"
	"net/http"
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

type RouteConfig struct {
	Method      Method
	Handler     http.HandlerFunc
	Middlewares []Middleware
	Route       Route
	Roles       []Role
}

func (a *API) PublicRoute(c RouteConfig) {
	handler := a.MiddlewareChain(
		c.Handler,
		a.MiddlewareLogging,
		a.MiddlewareSecurityHeaders,
		a.MiddlewarePanicRecovery,
		a.MiddlewareMaxBytesLimit,
	)
	handler = a.MiddlewareChain(handler, c.Middlewares...)

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

	handler := a.MiddlewareChain(
		c.Handler,
		a.MiddlewareLogging,
		a.MiddlewareSecurityHeaders,
		a.MiddlewarePanicRecovery,
		a.MiddlewareMaxBytesLimit,
		a.MiddlewareAuthorization(rolesMap),
	)
	handler = a.MiddlewareChain(handler, c.Middlewares...)

	a.Mux.Handle(
		fmt.Sprintf("%s %s", c.Method, c.Route),
		handler,
	)
}

func (a *API) RegisterRoutes() {
	slog.Debug("registering routes")

	a.PublicRoute(RouteConfig{
		Method:  MethodGet,
		Route:   RouteOpenAPI,
		Handler: a.HandlerOpenAPI,
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodGet,
		Route:   RouteDocs,
		Handler: a.HandlerDocs,
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodGet,
		Route:   RouteHealth,
		Handler: a.HandlerHealth,
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodGet,
		Route:   RoutePasskeys,
		Handler: a.HandlerPasskeys,
	})

	a.PublicRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RouteRefresh,
		Handler:     a.HandleRefresh(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PublicRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RouteUsers,
		Handler:     a.HandlerCreateUser(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodPost,
		Route:   RouteWebauthnRegisterStart,
		Handler: a.HandlerWebAuthnRegisterStart(),
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodPost,
		Route:   RouteWebauthnRegisterFinish,
		Handler: a.HandlerWebAuthnRegisterFinish(),
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodPost,
		Route:   RouteWebauthnLoginStart,
		Handler: a.HandlerWebAuthnLoginStart(),
	})

	a.PublicRoute(RouteConfig{
		Method:  MethodPost,
		Route:   RouteWebauthnLoginFinish,
		Handler: a.HandlerWebAuthnLoginFinish(),
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteUser,
		Handler:     a.HandlerGetUser(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPatch,
		Route:       RouteUser,
		Handler:     a.HandlerPatchUser(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodDelete,
		Route:       RouteUser,
		Handler:     a.HandlerDeleteUser(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RouteCandidates,
		Handler:     a.HandlerCreateCandidate(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteCandidate,
		Handler:     a.HandlerGetCandidate(),
		Roles:       []Role{RoleRecruiter, RoleCandidate},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPatch,
		Route:       RouteCandidate,
		Handler:     a.HandlerPatchCandidate(),
		Roles:       []Role{RoleCandidate},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodDelete,
		Route:       RouteCandidate,
		Handler:     a.HandlerDeleteCandidate(),
		Roles:       []Role{RoleCandidate},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RouteRecruiters,
		Handler:     a.HandlerCreateRecruiter(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteRecruiter,
		Handler:     a.HandlerGetRecruiter(),
		Roles:       []Role{RoleRecruiter, RoleCandidate},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodDelete,
		Route:       RouteRecruiter,
		Handler:     a.HandlerDeleteRecruiter(),
		Roles:       []Role{RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RoutePositions,
		Handler:     a.HandlerCreatePosition(),
		Roles:       []Role{RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RoutePositions,
		Handler:     a.HandlerGetPositions(),
		Roles:       []Role{RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RoutePosition,
		Handler:     a.HandlerGetPosition(),
		Roles:       []Role{RoleRecruiter, RoleCandidate},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPatch,
		Route:       RoutePosition,
		Handler:     a.HandlerPatchPosition(),
		Roles:       []Role{RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodDelete,
		Route:       RoutePosition,
		Handler:     a.HandlerDeletePosition(),
		Roles:       []Role{RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteRecommendations,
		Handler:     a.HandlerGetRecommendations(),
		Roles:       []Role{RoleCandidate, RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteRecommendation,
		Handler:     a.HandlerGetRecommendation(),
		Roles:       []Role{RoleCandidate, RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteReactions,
		Handler:     a.HandlerGetReactions(),
		Roles:       []Role{RoleCandidate, RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodPost,
		Route:       RouteReactions,
		Handler:     a.HandlerCreateReaction(),
		Roles:       []Role{RoleCandidate, RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PrivateRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteMatches,
		Handler:     a.HandlerGetMatches(),
		Roles:       []Role{RoleCandidate, RoleRecruiter},
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PublicRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteLocations,
		Handler:     a.HandlerSearchLocations(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})

	a.PublicRoute(RouteConfig{
		Method:      MethodGet,
		Route:       RouteLocation,
		Handler:     a.HandlerGetLocation(),
		Middlewares: []Middleware{a.MiddlewareAPIKey},
	})
}
