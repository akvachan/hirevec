// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"uuid"

	"github.com/go-webauthn/webauthn/webauthn"
	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"
)

type Store struct {
	Database *sql.DB
}

type StoreConfig struct {
	LlamaCppEmbeddingsDim uint32
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Reaction struct {
	ReactedAt        time.Time    `json:"reacted_at"`
	RecommendationID uuid.UUID    `json:"recommendation_id"`
	ReactionType     ReactionType `json:"reaction_type"`
}

type Recommendation struct {
	ID          uuid.UUID `json:"id"`
	PositionID  uuid.UUID `json:"position_id"`
	CandidateID uuid.UUID `json:"candidate_id"`
}

type RecommendationForCandidate struct {
	PositionSkills      []string  `json:"position_skills"`
	RecommendationID    uuid.UUID `json:"recommendation_id"`
	PositionID          uuid.UUID `json:"position_id"`
	PositionTitle       string    `json:"position_title"`
	PositionCompany     string    `json:"position_company"`
	PositionDescription string    `json:"position_description"`
	PositionLocation    *Location `json:"position_location"`
	PositionIsRemote    bool      `json:"position_is_remote"`
}

type RecommendationForRecruiter struct {
	CandidateExperiences   []CandidateExperience `json:"candidate_experiences"`
	CandidatePrefLocations []Location            `json:"candidate_pref_locations"`
	CandidatePrefTitles    []string              `json:"candidate_pref_titles"`
	PositionTitle          string                `json:"position_title"`
	CandidateFullName      string                `json:"candidate_full_name"`
	CandidateAbout         string                `json:"candidate_about"`
	RecommendationID       uuid.UUID             `json:"recommendation_id"`
	PositionID             uuid.UUID             `json:"position_id"`
	CandidateID            uuid.UUID             `json:"candidate_id"`
	CandidatePrefRemote    *bool                 `json:"candidate_pref_remote"`
}

type MatchForCandidate struct {
	MatchedAt         time.Time `json:"matched_at"`
	PositionID        uuid.UUID `json:"position_id"`
	PositionTitle     string    `json:"position_title"`
	PositionCompany   string    `json:"position_company"`
	RecruiterFullName string    `json:"recruiter_full_name"`
}

type Page struct {
	Cursor  string `json:"cursor,omitempty"`
	HasNext bool   `json:"has_next"`
	Limit   uint8  `json:"limit"`
	Count   uint8  `json:"count"`
}

type MatchForRecruiter struct {
	CandidateID       uuid.UUID `json:"candidate_id"`
	PositionID        uuid.UUID `json:"position_id"`
	CandidateFullName string    `json:"candidate_full_name"`
	PositionTitle     string    `json:"position_title"`
}

type Recruiter struct {
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
}

type ExperienceType string

const (
	ExperienceTypeWork             ExperienceType = "work"
	ExperienceTypeEducation        ExperienceType = "education"
	ExperienceTypeCertification    ExperienceType = "certification"
	ExperienceTypeInternship       ExperienceType = "internship"
	ExperienceTypePersonalProject  ExperienceType = "personal project"
	ExperienceTypeEnterpreneurship ExperienceType = "enterpreneurship"
	ExperienceTypeApprenticeship   ExperienceType = "apprenticeship"
	ExperienceTypeVolunteering     ExperienceType = "volunteering"
	ExperienceTypeOther            ExperienceType = "other"
)

type CandidateExperience struct {
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Company     string    `json:"company,omitempty"`
	Skill1      string    `json:"skill_1,omitempty"`
	Skill2      string    `json:"skill_2,omitempty"`
	Skill3      string    `json:"skill_3,omitempty"`
	Skill4      string    `json:"skill_4,omitempty"`
	Skill5      string    `json:"skill_5,omitempty"`
	ID          uuid.UUID `json:"id"`
	CandidateID uuid.UUID `json:"candidate_id"`

	ExperienceType ExperienceType `json:"experience_type"`
}

type Candidate struct {
	LastRecommendedAt time.Time `json:"last_recommended_at"`
	About             string    `json:"about"`
	PrefTitle1        string    `json:"pref_title_1,omitempty"`
	PrefTitle2        string    `json:"pref_title_2,omitempty"`
	PrefTitle3        string    `json:"pref_title_3,omitempty"`
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	PrefRemote        *bool     `json:"pref_remote,omitempty"`
	PrefLocation1ID   uint32    `json:"pref_location_1_id,omitempty"`
	PrefLocation2ID   uint32    `json:"pref_location_2_id,omitempty"`
	PrefLocation3ID   uint32    `json:"pref_location_3_id,omitempty"`
}

type Location struct {
	ID      int    `json:"id"`
	Country string `json:"country"`
	City    string `json:"city"`
	State   string `json:"state,omitempty"`
}

type Position struct {
	ID          uuid.UUID `json:"id"`
	RecruiterID uuid.UUID `json:"recruiter_id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Company     string    `json:"company"`
	Skill1      string    `json:"skill_1,omitempty"`
	Skill2      string    `json:"skill_2,omitempty"`
	Skill3      string    `json:"skill_3,omitempty"`
	Skill4      string    `json:"skill_4,omitempty"`
	Skill5      string    `json:"skill_5,omitempty"`
	LocationID  uint32    `json:"location_id,omitempty"`
	IsRemote    bool      `json:"is_remote"`
	IsActive    bool      `json:"is_active"`
}

type ReactionType string

const (
	ReactionTypePositive ReactionType = "positive"
	ReactionTypeNegative ReactionType = "negative"
	ReactionTypeNeutral  ReactionType = "neutral"
)

type ReactorType string

const (
	ReactorTypeCandidate ReactorType = "candidate"
	ReactorTypeRecruiter ReactorType = "recruiter"
)

type EmbeddingStatus string

const (
	EmbeddingStatusPending = "pending"
	EmbeddingStatusDone    = "done"
	EmbeddingStatusFailed  = "failed"
)

type EntityType string

const (
	EmbeddingEntityTypeCandidate = "candidate"
	EmbeddingEntityTypePosition  = "position"
)

const DefaultMaxRefreshTokensCount = 5

const DefaultWebAuthnSessionTTL = 5 * time.Minute

var (
	ErrRecommendationNotFound      = errors.New("recommendation not found")
	ErrUserAlreadyExists           = errors.New("user already exists")
	ErrReactionAlreadyExists       = errors.New("reaction already exists")
	ErrRecommendationAlreadyExists = errors.New("recommendation already exists")
	ErrEmbeddingsCountConflict     = errors.New("mismatch between count of embedding IDs and embeddings")
	ErrRecruiterExists             = errors.New("recruiter already exists")
	ErrPositionAlreadyExists       = errors.New("position already exists")
	ErrCandidateAlreadyExists      = errors.New("candidate already exists")
	ErrCandidateNotFound           = errors.New("candidate not found")
	ErrRecruiterNotFound           = errors.New("recruiter not found")
	ErrPositionNotFound            = errors.New("position not found")
	ErrInvalidLocationCursor       = errors.New("invalid location cursor; must be a positive integer")
	ErrLocationNotFound            = errors.New("location not found")
	ErrUserNoRole                  = errors.New("user has no role")
	ErrUserNotFound                = errors.New("user not found")
	ErrRefreshTokenReuse           = errors.New("refresh token reuse detected")
	ErrWebAuthnSessionNotFound     = errors.New("webauthn session not found")
	ErrWebAuthnCredentialNotFound  = errors.New("webauthn credential not found")
	ErrWebAuthnCredentialExists    = errors.New("webauthn credential already exists")
)

var DefaultSQLiteConn = "file:bin/hirevec.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

func ConnectSQLite() (*sql.DB, error) {
	db, err := sql.Open("sqlite", DefaultSQLiteConn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	var enabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled)
	if err != nil {
		return nil, fmt.Errorf("failed to scan: %w", err)
	}
	slog.Debug("enabling foreign keys")

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(10 * time.Minute)
	db.SetConnMaxLifetime(1 * time.Hour)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

const SQLVectorSQLite = `
-- Vector search setup for SQLite via the sqlite-vec extension,
-- which ships transpiled with modernc.org/sqlite and auto-registers
-- on every connection through its dedicated import.

begin transaction;

create table if not exists embeddings (
  id integer primary key,
  entity_id text not null, -- UUID
  entity_type text not null,
  entity_text text not null,
  embedding_status text not null,
  embedding_updated_at timestamp not null, -- UTC, RFC3339
  check (entity_type in ('candidate', 'position')),
  check (embedding_status in ('done', 'pending', 'failed'))
);

create unique index if not exists idx_embeddings_entity_id
on embeddings(entity_id);

create virtual table if not exists embeddings_vec using vec0(
  entity_id text primary key,
  embedding float[%d] distance_metric=cosine
);

commit;
`

func NewStore(c StoreConfig) (Store, error) {
	slog.Debug("initializing store")

	s := Store{}
	var err error

	if s.Database, err = ConnectSQLite(); err != nil {
		return Store{}, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	slog.Debug("creating database schema")
	if _, err := s.Database.Exec(InitSQL); err != nil {
		return Store{}, fmt.Errorf("failed to create database schema: %w", err)
	}

	slog.Debug("creating embeddings schema")
	if _, err := s.Database.Exec(
		fmt.Sprintf(SQLVectorSQLite, c.LlamaCppEmbeddingsDim),
	); err != nil {
		return Store{}, fmt.Errorf("failed to create embeddings schema: %w", err)
	}

	return s, nil
}

func (s Store) GetRecommendation(recommendationID uuid.UUID, userID uuid.UUID) (Recommendation, error) {
	var candidateID, positionID uuid.UUID
	if err := s.Database.QueryRow(`
		select 
			r.position_id, 
			r.candidate_id
		from recommendations r
		where r.id = $1
			and (
					exists (
							select 1
							from candidates c
							where c.id = r.candidate_id
								and c.user_id = $2
					)
					or exists (
							select 1
							from positions p
							join recruiters rec on rec.id = p.recruiter_id
							where p.id = r.position_id
								and rec.user_id = $2
					)
			);
	`, recommendationID, userID).Scan(
		&candidateID,
		&positionID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Recommendation{}, ErrRecommendationNotFound
		}
		return Recommendation{}, fmt.Errorf("failed to scan: %w", err)
	}

	return Recommendation{
		ID:          recommendationID,
		PositionID:  positionID,
		CandidateID: candidateID,
	}, nil
}

func (s Store) GetUserAndRolesByEmail(email string) (User, map[Role]uuid.UUID, error) {
	var userID uuid.UUID
	var updatedAt time.Time
	var fullName string
	var candidateID, recruiterID sql.NullString

	if err := s.Database.QueryRow(`
		select
			u.id,
			coalesce(u.full_name, ''),
			u.updated_at,
			c.id as candidate_id,
			r.id as recruiter_id
		from users u
		left join candidates c on c.user_id = u.id
		left join recruiters r on r.user_id = u.id
		where u.email = $1
	`, email).Scan(
		&userID,
		&fullName,
		&updatedAt,
		&candidateID,
		&recruiterID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, nil, ErrUserNotFound
		}
		return User{}, nil, fmt.Errorf("failed to scan: %w", err)
	}

	user := User{
		ID:        userID,
		Email:     email,
		FullName:  fullName,
		UpdatedAt: updatedAt,
	}

	roles := make(map[Role]uuid.UUID, 2)
	var err error
	if candidateID.Valid {
		if roles[RoleCandidate], err = uuid.Parse(candidateID.String); err != nil {
			return user, roles, err
		}
	}
	if recruiterID.Valid {
		if roles[RoleRecruiter], err = uuid.Parse(recruiterID.String); err != nil {
			return user, roles, err
		}
	}
	if len(roles) == 0 {
		return user, nil, ErrUserNoRole
	}

	return user, roles, nil
}

func (s Store) GetUserRoles(userID uuid.UUID) (map[Role]uuid.UUID, error) {
	var candidateID, recruiterID sql.NullString
	err := s.Database.QueryRow(`
		select
				(select c.id from candidates c where c.user_id = u.id) as candidate_id,
				(select r.id from recruiters r where r.user_id = u.id) as recruiter_id
		from users u
		where u.id = $1
	`, userID).Scan(
		&candidateID,
		&recruiterID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan: %w", err)
	}

	roles := make(map[Role]uuid.UUID, 2)
	if candidateID.Valid {
		if roles[RoleCandidate], err = uuid.Parse(candidateID.String); err != nil {
			return nil, fmt.Errorf("failed to parse candidateID: %w", err)
		}
	}
	if recruiterID.Valid {
		if roles[RoleRecruiter], err = uuid.Parse(recruiterID.String); err != nil {
			return nil, fmt.Errorf("failed to parse recruiterID: %w", err)
		}
	}
	if len(roles) == 0 {
		return nil, ErrUserNoRole
	}

	return roles, nil
}

func CurrentTimestamp(offset ...time.Duration) string {
	t := time.Now()
	if len(offset) > 0 {
		t = t.Add(offset[0])
	}
	return t.UTC().Format(time.RFC3339)
}

func (s Store) CreateUser(email string, fullName string) (uuid.UUID, error) {
	id := uuid.NewV7()
	result, err := s.Database.Exec(`
		insert into users (
			id,
		  email,
		  full_name,
			updated_at
		)
		values ($1, $2, nullif($3, ''), $4)
	`, id, email, fullName, CurrentTimestamp())
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return uuid.Nil(), ErrUserAlreadyExists
	}

	return id, nil
}

func (r ReactionType) IsValid() bool {
	return r == ReactionTypePositive ||
		r == ReactionTypeNegative ||
		r == ReactionTypeNeutral
}

func (r ReactorType) IsValid() bool {
	return r == ReactorTypeCandidate ||
		r == ReactorTypeRecruiter
}

func (s Store) CreateCandidateReaction(recommendationID uuid.UUID, candidateID uuid.UUID, reactionType ReactionType) error {
	var inserted int
	err := s.Database.QueryRow(`
		insert into candidate_reactions (
				recommendation_id,
				candidate_id,
				reaction_type,
				reacted_at
		)
		select $1, $2, $3, $4
		where exists (
				select 1
				from recommendations r
				where r.id = $1
					and r.candidate_id = $2
		)
		on conflict (recommendation_id, candidate_id) do nothing
		returning 1;
	`, recommendationID, candidateID, reactionType, CurrentTimestamp()).Scan(&inserted)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to create candidate reaction: %w", err)
		}

		var belongsToCandidate bool
		if err := s.Database.QueryRow(`
			select exists (
					select 1
					from recommendations
					where id = $1
						and candidate_id = $2
			)
    `, recommendationID, candidateID).Scan(&belongsToCandidate); err != nil {
			return fmt.Errorf("failed to check recommendation ownership: %w", err)
		}
		if !belongsToCandidate {
			return ErrRecommendationNotFound
		}

		return ErrReactionAlreadyExists
	}

	return nil
}

func (s Store) CreateRecruiterReaction(recommendationID uuid.UUID, recruiterID uuid.UUID, reactionType ReactionType) error {
	var inserted int
	err := s.Database.QueryRow(`
		insert into recruiter_reactions (
				recommendation_id,
				recruiter_id,
				reaction_type,
				reacted_at
		)
		select $1, $2, $3, $4
		where exists (
				select 1
				from recommendations r
				join positions p on p.id = r.position_id
				where r.id = $1
					and p.recruiter_id = $2
		)
		on conflict (recommendation_id, recruiter_id) do nothing
		returning 1;
	`, recommendationID, recruiterID, reactionType, CurrentTimestamp()).Scan(&inserted)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to create recruiter reaction: %w", err)
		}

		var belongsToRecruiter bool
		if err := s.Database.QueryRow(`
			select exists (
					select 1
					from recommendations r
					join positions p on p.id = r.position_id
					where r.id = $1
						and p.recruiter_id = $2
			)
		`, recommendationID, recruiterID).Scan(&belongsToRecruiter); err != nil {
			return fmt.Errorf("failed to check recommendation ownership: %w", err)
		}
		if !belongsToRecruiter {
			return ErrRecommendationNotFound
		}

		return ErrReactionAlreadyExists
	}

	return nil
}

func (s Store) IsRevokedRefreshToken(jti uuid.UUID) (bool, error) {
	var isRevoked bool
	if err := s.Database.QueryRow(`
		select revoked 
		from refresh_tokens 
		where jti = $1 
	`, jti).Scan(&isRevoked); err != nil {
		return true, fmt.Errorf("failed to scan: %w", err)
	}

	return isRevoked, nil
}

func (s Store) CreateRefreshToken(userID uuid.UUID) (jti uuid.UUID, err error) {
	jti = uuid.NewV7()

	tx, err := s.Database.Begin()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Debug("failed to rollback transaction", "err", rollbackErr)
			}
		}
	}()

	currentTimestamp := CurrentTimestamp()

	var count int
	if err = tx.QueryRow(`
		select count(*)
		from refresh_tokens
		where user_id = $1
		  and revoked = false
	`, userID).Scan(&count); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to scan: %w", err)
	}
	if count >= DefaultMaxRefreshTokensCount {
		if _, err = tx.Exec(`
			update refresh_tokens
			set revoked = true
			where jti in (
				select jti
				from refresh_tokens
				where user_id = $1
				  and revoked = false
				order by created_at asc
				limit 1
			)
		`, userID); err != nil {
			return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
		}
	}

	if _, err = tx.Exec(`
		insert into refresh_tokens (jti, user_id, created_at)
		values ($1, $2, $3)
	`, jti, userID, currentTimestamp); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to commit transaction: %w", err)
	}

	return jti, nil
}

func (s Store) RotateRefreshToken(userID uuid.UUID, oldJTI uuid.UUID) (jti uuid.UUID, err error) {
	newJTI := uuid.NewV7()

	tx, err := s.Database.Begin()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Debug("failed to rollback transaction", "err", rollbackErr)
			}
		}
	}()

	currentTimestamp := CurrentTimestamp()

	result, err := tx.Exec(`
		update refresh_tokens
		set revoked = true
		where jti = $1
			and revoked = false
	`, oldJTI)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to revoke used refresh token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		if _, err = tx.Exec(`
			update refresh_tokens
			set revoked = true
			where user_id = $1
				and revoked = false
		`, userID); err != nil {
			return uuid.Nil(), fmt.Errorf("failed to revoke user refresh tokens: %w", err)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return uuid.Nil(), fmt.Errorf("failed to commit transaction: %w", commitErr)
		}

		return uuid.Nil(), ErrRefreshTokenReuse
	}

	var count int
	if err = tx.QueryRow(`
		select count(*)
		from refresh_tokens
		where user_id = $1
			and revoked = false
	`, userID).Scan(&count); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to scan: %w", err)
	}
	if count >= DefaultMaxRefreshTokensCount {
		if _, err = tx.Exec(`
			update refresh_tokens
			set revoked = true
			where jti in (
				select jti
				from refresh_tokens
				where user_id = $1
					and revoked = false
				order by created_at asc
				limit 1
			)
		`, userID); err != nil {
			return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
		}
	}

	if _, err = tx.Exec(`
		insert into refresh_tokens (jti, user_id, created_at)
		values ($1, $2, $3)
	`, newJTI, userID, currentTimestamp); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newJTI, nil
}

type WebAuthnSessionType string

const (
	WebAuthnSessionTypeRegister WebAuthnSessionType = "register"
	WebAuthnSessionTypeLogin    WebAuthnSessionType = "login"
)

type WebAuthnSession struct {
	ID          uuid.UUID
	Type        WebAuthnSessionType
	UserID      uuid.UUID // uuid.Nil() when the session has no user yet
	Email       string
	FullName    string
	SessionData webauthn.SessionData
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

func (s Store) CreateWebAuthnSession(sessionType WebAuthnSessionType, userID uuid.UUID, email string, fullName string, sessionData webauthn.SessionData, ttl time.Duration) (uuid.UUID, error) {
	id := uuid.NewV7()

	sessionDataJSON, err := json.Marshal(sessionData)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to marshal session data: %w", err)
	}

	var userIDValue any
	if userID != uuid.Nil() {
		userIDValue = userID
	}

	if _, err := s.Database.Exec(`
		insert into webauthn_sessions (
			id,
			type,
			user_id,
			email,
			full_name,
			session_data,
			expires_at,
			created_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, sessionType, userIDValue, email, fullName, sessionDataJSON, CurrentTimestamp(ttl), CurrentTimestamp()); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	return id, nil
}

func (s Store) GetWebAuthnSession(id uuid.UUID) (WebAuthnSession, error) {
	var (
		session         WebAuthnSession
		sessionType     string
		userID          sql.NullString
		email           sql.NullString
		fullName        sql.NullString
		sessionDataJSON []byte
		expiresAt       time.Time
		createdAt       time.Time
	)

	err := s.Database.QueryRow(`
		select
			type,
			user_id,
			email,
			full_name,
			session_data,
			expires_at,
			created_at
		from webauthn_sessions
		where id = $1
	`, id).Scan(
		&sessionType,
		&userID,
		&email,
		&fullName,
		&sessionDataJSON,
		&expiresAt,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return WebAuthnSession{}, ErrWebAuthnSessionNotFound
	}
	if err != nil {
		return WebAuthnSession{}, fmt.Errorf("failed to scan: %w", err)
	}

	if err := json.Unmarshal(sessionDataJSON, &session.SessionData); err != nil {
		return WebAuthnSession{}, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	session.ID = id
	session.Type = WebAuthnSessionType(sessionType)
	if userID.Valid {
		if session.UserID, err = uuid.Parse(userID.String); err != nil {
			return WebAuthnSession{}, fmt.Errorf("failed to parse userID: %w", err)
		}
	}
	session.Email = email.String
	session.FullName = fullName.String
	session.ExpiresAt = expiresAt
	session.CreatedAt = createdAt

	return session, nil
}

func (s Store) DeleteWebAuthnSession(id uuid.UUID) error {
	if _, err := s.Database.Exec(`
		delete from webauthn_sessions
		where id = $1
	`, id); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}

func (s Store) DeleteExpiredWebAuthnSessions() (int64, error) {
	result, err := s.Database.Exec(`
		delete from webauthn_sessions
		where expires_at < $1
	`, CurrentTimestamp())
	if err != nil {
		return 0, fmt.Errorf("failed to execute SQL: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to determine number of affected rows: %w", err)
	}

	return count, nil
}

func (s Store) CreateWebAuthnCredential(userID uuid.UUID, credential webauthn.Credential) error {
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("failed to marshal credential: %w", err)
	}

	result, err := s.Database.Exec(`
		insert into webauthn_credentials (
			id,
			user_id,
			credential,
			last_used_at,
			created_at
		)
		values ($1, $2, $3, $4, $4)
		on conflict (id) do nothing
	`, base64.RawURLEncoding.EncodeToString(credential.ID), userID, credentialJSON, CurrentTimestamp())
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrWebAuthnCredentialExists
	}

	return nil
}

func (s Store) UpdateWebAuthnCredential(credential webauthn.Credential) error {
	credentialJSON, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("failed to marshal credential: %w", err)
	}

	result, err := s.Database.Exec(`
		update webauthn_credentials
		set credential = $1,
			last_used_at = $2
		where id = $3
	`, credentialJSON, CurrentTimestamp(), base64.RawURLEncoding.EncodeToString(credential.ID))
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrWebAuthnCredentialNotFound
	}

	return nil
}

func (s Store) GetWebAuthnCredentialsByUserID(userID uuid.UUID) ([]webauthn.Credential, error) {
	rows, err := s.Database.Query(`
		select credential
		from webauthn_credentials
		where user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	credentials := []webauthn.Credential{}
	for rows.Next() {
		var credentialJSON []byte
		if err := rows.Scan(&credentialJSON); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		var credential webauthn.Credential
		if err := json.Unmarshal(credentialJSON, &credential); err != nil {
			return nil, fmt.Errorf("failed to unmarshal credential: %w", err)
		}

		credentials = append(credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return credentials, nil
}

func (s Store) CreateRecommendation(positionID uuid.UUID, candidateID uuid.UUID) (uuid.UUID, error) {
	id := uuid.NewV7()

	result, err := s.Database.Exec(`
		insert into recommendations (id, position_id, candidate_id)
		values ($1, $2, $3)
		on conflict do nothing
	`, id, positionID, candidateID)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return uuid.Nil(), ErrRecommendationAlreadyExists
	}

	return id, nil
}

func (s Store) GetRecommendationsForCandidate(candidateID uuid.UUID, page Page, excludeReacted bool) ([]RecommendationForCandidate, Page, error) {
	rows, err := s.Database.Query(`
		select
			r.id,
			p.id,
			p.title,
			coalesce(p.company, ''),
			p.description,
			p.is_remote,
			coalesce(l.id, 0), coalesce(l.country, ''), coalesce(l.city, ''), coalesce(l.state, ''),
			p.skill_1, p.skill_2, p.skill_3, p.skill_4, p.skill_5
		from recommendations r
		join positions p on p.id = r.position_id
		join candidates c on c.id = $1
		left join locations l on l.id = p.location_id
		where r.candidate_id = $1
				and p.is_active = 1
				and ($2 = '' or r.id > $2)
				and ($4 = false or not exists (
						select 1
						from candidate_reactions cr
						where cr.recommendation_id = r.id
							and cr.candidate_id = $1
				))
				and (c.pref_remote is null or c.pref_remote = 0 or p.is_remote = 1)
				and (
						c.pref_location_1_id is null and c.pref_location_2_id is null and c.pref_location_3_id is null
						or p.is_remote = 1
						or p.location_id is null
						or p.location_id in (c.pref_location_1_id, c.pref_location_2_id, c.pref_location_3_id)
				)
		order by r.id desc
		limit $3
	`, candidateID, page.Cursor, int(page.Limit)+1, excludeReacted)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	recommendations := make([]RecommendationForCandidate, 0, page.Limit)
	for rows.Next() {
		var (
			recommendation                         RecommendationForCandidate
			recommendationIDStr, positionIDStr     string
			isRemote                               int
			locID                                  int
			country, city, state                   string
			skill1, skill2, skill3, skill4, skill5 string
		)
		if err := rows.Scan(
			&recommendationIDStr,
			&positionIDStr,
			&recommendation.PositionTitle,
			&recommendation.PositionCompany,
			&recommendation.PositionDescription,
			&isRemote,
			&locID,
			&country, &city, &state,
			&skill1, &skill2, &skill3, &skill4, &skill5,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}

		recommendation.RecommendationID, err = uuid.Parse(recommendationIDStr)
		if err != nil {
			return nil, Page{}, fmt.Errorf("failed to parse recommendation_id: %w", err)
		}
		recommendation.PositionID, err = uuid.Parse(positionIDStr)
		if err != nil {
			return nil, Page{}, fmt.Errorf("failed to parse position_id: %w", err)
		}

		recommendation.PositionIsRemote = isRemote == 1

		if locID != 0 {
			recommendation.PositionLocation = &Location{
				ID:      locID,
				Country: country,
				City:    city,
				State:   state,
			}
		}

		recommendation.PositionSkills = make([]string, 0, 5)
		if skill1 != "" {
			recommendation.PositionSkills = append(recommendation.PositionSkills, skill1)
		}
		if skill2 != "" {
			recommendation.PositionSkills = append(recommendation.PositionSkills, skill2)
		}
		if skill3 != "" {
			recommendation.PositionSkills = append(recommendation.PositionSkills, skill3)
		}
		if skill4 != "" {
			recommendation.PositionSkills = append(recommendation.PositionSkills, skill4)
		}
		if skill5 != "" {
			recommendation.PositionSkills = append(recommendation.PositionSkills, skill5)
		}

		recommendations = append(recommendations, recommendation)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	hasNext := len(recommendations) > int(page.Limit)
	var nextCursor uuid.UUID
	if hasNext {
		nextCursor = recommendations[page.Limit-1].RecommendationID
		recommendations = recommendations[:page.Limit]
	}

	return recommendations, Page{
		Cursor:  nextCursor.String(),
		Limit:   page.Limit,
		Count:   uint8(len(recommendations)),
		HasNext: hasNext,
	}, nil
}

func (s Store) GetRecommendationsForRecruiter(recruiterID uuid.UUID, page Page, includeReacted bool) ([]RecommendationForRecruiter, Page, error) {
	jsonAggExpr := `
			coalesce(json_group_array(
				json_object(
					'id', ce.id,
					'title', ce.title,
					'started_at', ce.started_at,
					'ended_at', ce.ended_at,
					'description', ce.description,
					'company', ce.company,
					'experience_type', ce.experience_type,
					'skill_1', ce.skill_1,
					'skill_2', ce.skill_2,
					'skill_3', ce.skill_3,
					'skill_4', ce.skill_4,
					'skill_5', ce.skill_5
				)
			), '[]')`

	rows, err := s.Database.Query(fmt.Sprintf(`
		select
			r.id,
			p.id,
			p.title,
			c.id,
			coalesce(u.full_name, ''),
			c.about,
			c.pref_remote = 1,
			coalesce(c.pref_title_1, ''), coalesce(c.pref_title_2, ''), coalesce(c.pref_title_3, ''),
			coalesce(l1.id, 0), coalesce(l1.country, ''), coalesce(l1.city, ''), coalesce(l1.state, ''),
			coalesce(l2.id, 0), coalesce(l2.country, ''), coalesce(l2.city, ''), coalesce(l2.state, ''),
			coalesce(l3.id, 0), coalesce(l3.country, ''), coalesce(l3.city, ''), coalesce(l3.state, ''),
			(
				select %s
				from candidate_experiences ce
				where ce.candidate_id = c.id
			) as experiences
		from recommendations r
		join positions p on p.id = r.position_id
		join candidates c on c.id = r.candidate_id
		join users u on u.id = c.user_id
		left join locations l1 on l1.id = c.pref_location_1_id
		left join locations l2 on l2.id = c.pref_location_2_id
		left join locations l3 on l3.id = c.pref_location_3_id
		where p.recruiter_id = $1
				and ($2 = '' or r.id > $2)
				and ($4 or not exists (
						select 1
						from recruiter_reactions rr
						where rr.recommendation_id = r.id
							and rr.recruiter_id = $1
				))
		order by r.id desc
		limit $3
	`, jsonAggExpr), recruiterID, page.Cursor, int(page.Limit)+1, includeReacted)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	recommendations := make([]RecommendationForRecruiter, 0, page.Limit)
	for rows.Next() {
		var (
			recommendation                                     RecommendationForRecruiter
			recommendationIDStr, positionIDStr, candidateIDStr string
			prefTitle1, prefTitle2, prefTitle3                 string
			loc1Country, loc1City, loc1State                   string
			loc2Country, loc2City, loc2State                   string
			loc3Country, loc3City, loc3State                   string
			loc1ID, loc2ID, loc3ID                             int
			experiencesJSON                                    []byte
		)
		if err := rows.Scan(
			&recommendationIDStr,
			&positionIDStr,
			&recommendation.PositionTitle,
			&candidateIDStr,
			&recommendation.CandidateFullName,
			&recommendation.CandidateAbout,
			&recommendation.CandidatePrefRemote,
			&prefTitle1, &prefTitle2, &prefTitle3,
			&loc1ID, &loc1Country, &loc1City, &loc1State,
			&loc2ID, &loc2Country, &loc2City, &loc2State,
			&loc3ID, &loc3Country, &loc3City, &loc3State,
			&experiencesJSON,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		if err := rows.Err(); err != nil {
			return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
		}

		recommendation.RecommendationID, err = uuid.Parse(recommendationIDStr)
		if err != nil {
			return nil, Page{}, fmt.Errorf("failed to parse recommendation_id: %w", err)
		}
		recommendation.PositionID, err = uuid.Parse(positionIDStr)
		if err != nil {
			return nil, Page{}, fmt.Errorf("failed to parse position_id: %w", err)
		}
		recommendation.CandidateID, err = uuid.Parse(candidateIDStr)
		if err != nil {
			return nil, Page{}, fmt.Errorf("failed to parse candidate_id: %w", err)
		}

		recommendation.CandidatePrefTitles = make([]string, 0, 3)
		if prefTitle1 != "" {
			recommendation.CandidatePrefTitles = append(recommendation.CandidatePrefTitles, prefTitle1)
		}
		if prefTitle2 != "" {
			recommendation.CandidatePrefTitles = append(recommendation.CandidatePrefTitles, prefTitle2)
		}
		if prefTitle3 != "" {
			recommendation.CandidatePrefTitles = append(recommendation.CandidatePrefTitles, prefTitle3)
		}

		recommendation.CandidatePrefLocations = make([]Location, 0, 3)
		if loc1ID != 0 {
			recommendation.CandidatePrefLocations = append(recommendation.CandidatePrefLocations, Location{
				ID:      loc1ID,
				Country: loc1Country,
				City:    loc1City,
				State:   loc1State,
			})
		}
		if loc2ID != 0 {
			recommendation.CandidatePrefLocations = append(recommendation.CandidatePrefLocations, Location{
				ID:      loc2ID,
				Country: loc2Country,
				City:    loc2City,
				State:   loc2State,
			})
		}
		if loc3ID != 0 {
			recommendation.CandidatePrefLocations = append(recommendation.CandidatePrefLocations, Location{
				ID:      loc3ID,
				Country: loc3Country,
				City:    loc3City,
				State:   loc3State,
			})
		}

		if err := json.Unmarshal(experiencesJSON, &recommendation.CandidateExperiences); err != nil {
			return nil, Page{}, fmt.Errorf("failed to unmarshal experiences: %w", err)
		}

		if recommendation.CandidateExperiences == nil {
			recommendation.CandidateExperiences = []CandidateExperience{}
		}

		recommendations = append(recommendations, recommendation)
	}

	hasNext := len(recommendations) > int(page.Limit)
	var nextCursor uuid.UUID
	if hasNext {
		nextCursor = recommendations[page.Limit-1].RecommendationID
		recommendations = recommendations[:page.Limit]
	}

	return recommendations, Page{
		Cursor:  nextCursor.String(),
		Limit:   page.Limit,
		Count:   uint8(len(recommendations)),
		HasNext: hasNext,
	}, nil
}

func (s Store) GetReactionsForRecruiter(recruiterID uuid.UUID, page Page) ([]Reaction, Page, error) {
	rows, err := s.Database.Query(`
		select recommendation_id, reaction_type, reacted_at
		from recruiter_reactions
		where recruiter_id = $1
		  and ($2 = '' or recommendation_id > $2)
		order by recommendation_id desc
		limit $3
	`, recruiterID, page.Cursor, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]Reaction, 0, page.Limit)
	for rows.Next() {
		var rx Reaction
		if err := rows.Scan(
			&rx.RecommendationID,
			&rx.ReactionType,
			&rx.ReactedAt,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		results = append(results, rx)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	var nextPage Page
	if len(results) > int(page.Limit) {
		results = results[:page.Limit]
		nextPage.Cursor = results[page.Limit-1].RecommendationID.String()
	}

	return results, nextPage, nil
}

func (s Store) GetReactionsForCandidate(candidateID uuid.UUID, page Page) ([]Reaction, Page, error) {
	rows, err := s.Database.Query(`
		select recommendation_id, reaction_type, reacted_at
		from candidate_reactions
		where candidate_id = $1
		  and ($2 = '' or recommendation_id > $2)
		order by recommendation_id desc
		limit $3
	`, candidateID, page.Cursor, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]Reaction, 0, page.Limit)
	for rows.Next() {
		var rx Reaction
		if err := rows.Scan(
			&rx.RecommendationID,
			&rx.ReactionType,
			&rx.ReactedAt,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		results = append(results, rx)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	var nextPage Page
	if len(results) > int(page.Limit) {
		results = results[:page.Limit]
		nextPage.Cursor = results[page.Limit-1].RecommendationID.String()
	}

	return results, nextPage, nil
}

func (s Store) GetMatchesForCandidate(candidateID uuid.UUID, page Page) ([]MatchForCandidate, Page, error) {
	rows, err := s.Database.Query(`
		select
			m.position_id,
			p.title,
			coalesce(p.company, ''),
			coalesce(ru.full_name, ''),
			m.matched_at
		from matches m
		join positions p on p.id = m.position_id
		join recruiters rec on rec.id = p.recruiter_id
		join users ru on ru.id = rec.user_id
		where m.candidate_id = $1
		  and ($2 = '' or m.position_id > $2)
		order by m.position_id desc
		limit $3
	`, candidateID, page.Cursor, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]MatchForCandidate, 0, page.Limit)
	for rows.Next() {
		var m MatchForCandidate
		if err := rows.Scan(
			&m.PositionID,
			&m.PositionTitle,
			&m.PositionCompany,
			&m.RecruiterFullName,
			&m.MatchedAt,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	var nextPage Page
	if len(results) > int(page.Limit) {
		results = results[:page.Limit]
		nextPage.Cursor = results[page.Limit-1].PositionID.String()
	}

	return results, nextPage, nil
}

func (s Store) GetMatchesForRecruiter(recruiterID uuid.UUID, page Page) ([]MatchForRecruiter, Page, error) {
	rows, err := s.Database.Query(`
		select
			c.id,
			coalesce(u.full_name, ''),
			m.position_id,
			p.title
		from matches m
		join positions p on p.id = m.position_id
		join candidates c on c.id = m.candidate_id
		join users u on u.id = c.user_id
		where p.recruiter_id = $1
		  and ($2 = '' or m.position_id > $2)
		order by m.position_id desc
		limit $3
	`, recruiterID, page.Cursor, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]MatchForRecruiter, 0, page.Limit)
	for rows.Next() {
		var m MatchForRecruiter
		if err := rows.Scan(
			&m.CandidateID,
			&m.CandidateFullName,
			&m.PositionID,
			&m.PositionTitle,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	var nextPage Page
	if len(results) > int(page.Limit) {
		results = results[:page.Limit]
		nextPage.Cursor = results[page.Limit-1].PositionID.String()
	}

	return results, nextPage, nil
}

func (s Store) UpsertPendingEmbedding(entityID uuid.UUID, entityType EntityType, text string) error {
	if _, err := s.Database.Exec(`
		insert into embeddings (entity_id, entity_type, entity_text, embedding_status, embedding_updated_at)
		values ($1, $2, $3, $4, $5)
		on conflict (entity_id) do update set
			entity_type = excluded.entity_type,
			entity_text = excluded.entity_text,
			embedding_status = excluded.embedding_status,
			embedding_updated_at = excluded.embedding_updated_at
	`, entityID, entityType, text, EmbeddingStatusPending, CurrentTimestamp(0)); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}

func (s Store) FetchPendingEmbeddings(limit uint8) ([]uuid.UUID, []string, error) {
	rows, err := s.Database.Query(`
		select entity_id, entity_text
		from embeddings
		where embedding_status in ('pending', 'failed')
		order by embedding_updated_at asc, id asc
		limit $1
  `, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	ids := make([]uuid.UUID, 0, limit)
	texts := make([]string, 0, limit)
	for rows.Next() {
		var id string
		var text string

		if err := rows.Scan(&id, &text); err != nil {
			return nil, nil, fmt.Errorf("failed to scan: %w", err)
		}

		uuid, err := uuid.Parse(id)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse entity_id: %w", err)
		}

		ids = append(ids, uuid)
		texts = append(texts, text)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return ids, texts, nil
}

func (s Store) MarkEmbeddingsStatusSQL(entityIDs []uuid.UUID, status EmbeddingStatus) (string, []any, error) {
	placeholders := make([]string, len(entityIDs))
	args := make([]any, 0, len(entityIDs)+2)
	args = append(args, status, CurrentTimestamp(0))
	for i, entityID := range entityIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, entityID)
	}
	return fmt.Sprintf(`
			update embeddings
			set embedding_status = $1,
				embedding_updated_at = $2
			where entity_id in (%s)
		`, strings.Join(placeholders, ", ")), args, nil
}

func (s Store) MarkEmbeddingsStatus(entityIDs []uuid.UUID, status EmbeddingStatus) error {
	if len(entityIDs) == 0 {
		return nil
	}

	query, args, err := s.MarkEmbeddingsStatusSQL(entityIDs, status)
	if err != nil {
		return err
	}

	if _, err := s.Database.Exec(query, args...); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}

func (s Store) MarkEmbeddingsStatusTx(tx *sql.Tx, entityIDs []uuid.UUID, status EmbeddingStatus) error {
	if len(entityIDs) == 0 {
		return nil
	}

	query, args, err := s.MarkEmbeddingsStatusSQL(entityIDs, status)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	return nil
}

func (s Store) GetPositionsForCandidateViaEmbeddings(candidateID uuid.UUID, topPositions uint8) ([]uuid.UUID, error) {
	var query string
	var args []any
	query = `
			with candidate as (
					select v.embedding as embedding
					from embeddings_vec v
					join embeddings e
							on e.entity_id = v.entity_id
					where v.entity_id = $1
							and e.entity_type = 'candidate'
							and e.embedding_status = 'done'
					limit 1
			)
			select p.id as position_id
			from positions p
			join embeddings e
					on e.entity_id = p.id
			cross join candidate c
			join embeddings_vec v
					on v.entity_id = p.id
			where p.is_active = 1
					and e.embedding_status = 'done'
					and not exists (
							select 1
							from recommendations r
							where r.position_id = p.id
								and r.candidate_id = $1
					)
			order by vec_distance_cosine(v.embedding, c.embedding)
			limit $2
		`
	args = []any{candidateID, topPositions}

	rows, err := s.Database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]uuid.UUID, 0, topPositions)
	for rows.Next() {
		var positionID string
		if err := rows.Scan(&positionID); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		uuid, err := uuid.Parse(positionID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse position_id: %w", err)
		}

		results = append(results, uuid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return results, nil
}

func BuildPositionText(title, description, company string, skill1, skill2, skill3, skill4, skill5 string) string {
	var builder strings.Builder
	builder.WriteString(title)
	builder.WriteString(" ")
	builder.WriteString(description)
	if company != "" {
		builder.WriteString(" ")
		builder.WriteString(company)
	}
	if skill1 != "" {
		builder.WriteString(" ")
		builder.WriteString(skill1)
	}
	if skill2 != "" {
		builder.WriteString(" ")
		builder.WriteString(skill2)
	}
	if skill3 != "" {
		builder.WriteString(" ")
		builder.WriteString(skill3)
	}
	if skill4 != "" {
		builder.WriteString(" ")
		builder.WriteString(skill4)
	}
	if skill5 != "" {
		builder.WriteString(" ")
		builder.WriteString(skill5)
	}
	return builder.String()
}

func BuildCandidateText(about string, experiences []CandidateExperience) string {
	var builder strings.Builder
	builder.WriteString(about)
	for _, experience := range experiences {
		builder.WriteString(" ")
		builder.WriteString(experience.Title)
		if experience.Description != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Description)
		}
		if experience.Company != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Company)
		}
		if experience.Skill1 != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Skill1)
		}
		if experience.Skill2 != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Skill2)
		}
		if experience.Skill3 != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Skill3)
		}
		if experience.Skill4 != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Skill4)
		}
		if experience.Skill5 != "" {
			builder.WriteString(" ")
			builder.WriteString(experience.Skill5)
		}
	}
	return builder.String()
}

func (s Store) GetPositionsTexts(positionIDs []uuid.UUID) ([]string, error) {
	if len(positionIDs) == 0 {
		return make([]string, 0), nil
	}

	placeholders := make([]string, len(positionIDs))
	args := make([]any, len(positionIDs))
	for i, positionID := range positionIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = positionID
	}

	rows, err := s.Database.Query(fmt.Sprintf(`
		select
			title,
			description,
			coalesce(company, ''),
			skill_1,
			skill_2,
			skill_3,
			skill_4,
			skill_5
		from positions
		where id in (%s)
		order by id
	`, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	texts := make([]string, 0, len(positionIDs))
	for rows.Next() {
		var (
			title       string
			description string
			company     string
			skill1      string
			skill2      string
			skill3      string
			skill4      string
			skill5      string
		)
		if err := rows.Scan(
			&title,
			&description,
			&company,
			&skill1,
			&skill2,
			&skill3,
			&skill4,
			&skill5,
		); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		texts = append(texts, BuildPositionText(title, description, company, skill1, skill2, skill3, skill4, skill5))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return texts, nil
}

func SQLiteVecBlob(vector []float32) []byte {
	blob := make([]byte, 4*len(vector))
	for i, value := range vector {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(value))
	}
	return blob
}

func (s Store) UpsertEmbeddingsTx(tx *sql.Tx, embeddingIDs []uuid.UUID, embeddings []EmbeddingData) error {
	if len(embeddingIDs) == 0 {
		return nil
	}
	if len(embeddingIDs) != len(embeddings) {
		return ErrEmbeddingsCountConflict
	}

	for i := range embeddingIDs {
		vector := SQLiteVecBlob(embeddings[i].Embedding)
		if _, err := tx.Exec(`
				delete from embeddings_vec
				where entity_id = $1
			`, embeddingIDs[i]); err != nil {
			return fmt.Errorf("failed to delete stale vector: %w", err)
		}

		if _, err := tx.Exec(`
				insert into embeddings_vec (entity_id, embedding)
				values ($1, $2)
			`, embeddingIDs[i], vector); err != nil {
			return fmt.Errorf("failed to insert vector: %w", err)
		}
	}

	return nil
}

func (s Store) GetCandidates(limit uint16, recommendationSpan time.Duration) ([]uuid.UUID, error) {
	cutoff := CurrentTimestamp(-recommendationSpan)
	rows, err := s.Database.Query(`
		select id
		from candidates
		where last_recommended_at <= $1
		   or last_recommended_at is null
		order by last_recommended_at asc
		limit $2
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		uuid, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse candidate_id: %w", err)
		}

		ids = append(ids, uuid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return ids, nil
}

func (s Store) CreateRecruiter(userID uuid.UUID) (uuid.UUID, error) {
	id := uuid.NewV7()

	result, err := s.Database.Exec(`
		insert into recruiters (id, user_id)
		values ($1, $2)
		on conflict (user_id) do nothing
	`, id, userID)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to exec SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return uuid.Nil(), ErrRecruiterExists
	}

	return id, nil
}

func (s Store) CreatePosition(position Position) (uuid.UUID, error) {
	position.ID = uuid.NewV7()

	isRemote := 0
	if position.IsRemote {
		isRemote = 1
	}

	isActive := 0
	if position.IsActive {
		isActive = 1
	}

	result, err := s.Database.Exec(
		`
		insert into positions (
			id,
			recruiter_id,
			title,
			description,
			company,
			is_remote,
			location_id,
			skill_1,
			skill_2,
			skill_3,
			skill_4,
			skill_5,
			is_active,
			created_at
		) values ($1, $2, $3, $4, $5, $6, nullif($7, 0), $8, $9, $10, $11, $12, $13, $14)
	`,
		position.ID,
		position.RecruiterID,
		position.Title,
		position.Description,
		position.Company,
		isRemote,
		position.LocationID,
		position.Skill1,
		position.Skill2,
		position.Skill3,
		position.Skill4,
		position.Skill5,
		isActive,
		CurrentTimestamp(0),
	)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to exec SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return uuid.Nil(), ErrPositionAlreadyExists
	}

	if err := s.UpsertPendingEmbedding(
		position.ID,
		EmbeddingEntityTypePosition,
		BuildPositionText(position.Title, position.Description, position.Company, position.Skill1, position.Skill2, position.Skill3, position.Skill4, position.Skill5),
	); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to create embedding metadata: %w", err)
	}

	return position.ID, nil
}

func (t ExperienceType) IsValid() bool {
	switch t {
	case ExperienceTypeWork,
		ExperienceTypeEducation,
		ExperienceTypeCertification,
		ExperienceTypeInternship,
		ExperienceTypePersonalProject,
		ExperienceTypeEnterpreneurship,
		ExperienceTypeApprenticeship,
		ExperienceTypeVolunteering,
		ExperienceTypeOther:
		return true
	}
	return false
}

func (s Store) CreateCandidateExperiences(tx *sql.Tx, candidateID uuid.UUID, experiences []CandidateExperience) error {
	for i := range experiences {
		experiences[i].CandidateID = candidateID
		if experiences[i].ID == uuid.Nil() {
			experiences[i].ID = uuid.NewV7()
		}

		_, err := tx.Exec(
			`
			insert into candidate_experiences (
				id,
				candidate_id,
				title,
				started_at,
				ended_at,
				description,
				company,
				experience_type,
				skill_1,
				skill_2,
				skill_3,
				skill_4,
				skill_5
			) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`,
			experiences[i].ID,
			experiences[i].CandidateID,
			experiences[i].Title,
			experiences[i].StartedAt.UTC().Format(time.RFC3339),
			experiences[i].EndedAt.UTC().Format(time.RFC3339),
			experiences[i].Description,
			experiences[i].Company,
			experiences[i].ExperienceType,
			experiences[i].Skill1,
			experiences[i].Skill2,
			experiences[i].Skill3,
			experiences[i].Skill4,
			experiences[i].Skill5,
		)
		if err != nil {
			return fmt.Errorf("failed to execute SQL: %w", err)
		}
	}

	return nil
}

func (s Store) CreateCandidate(candidate Candidate, experiences []CandidateExperience) (uuid.UUID, error) {
	candidate.ID = uuid.NewV7()

	tx, err := s.Database.Begin()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Debug("failed to rollback transaction", "err", rollbackErr)
			}
		}
	}()

	result, err := tx.Exec(
		`
		insert into candidates (
			id,
			user_id,
			about,
			pref_remote,
			pref_title_1,
			pref_title_2,
			pref_title_3,
			pref_location_1_id,
			pref_location_2_id,
			pref_location_3_id,
			last_recommended_at
		) values ($1, $2, $3, cast(cast($4 as boolean) as int4), $5, $6, $7, nullif($8, 0), nullif($9, 0), nullif($10, 0), $11)
		on conflict (user_id) do nothing
	`,
		candidate.ID,
		candidate.UserID,
		candidate.About,
		candidate.PrefRemote,
		candidate.PrefTitle1,
		candidate.PrefTitle2,
		candidate.PrefTitle3,
		candidate.PrefLocation1ID,
		candidate.PrefLocation2ID,
		candidate.PrefLocation3ID,
		CurrentTimestamp(-7*24*time.Hour),
	)
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return uuid.Nil(), fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return uuid.Nil(), ErrCandidateAlreadyExists
	}

	if err = s.CreateCandidateExperiences(tx, candidate.ID, experiences); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to insert candidate experiences: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to commit transaction: %w", err)
	}

	if err = s.UpsertPendingEmbedding(
		candidate.ID,
		EmbeddingEntityTypeCandidate,
		BuildCandidateText(candidate.About, experiences),
	); err != nil {
		return uuid.Nil(), fmt.Errorf("failed to create embedding metadata: %w", err)
	}

	return candidate.ID, nil
}

func (s Store) UserExistsByEmail(email string) (bool, error) {
	var exists bool
	err := s.Database.QueryRow(`
		select exists(
			select 1
			from users
			where email = $1
		)
	`, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to scan: %w", err)
	}

	return exists, nil
}

func (s Store) GetUserAndRoles(userID uuid.UUID) (User, map[Role]uuid.UUID, error) {
	var updatedAt time.Time
	var fullName, email string
	var candidateID, recruiterID sql.NullString
	err := s.Database.QueryRow(`
		select
			u.email,
			coalesce(u.full_name, ''),
			u.updated_at,
			c.id as candidate_id,
			r.id as recruiter_id
		from users u
		left join candidates c on c.user_id = u.id
		left join recruiters r on r.user_id = u.id
		where u.id = $1
	`, userID).Scan(
		&email,
		&fullName,
		&updatedAt,
		&candidateID,
		&recruiterID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, nil, ErrUserNotFound
		}
		return User{}, nil, fmt.Errorf("failed to scan: %w", err)
	}

	roles := make(map[Role]uuid.UUID, 2)
	if candidateID.Valid {
		if roles[RoleCandidate], err = uuid.Parse(candidateID.String); err != nil {
			return User{}, nil, fmt.Errorf("failed to parse candidateID: %w", err)
		}
	}
	if recruiterID.Valid {
		if roles[RoleRecruiter], err = uuid.Parse(recruiterID.String); err != nil {
			return User{}, nil, fmt.Errorf("failed to parse recruiterID: %w", err)
		}
	}
	if len(roles) == 0 {
		return User{}, nil, ErrUserNoRole
	}

	return User{
		ID:        userID,
		Email:     email,
		FullName:  fullName,
		UpdatedAt: updatedAt,
	}, roles, nil
}

func (s Store) GetUser(userID uuid.UUID) (User, error) {
	var updatedAt time.Time
	var fullName, email string
	err := s.Database.QueryRow(`
		select
			coalesce(email, ''),
			coalesce(full_name, ''),
			updated_at
		from users
		where id = $1
	`, userID).Scan(
		&email,
		&fullName,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("failed to scan: %w", err)
	}

	return User{
		ID:        userID,
		Email:     email,
		FullName:  fullName,
		UpdatedAt: updatedAt,
	}, nil
}

func (s Store) UpdateUser(
	userID uuid.UUID,
	newFullName string,
) error {
	result, err := s.Database.Exec(`
		update users
		set
			full_name = $1,
			updated_at = $2
		where id = $3
	`, newFullName, CurrentTimestamp(), userID)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s Store) UpdateUserAndReturn(userID uuid.UUID, newFullName string) (User, error) {
	var updatedAt time.Time
	var email string
	err := s.Database.QueryRow(`
		update users
		set
			full_name = $1,
			updated_at = $2
		where id = $3
		returning
			email,
			full_name,
			updated_at
	`, newFullName, CurrentTimestamp(), userID).Scan(
		&email,
		&newFullName,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("failed to scan: %w", err)
	}

	return User{
		ID:        userID,
		Email:     email,
		FullName:  newFullName,
		UpdatedAt: updatedAt,
	}, nil
}

func (s Store) DeleteUser(userID uuid.UUID) error {
	res, err := s.Database.Exec(`
		delete from users
		where id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (s Store) GetCandidate(candidateID uuid.UUID) (Candidate, error) {
	var userIDStr string

	var candidate Candidate
	err := s.Database.QueryRow(`
		select
			user_id,
			about,
			pref_remote = 1,
			coalesce(pref_title_1, ''),
			coalesce(pref_title_2, ''),
			coalesce(pref_title_3, ''),
			coalesce(pref_location_1_id, 0),
			coalesce(pref_location_2_id, 0),
			coalesce(pref_location_3_id, 0),
			last_recommended_at
		from candidates
		where id = $1
	`, candidateID).Scan(
		&userIDStr,
		&candidate.About,
		&candidate.PrefRemote,
		&candidate.PrefTitle1,
		&candidate.PrefTitle2,
		&candidate.PrefTitle3,
		&candidate.PrefLocation1ID,
		&candidate.PrefLocation2ID,
		&candidate.PrefLocation3ID,
		&candidate.LastRecommendedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Candidate{}, ErrCandidateNotFound
		}
		return Candidate{}, fmt.Errorf("failed to scan: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return Candidate{}, fmt.Errorf("failed to parse user_id: %w", err)
	}

	candidate.ID = candidateID
	candidate.UserID = userID

	return candidate, nil
}

func (s Store) UpdateCandidate(candidate Candidate, experiences []CandidateExperience) error {
	tx, err := s.Database.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Debug("failed to rollback transaction", "err", rollbackErr)
			}
		}
	}()

	result, err := tx.Exec(
		`
		update candidates
		set
			about = $1,
			pref_remote = cast(cast($2 as boolean) as int4),
			pref_title_1 = $3,
			pref_title_2 = $4,
			pref_title_3 = $5,
			pref_location_1_id = nullif($6, 0),
			pref_location_2_id = nullif($7, 0),
			pref_location_3_id = nullif($8, 0)
		where id = $9
	`,
		candidate.About,
		candidate.PrefRemote,
		candidate.PrefTitle1,
		candidate.PrefTitle2,
		candidate.PrefTitle3,
		candidate.PrefLocation1ID,
		candidate.PrefLocation2ID,
		candidate.PrefLocation3ID,
		candidate.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrCandidateNotFound
	}

	if experiences != nil {
		if _, err = tx.Exec(`
			delete from candidate_experiences
			where candidate_id = $1
		`, candidate.ID); err != nil {
			return fmt.Errorf("failed to execute SQL: %w", err)
		}

		if err = s.CreateCandidateExperiences(tx, candidate.ID, experiences); err != nil {
			return fmt.Errorf("failed to insert candidate experiences: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	if experiences == nil {
		if experiences, err = s.GetCandidateExperiences(candidate.ID); err != nil {
			return fmt.Errorf("failed to get candidate experiences: %w", err)
		}
	}

	if err = s.UpsertPendingEmbedding(
		candidate.ID,
		EmbeddingEntityTypeCandidate,
		BuildCandidateText(candidate.About, experiences),
	); err != nil {
		return fmt.Errorf("failed to upsert pending embedding: %w", err)
	}

	return nil
}

func (s Store) UpdateCandidateAndReturn(candidateID uuid.UUID, newAbout string) (Candidate, error) {
	candidate := Candidate{ID: candidateID}
	err := s.Database.QueryRow(`
		update candidates
		set about = $1
		where id = $2
		returning user_id, about, last_recommended_at
	`, newAbout, candidateID).Scan(
		&candidate.UserID,
		&candidate.About,
		&candidate.LastRecommendedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Candidate{}, ErrCandidateNotFound
		}
		return Candidate{}, fmt.Errorf("failed to scan: %w", err)
	}

	experiences, err := s.GetCandidateExperiences(candidateID)
	if err != nil {
		return Candidate{}, fmt.Errorf("failed to get candidate experiences: %w", err)
	}
	if err := s.UpsertPendingEmbedding(
		candidateID,
		EmbeddingEntityTypeCandidate,
		BuildCandidateText(newAbout, experiences),
	); err != nil {
		return Candidate{}, fmt.Errorf("failed to upsert pending embedding: %w", err)
	}

	return candidate, nil
}

func (s Store) GetCandidateExperiences(candidateID uuid.UUID) ([]CandidateExperience, error) {
	rows, err := s.Database.Query(`
		select 
			id,
			candidate_id,
			title,
			started_at,
			ended_at,
			coalesce(description, ''),
			coalesce(company, ''),
			experience_type,
			coalesce(skill_1, ''),
			coalesce(skill_2, ''),
			coalesce(skill_3, ''),
			coalesce(skill_4, ''),
			coalesce(skill_5, '')
		from candidate_experiences
		where candidate_id = $1
		order by started_at desc
	`, candidateID)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	experiences := []CandidateExperience{}
	for rows.Next() {
		var experience CandidateExperience
		err := rows.Scan(&experience.ID, &experience.CandidateID, &experience.Title, &experience.StartedAt, &experience.EndedAt, &experience.Description, &experience.Company, &experience.ExperienceType, &experience.Skill1, &experience.Skill2, &experience.Skill3, &experience.Skill4, &experience.Skill5)
		if err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		experiences = append(experiences, experience)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return experiences, nil
}

func (s Store) DeleteCandidate(candidateID uuid.UUID) error {
	res, err := s.Database.Exec(`
		delete from candidates
		where id = $1
	`, candidateID)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrCandidateNotFound
	}

	return nil
}

func (s Store) GetRecruiter(recruiterID uuid.UUID) (Recruiter, error) {
	var userID uuid.UUID
	err := s.Database.QueryRow(`
		select user_id,
		from recruiters
		where id = $1
	`, recruiterID).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Recruiter{}, ErrRecruiterNotFound
		}
		return Recruiter{}, fmt.Errorf("failed to scan: %w", err)
	}

	return Recruiter{
		recruiterID,
		userID,
	}, nil
}

func (s Store) DeleteRecruiter(recruiterID uuid.UUID) error {
	res, err := s.Database.Exec(`
		delete from recruiters
		where id = $1
	`, recruiterID)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrRecruiterNotFound
	}

	return nil
}

func (s Store) RecruiterExists(recruiterID uuid.UUID) (bool, error) {
	var exists bool
	err := s.Database.QueryRow(`
		select exists(
			select 1
			from recruiters
			where id = $1
		)
	`, recruiterID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to scan: %w", err)
	}

	return exists, nil
}

func (s Store) LocationExists(locationID int) (bool, error) {
	var exists bool
	err := s.Database.QueryRow(`
		select exists(
			select 1
			from locations
			where id = $1
		)
	`, locationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to scan: %w", err)
	}

	return exists, nil
}

func (s Store) GetPosition(positionID uuid.UUID) (Position, error) {
	var isRemote, isActive int
	position := Position{}
	err := s.Database.QueryRow(`
		select
			recruiter_id,
			title,
			description,
			coalesce(company, ''),
			is_remote,
			coalesce(location_id, 0),
			skill_1,
			skill_2,
			skill_3,
			skill_4,
			skill_5,
			is_active
		from positions
		where id = $1
	`, positionID).Scan(
		&position.RecruiterID,
		&position.Title,
		&position.Description,
		&position.Company,
		&isRemote,
		&position.LocationID,
		&position.Skill1,
		&position.Skill2,
		&position.Skill3,
		&position.Skill4,
		&position.Skill5,
		&isActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Position{}, ErrPositionNotFound
		}
		return Position{}, fmt.Errorf("failed to query: %w", err)
	}
	position.ID = positionID
	position.IsRemote = isRemote == 1
	position.IsActive = isActive == 1
	return position, nil
}

func (s Store) GetPositions(recruiterID uuid.UUID, page Page) ([]Position, Page, error) {
	rows, err := s.Database.Query(`
		select
			id,
			title,
			description,
			coalesce(company, ''),
			is_remote,
			coalesce(location_id, 0),
			skill_1,
			skill_2,
			skill_3,
			skill_4,
			skill_5,
			is_active
		from positions
		where recruiter_id = $1 and ($2 = '' or id > $2)
		order by id desc
		limit $3
	`, recruiterID, page.Cursor, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	positions := make([]Position, 0, page.Limit)
	for rows.Next() {
		var isRemote, isActive int
		var position Position
		if err := rows.Scan(
			&position.ID,
			&position.Title,
			&position.Description,
			&position.Company,
			&isRemote,
			&position.LocationID,
			&position.Skill1,
			&position.Skill2,
			&position.Skill3,
			&position.Skill4,
			&position.Skill5,
			&isActive,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		position.IsRemote = isRemote == 1
		position.IsActive = isActive == 1
		positions = append(positions, position)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	var nextPage Page
	if len(positions) > int(page.Limit) {
		positions = positions[:page.Limit]
		page.Cursor = positions[page.Limit-1].ID.String()
	}
	return positions, nextPage, nil
}

func (s Store) SearchLocations(query string, page Page) ([]Location, Page, error) {
	cursor := 0
	if page.Cursor != "" {
		parsed, err := strconv.Atoi(page.Cursor)
		if err != nil || parsed < 0 {
			return nil, Page{}, ErrInvalidLocationCursor
		}
		cursor = parsed
	}

	search := strings.TrimSpace(query)

	rows, err := s.Database.Query(`
		select id, country, city, coalesce(state, '')
		from locations
		where ($1 = 0 or id > $1)
				and ($2 = ''
						or lower(country) like '%' || lower($2) || '%'
						or lower(city) like '%' || lower($2) || '%')
		order by id
		limit $3
	`, cursor, search, int(page.Limit)+1)
	if err != nil {
		return nil, Page{}, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	locations := make([]Location, 0, page.Limit)
	for rows.Next() {
		var location Location
		if err := rows.Scan(
			&location.ID,
			&location.Country,
			&location.City,
			&location.State,
		); err != nil {
			return nil, Page{}, fmt.Errorf("failed to scan: %w", err)
		}
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return nil, Page{}, fmt.Errorf("failed to iterate rows: %w", err)
	}

	hasNext := len(locations) > int(page.Limit)
	var nextCursor int
	if hasNext {
		locations = locations[:page.Limit]
		nextCursor = locations[page.Limit-1].ID
	}

	return locations, Page{
		Cursor:  strconv.Itoa(nextCursor),
		Limit:   page.Limit,
		Count:   uint8(len(locations)),
		HasNext: hasNext,
	}, nil
}

func (s Store) GetLocation(locationID int) (Location, error) {
	location := Location{}
	err := s.Database.QueryRow(`
		select id, country, city, coalesce(state, '')
		from locations
		where id = $1
	`, locationID).Scan(
		&location.ID,
		&location.Country,
		&location.City,
		&location.State,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Location{}, ErrLocationNotFound
		}
		return Location{}, fmt.Errorf("failed to query: %w", err)
	}
	return location, nil
}

func (s Store) UpdatePosition(position Position) error {
	isRemote := 0
	if position.IsRemote {
		isRemote = 1
	}
	isActive := 0
	if position.IsActive {
		isActive = 1
	}

	result, err := s.Database.Exec(
		`
		update positions
		set
			title = $2,
			description = $3,
			company = $4,
			is_remote = $5,
			location_id = nullif($6, 0),
			skill_1 = $7,
			skill_2 = $8,
			skill_3 = $9,
			skill_4 = $10,
			skill_5 = $11,
			is_active = $12
		where id = $1
	`,
		position.ID,
		position.Title,
		position.Description,
		position.Company,
		isRemote,
		position.LocationID,
		position.Skill1,
		position.Skill2,
		position.Skill3,
		position.Skill4,
		position.Skill5,
		isActive,
	)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrPositionNotFound
	}

	if err := s.UpsertPendingEmbedding(
		position.ID,
		EmbeddingEntityTypePosition,
		BuildPositionText(position.Title, position.Description, position.Company, position.Skill1, position.Skill2, position.Skill3, position.Skill4, position.Skill5),
	); err != nil {
		return fmt.Errorf("failed to upsert pending embedding: %w", err)
	}

	return nil
}

func (s Store) DeletePosition(positionID uuid.UUID) error {
	res, err := s.Database.Exec(`
		delete from positions
		where id = $1
	`, positionID)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine number of affected rows: %w", err)
	}
	if rows == 0 {
		return ErrPositionNotFound
	}

	return nil
}

func BuildTextSearchTerms(text string, quote bool) string {
	tokens := strings.Fields(text)
	terms := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.ReplaceAll(token, `"`, "")
		token = strings.TrimLeft(token, "-")
		if !strings.ContainsFunc(token, func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsDigit(r)
		}) {
			continue
		}
		if quote {
			token = `"` + token + `"`
		}
		terms = append(terms, token)
	}
	return strings.Join(terms, " OR ")
}

func (s Store) GetPositionsForCandidateViaFTS(candidateID uuid.UUID, topPositions uint8) ([]uuid.UUID, error) {
	var candidateAbout string
	err := s.Database.QueryRow(`
		select about
		from candidates
		where id = $1
	`, candidateID).Scan(&candidateAbout)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []uuid.UUID{}, ErrCandidateNotFound
		}
		return nil, fmt.Errorf("failed to scan: %w", err)
	}

	if strings.TrimSpace(candidateAbout) == "" {
		return []uuid.UUID{}, nil
	}

	var query string
	var args []any
	query = `
			select p.id as position_id
			from positions_fts
			join positions p on p.id = positions_fts.id
			where positions_fts match $1
					and p.is_active = 1
					and not exists (
							select 1
							from recommendations r
							where r.position_id = p.id
								and r.candidate_id = $2
					)
			order by bm25(positions_fts) asc
			limit $3
		`
	args = []any{BuildTextSearchTerms(candidateAbout, true), candidateID, topPositions}

	rows, err := s.Database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]uuid.UUID, 0, topPositions)
	for rows.Next() {
		var positionIDStr string
		if err := rows.Scan(&positionIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		positionID, err := uuid.Parse(positionIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse position_id: %w", err)
		}

		results = append(results, positionID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return results, nil
}

func (s Store) GetCandidatesForPositionViaFTS(positionID uuid.UUID, topCandidates uint8) ([]uuid.UUID, error) {
	var title, description, company string
	err := s.Database.QueryRow(`
		select
			title,
			description,
			coalesce(company, '')
		from positions
		where id = $1
	`, positionID).Scan(&title, &description, &company)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []uuid.UUID{}, ErrPositionNotFound
		}
		return nil, fmt.Errorf("failed to scan: %w", err)
	}

	positionText := strings.Join(strings.Fields(title+" "+description+" "+company), " ")
	if positionText == "" {
		return []uuid.UUID{}, nil
	}

	var query string
	var args []any
	query = `
			select c.id as candidate_id
			from candidates_fts
			join candidates c on c.id = candidates_fts.id
			where candidates_fts match $1
					and not exists (
							select 1
							from recommendations r
							where r.position_id = $2
								and r.candidate_id = c.id
					)
			order by bm25(candidates_fts) asc
			limit $3
		`
	args = []any{BuildTextSearchTerms(positionText, true), positionID, topCandidates}

	rows, err := s.Database.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Error("failed to close rows", "err", err)
		}
	}()

	results := make([]uuid.UUID, 0, topCandidates)
	for rows.Next() {
		var candidateIDStr string
		if err := rows.Scan(&candidateIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}

		candidateID, err := uuid.Parse(candidateIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse candidate_id: %w", err)
		}

		results = append(results, candidateID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return results, nil
}
