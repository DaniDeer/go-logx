package main

import (
	"context"
	"net/http"
)

type contextKey int

const logContextKey contextKey = 0

// LogContext accumulates request-scoped metadata during the handler chain.
// The outer requestLogger middleware reads it after the chain returns to emit
// one consolidated log line per request.
type LogContext struct {
	RequestID string
	Username  string
	Error     error
}

// httpError stashes err in the LogContext (if present) so the consolidated
// request log captures it, then sends the HTTP error response.
func httpError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
		logCtx.Error = err
	}
	http.Error(w, err.Error(), status)
}
