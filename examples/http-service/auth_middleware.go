package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/DaniDeer/go-logx/logx"
)

// authMiddleware extracts a Bearer token from the Authorization header and adds
// user_id to the logger already enriched by reqLoggerMiddleware.
//
// It must run inside reqLoggerMiddleware in the chain so the rejection and
// downstream logs carry both request fields and user_id:
//
//	reqLoggerMiddleware(logger)(authMiddleware()(mux))
func authMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Read the logger already enriched by reqLoggerMiddleware from the
			// ResponseWriter context. Fall back to the request context during testing.
			var ctx context.Context
			if cw, ok := w.(interface{ Context() context.Context }); ok {
				ctx = cw.Context()
			} else {
				ctx = r.Context()
			}
			logger := logx.FromContext(ctx)

			token, ok := bearerToken(r)
			if !ok {
				logger.Warn("request rejected: missing or invalid authorization")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			// Enrich the logger with user_id and propagate it via a new ResponseWriter context.
			authedCtx := logx.WithLogger(ctx, logger.With(slog.String("user_id", token)))
			next.ServeHTTP(&responseWriterWithContext{ResponseWriter: w, ctx: authedCtx}, r)
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
