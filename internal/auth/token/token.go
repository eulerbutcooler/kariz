package token

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/eulerbutcooler/kariz/internal/auth"
	"github.com/eulerbutcooler/kariz/internal/store"
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

func (t *Token) Authenticate(secret string) (auth.Identity, error) {
	tok, err := t.st.TokenByHash(context.Background(), hashToken(secret))
	if errors.Is(err, store.ErrTokenNotFound) {
		return auth.Identity{}, auth.ErrAuthFailed
	}
	if err != nil {
		return auth.Identity{}, fmt.Errorf("token: lookup: %w", err)
	}
	if tok.Revoked {
		return auth.Identity{}, auth.ErrAuthFailed
	}
	return auth.Identity{}, nil
}

func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
