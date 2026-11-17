// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	"aidanwoods.dev/go-paseto"
)

type VaultConfig struct {
	ServerURL         *url.URL
	SymmetricKeyPath  string
	AsymmetricKeyPath string
}

type Vault struct {
	AccessTokenParser     paseto.Parser
	RefreshTokenParser    paseto.Parser
	V4AsymmetricPublicKey paseto.V4AsymmetricPublicKey
	V4AsymmetricSecretKey paseto.V4AsymmetricSecretKey // TODO: Make a secret
	V4SymmetricKey        paseto.V4SymmetricKey        // TODO: Make a secret
}

type AccessTokenClaims struct {
	UserID uuid.UUID
	Roles  map[Role]uuid.UUID
}

type AccessToken struct {
	AccessToken string    `json:"access_token"` // TODO: Make a secret
	TokenType   string    `json:"token_type"`
	ExpiresIn   uint32    `json:"expires_in"`
	Scope       string    `json:"scope"`
	UserID      uuid.UUID `json:"user_id"`
}

type RefreshTokenClaims struct {
	UserID uuid.UUID
	JTI    uuid.UUID
}

type RefreshToken struct {
	RefreshToken string    `json:"refresh_token"`
	UserID       uuid.UUID `json:"user_id"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    uint32    `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	Scope        string    `json:"scope"`
	UserID       uuid.UUID `json:"user_id"`
}

type Role string

const (
	RoleCandidate Role = "candidate"
	RoleRecruiter Role = "recruiter"
)

type Scope string

const (
	ScopeRoleCandidate Scope = "role:candidate"
	ScopeRoleRecruiter Scope = "role:recruiter"
)

var ErrInvalidRole = errors.New("invalid role")

const (
	DefaultAccessTokenExpiration  = 30 * time.Minute     // 30 minutes
	DefaultRefreshTokenExpiration = 730 * 24 * time.Hour // 2 years
)

const (
	TokenAudience = "hirevec.com"
	TokenIssuer   = "hirevec.com"
)

func StringToRole(roleStr string) (Role, error) {
	if roleStr == string(RoleCandidate) {
		return RoleCandidate, nil
	} else if roleStr == string(RoleRecruiter) {
		return RoleRecruiter, nil
	} else {
		return Role(""), ErrInvalidRole
	}
}

func NewVault(ctx context.Context, c VaultConfig) (Vault, error) {
	slog.Debug("initializing vault")

	accessTokenParser := paseto.NewParser()
	accessTokenParser.AddRule(paseto.ForAudience(TokenAudience))
	accessTokenParser.AddRule(paseto.IssuedBy(TokenIssuer))
	accessTokenParser.AddRule(paseto.NotExpired())
	accessTokenParser.AddRule(paseto.NotBeforeNbf())

	refreshTokenParser := paseto.NewParser()
	refreshTokenParser.AddRule(paseto.ForAudience(TokenAudience))
	refreshTokenParser.AddRule(paseto.IssuedBy(TokenIssuer))
	refreshTokenParser.AddRule(paseto.NotExpired())
	refreshTokenParser.AddRule(paseto.NotBeforeNbf())

	sk, err := LoadOrCreateSymmetricKey(c.SymmetricKeyPath)
	if err != nil {
		slog.Error("failed to init symmetric key", "err", err)
		return Vault{}, fmt.Errorf("failed to init symmetric key: %w", err)
	}

	ak, err := LoadOrCreateAsymmetricKey(c.AsymmetricKeyPath)
	if err != nil {
		slog.Error("failed to init asymmetric key", "err", err)
		return Vault{}, fmt.Errorf("failed to init asymmetric key: %w", err)
	}

	vault := Vault{
		AccessTokenParser:     accessTokenParser,
		RefreshTokenParser:    refreshTokenParser,
		V4AsymmetricSecretKey: ak,
		V4AsymmetricPublicKey: ak.Public(),
		V4SymmetricKey:        sk,
	}

	return vault, nil
}

func (v Vault) ParseAccessToken(tokenString string) (AccessTokenClaims, error) {
	parsedToken, err := v.AccessTokenParser.ParseV4Public(
		v.V4AsymmetricPublicKey,
		tokenString,
		nil,
	)
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to parse token: %w", err)
	}

	userIDRaw, err := parsedToken.GetSubject()
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to get subject: %w", err)
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return AccessTokenClaims{}, fmt.Errorf("failed to parse userID: %w", err)
	}

	roles := make(map[Role]uuid.UUID, 3)
	recruiterID, _ := parsedToken.GetString("recruiter_id")
	candidateID, _ := parsedToken.GetString("candidate_id")
	if recruiterID != "" {
		if roles[RoleRecruiter], err = uuid.Parse(recruiterID); err != nil {
			return AccessTokenClaims{}, fmt.Errorf("failed to parse recruiterID: %w", err)
		}
	}
	if candidateID != "" {
		if roles[RoleCandidate], err = uuid.Parse(candidateID); err != nil {
			return AccessTokenClaims{}, fmt.Errorf("failed to parse candidateID: %w", err)
		}
	}

	return AccessTokenClaims{
		UserID: userID,
		Roles:  roles,
	}, nil
}

func (v Vault) ParseRefreshToken(tokenString string) (RefreshTokenClaims, error) {
	parsedToken, err := v.RefreshTokenParser.ParseV4Local(
		v.V4SymmetricKey,
		tokenString,
		nil,
	)
	if err != nil {
		slog.Error("failed to parse refresh token", "err", err)
		return RefreshTokenClaims{}, fmt.Errorf("failed to parse token: %w", err)
	}

	userIDRaw, err := parsedToken.GetSubject()
	if err != nil {
		return RefreshTokenClaims{}, fmt.Errorf("failed to get subject: %w", err)
	}
	userID, err := uuid.Parse(userIDRaw)
	if err != nil {
		return RefreshTokenClaims{}, fmt.Errorf("failed to parse userID: %w", err)
	}

	jtiRaw, err := parsedToken.GetJti()
	if err != nil {
		return RefreshTokenClaims{}, fmt.Errorf("failed to get JTI: %w", err)
	}
	jti, err := uuid.Parse(jtiRaw)
	if err != nil {
		return RefreshTokenClaims{}, fmt.Errorf("failed to parse JTI: %w", err)
	}

	return RefreshTokenClaims{
		UserID: userID,
		JTI:    jti,
	}, nil
}

func (v Vault) CreateAccessToken(userID uuid.UUID, roles map[Role]uuid.UUID) (AccessToken, error) {
	now := time.Now().UTC()

	token := paseto.NewToken()
	token.SetAudience(TokenAudience)
	token.SetIssuer(TokenIssuer)
	token.SetSubject(userID.String())
	token.SetExpiration(now.Add(DefaultAccessTokenExpiration))
	token.SetNotBefore(now)
	token.SetIssuedAt(now)

	var scopes []string

	if _, ok := roles[RoleCandidate]; ok {
		if err := token.Set("candidate_id", roles[RoleCandidate]); err != nil {
			return AccessToken{}, fmt.Errorf("failed to set candidate_id: %w", err)
		}
		scopes = append(scopes, string(ScopeRoleCandidate))
	}

	if _, ok := roles[RoleRecruiter]; ok {
		if err := token.Set("recruiter_id", roles[RoleRecruiter]); err != nil {
			return AccessToken{}, fmt.Errorf("failed to set recruiter_id: %w", err)
		}
		scopes = append(scopes, string(ScopeRoleRecruiter))
	}

	scopeStr := strings.Join(scopes, " ")
	token.SetString("scope", scopeStr)

	return AccessToken{
		AccessToken: token.V4Sign(v.V4AsymmetricSecretKey, nil),
		TokenType:   "Bearer",
		ExpiresIn:   uint32(DefaultAccessTokenExpiration.Seconds()),
		Scope:       scopeStr,
		UserID:      userID,
	}, nil
}

func (v Vault) CreateRefreshToken(userID uuid.UUID, jti uuid.UUID) (RefreshToken, error) {
	now := time.Now().UTC()

	token := paseto.NewToken()
	token.SetAudience(TokenAudience)
	token.SetIssuer(TokenIssuer)
	token.SetSubject(userID.String())
	token.SetExpiration(now.Add(DefaultRefreshTokenExpiration))
	token.SetNotBefore(now)
	token.SetIssuedAt(now)
	token.SetJti(jti.String())

	return RefreshToken{
		RefreshToken: token.V4Encrypt(v.V4SymmetricKey, nil),
		UserID:       userID,
	}, nil
}

func (v Vault) CreateTokenPair(userID uuid.UUID, jti uuid.UUID, roles map[Role]uuid.UUID) (TokenPair, error) {
	accessToken, err := v.CreateAccessToken(userID, roles)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to create access token: %w", err)
	}

	refreshToken, err := v.CreateRefreshToken(userID, jti)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken.AccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    uint32(DefaultAccessTokenExpiration.Seconds()),
		RefreshToken: refreshToken.RefreshToken,
		Scope:        accessToken.Scope,
		UserID:       userID,
	}, nil
}

func DefaultKeyPath(filename string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".hirevec", filename), nil
}

func LoadOrCreateKey(keyPath string, defaultFilename string, generator func() string) (string, error) {
	if keyPath == "" {
		var err error
		keyPath, err = DefaultKeyPath(defaultFilename)
		if err != nil {
			return "", err
		}
	}

	keyPath = filepath.Clean(keyPath)

	data, err := os.ReadFile(keyPath)
	if err == nil {
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("key file %q is empty", keyPath)
		}
		return value, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to read key file %q: %w", keyPath, err)
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create key directory: %w", err)
	}

	value := generator()

	tmp, err := os.CreateTemp(dir, ".hirevec-key-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary key file: %w", err)
	}

	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("failed to set key file permissions: %w", err)
	}

	if _, err := tmp.WriteString(value + "\n"); err != nil {
		tmp.Close()
		return "", fmt.Errorf("failed to write key file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("failed to sync key file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("failed to close key file: %w", err)
	}

	if err := os.Link(tmpPath, keyPath); err != nil {
		if errors.Is(err, fs.ErrExist) {
			data, readErr := os.ReadFile(keyPath)
			if readErr != nil {
				return "", fmt.Errorf("failed to read concurrently created key: %w", readErr)
			}

			value = strings.TrimSpace(string(data))
			if value == "" {
				return "", fmt.Errorf("key file %q is empty", keyPath)
			}

			return value, nil
		}

		return "", fmt.Errorf("failed to create key file %q: %w", keyPath, err)
	}

	return value, nil
}

func LoadOrCreateSymmetricKey(keyPath string) (paseto.V4SymmetricKey, error) {
	value, err := LoadOrCreateKey(keyPath, ".symkey.txt", paseto.NewV4SymmetricKey().ExportHex)
	if err != nil {
		return paseto.V4SymmetricKey{}, fmt.Errorf("failed to load or create symmetric key: %w", err)
	}

	key, err := paseto.V4SymmetricKeyFromHex(value)
	if err != nil {
		return paseto.V4SymmetricKey{}, fmt.Errorf("invalid symmetric key: %w", err)
	}

	return key, nil
}

func LoadOrCreateAsymmetricKey(keyPath string) (paseto.V4AsymmetricSecretKey, error) {
	value, err := LoadOrCreateKey(keyPath, ".asymkey.txt", paseto.NewV4AsymmetricSecretKey().ExportHex)
	if err != nil {
		return paseto.V4AsymmetricSecretKey{}, fmt.Errorf("failed to load or create asymmetric key: %w", err)
	}

	key, err := paseto.NewV4AsymmetricSecretKeyFromHex(value)
	if err != nil {
		return paseto.V4AsymmetricSecretKey{}, fmt.Errorf("invalid asymmetric key: %w", err)
	}

	return key, nil
}
