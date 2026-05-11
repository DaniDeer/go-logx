package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
	"github.com/google/uuid"
)

type server struct {
	httpServer *http.Server
	cancel     context.CancelFunc
	logger     *slog.Logger
}

func NewServer(port int, cancel context.CancelFunc, logger *slog.Logger) *server {

	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: reqLoggerMiddleware(logger)(mux), // Wrap the mux with the request logging middleware to log all incoming requests.
	}

	s := &server{
		httpServer: srv,
		cancel:     cancel,
		logger:     logger,
	}

	// Define routes and handlers for the HTTP server.
	mux.HandleFunc("/users", s.handleUsers)

	return s

}

func (s *server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return errx.Wrap(err, "failed to start server", "component", "http_server")
	}

	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		port := tcpAddr.Port
		s.logger.Info("http server started", slog.Int("port", port))
	} else {
		s.logger.Info("http server started", slog.String("address", ln.Addr().String()))
	}

	if err := s.httpServer.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error("http server failed", slog.Any("error", err))
		return errx.Wrap(err, "http server failed", "component", "http_server")
	}

	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("http server shutdown failed", slog.Any("error", err))
		return errx.Wrap(err, "http server shutdown failed", "component", "http_server")
	}
	return nil
}

// responseWriterWithContext wraps http.ResponseWriter and carries a custom context
// separate from the request context. This demonstrates a pattern where framework-specific
// data (like loggers with request attributes) is kept separate from the request's context
// to avoid inflating it with implementation details.
type responseWriterWithContext struct {
	http.ResponseWriter
	ctx context.Context
}

// Context returns the custom context attached to this response writer.
// Handlers can use this to access request-scoped data without modifying the request context.
func (w *responseWriterWithContext) Context() context.Context {
	return w.ctx
}

// Define a middleware function that takes a logger and returns an http.Handler that wraps the next handler.
// The middleware will log the start and end of each request, along with the request method, path, remote address, and duration.
// The middleware uses a custom context pattern: instead of modifying the request's context with r.WithContext(),
// it wraps the ResponseWriter with a custom type that carries its own context. This keeps the request context
// clean while still providing request-scoped data to handlers.
func reqLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			requestID := uuid.NewString()

			reqLogger := logger.With(
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
			)

			// Create a custom context that includes the request-specific logger.
			// This context is kept separate from r.Context() to demonstrate an alternative pattern
			// where framework data doesn't inflate the request context.
			ctx := logx.WithLogger(r.Context(), reqLogger)

			// Log the start of the request with the request-specific logger.
			reqLogger.Info("request started")
			start := time.Now()

			// Forward the request to the next handler with a custom ResponseWriter that carries
			// the logger context. Handlers can extract this context using type assertion or a helper function.
			// This pattern keeps the original request context unchanged.
			next.ServeHTTP(&responseWriterWithContext{
				ResponseWriter: w,
				ctx:            ctx,
			}, r)

			// After the next handler returns, we can log the completion of the request with the duration.
			reqLogger.Info(
				"request completed",
				slog.Duration("duration", time.Since(start)),
			)

		})
	}
}
