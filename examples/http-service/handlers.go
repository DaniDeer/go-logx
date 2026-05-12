package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

// UserService handles user-related operations.
//
// logger is reserved for lifecycle events (startup, background jobs, health probes)
// that run outside any request context. Request-scoped methods use logx.FromContext(ctx)
// instead, which carries request_id, user_id, and any other fields added by the middleware
// chain — and falls back to slog.Default() when called outside a request.
type UserService struct {
	logger *slog.Logger
}

func NewUserService(logger *slog.Logger) *UserService {
	return &UserService{
		logger: logger.With(slog.String("component", "user_service")),
	}
}

// FetchUsers retrieves all users.
//
// logx.FromContext(ctx) gives the fully-enriched request logger when called from a handler.
// Outside a request context (background jobs, tests) it falls back to slog.Default().
// This means the method never needs to branch on "is there a request?" — the context handles it.
func (s *UserService) FetchUsers(ctx context.Context) ([]User, error) {
	logger := logx.FromContext(ctx)

	logger.Debug("querying database")

	time.Sleep(150 * time.Millisecond)

	logger.Debug("database query completed")

	return []User{
		{ID: "u-100", Email: "alice@example.com"},
		{ID: "u-200", Email: "invalid-email"},
	}, nil
}

func (s *server) handleUsers(w http.ResponseWriter, r *http.Request) {

	// Extract the context from the ResponseWriter — it carries the logger enriched by the
	// middleware chain (request_id, user_id, etc.) without inflating r.Context().
	var ctx context.Context
	if ctxWriter, ok := w.(interface{ Context() context.Context }); ok {
		ctx = ctxWriter.Context()
	} else {
		// Fallback to request context if not wrapped (e.g., during testing)
		ctx = r.Context()
	}

	logger := logx.FromContext(ctx)

	users, err := s.userService.FetchUsers(ctx)
	if err != nil {
		err = errx.Wrap(err, "failed to fetch users", "component", "user_service")
		logger.Error("request failed", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("users fetched successfully", slog.Int("user_count", len(users)))

	// validateUser does not log — it returns structured errors for the caller to log
	// with the full request context.
	for _, user := range users {
		if err := validateUser(user); err != nil {
			err = errx.With(
				fmt.Errorf("user validation failed: %w", err),
				"user_id", user.ID,
				"user_email", user.Email,
			)
			logger.Error("request failed", slog.Any("error", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	logger.Info("all users validated successfully")

	_, err = w.Write([]byte("ok\n"))
	if err != nil {
		logger.Error("failed to write response", slog.Any("error", err))
	}
}

// validateUser checks if the user's email is valid.
// Returns a structured error for the caller to wrap and log with request context.
func validateUser(user User) error {

	if user.Email == "" {
		return errx.New("email is empty", "validation", "email_required")
	}

	if user.Email == "invalid-email" {
		return errx.With(
			errors.New("invalid email format"),
			"validation", "email_format",
			"email", user.Email,
		)
	}

	return nil
}

