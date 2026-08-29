package token

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/eulerbutcooler/surang/internal/store"
)

func Mint(st store.Store, userID, expiresAt string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: generate: %w", err)
	}
	plain := hex.EncodeToString(b)
	if err := st.CreateToken(context.Background(), hashToken(plain), userID, expiresAt); err != nil {
		return "", fmt.Errorf("token: store: %w", err)
	}
	return plain, nil
}
