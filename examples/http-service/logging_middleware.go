package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/DaniDeer/go-logx/logx"
)

// http.ResponseWriter is an interface, so we wrap it to capture the status code
// and bytes written — details that net/http doesn't expose after the fact.

// spyReadCloser wraps io.ReadCloser and counts the bytes read from the request body.
type spyReadCloser struct {
	io.ReadCloser
	bytesRead int
}

func (r *spyReadCloser) Read(b []byte) (int, error) {
	n, err := r.ReadCloser.Read(b)
	r.bytesRead += n
	return n, err
}

// spyResponseWriter wraps http.ResponseWriter and captures the status code and bytes written.
type spyResponseWriter struct {
	http.ResponseWriter
	bytesWritten int
	statusCode   int
}

func (w *spyResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

func (w *spyResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// responseWriterWithContext wraps http.ResponseWriter and carries a custom context
// separate from r.Context(). This keeps request-scoped logger data out of the
// request context while still making it accessible to handlers via type assertion.
type responseWriterWithContext struct {
	http.ResponseWriter
	ctx context.Context
}

// Context returns the custom context carried by this response writer.
// Handlers retrieve the enriched logger via logx.FromContext(w.(interface{ Context() context.Context }).Context()).
func (w *responseWriterWithContext) Context() context.Context {
	return w.ctx
}

// reqLoggerMiddleware enriches the logger with request-scoped fields, stores it in a
// custom context on the ResponseWriter, and logs request start and completion.
// Using the ResponseWriter context instead of r.WithContext() keeps the request context clean.
func reqLoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			reqLogger := logger.With(
				slog.String("request_id", requestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
			)

			ctx := logx.WithLogger(r.Context(), reqLogger)

			reqLogger.Info("request started")
			start := time.Now()

			// Wrap body and writer to capture sizes and status code.
			sr := &spyReadCloser{ReadCloser: r.Body}
			r.Body = sr
			sw := &spyResponseWriter{ResponseWriter: w}
			// Forward the wrapped ResponseWriter with the custom context to the next handler. 
			// This allows handlers to access the enriched logger via the context on the ResponseWriter.
			next.ServeHTTP(&responseWriterWithContext{
				ResponseWriter: sw,
				ctx:            ctx,
			}, r)

			reqLogger.Info(
				"request completed",
				slog.Duration("duration", time.Since(start)),
				slog.Int("request_body_bytes", sr.bytesRead),
				slog.Int("response_status", sw.statusCode),
				slog.Int("response_body_bytes", sw.bytesWritten),
			)
		})
	}
}