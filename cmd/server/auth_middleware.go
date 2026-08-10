package main

import (
	"context"
	"net/http"
	"strings"
)

const ctxUserKey ctxKey = "auth-user"

func withAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		r.Header.Set("X-User-Id", claims.Email)
		ctx := context.WithValue(r.Context(), ctxUserKey, claims)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withRole wraps withAuth and additionally requires an authenticated user with
// one of the given roles. It re-writes the auth context for downstream handlers.
func withRole(roles ...string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(h http.Handler) http.Handler {
		return withAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := userClaims(r)
			if claims == nil || !allowed[claims.Role] {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			h.ServeHTTP(w, r)
		}))
	}
}

func userClaims(r *http.Request) *jwtClaims {
	c, _ := r.Context().Value(ctxUserKey).(*jwtClaims)
	return c
}

func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
