package main

import (
	"net/http"
	"strings"
)

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
		r.Header.Set("X-User-Id", claims.Username)
		h.ServeHTTP(w, r)
	})
}

func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}
