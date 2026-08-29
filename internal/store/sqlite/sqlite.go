package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/eulerbutcooler/kariz/internal/store"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("sqlite: schema: %w", err)
	}
	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS tokens(
token_hash TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT 'admin',
created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
revoked INTEGER NOT NULL DEFAULT 0,
expires_at TEXT
);
CREATE TABLE IF NOT EXISTS users(
email TEXT PRIMARY KEY,
password_hash TEXT NOT NULL,
created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

var _ store.Store = (*Store)(nil)

func (s *Store) CreateToken(ctx context.Context, hash, userID, expiresAt string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO tokens (token_hash, user_id, revoked, expires_at) VALUES (?,?,0, ?)", hash, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("sqlite: create token: %w", err)
	}
	return nil
}

func (s *Store) TokenByHash(ctx context.Context, hash string) (store.Token, error) {
	var t store.Token
	err := s.db.QueryRowContext(ctx,
		"SELECT token_hash, user_id, revoked, expires_at FROM tokens WHERE token_hash = ?", hash).
		Scan(&t.Hash, &t.UserID, &t.Revoked, &t.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Token{}, store.ErrTokenNotFound
	}
	if err != nil {
		return store.Token{}, fmt.Errorf("sqlite: token by hash: %w", err)
	}
	return t, nil
}

func (s *Store) RevokeToken(ctx context.Context, hash string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE tokens SET revoked = 1 WHERE token_hash = ?", hash)
	if err != nil {
		return fmt.Errorf("sqlite: revoke: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: revoke rows: %w", err)
	}
	if n == 0 {
		return store.ErrTokenNotFound
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (email, password_hash) VALUES (?,?)", email, passwordHash)
	if err == nil {
		return nil
	}
	var exists string
	qerr := s.db.QueryRowContext(ctx,
		"SELECT email FROM users WHERE email = ?", email).Scan(&exists)
	if qerr == nil {
		return store.ErrUserExists
	}
	return fmt.Errorf("sqlite: create user: %w", err)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx,
		"SELECT email, password_hash, created_at FROM users WHERE email = ?", email).
		Scan(&u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.User{}, store.ErrUserNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("sqlite: user by email: %w", err)
	}
	return u, nil
}
