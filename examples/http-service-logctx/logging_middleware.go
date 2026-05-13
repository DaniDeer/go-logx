package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// spyReadCloser wraps io.ReadCloser and counts bytes read from the request body.
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

// requestLogger creates a LogContext, stores it in the request context so
// downstream middlewares and handlers can enrich it, then emits one
// consolidated log line after the entire handler chain returns.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Create a LogContext and store it in the request context so that
			// handlers and middlewares can add attributes to it.
			logCtx := &LogContext{RequestID: requestIDFrom(r.Context())}
			r = r.WithContext(context.WithValue(r.Context(), logContextKey, logCtx))

			start := time.Now()

			// Spy wrappers capture bytes read/written and the response status
			// without any changes to handler code.
			sr := &spyReadCloser{ReadCloser: r.Body}
			r.Body = sr
			sw := &spyResponseWriter{ResponseWriter: w}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			// Read back the (possibly enriched) LogContext from the request context.
			logCtx, _ = r.Context().Value(logContextKey).(*LogContext)

			attrs := []any{
				slog.String("request_id", logCtx.RequestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("client_ip", r.RemoteAddr),
				slog.Duration("duration", duration),
				slog.Int("request_body_bytes", sr.bytesRead),
				slog.Int("response_status", sw.statusCode),
				slog.Int("response_body_bytes", sw.bytesWritten),
			}

			if logCtx != nil && logCtx.Username != "" {
				attrs = append(attrs, slog.String("user", logCtx.Username))
			}

			if logCtx != nil && logCtx.Error != nil {
				attrs = append(attrs, slog.Any("error", logCtx.Error))
			}

			logger.Info("served request", attrs...)
		})
	}
}
