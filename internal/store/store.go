package store

import (
	"context"
	"errors"
)

var ErrTokenNotFound = errors.New("store: token not found")
var ErrUserExists = errors.New("store: user already exists")
var ErrUserNotFound = errors.New("store: user not found")

type Token struct {
	Hash      string
	UserID    string
	Revoked   bool
	ExpiresAt string
}

type User struct {
	Email        string
	PasswordHash string
	CreatedAt    string
}

type Store interface {
	CreateToken(ctx context.Context, hash, userID, expiresAt string) error
	TokenByHash(ctx context.Context, hash string) (Token, error)
	RevokeToken(ctx context.Context, hash string) error
	CreateUser(ctx context.Context, email, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (User, error)
}
