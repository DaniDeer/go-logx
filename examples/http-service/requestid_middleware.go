package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey int

const requestIDContextKey requestIDKey = 0

const headerRequestID = "X-Request-ID"

// requestIDMiddleware propagates or generates a request ID for each request.
// If the incoming request carries an X-Request-ID header, that value is reused
// (allowing callers and proxies to trace requests end-to-end). Otherwise a new
// UUID is generated. In both cases the ID is stored in the request context and
// echoed back in the X-Request-ID response header.
// It must be the outermost middleware so the ID is available on all paths.
func requestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(headerRequestID)
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set(headerRequestID, id)
			r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey, id))
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDFrom retrieves the request ID from ctx.
// Returns an empty string if no ID is present (e.g., outside the middleware chain).
func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey).(string)
	return id
}
