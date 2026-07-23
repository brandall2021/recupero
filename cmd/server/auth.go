package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrUserExists = errors.New("email already taken")
)

type authStore struct{ db *sql.DB }

type userRow struct {
	ID       int64
	Email    string
	Name     string
	Password string
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
		id         BIGSERIAL PRIMARY KEY,
		email      TEXT UNIQUE NOT NULL,
		name       TEXT NOT NULL DEFAULT '',
		password   TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return nil, err
	}
	// Migration: add name column if missing (for existing installs)
	_, _ = db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT ''`)
	return &authStore{db: db}, nil
}

func (s *authStore) Seed(ctx context.Context) error {
	users := []struct{ email, password, name string }{
		{"admin@wacalls.com", "admin123", "Administrador"},
		{"operador@wacalls.com", "operador123", "Operador"},
		{"demo@wacalls.com", "demo123", "Demo"},
	}
	for _, u := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO users (email, name, password) VALUES ($1, $2, $3)
			 ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name, password = EXCLUDED.password`,
			u.email, u.name, string(hash),
		); err != nil {
			return fmt.Errorf("seed user %s: %w", u.email, err)
		}
	}
	return nil
}

func (s *authStore) Register(ctx context.Context, email, name, password string) (int64, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	if email == "" || password == "" {
		return 0, ErrBadLogin
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, name, password) VALUES ($1, $2, $3) RETURNING id`,
		email, name, string(hash),
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return 0, ErrUserExists
		}
		return 0, err
	}
	return id, nil
}

func (s *authStore) Login(ctx context.Context, email, password string) (*userRow, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var u userRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, password FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password)
	if err != nil {
		return nil, ErrBadLogin
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return nil, ErrBadLogin
	}
	return &u, nil
}

func (s *authStore) getByID(ctx context.Context, id int64) (*userRow, error) {
	var u userRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, password FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Password)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *authStore) listAll(ctx context.Context) ([]userRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, email, name, '' FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Password); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *authStore) updatePassword(ctx context.Context, id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password = $1 WHERE id = $2`, string(hash), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *authStore) updateName(ctx context.Context, id int64, name string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET name = $1 WHERE id = $2`, strings.TrimSpace(name), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *authStore) delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

type jwtClaims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func generateToken(userID int64, email string) (string, error) {
	claims := jwtClaims{
		UserID: userID,
		Email:  email,
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
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	id, err := s.auth.Register(r.Context(), body.Email, body.Name, body.Password)
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
	token, err := generateToken(id, strings.TrimSpace(strings.ToLower(body.Email)))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  map[string]any{"id": id, "email": strings.TrimSpace(strings.ToLower(body.Email)), "name": body.Name},
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	u, err := s.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := generateToken(u.ID, u.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  map[string]any{"id": u.ID, "email": u.Email, "name": u.Name},
	})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractToken(r)
	if tokenStr == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing token"})
		return
	}
	claims, err := parseToken(tokenStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	u, err := s.auth.getByID(r.Context(), claims.UserID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "name": u.Name},
	})
}

func (s *server) handleUserList(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.listAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type userJSON struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	out := make([]userJSON, len(users))
	for i, u := range users {
		out[i] = userJSON{ID: u.ID, Email: u.Email, Name: u.Name}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}
	id, err := s.auth.Register(r.Context(), body.Email, body.Name, body.Password)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, ErrUserExists) {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{"id": id, "email": strings.TrimSpace(strings.ToLower(body.Email)), "name": body.Name},
	})
}

func (s *server) handleUserUpdatePassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password required"})
		return
	}
	if err := s.auth.updatePassword(r.Context(), id, body.Password); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if body.Name != "" {
		if err := s.auth.updateName(r.Context(), id, body.Name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
	}
	if body.Password != "" {
		if err := s.auth.updatePassword(r.Context(), id, body.Password); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	if err := s.auth.delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleReseed(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Seed(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "users seeded"})
}
