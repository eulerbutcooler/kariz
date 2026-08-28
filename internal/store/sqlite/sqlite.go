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
revoked INTEGER NOT NULL DEFAULT 0
);`

var _ store.Store = (*Store)(nil)

func (s *Store) CreateToken(ctx context.Context, hash, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO tokens (token_hash, user_id, revoked) VALUES (?,?,0)", hash, userID)
	if err != nil {
		return fmt.Errorf("sqlite: create token: %w", err)
	}
	return nil
}

func (s *Store) TokenByHash(ctx context.Context, hash string) (store.Token, error) {
	var t store.Token
	err := s.db.QueryRowContext(ctx,
		"SELECT token_hash, user_id, revoked FROM tokens WHERE token_hash = ?", hash).
		Scan(&t.Hash, &t.UserID, &t.Revoked)
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
