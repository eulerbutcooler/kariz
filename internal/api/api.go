package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/eulerbutcooler/kariz/internal/auth"
	"github.com/eulerbutcooler/kariz/internal/auth/token"
	"github.com/eulerbutcooler/kariz/internal/store"
)

type API struct {
	addr string
	st   store.Store
	log  *slog.Logger
	mux  *http.ServeMux
}

func NewAPI(addr string, st store.Store, log *slog.Logger) *API {
	a := &API{addr: addr, st: st, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/signup", a.handleSignup)
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/tokens", a.handleTokens)
	a.mux = mux
	return a
}

var tokenTTLs = map[string]time.Duration{
	"1h": time.Hour,
	"1d": 24 * time.Hour,
	"1w": 7 * 24 * time.Hour,
}

func (a *API) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    a.addr,
		Handler: a.mux,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("api: %w", err)
	case <-ctx.Done():
		ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx2)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (a *API) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "email required (must contain @)")
		return
	}
	if len(req.Password) < 6 {
		writeError(w, http.StatusBadRequest, "password too short (min 6 chars")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		a.log.Error("hash password", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if err := a.st.CreateUser(r.Context(), email, hash); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		a.log.Error("create user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"email": email,
	})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := a.st.GetUserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrUserNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		a.log.Error("get user", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	plain, err := token.Mint(a.st, u.Email, expiresIn(10*time.Minute))
	if err != nil {
		a.log.Error("mint token", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"email": u.Email, "session_token": plain})
}

func (a *API) handleTokens(w http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	sessionToken := strings.TrimPrefix(authz, "Bearer ")
	if sessionToken == "" || authz == sessionToken {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	userID, err := token.ValidateSession(a.st, sessionToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	var req struct {
		Expires string `json:"expires"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	expiresAt := ""
	if req.Expires != "never" {
		d, ok := tokenTTLs[req.Expires]
		if !ok {
			writeError(w, http.StatusBadRequest, "expires must be one of 1h, 1d, 1w, never")
			return
		}
		expiresAt = expiresIn(d)
	}

	plain, err := token.Mint(a.st, userID, expiresAt)
	if err != nil {
		a.log.Error("mint token", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_token": plain})
}

func expiresIn(d time.Duration) string {
	return time.Now().UTC().Add(d).Format(time.RFC3339)
}
