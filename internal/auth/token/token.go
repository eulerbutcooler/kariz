package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/eulerbutcooler/surang/internal/auth"
	"github.com/eulerbutcooler/surang/internal/store"
)

type Token struct {
	st store.Store
}

func New(st store.Store) *Token {
	return &Token{
		st: st,
	}
}

var _ auth.Authenticator = (*Token)(nil)

func validateTokenHash(st store.Store, hash string) (store.Token, error) {
	tok, err := st.TokenByHash(context.Background(), hash)
	if errors.Is(err, store.ErrTokenNotFound) {
		return store.Token{}, auth.ErrAuthFailed
	}
	if err != nil {
		return store.Token{}, fmt.Errorf("token: lookup: %w", err)
	}
	if tok.Revoked {
		return store.Token{}, auth.ErrAuthFailed
	}
	if tok.ExpiresAt != "" {
		exp, err := time.Parse(time.RFC3339, tok.ExpiresAt)
		if err != nil {
			return store.Token{}, fmt.Errorf("token: parse expiry: %w", err)
		}
		if time.Now().UTC().After(exp) {
			return store.Token{}, auth.ErrTokenExpired
		}
	}
	return tok, nil
}

func (t *Token) Authenticate(secret string) (auth.Identity, error) {
	if _, err := validateTokenHash(t.st, hashToken(secret)); err != nil {
		return auth.Identity{}, err
	}
	return auth.Identity{}, nil
}

func ValidateSession(st store.Store, sessionToken string) (string, error) {
	tok, err := validateTokenHash(st, hashToken(sessionToken))
	if err != nil {
		return "", err
	}
	return tok.UserID, nil
}

func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
