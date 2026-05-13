package main

import (
	"errors"
	"net/http"
	"strings"
)

// authMiddleware extracts a Bearer token from the Authorization header and
// sets logCtx.Username so the consolidated request log includes the user.
//
// It must run inside requestLogger in the chain so rejection logs carry all
// request fields:
//
//	requestLogger(logger)(authMiddleware()(mux))
func authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpError(r.Context(), w, http.StatusUnauthorized,
					errors.New("missing or invalid authorization"))
				return
			}

			// Enrich the LogContext so the consolidated log line carries the user.
			if logCtx, ok := r.Context().Value(logContextKey).(*LogContext); ok {
				logCtx.Username = token
			}

			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	token, found := strings.CutPrefix(header, "Bearer ")
	if !found || token == "" {
		return "", false
	}
	return token, true
}
