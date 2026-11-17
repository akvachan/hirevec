// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"time"
	"uuid"

	_ "embed"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type ProblemSource struct {
	Pointer   string `json:"pointer,omitempty"`   // Pointer is a field within the body
	Parameter string `json:"parameter,omitempty"` // Parameter is a query parameter within a URL
	Header    string `json:"header,omitempty"`    // Header is an HTTP header
	Cookie    string `json:"cookie,omitempty"`    // Cookie is an HTTP cookie
}

type Problem struct {
	Type   ProblemType    `json:"type,omitempty"`
	Detail string         `json:"detail,omitempty"`
	Source *ProblemSource `json:"source,omitempty"`
}

type RequestBodyRefresh struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

type RequestBodyPatchUser struct {
	FullName *string `json:"full_name"`
}

type RequestBodyPatchCandidate struct {
	About       *string                           `json:"about"`
	Experiences *[]RequestBodyCandidateExperience `json:"experiences"`
	RequestBodyCandidatePrefs
}

type RequestBodyCreatePosition struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Company     string `json:"company"`
	IsRemote    bool   `json:"is_remote"`
	LocationID  uint32 `json:"location_id"`
	Skill1      string `json:"skill_1"`
	Skill2      string `json:"skill_2"`
	Skill3      string `json:"skill_3"`
	Skill4      string `json:"skill_4"`
	Skill5      string `json:"skill_5"`
}

type RequestBodyPatchPosition struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Company     *string `json:"company"`
	Skill1      *string `json:"skill_1"`
	Skill2      *string `json:"skill_2"`
	Skill3      *string `json:"skill_3"`
	Skill4      *string `json:"skill_4"`
	Skill5      *string `json:"skill_5"`
	IsRemote    *bool   `json:"is_remote"`
	IsActive    *bool   `json:"is_active"`
	LocationID  *uint32 `json:"location_id"`
}

type RequestBodyCreateUser struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

type RequestBodyWebAuthnRegisterStart struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

type RequestBodyWebAuthnRegisterFinish struct {
	SessionID  uuid.UUID       `json:"session_id"`
	Credential json.RawMessage `json:"credential"`
}

type RequestBodyWebAuthnLoginFinish struct {
	SessionID  uuid.UUID       `json:"session_id"`
	Credential json.RawMessage `json:"credential"`
}

type RequestBodyCreateReaction struct {
	RecommendationID uuid.UUID    `json:"recommendation_id"`
	ReactionType     ReactionType `json:"reaction_type"`
}

type RequestBodyCandidatePrefs struct {
	PrefTitle1      *string `json:"pref_title_1"`
	PrefTitle2      *string `json:"pref_title_2"`
	PrefTitle3      *string `json:"pref_title_3"`
	PrefRemote      *bool   `json:"pref_remote"`
	PrefLocation1ID *uint32 `json:"pref_location_1_id"`
	PrefLocation2ID *uint32 `json:"pref_location_2_id"`
	PrefLocation3ID *uint32 `json:"pref_location_3_id"`
}

type RequestBodyCandidateExperience struct {
	Title          string    `json:"title"`
	StartedAt      time.Time `json:"started_at"`
	EndedAt        time.Time `json:"ended_at"`
	Description    string    `json:"description"`
	Company        string    `json:"company"`
	ExperienceType string    `json:"experience_type"`
	Skill1         string    `json:"skill_1"`
	Skill2         string    `json:"skill_2"`
	Skill3         string    `json:"skill_3"`
	Skill4         string    `json:"skill_4"`
	Skill5         string    `json:"skill_5"`
}

type RequestBodyCreateCandidate struct {
	About       string                           `json:"about"`
	Experiences []RequestBodyCandidateExperience `json:"experiences"`
	RequestBodyCandidatePrefs
}

type ResponseBodyHealth struct {
	Status string `json:"status"`
}

type ResponseBodyGetUser struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ResponseBodyWebAuthnRegisterStart struct {
	SessionID uuid.UUID                    `json:"session_id"`
	Options   *protocol.CredentialCreation `json:"options"`
}

type ResponseBodyWebAuthnLoginStart struct {
	SessionID uuid.UUID                     `json:"session_id"`
	Options   *protocol.CredentialAssertion `json:"options"`
}

type ResponseBodyGetCandidate struct {
	Candidate
	Experiences []CandidateExperience `json:"experiences"`
}

type ResponseWriter struct {
	http.ResponseWriter
	status int
}

type ContextKey string

const ContextKeyClaims ContextKey = "claims"

type ProblemType string

const (
	ProblemTypeRecruiterNotFound                ProblemType = "urn:hirevec:recruiter-not-found"
	ProblemTypeInvalidRecruiterID               ProblemType = "urn:hirevec:invalid-recruiter-id"
	ProblemTypeInvalidCandidateID               ProblemType = "urn:hirevec:invalid-candidate-id"
	ProblemTypeMissingRequiredRole              ProblemType = "urn:hirevec:missing-required-role"
	ProblemTypeCandidateNotFound                ProblemType = "urn:hirevec:candidate-not-found"
	ProblemTypeUserIDRequired                   ProblemType = "urn:hirevec:user-id-required"
	ProblemTypeInvalidUserID                    ProblemType = "urn:hirevec:invalid-user-id"
	ProblemTypeUserNotFound                     ProblemType = "urn:hirevec:user-not-found"
	ProblemTypeForbidden                        ProblemType = "urn:hirevec:forbidden"
	ProblemTypeCandidateAlreadyExists           ProblemType = "urn:hirevec:candiate-already-exists"
	ProblemTypeInvalidCandidateAboutFormat      ProblemType = "urn:hirevec:invalid-candidate-about-format"
	ProblemTypeInvalidCandidatePrefsFormat      ProblemType = "urn:hirevec:invalid-candidate-prefs-format"
	ProblemTypeInvalidCandidateExperienceFormat ProblemType = "urn:hirevec:invalid-candidate-experience-format"
	ProblemTypeRecruiterExists                  ProblemType = "urn:hirevec:recruiter-exists"
	ProblemTypeInvalidUserEmailFormat           ProblemType = "urn:hirevec:invalid-user-email-format"
	ProblemTypeInvalidUserFullNameFormat        ProblemType = "urn:hirevec:invalid-user-full-name-format"
	ProblemTypeUserAlreadyExists                ProblemType = "urn:hirevec:user-exists"
	ProblemTypeRecommendationIDRequired         ProblemType = "urn:hirevec:recommendation-id-required"
	ProblemTypeRecommendationNotFound           ProblemType = "urn:hirevec:recommendation-not-found"
	ProblemTypeReactionExists                   ProblemType = "urn:hirevec:reaction-exists"
	ProblemTypeInvalidReactionType              ProblemType = "urn:hirevec:invalid-reaction-type"
	ProblemTypeResourceIDRequired               ProblemType = "urn:hirevec:resource-id-required"
	ProblemTypeInvalidRecommendationID          ProblemType = "urn:hirevec:invalid-recommendation-id"
	ProblemTypeInvalidRequestBody               ProblemType = "urn:hirevec:invalid-request-body"
	ProblemTypeInvalidGrantType                 ProblemType = "urn:hirevec:invalid-grant-type"
	ProblemTypeRefreshTokenRequired             ProblemType = "urn:hirevec:refresh-token-required"
	ProblemTypeInvalidRefreshToken              ProblemType = "urn:hirevec:invalid-refresh-token"
	ProblemTypeBearerTokenRequired              ProblemType = "urn:hirevec:bearer-token-required"
	ProblemTypeAPIKeyRequired                   ProblemType = "urn:hirevec:api-key-required"
	ProblemTypeInvalidAPIKey                    ProblemType = "urn:hirevec:invalid-api-key"
	ProblemTypeInvalidAccessToken               ProblemType = "urn:hirevec:invalid-access-token"
	ProblemTypeUnauthorized                     ProblemType = "urn:hirevec:unauthorized"
	ProblemTypeInvalidIncludeReacted            ProblemType = "urn:hirevec:invalid-include-reacted"
	ProblemTypeInvalidLimit                     ProblemType = "urn:hirevec:invalid-limit"
	ProblemTypeInvalidRole                      ProblemType = "urn:hirevec:invalid-role"
	ProblemTypeInvalidPositionTitleFormat       ProblemType = "urn:hirevec:invalid-position-title-format"
	ProblemTypeInvalidPositionDescriptionFormat ProblemType = "urn:hirevec:invalid-position-description-format"
	ProblemTypeInvalidPositionCompanyFormat     ProblemType = "urn:hirevec:invalid-position-company-format"
	ProblemTypeInvalidPositionSkillFormat       ProblemType = "urn:hirevec:invalid-position-skill-format"
	ProblemTypeInvalidLocationCursor            ProblemType = "urn:hirevec:invalid-location-cursor"
	ProblemTypeLocationIDRequired               ProblemType = "urn:hirevec:location-id-required"
	ProblemTypeLocationNotFound                 ProblemType = "urn:hirevec:location-not-found"
	ProblemTypeEmailRequired                    ProblemType = "urn:hirevec:email-required"
	ProblemTypePositionNotFound                 ProblemType = "urn:hirevec:position-not-found"
	ProblemTypeInvalidPositionID                ProblemType = "urn:hirevec:invalid-position-id"
	ProblemTypeWebAuthnSessionNotFound          ProblemType = "urn:hirevec:webauthn-session-not-found"
	ProblemTypeInvalidWebAuthnCredential        ProblemType = "urn:hirevec:invalid-webauthn-credential"
	ProblemTypeWebAuthnCredentialExists         ProblemType = "urn:hirevec:webauthn-credential-exists"
)

const (
	DefaultPageSize                       uint8 = 32
	DefaultMaxPageSize                    uint8 = 255
	DefaultUserFullNameMinLength                = 2
	DefaultUserFullNameMaxLength                = 512
	DefaultPositionTitleMinLength               = 4
	DefaultPositionTitleMaxLength               = 64
	DefaultPositionDescriptionMinLength         = 0
	DefaultPositionDescriptionMaxLength         = 2048
	DefaultPositionCompanyMinLength             = 2
	DefaultPositionCompanyMaxLength             = 512
	DefaultCandidateAboutMinLength              = 0
	DefaultCandidateAboutMaxLength              = 1024
	DefaultSkillMinLength                       = 0
	DefaultSkillMaxLength                       = 128
	DefaultExperienceTitleMinLength             = 4
	DefaultExperienceTitleMaxLength             = 64
	DefaultExperienceDescriptionMinLength       = 0
	DefaultExperienceDescriptionMaxLength       = 1024
)

var (
	ErrInvalidRequestBody                      = errors.New("invalid request body")
	ErrInvalidGrantType                        = errors.New("grant_type must be refresh_token")
	ErrRefreshTokenRequired                    = errors.New("refresh_token is required")
	ErrInvalidRefreshToken                     = errors.New("invalid refresh_token")
	ErrEmailRequired                           = errors.New("email is required")
	ErrInvalidExperienceType                   = errors.New("invalid experience type")
	ErrInvalidExperienceDates                  = errors.New("started_at must not be after ended_at")
	ErrNoNextPageToLink                        = errors.New("failed to process page due to missing next page")
	ErrEmptyCursor                             = errors.New("failed to process page due to cursor being empty")
	ErrZeroLimit                               = errors.New("failed to process page due to limit being zero")
	ErrInvalidIncludeReacted                   = errors.New("include_reacted must be one of: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False")
	ErrMissingRequiredRoleRecruiter            = errors.New("missing required role: recruiter")
	ErrMissingRequiredRoleCandidate            = errors.New("missing required role: candidate")
	ErrMissingRequiredRoleRecruiterOrCandidate = errors.New("missing one of required roles: recruiter, candidate")
	ErrResourceIDRequired                      = errors.New("id is required")
	ErrInvalidResourceID                       = errors.New("invalid id")
	ErrExtraDataDecoded                        = errors.New("extra data decoded")
	ErrInvalidReactionType                     = errors.New("invalid reaction_type")
	ErrForbidden                               = errors.New("forbidden")
	ErrInvalidEmail                            = errors.New("invalid email")
	ErrInvalidLocationID                       = errors.New("invalid location id; must be positive integer")
	ErrInvalidPositionID                       = errors.New("invalid position_id")
	ErrInvalidLimit                            = fmt.Errorf("limit must be an integer between 1 and %d", DefaultMaxPageSize)
)

//go:embed openapi.yaml
var OpenAPISchema []byte

func GetClaims(r *http.Request) (AccessTokenClaims, bool) {
	claims, ok := r.Context().Value(ContextKeyClaims).(AccessTokenClaims)
	return claims, ok
}

func (a *API) PageFromQuery(w http.ResponseWriter, r *http.Request) (Page, bool) {
	page := Page{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  DefaultPageSize,
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidLimit,
				Detail: ErrInvalidLimit.Error(),
				Source: &ProblemSource{Parameter: "limit"},
			}, http.StatusBadRequest)
			return Page{}, false
		}
		page.Limit = uint8(min(parsed, int(DefaultMaxPageSize)))
	}

	return page, true
}

func DecodeRequestBody[T any](r *http.Request) (T, error) {
	var data T

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&data); err != nil {
		return data, err
	}
	if decoder.More() {
		return data, ErrExtraDataDecoded
	}

	if err := r.Body.Close(); err != nil {
		return data, fmt.Errorf("failed to close body: %w", err)
	}

	return data, nil
}

func JSON(w http.ResponseWriter, responseBody any, status int) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(responseBody); err != nil {
		slog.Error("failed to encode response", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Debug("failed to write response to client", "err", err)
	}
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (a *API) AddNextLink(w http.ResponseWriter, route Route, nextPage Page) error {
	if !nextPage.HasNext {
		return ErrNoNextPageToLink
	}
	if nextPage.Cursor == "" {
		return ErrEmptyCursor
	}
	if nextPage.Limit == 0 {
		return ErrZeroLimit
	}

	u := a.ServerURL.JoinPath(string(route))

	query := url.Values{}
	query.Set("cursor", nextPage.Cursor)
	query.Set("limit", strconv.Itoa(int(nextPage.Limit)))
	u.RawQuery = query.Encode()

	w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="next"`, u.String()))

	return nil
}

func (a *API) HandleNextLink(w http.ResponseWriter, route Route, nextPage Page) bool {
	err := a.AddNextLink(w, route, nextPage)
	if err == nil || errors.Is(err, ErrNoNextPageToLink) {
		return true
	}

	slog.Error("failed to add next link", "route", route, "err", err)
	w.WriteHeader(http.StatusInternalServerError)
	return false
}

func (a *API) LocationExistsOrRespond(w http.ResponseWriter, locationID uint32, pointer string) bool {
	if locationID == 0 {
		return true
	}

	exists, err := a.Store.LocationExists(int(locationID))
	if err != nil {
		slog.Error("failed to check location existence", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	if !exists {
		JSON(w, Problem{
			Type:   ProblemTypeLocationNotFound,
			Detail: ErrLocationNotFound.Error(),
			Source: &ProblemSource{Pointer: pointer},
		}, http.StatusBadRequest)
		return false
	}

	return true
}

func (a *API) HandleRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := DecodeRequestBody[RequestBodyRefresh](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		if body.GrantType != "refresh_token" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidGrantType,
				Detail: ErrInvalidGrantType.Error(),
				Source: &ProblemSource{Pointer: "/grant_type"},
			}, http.StatusBadRequest)
			return
		}

		if body.RefreshToken == "" {
			JSON(w, Problem{
				Type:   ProblemTypeRefreshTokenRequired,
				Detail: ErrRefreshTokenRequired.Error(),
				Source: &ProblemSource{Pointer: "/refresh_token"},
			}, http.StatusBadRequest)
			return
		}

		claims, err := a.Vault.ParseRefreshToken(body.RefreshToken)
		if err != nil {
			slog.Error(
				"failed to parse refresh token",
				"ip", a.ClientIP(r),
				"err", err,
			)

			JSON(w, Problem{
				Type:   ProblemTypeInvalidRefreshToken,
				Detail: ErrInvalidRefreshToken.Error(),
				Source: &ProblemSource{Pointer: "/refresh_token"},
			}, http.StatusBadRequest)
			return
		}

		roles, err := a.Store.GetUserRoles(claims.UserID)
		if err != nil {
			if errors.Is(err, ErrUserNoRole) {
				roles = map[Role]uuid.UUID{}
			} else if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidRefreshToken,
					Detail: ErrInvalidRefreshToken.Error(),
					Source: &ProblemSource{Pointer: "/refresh_token"},
				}, http.StatusBadRequest)
				return
			} else {
				slog.Error(
					"failed to get user roles",
					"user_id", claims.UserID,
					"ip", a.ClientIP(r),
					"err", err,
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		newJTI, err := a.Store.RotateRefreshToken(claims.UserID, claims.JTI)
		if err != nil {
			if errors.Is(err, ErrRefreshTokenReuse) {
				slog.Warn(
					"revoked token reuse attempt",
					"jti", claims.JTI,
					"user_id", claims.UserID,
					"ip", a.ClientIP(r),
				)
				JSON(w, Problem{
					Type:   ProblemTypeInvalidRefreshToken,
					Detail: ErrInvalidRefreshToken.Error(),
					Source: &ProblemSource{Pointer: "/meta/refresh_token"},
				}, http.StatusBadRequest)
				return
			}
			slog.Error(
				"failed to rotate refresh token",
				"err", err,
				"jti", claims.JTI,
				"user_id", claims.UserID,
				"ip", a.ClientIP(r),
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tokenPair, err := a.Vault.CreateTokenPair(claims.UserID, newJTI, roles)
		if err != nil {
			slog.Error("failed to create token pair", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, tokenPair, http.StatusOK)
	}
}

func (a *API) HandlerOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Length", strconv.Itoa(len(OpenAPISchema)))
	w.Write(OpenAPISchema)
}

func (a *API) HandlerDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src unpkg.com 'sha256-0F3vu4DV3bn1WDlNRkYeWPZeFoDcHJvUhyxRFYbnjt4='; style-src unpkg.com 'unsafe-inline'; img-src unpkg.com data:; font-src unpkg.com; connect-src 'self' unpkg.com; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(SwaggerUIHTML)))
	fmt.Fprint(w, SwaggerUIHTML)
}

func (a *API) HandlerPasskeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(PasskeysPageHTML)))
	fmt.Fprint(w, PasskeysPageHTML)
}

func (a *API) HandlerHealth(w http.ResponseWriter, r *http.Request) {
	JSON(w, ResponseBodyHealth{Status: "ok"}, http.StatusOK)
}

func (a *API) HandlerGetRecommendations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateID, isCandidate := claims.Roles[RoleCandidate]
		recruiterID, isRecruiter := claims.Roles[RoleRecruiter]

		roleStr := r.URL.Query().Get("role")
		role, err := StringToRole(roleStr)
		if err != nil && roleStr != "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRole,
				Detail: ErrInvalidRole.Error(),
				Source: &ProblemSource{Parameter: "role"},
			}, http.StatusBadRequest)
			return
		}

		includeReactedStr := r.URL.Query().Get("include_reacted")
		includeReacted, err := strconv.ParseBool(includeReactedStr)
		if err != nil && includeReactedStr != "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidIncludeReacted,
				Detail: ErrInvalidIncludeReacted.Error(),
				Source: &ProblemSource{Parameter: "include_reacted"},
			}, http.StatusBadRequest)
			return
		}

		if role == RoleRecruiter && !isRecruiter {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}
		if role == RoleCandidate && !isCandidate {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		if (role == RoleRecruiter || role == "") && isRecruiter {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			recommendations, nextPage, err := a.Store.GetRecommendationsForRecruiter(recruiterID, page, includeReacted)
			if err != nil {
				slog.Error("failed to fetch recommendations", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteRecommendations, nextPage) {
				return
			}

			JSON(w, recommendations, http.StatusOK)
			return

		} else if (role == RoleCandidate || role == "") && isCandidate {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			recommendations, nextPage, err := a.Store.GetRecommendationsForCandidate(candidateID, page, includeReacted)
			if err != nil {
				slog.Error("failed to fetch recommendations", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteRecommendations, nextPage) {
				return
			}

			JSON(w, recommendations, http.StatusOK)
			return

		} else {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiterOrCandidate.Error(),
			}, http.StatusForbidden)
			return
		}
	}
}

func (a *API) HandlerGetRecommendation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		recommendationIDStr := r.PathValue("id")
		if recommendationIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}
		recommendationID, err := uuid.Parse(recommendationIDStr)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRecommendationID,
				Detail: ErrInvalidResourceID.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		recommendation, err := a.Store.GetRecommendation(recommendationID, claims.UserID)
		if err != nil {
			if errors.Is(err, ErrRecommendationNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeRecommendationNotFound,
					Detail: ErrRecommendationNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to fetch recommendation", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, recommendation, http.StatusOK)
	}
}

func (a *API) HandlerCreateReaction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateID, isCandidate := claims.Roles[RoleCandidate]
		recruiterID, isRecruiter := claims.Roles[RoleRecruiter]

		body, err := DecodeRequestBody[RequestBodyCreateReaction](r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if body.RecommendationID == uuid.Nil() {
			JSON(w, Problem{
				Type:   ProblemTypeRecommendationIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}
		if !body.ReactionType.IsValid() {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidReactionType,
				Detail: ErrInvalidReactionType.Error(),
				Source: &ProblemSource{Pointer: "/reaction_type"},
			}, http.StatusBadRequest)
			return
		}

		var storeError error
		if isCandidate {
			storeError = a.Store.CreateCandidateReaction(body.RecommendationID, candidateID, body.ReactionType)
		} else if isRecruiter {
			storeError = a.Store.CreateRecruiterReaction(body.RecommendationID, recruiterID, body.ReactionType)
		} else {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiterOrCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		if storeError != nil {
			if errors.Is(storeError, ErrReactionAlreadyExists) {
				JSON(w, Problem{
					Type:   ProblemTypeReactionExists,
					Detail: ErrReactionAlreadyExists.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusConflict)
				return
			} else if errors.Is(storeError, ErrRecommendationNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeRecommendationNotFound,
					Detail: ErrRecommendationNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to create reaction", "err", storeError)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) HandlerGetReactions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateID, isCandidate := claims.Roles[RoleCandidate]
		recruiterID, isRecruiter := claims.Roles[RoleRecruiter]

		roleStr := r.URL.Query().Get("role")
		role, err := StringToRole(roleStr)
		if err != nil && roleStr != "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRole,
				Detail: ErrInvalidRole.Error(),
				Source: &ProblemSource{Parameter: "role"},
			}, http.StatusBadRequest)
			return
		}

		if role == RoleRecruiter && !isRecruiter {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}
		if role == RoleCandidate && !isCandidate {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		if (role == RoleRecruiter || role == "") && isRecruiter {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			reactions, nextPage, err := a.Store.GetReactionsForRecruiter(recruiterID, page)
			if err != nil {
				slog.Error("failed to fetch reactions", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteReactions, nextPage) {
				return
			}

			JSON(w, reactions, http.StatusOK)
			return

		} else if (role == RoleCandidate || role == "") && isCandidate {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			reactions, nextPage, err := a.Store.GetReactionsForCandidate(candidateID, page)
			if err != nil {
				slog.Error("failed to fetch reactions", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteReactions, nextPage) {
				return
			}

			JSON(w, reactions, http.StatusOK)
			return

		} else {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiterOrCandidate.Error(),
			}, http.StatusForbidden)
			return
		}
	}
}

func (a *API) HandlerGetMatches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		candidateID, isCandidate := claims.Roles[RoleCandidate]
		recruiterID, isRecruiter := claims.Roles[RoleRecruiter]

		roleStr := r.URL.Query().Get("role")
		role, err := StringToRole(roleStr)
		if err != nil && roleStr != "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRole,
				Detail: ErrInvalidRole.Error(),
				Source: &ProblemSource{Parameter: "role"},
			}, http.StatusBadRequest)
			return
		}

		if role == RoleRecruiter && !isRecruiter {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}
		if role == RoleCandidate && !isCandidate {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		if (role == RoleRecruiter || role == "") && isRecruiter {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			matches, nextPage, err := a.Store.GetMatchesForRecruiter(recruiterID, page)
			if err != nil {
				slog.Error("failed to fetch matches", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteMatches, nextPage) {
				return
			}

			JSON(w, matches, http.StatusOK)
			return

		} else if (role == RoleCandidate || role == "") && isCandidate {
			page, ok := a.PageFromQuery(w, r)
			if !ok {
				return
			}
			matches, nextPage, err := a.Store.GetMatchesForCandidate(candidateID, page)
			if err != nil {
				slog.Error("failed to fetch matches", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !a.HandleNextLink(w, RouteMatches, nextPage) {
				return
			}

			JSON(w, matches, http.StatusOK)
			return

		} else {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiterOrCandidate.Error(),
			}, http.StatusForbidden)
			return
		}
	}
}

func (a *API) HandlerCreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := DecodeRequestBody[RequestBodyCreateUser](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		if body.Email == "" {
			JSON(w, Problem{
				Type:   ProblemTypeEmailRequired,
				Detail: ErrEmailRequired.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusBadRequest)
			return
		}

		email, err := mail.ParseAddress(body.Email)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidUserEmailFormat,
				Detail: ErrInvalidEmail.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusBadRequest)
			return
		}

		fullName, err := NormalizeAndValidateText(body.FullName, "full_name", DefaultUserFullNameMinLength, DefaultUserFullNameMaxLength, true)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidUserFullNameFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/full_name"},
			}, http.StatusBadRequest)
			return
		}

		exists, err := a.Store.UserExistsByEmail(email.Address)
		if err != nil {
			slog.Error("failed to check user existance", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if exists {
			JSON(w, Problem{
				Type:   ProblemTypeUserAlreadyExists,
				Detail: ErrUserAlreadyExists.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusConflict)
			return
		}

		userID, err := a.Store.CreateUser(email.Address, fullName)
		if err != nil {
			slog.Error("failed to create user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jti, err := a.Store.CreateRefreshToken(userID)
		if err != nil {
			slog.Error("failed to create refresh token", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tokenPair, err := a.Vault.CreateTokenPair(userID, jti, map[Role]uuid.UUID{})
		if err != nil {
			slog.Error("failed to create token pair", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, tokenPair, http.StatusCreated)
	}
}

func (a *API) HandlerWebAuthnRegisterStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := DecodeRequestBody[RequestBodyWebAuthnRegisterStart](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		if body.Email == "" {
			JSON(w, Problem{
				Type:   ProblemTypeEmailRequired,
				Detail: ErrEmailRequired.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusBadRequest)
			return
		}

		email, err := mail.ParseAddress(body.Email)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidUserEmailFormat,
				Detail: ErrInvalidEmail.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusBadRequest)
			return
		}

		fullName, err := NormalizeAndValidateText(body.FullName, "full_name", DefaultUserFullNameMinLength, DefaultUserFullNameMaxLength, true)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidUserFullNameFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/full_name"},
			}, http.StatusBadRequest)
			return
		}

		exists, err := a.Store.UserExistsByEmail(email.Address)
		if err != nil {
			slog.Error("failed to check user existance", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if exists {
			JSON(w, Problem{
				Type:   ProblemTypeUserAlreadyExists,
				Detail: ErrUserAlreadyExists.Error(),
				Source: &ProblemSource{Pointer: "/email"},
			}, http.StatusConflict)
			return
		}

		userID, err := a.Store.CreateUser(email.Address, fullName)
		if err != nil {
			slog.Error("failed to create user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		user := WebAuthnUser{
			ID:       userID,
			Email:    email.Address,
			FullName: fullName,
		}

		options, session, err := a.WebAuthn.BeginRegistration(
			user,
			webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
				ResidentKey:      protocol.ResidentKeyRequirementRequired,
				UserVerification: protocol.VerificationPreferred,
			}),
		)
		if err != nil {
			slog.Error("failed to begin webauthn registration", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessionID, err := a.Store.CreateWebAuthnSession(WebAuthnSessionTypeRegister, userID, email.Address, fullName, *session, DefaultWebAuthnSessionTTL)
		if err != nil {
			slog.Error("failed to create webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, ResponseBodyWebAuthnRegisterStart{
			SessionID: sessionID,
			Options:   options,
		}, http.StatusCreated)
	}
}

func (a *API) HandlerWebAuthnRegisterFinish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := DecodeRequestBody[RequestBodyWebAuthnRegisterFinish](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		session, err := a.Store.GetWebAuthnSession(body.SessionID)
		if err != nil {
			if errors.Is(err, ErrWebAuthnSessionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeWebAuthnSessionNotFound,
					Detail: ErrWebAuthnSessionNotFound.Error(),
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if session.Type != WebAuthnSessionTypeRegister {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		if time.Now().UTC().After(session.ExpiresAt) {
			JSON(w, Problem{
				Type:   ProblemTypeWebAuthnSessionNotFound,
				Detail: ErrWebAuthnSessionNotFound.Error(),
			}, http.StatusNotFound)
			return
		}

		user, err := a.Store.GetUser(session.UserID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(body.Credential))
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidWebAuthnCredential,
				Detail: err.Error(),
			}, http.StatusBadRequest)
			return
		}

		credential, err := a.WebAuthn.CreateCredential(WebAuthnUser{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
		}, session.SessionData, parsedResponse)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidWebAuthnCredential,
				Detail: err.Error(),
			}, http.StatusBadRequest)
			return
		}

		if err := a.Store.CreateWebAuthnCredential(session.UserID, *credential); err != nil {
			if errors.Is(err, ErrWebAuthnCredentialExists) {
				JSON(w, Problem{
					Type:   ProblemTypeWebAuthnCredentialExists,
					Detail: ErrWebAuthnCredentialExists.Error(),
				}, http.StatusConflict)
				return
			}
			slog.Error("failed to create webauthn credential", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := a.Store.DeleteWebAuthnSession(body.SessionID); err != nil {
			slog.Error("failed to delete webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jti, err := a.Store.CreateRefreshToken(session.UserID)
		if err != nil {
			slog.Error("failed to create refresh token", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tokenPair, err := a.Vault.CreateTokenPair(session.UserID, jti, map[Role]uuid.UUID{})
		if err != nil {
			slog.Error("failed to create token pair", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, tokenPair, http.StatusCreated)
	}
}

func (a *API) HandlerWebAuthnLoginStart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		options, session, err := a.WebAuthn.BeginDiscoverableLogin(
			webauthn.WithUserVerification(protocol.VerificationPreferred),
		)
		if err != nil {
			slog.Error("failed to begin webauthn login", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		sessionID, err := a.Store.CreateWebAuthnSession(WebAuthnSessionTypeLogin, uuid.Nil(), "", "", *session, DefaultWebAuthnSessionTTL)
		if err != nil {
			slog.Error("failed to create webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, ResponseBodyWebAuthnLoginStart{
			SessionID: sessionID,
			Options:   options,
		}, http.StatusOK)
	}
}

func (a *API) HandlerWebAuthnLoginFinish() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := DecodeRequestBody[RequestBodyWebAuthnLoginFinish](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		session, err := a.Store.GetWebAuthnSession(body.SessionID)
		if err != nil {
			if errors.Is(err, ErrWebAuthnSessionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeWebAuthnSessionNotFound,
					Detail: ErrWebAuthnSessionNotFound.Error(),
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if session.Type != WebAuthnSessionTypeLogin {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		if time.Now().UTC().After(session.ExpiresAt) {
			JSON(w, Problem{
				Type:   ProblemTypeWebAuthnSessionNotFound,
				Detail: ErrWebAuthnSessionNotFound.Error(),
			}, http.StatusNotFound)
			return
		}

		parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body.Credential))
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidWebAuthnCredential,
				Detail: err.Error(),
			}, http.StatusBadRequest)
			return
		}

		validatedUser, validatedCredential, err := a.WebAuthn.ValidatePasskeyLogin(a.Store.LoadWebAuthnUser, session.SessionData, parsedResponse)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidWebAuthnCredential,
				Detail: err.Error(),
			}, http.StatusUnauthorized)
			return
		}

		waUser, ok := validatedUser.(WebAuthnUser)
		if !ok {
			slog.Error("unexpected webauthn user implementation")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := a.Store.UpdateWebAuthnCredential(*validatedCredential); err != nil {
			if errors.Is(err, ErrWebAuthnCredentialNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidWebAuthnCredential,
					Detail: ErrWebAuthnCredentialNotFound.Error(),
				}, http.StatusUnauthorized)
				return
			}
			slog.Error("failed to update webauthn credential", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := a.Store.DeleteWebAuthnSession(body.SessionID); err != nil {
			slog.Error("failed to delete webauthn session", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		roles, err := a.Store.GetUserRoles(waUser.ID)
		if err != nil {
			if errors.Is(err, ErrUserNoRole) {
				roles = map[Role]uuid.UUID{}
			} else if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
				}, http.StatusNotFound)
				return
			} else {
				slog.Error("failed to get user roles", "user_id", waUser.ID, "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		jti, err := a.Store.CreateRefreshToken(waUser.ID)
		if err != nil {
			slog.Error("failed to create refresh token", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		tokenPair, err := a.Vault.CreateTokenPair(waUser.ID, jti, roles)
		if err != nil {
			slog.Error("failed to create token pair", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, tokenPair, http.StatusOK)
	}
}

func (a *API) HandlerCreateRecruiter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if _, err := a.Store.CreateRecruiter(claims.UserID); err != nil {
			if errors.Is(err, ErrRecruiterExists) {
				JSON(w, Problem{
					Type:   ProblemTypeRecruiterExists,
					Detail: ErrRecruiterExists.Error(),
				}, http.StatusConflict)
				return
			}
			slog.Error("failed to create recruiter", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		roles, err := a.Store.GetUserRoles(claims.UserID)
		if err != nil {
			slog.Error("failed to get user roles", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		accessToken, err := a.Vault.CreateAccessToken(claims.UserID, roles)
		if err != nil {
			slog.Error("failed to create access token", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, accessToken, http.StatusCreated)
	}
}

func (a *API) HandlerCreateCandidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		body, err := DecodeRequestBody[RequestBodyCreateCandidate](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		about, err := NormalizeAndValidateText(body.About, "about", DefaultCandidateAboutMinLength, DefaultCandidateAboutMaxLength, false)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidCandidateAboutFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/about"},
			}, http.StatusBadRequest)
			return
		}

		experiences, err := NormalizeAndValidateCandidateExperiences(body.Experiences)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidCandidateExperienceFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/experiences"},
			}, http.StatusBadRequest)
			return
		}

		candidate := Candidate{
			UserID: claims.UserID,
			About:  about,
		}

		if body.PrefRemote != nil {
			candidate.PrefRemote = body.PrefRemote
		}

		if body.PrefTitle1 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle1, "pref_title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_1"},
				}, http.StatusBadRequest)
				return
			}
			candidate.PrefTitle1 = title
		}

		if body.PrefTitle2 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle2, "pref_title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_2"},
				}, http.StatusBadRequest)
				return
			}
			candidate.PrefTitle2 = title
		}

		if body.PrefTitle3 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle3, "pref_title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_3"},
				}, http.StatusBadRequest)
				return
			}
			candidate.PrefTitle3 = title
		}

		prefLocations := []struct {
			id    *uint32
			field string
		}{
			{body.PrefLocation1ID, "/pref_location_1_id"},
			{body.PrefLocation2ID, "/pref_location_2_id"},
			{body.PrefLocation3ID, "/pref_location_3_id"},
		}
		for _, prefLocation := range prefLocations {
			if prefLocation.id == nil {
				continue
			}
			if !a.LocationExistsOrRespond(w, *prefLocation.id, prefLocation.field) {
				return
			}
		}

		if body.PrefLocation1ID != nil {
			candidate.PrefLocation1ID = *body.PrefLocation1ID
		}
		if body.PrefLocation2ID != nil {
			candidate.PrefLocation2ID = *body.PrefLocation2ID
		}
		if body.PrefLocation3ID != nil {
			candidate.PrefLocation3ID = *body.PrefLocation3ID
		}

		candidateID, err := a.Store.CreateCandidate(candidate, experiences)
		if err != nil {
			if errors.Is(err, ErrCandidateAlreadyExists) {
				JSON(w, Problem{
					Type:   ProblemTypeCandidateAlreadyExists,
					Detail: ErrCandidateAlreadyExists.Error(),
				}, http.StatusConflict)
				return
			}
			slog.Error("failed to create candidate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Create cold-start recommendations
		positionIDs, err := a.Store.GetPositionsForCandidateViaFTS(candidateID, DefaultTopPositions)
		if err != nil {
			slog.Error("failed to find positions for candidate via FTS", "candidateID", candidateID, "err", err)
		} else {
			limit := min(len(positionIDs), int(DefaultDailyRecommendations))
			for i := range limit {
				if _, err := a.Store.CreateRecommendation(positionIDs[i], candidateID); err != nil && !errors.Is(err, ErrRecommendationAlreadyExists) {
					slog.Error("failed to create cold start recommendation", "candidateID", candidateID, "err", err)
					break
				}
			}
		}

		roles, err := a.Store.GetUserRoles(claims.UserID)
		if err != nil {
			slog.Error("failed to get user roles", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		accessToken, err := a.Vault.CreateAccessToken(claims.UserID, roles)
		if err != nil {
			slog.Error("failed to create access token", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		JSON(w, accessToken, http.StatusCreated)
	}
}

func (a *API) HandlerGetUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userIDStr := r.PathValue("id")
		if userIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		userID := claims.UserID
		if userIDStr != "me" {
			var err error
			userID, err = uuid.Parse(userIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidUserID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if userID != claims.UserID {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		user, err := a.Store.GetUser(userID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, ResponseBodyGetUser{
			ID:        userID,
			Email:     user.Email,
			FullName:  user.FullName,
			UpdatedAt: user.UpdatedAt,
		}, http.StatusOK)
	}
}

func (a *API) HandlerPatchUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userIDStr := r.PathValue("id")
		if userIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		userID := claims.UserID
		if userIDStr != "me" {
			var err error
			userID, err = uuid.Parse(userIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidUserID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if userID != claims.UserID {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		body, err := DecodeRequestBody[RequestBodyPatchUser](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		user, err := a.Store.GetUser(userID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		changed := false
		if body.FullName != nil {
			fullName, err := NormalizeAndValidateText(*body.FullName, "full_name", DefaultUserFullNameMinLength, DefaultUserFullNameMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidUserFullNameFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/full_name"},
				}, http.StatusBadRequest)
				return
			}
			if user.FullName != fullName {
				user.FullName = fullName
				changed = true
			}
		}

		if !changed {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err = a.Store.UpdateUser(user.ID, user.FullName); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to update user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userIDStr := r.PathValue("id")
		if userIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		userID := claims.UserID
		if userIDStr != "me" {
			var err error
			userID, err = uuid.Parse(userIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidUserID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if userID != claims.UserID {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		if err := a.Store.DeleteUser(userID); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeUserNotFound,
					Detail: ErrUserNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to delete user", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerGetCandidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateIDClaims, isCandidate := claims.Roles[RoleCandidate]
		_, isRecruiter := claims.Roles[RoleRecruiter]

		candidateIDStr := r.PathValue("id")
		if candidateIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		var candidateID uuid.UUID
		if candidateIDStr == "me" {
			if !isCandidate {
				JSON(w, Problem{
					Type:   ProblemTypeMissingRequiredRole,
					Detail: ErrMissingRequiredRoleCandidate.Error(),
				}, http.StatusForbidden)
				return
			}
			candidateID = candidateIDClaims
		} else {
			parsedID, err := uuid.Parse(candidateIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidateID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if isCandidate && parsedID != candidateIDClaims {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
			if !isCandidate && !isRecruiter {
				JSON(w, Problem{
					Type:   ProblemTypeMissingRequiredRole,
					Detail: ErrMissingRequiredRoleCandidate.Error(),
				}, http.StatusForbidden)
				return
			}

			candidateID = parsedID
		}

		candidate, err := a.Store.GetCandidate(candidateID)
		if err != nil {
			if errors.Is(err, ErrCandidateNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeCandidateNotFound,
					Detail: ErrCandidateNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get candidate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		experiences, err := a.Store.GetCandidateExperiences(candidateID)
		if err != nil {
			slog.Error("failed to get candidate experiences", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, ResponseBodyGetCandidate{
			Candidate:   candidate,
			Experiences: experiences,
		}, http.StatusOK)
	}
}

func (a *API) HandlerPatchCandidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateIDClaims, ok := claims.Roles[RoleCandidate]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		candidateIDStr := r.PathValue("id")
		if candidateIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		candidateID := candidateIDClaims
		if candidateIDStr != "me" {
			var err error
			candidateID, err = uuid.Parse(candidateIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidateID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if candidateID != candidateIDClaims {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		body, err := DecodeRequestBody[RequestBodyPatchCandidate](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		candidate, err := a.Store.GetCandidate(candidateID)
		if err != nil {
			if errors.Is(err, ErrCandidateNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeCandidateNotFound,
					Detail: ErrCandidateNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get candidate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		changed := false

		var experiences []CandidateExperience
		if body.Experiences != nil {
			experiences, err = NormalizeAndValidateCandidateExperiences(*body.Experiences)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidateExperienceFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/experiences"},
				}, http.StatusBadRequest)
				return
			}
			changed = true
		}

		if body.About != nil {
			about, err := NormalizeAndValidateText(*body.About, "about", DefaultCandidateAboutMinLength, DefaultCandidateAboutMaxLength, false)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidateAboutFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/about"},
				}, http.StatusBadRequest)
				return
			}
			if candidate.About != about {
				candidate.About = about
				changed = true
			}
		}

		if body.PrefRemote != nil {
			if candidate.PrefRemote == nil || *candidate.PrefRemote != *body.PrefRemote {
				candidate.PrefRemote = body.PrefRemote
				changed = true
			}
		}
		if body.PrefTitle1 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle1, "title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_1"},
				}, http.StatusBadRequest)
				return
			}
			if candidate.PrefTitle1 != title {
				candidate.PrefTitle1 = title
				changed = true
			}
		}
		if body.PrefTitle2 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle2, "title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_2"},
				}, http.StatusBadRequest)
				return
			}
			if candidate.PrefTitle2 != title {
				candidate.PrefTitle2 = title
				changed = true
			}
		}
		if body.PrefTitle3 != nil {
			title, err := NormalizeAndValidateText(*body.PrefTitle3, "title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidatePrefsFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/pref_title_3"},
				}, http.StatusBadRequest)
				return
			}
			if candidate.PrefTitle3 != title {
				candidate.PrefTitle3 = title
				changed = true
			}
		}

		if body.PrefLocation1ID != nil {
			if !a.LocationExistsOrRespond(w, *body.PrefLocation1ID, "/pref_location_1_id") {
				return
			}
			if candidate.PrefLocation1ID != *body.PrefLocation1ID {
				candidate.PrefLocation1ID = *body.PrefLocation1ID
				changed = true
			}
		}
		if body.PrefLocation2ID != nil {
			if !a.LocationExistsOrRespond(w, *body.PrefLocation2ID, "/pref_location_2_id") {
				return
			}
			if candidate.PrefLocation2ID != *body.PrefLocation2ID {
				candidate.PrefLocation2ID = *body.PrefLocation2ID
				changed = true
			}
		}
		if body.PrefLocation3ID != nil {
			if !a.LocationExistsOrRespond(w, *body.PrefLocation3ID, "/pref_location_3_id") {
				return
			}
			if candidate.PrefLocation3ID != *body.PrefLocation3ID {
				candidate.PrefLocation3ID = *body.PrefLocation3ID
				changed = true
			}
		}

		if !changed {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err = a.Store.UpdateCandidate(candidate, experiences); err != nil {
			if errors.Is(err, ErrCandidateNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeCandidateNotFound,
					Detail: ErrCandidateNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to update candidate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerDeleteCandidate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		candidateIDClaims, ok := claims.Roles[RoleCandidate]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleCandidate.Error(),
			}, http.StatusForbidden)
			return
		}

		candidateIDStr := r.PathValue("id")
		if candidateIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		candidateID := candidateIDClaims
		if candidateIDStr != "me" {
			var err error
			candidateID, err = uuid.Parse(candidateIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidCandidateID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if candidateID != candidateIDClaims {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		if err := a.Store.DeleteCandidate(candidateID); err != nil {
			if errors.Is(err, ErrCandidateNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeCandidateNotFound,
					Detail: ErrCandidateNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to delete candidate", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerGetRecruiter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		recruiterIDClaims, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		recruiterIDStr := r.PathValue("id")
		if recruiterIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		recruiterID := recruiterIDClaims
		if recruiterIDStr != "me" {
			var err error
			recruiterID, err = uuid.Parse(recruiterIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidRecruiterID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if recruiterID != recruiterIDClaims {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		recruiter, err := a.Store.GetRecruiter(recruiterID)
		if err != nil {
			if errors.Is(err, ErrRecruiterNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeRecruiterNotFound,
					Detail: "Recruiter not found",
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to get recruiter", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, recruiter, http.StatusOK)
	}
}

func (a *API) HandlerDeleteRecruiter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		recruiterIDClaims, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		recruiterIDStr := r.PathValue("id")
		if recruiterIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		recruiterID := recruiterIDClaims
		if recruiterIDStr != "me" {
			var err error
			recruiterID, err = uuid.Parse(recruiterIDStr)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidRecruiterID,
					Detail: ErrInvalidResourceID.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusBadRequest)
				return
			}

			if recruiterID != recruiterIDClaims {
				JSON(w, Problem{
					Type:   ProblemTypeForbidden,
					Detail: ErrForbidden.Error(),
				}, http.StatusForbidden)
				return
			}
		}

		if err := a.Store.DeleteRecruiter(recruiterID); err != nil {
			if errors.Is(err, ErrRecruiterNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeRecruiterNotFound,
					Detail: "Recruiter not found",
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to delete recruiter", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerCreatePosition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		recruiterID, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		body, err := DecodeRequestBody[RequestBodyCreatePosition](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		title, err := NormalizeAndValidateText(body.Title, "title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionTitleFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/title"},
			}, http.StatusBadRequest)
			return
		}
		description, err := NormalizeAndValidateText(body.Description, "description", DefaultPositionDescriptionMinLength, DefaultPositionDescriptionMaxLength, false)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionDescriptionFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/description"},
			}, http.StatusBadRequest)
			return
		}
		company, err := NormalizeAndValidateText(body.Company, "company", DefaultPositionCompanyMinLength, DefaultPositionCompanyMaxLength, true)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionCompanyFormat,
				Detail: err.Error(),
				Source: &ProblemSource{Pointer: "/company"},
			}, http.StatusBadRequest)
			return
		}
		skills := []string{body.Skill1, body.Skill2, body.Skill3, body.Skill4, body.Skill5}
		for i := range skills {
			skill, err := NormalizeAndValidateText(skills[i], "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: fmt.Sprintf("/skill_%d", i+1)},
				}, http.StatusBadRequest)
				return
			}
			skills[i] = skill
		}

		if body.LocationID != 0 {
			exists, err := a.Store.LocationExists(int(body.LocationID))
			if err != nil {
				slog.Error("failed to check location existence", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if !exists {
				JSON(w, Problem{
					Type:   ProblemTypeLocationNotFound,
					Detail: ErrLocationNotFound.Error(),
					Source: &ProblemSource{Parameter: "location_id"},
				}, http.StatusBadRequest)
				return
			}
		}

		positionID, err := a.Store.CreatePosition(Position{
			RecruiterID: recruiterID,
			Title:       title,
			Description: description,
			Company:     company,
			IsRemote:    body.IsRemote,
			LocationID:  body.LocationID,
			Skill1:      skills[0],
			Skill2:      skills[1],
			Skill3:      skills[2],
			Skill4:      skills[3],
			Skill5:      skills[4],
			IsActive:    true,
		})
		if err != nil {
			slog.Error("failed to create position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Create cold-start recommendations
		candidateIDs, err := a.Store.GetCandidatesForPositionViaFTS(positionID, DefaultDailyRecommendations)
		if err != nil {
			slog.Error("failed to find candidates for position via FTS", "positionID", positionID, "err", err)
		} else {
			limit := min(len(candidateIDs), int(DefaultDailyRecommendations))
			for i := range limit {
				if _, err := a.Store.CreateRecommendation(positionID, candidateIDs[i]); err != nil && !errors.Is(err, ErrRecommendationAlreadyExists) {
					slog.Error("failed to create cold start recommendation", "positionID", positionID, "err", err)
					break
				}
			}
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) HandlerGetPositions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		recruiterID, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		requestedPage, ok := a.PageFromQuery(w, r)
		if !ok {
			return
		}

		positions, page, err := a.Store.GetPositions(recruiterID, requestedPage)
		if err != nil {
			slog.Error("failed to fetch positions", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !a.HandleNextLink(w, RoutePositions, page) {
			return
		}

		JSON(w, positions, http.StatusOK)
	}
}

func (a *API) HandlerSearchLocations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")

		requestedPage, ok := a.PageFromQuery(w, r)
		if !ok {
			return
		}

		locations, page, err := a.Store.SearchLocations(query, requestedPage)
		if err != nil {
			if errors.Is(err, ErrInvalidLocationCursor) {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidLocationCursor,
					Detail: ErrInvalidLocationCursor.Error(),
					Source: &ProblemSource{Parameter: "cursor"},
				}, http.StatusBadRequest)
				return
			}
			slog.Error("failed to fetch locations", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !a.HandleNextLink(w, RouteLocations, page) {
			return
		}

		JSON(w, locations, http.StatusOK)
	}
}

func (a *API) HandlerGetLocation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		locationIDStr := r.PathValue("id")
		if locationIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeLocationIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		locationID, err := strconv.Atoi(locationIDStr)
		if err != nil || locationID < 0 {
			JSON(w, Problem{
				Type:   ProblemTypeLocationIDRequired,
				Detail: ErrInvalidLocationID.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		location, err := a.Store.GetLocation(locationID)
		if err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypeLocationNotFound,
					Detail: ErrLocationNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to fetch location", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, location, http.StatusOK)
	}
}

func (a *API) HandlerGetPosition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		positionIDStr := r.PathValue("id")
		if positionIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionID,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		positionID, err := uuid.Parse(positionIDStr)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionID,
				Detail: ErrInvalidPositionID.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		position, err := a.Store.GetPosition(positionID)
		if err != nil {
			if errors.Is(err, ErrPositionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypePositionNotFound,
					Detail: ErrPositionNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to fetch position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		JSON(w, position, http.StatusOK)
	}
}

func (a *API) HandlerPatchPosition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		recruiterID, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		positionIDStr := r.PathValue("id")
		if positionIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeResourceIDRequired,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		positionID, err := uuid.Parse(positionIDStr)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionID,
				Detail: ErrInvalidPositionID.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		body, err := DecodeRequestBody[RequestBodyPatchPosition](r)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidRequestBody,
				Detail: ErrInvalidRequestBody.Error(),
			}, http.StatusBadRequest)
			return
		}

		position, err := a.Store.GetPosition(positionID)
		if err != nil {
			if errors.Is(err, ErrPositionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypePositionNotFound,
					Detail: ErrPositionNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to fetch position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if position.RecruiterID != recruiterID {
			JSON(w, Problem{
				Type:   ProblemTypeForbidden,
				Detail: ErrForbidden.Error(),
			}, http.StatusForbidden)
			return
		}

		changed := false
		if body.Title != nil {
			title, err := NormalizeAndValidateText(*body.Title, "title", DefaultPositionTitleMinLength, DefaultPositionTitleMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionTitleFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/title"},
				}, http.StatusBadRequest)
				return
			}
			if position.Title != title {
				position.Title = title
				changed = true
			}
		}

		if body.Description != nil {
			description, err := NormalizeAndValidateText(*body.Description, "description", DefaultPositionDescriptionMinLength, DefaultPositionDescriptionMaxLength, false)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionDescriptionFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/description"},
				}, http.StatusBadRequest)
				return
			}
			if position.Description != description {
				position.Description = description
				changed = true
			}
		}

		if body.Company != nil {
			company, err := NormalizeAndValidateText(*body.Company, "company", DefaultPositionCompanyMinLength, DefaultPositionCompanyMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionCompanyFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/company"},
				}, http.StatusBadRequest)
				return
			}
			if position.Company != company {
				position.Company = company
				changed = true
			}
		}

		if body.IsRemote != nil {
			if position.IsRemote != *body.IsRemote {
				position.IsRemote = *body.IsRemote
				changed = true
			}
		}

		if body.LocationID != nil {
			if *body.LocationID != 0 {
				exists, err := a.Store.LocationExists(int(*body.LocationID))
				if err != nil {
					slog.Error("failed to check location existence", "err", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if !exists {
					JSON(w, Problem{
						Type:   ProblemTypeLocationNotFound,
						Detail: ErrLocationNotFound.Error(),
						Source: &ProblemSource{Parameter: "location_id"},
					}, http.StatusBadRequest)
					return
				}
			}
			if position.LocationID != *body.LocationID {
				position.LocationID = *body.LocationID
				changed = true
			}
		}

		if body.Skill1 != nil {
			skill, err := NormalizeAndValidateText(*body.Skill1, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/skill_1"},
				}, http.StatusBadRequest)
				return
			}
			if position.Skill1 != skill {
				position.Skill1 = skill
				changed = true
			}
		}

		if body.Skill2 != nil {
			skill, err := NormalizeAndValidateText(*body.Skill2, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/skill_2"},
				}, http.StatusBadRequest)
				return
			}
			if position.Skill2 != skill {
				position.Skill2 = skill
				changed = true
			}
		}

		if body.Skill3 != nil {
			skill, err := NormalizeAndValidateText(*body.Skill3, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/skill_3"},
				}, http.StatusBadRequest)
				return
			}
			if position.Skill3 != skill {
				position.Skill3 = skill
				changed = true
			}
		}

		if body.Skill4 != nil {
			skill, err := NormalizeAndValidateText(*body.Skill4, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/skill_4"},
				}, http.StatusBadRequest)
				return
			}
			if position.Skill4 != skill {
				position.Skill4 = skill
				changed = true
			}
		}

		if body.Skill5 != nil {
			skill, err := NormalizeAndValidateText(*body.Skill5, "skill", DefaultSkillMinLength, DefaultSkillMaxLength, true)
			if err != nil {
				JSON(w, Problem{
					Type:   ProblemTypeInvalidPositionSkillFormat,
					Detail: err.Error(),
					Source: &ProblemSource{Pointer: "/skill_5"},
				}, http.StatusBadRequest)
				return
			}
			if position.Skill5 != skill {
				position.Skill5 = skill
				changed = true
			}
		}

		if body.IsActive != nil {
			if position.IsActive != *body.IsActive {
				position.IsActive = *body.IsActive
				changed = true
			}
		}

		if !changed {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err = a.Store.UpdatePosition(position); err != nil {
			if errors.Is(err, ErrPositionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypePositionNotFound,
					Detail: ErrPositionNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to update position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *API) HandlerDeletePosition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := GetClaims(r)
		if !ok {
			slog.Error("failed to access claims")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		recruiterID, ok := claims.Roles[RoleRecruiter]
		if !ok {
			JSON(w, Problem{
				Type:   ProblemTypeMissingRequiredRole,
				Detail: ErrMissingRequiredRoleRecruiter.Error(),
			}, http.StatusForbidden)
			return
		}

		positionIDStr := r.PathValue("id")
		if positionIDStr == "" {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionID,
				Detail: ErrResourceIDRequired.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		positionID, err := uuid.Parse(positionIDStr)
		if err != nil {
			JSON(w, Problem{
				Type:   ProblemTypeInvalidPositionID,
				Detail: ErrInvalidPositionID.Error(),
				Source: &ProblemSource{Parameter: "id"},
			}, http.StatusBadRequest)
			return
		}

		position, err := a.Store.GetPosition(positionID)
		if err != nil {
			if errors.Is(err, ErrPositionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypePositionNotFound,
					Detail: ErrPositionNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to fetch position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if position.RecruiterID != recruiterID {
			JSON(w, Problem{
				Type:   ProblemTypeForbidden,
				Detail: ErrForbidden.Error(),
			}, http.StatusForbidden)
			return
		}

		err = a.Store.DeletePosition(positionID)
		if err != nil {
			if errors.Is(err, ErrPositionNotFound) {
				JSON(w, Problem{
					Type:   ProblemTypePositionNotFound,
					Detail: ErrPositionNotFound.Error(),
					Source: &ProblemSource{Parameter: "id"},
				}, http.StatusNotFound)
				return
			}
			slog.Error("failed to delete position", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
