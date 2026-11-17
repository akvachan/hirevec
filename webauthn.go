// Copyright (c) 2026 Arsenii Kvachan
// SPDX-License-Identifier: Unlicense

package hirevec

import (
	"fmt"
	"uuid"

	"github.com/go-webauthn/webauthn/webauthn"
)

type WebAuthnUser struct {
	ID          uuid.UUID
	Email       string
	FullName    string
	Credentials []webauthn.Credential
}

func (u WebAuthnUser) WebAuthnID() []byte {
	return u.ID[:]
}

func (u WebAuthnUser) WebAuthnName() string {
	return u.Email
}

func (u WebAuthnUser) WebAuthnDisplayName() string {
	return u.FullName
}

func (u WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

func (s Store) LoadWebAuthnUser(rawID []byte, userHandle []byte) (webauthn.User, error) {
	if len(userHandle) != 16 {
		return nil, fmt.Errorf("invalid user handle length: %d", len(userHandle))
	}

	var userID uuid.UUID
	copy(userID[:], userHandle)

	user, err := s.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	credentials, err := s.GetWebAuthnCredentialsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get webauthn credentials: %w", err)
	}

	return WebAuthnUser{
		ID:          user.ID,
		Email:       user.Email,
		FullName:    user.FullName,
		Credentials: credentials,
	}, nil
}
