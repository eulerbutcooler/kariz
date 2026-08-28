package store

import (
	"context"
	"errors"
)

var ErrTokenNotFound = errors.New("store: token not found")

type Token struct {
	Hash    string
	UserID  string
	Revoked bool
}

type Store interface {
	CreateToken(ctx context.Context, hash, userID string) error
	TokenByHash(ctx context.Context, hash string) (Token, error)
	RevokeToken(ctx context.Context, hash string) error
}
