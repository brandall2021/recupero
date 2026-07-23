package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecret     []byte
	ErrBadLogin   = errors.New("invalid credentials")
	ErrUserExists = errors.New("username already taken")
)

type authStore struct{ db *sql.DB }

type userRow struct {
	ID       int64
	Username string
	Password string // bcrypt hash
}

func initJWTSecret() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "wacalls-default-secret-change-me"
	}
	jwtSecret = []byte(secret)
}

func newAuthStore(ctx context.Context, db *sql.DB) (*authStore, error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id        BIGSERIAL PRIMARY KEY,
		username  TEXT UNIQUE NOT NULL,
		password  TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return nil, err
	}
	return &authStore{db: db}, nil
}

func (s *authStore) Seed(ctx context.Context) error {
	users := []struct{ username, password, role string }{
		{"admin@wacalls.com", "admin123", "Administrador"},
		{"operador@wacalls.com", "operador123", "Operador"},
		{"demo@wacalls.com", "demo123", "Demo"},
	}
	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, _ = s.db.ExecContext(ctx,
			`INSERT INTO users (username, password) VALUES ($1, $2) ON CONFLICT (username) DO NOTHING`,
			u.username, string(hash),
		)
	}
	return nil
}

func (s *authStore) Register(ctx context.Context, username, password string) (int64, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" || password == "" {
		return 0, ErrBadLogin
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id`,
		username, string(hash),
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return 0, ErrUserExists
		}
		return 0, err
	}
	return id, nil
}

func (s *authStore) Login(ctx context.Context, username, password string) (*userRow, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	var u userRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.Password)
	if err != nil {
		return nil, ErrBadLogin
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrBadLogin
	}
	return &u, nil
}

type jwtClaims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"sub"`
	jwt.RegisteredClaims
}

func generateToken(userID int64, username string) (string, error) {
	claims := jwtClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret)
}

func parseToken(tokenStr string) (*jwtClaims, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := t.Claims.(*jwtClaims); ok && t.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Username) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}
	id, err := s.auth.Register(r.Context(), body.Username, body.Password)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, ErrUserExists) {
			code = http.StatusConflict
		} else if errors.Is(err, ErrBadLogin) {
			code = http.StatusBadRequest
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	token, err := generateToken(id, strings.TrimSpace(strings.ToLower(body.Username)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"userId":   id,
		"username": strings.TrimSpace(strings.ToLower(body.Username)),
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Username) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}
	u, err := s.auth.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := generateToken(u.ID, u.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":    token,
		"userId":   u.ID,
		"username": u.Username,
	})
}
