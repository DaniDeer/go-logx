package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/DaniDeer/go-logx/errx"
)

// UserService handles user-related operations.
//
// logger is used for lifecycle events (startup, background jobs) that run
// outside any request context. It does not carry request-scoped fields —
// those are captured in the LogContext by the middleware chain and emitted
// as a single consolidated log line per request.
type UserService struct {
	logger *slog.Logger
}

// NewUserService creates a UserService with a component-tagged logger.
func NewUserService(logger *slog.Logger) *UserService {
	return &UserService{
		logger: logger.With(slog.String("component", "user_service")),
	}
}

// FetchUsers retrieves all users.
//
// The service uses its own injected logger. Request-scoped telemetry
// (user, status, duration) is handled by the middleware chain via LogContext,
// not by propagating an enriched logger through the context.
func (s *UserService) FetchUsers(ctx context.Context) ([]User, error) {
	s.logger.Debug("querying database")

	time.Sleep(150 * time.Millisecond)

	s.logger.Debug("database query completed")

	return []User{
		{ID: "u-100", Email: "alice@example.com"},
		{ID: "u-200", Email: "invalid-email"},
	}, nil
}

// handleUsers is an HTTP handler on server, separate from UserService.
// It delegates business logic to UserService and captures any errors in the
// LogContext via httpError so the consolidated request log carries them.
func (s *server) handleUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	users, err := s.userService.FetchUsers(ctx)
	if err != nil {
		httpError(ctx, w, http.StatusInternalServerError,
			errx.Wrap(err, "failed to fetch users", "component", "user_service"))
		return
	}

	for _, user := range users {
		if err := validateUser(user); err != nil {
			httpError(ctx, w, http.StatusInternalServerError,
				errx.With(
					fmt.Errorf("user validation failed: %w", err),
					"user_id", user.ID,
					"user_email", user.Email,
				))
			return
		}
	}

	if _, err = w.Write([]byte("ok\n")); err != nil {
		// Response headers are already sent at this point, so httpError cannot
		// be used. Fall back to the server logger to record the write failure.
		s.logger.Error("failed to write response", slog.Any("error", err))
	}
}

// validateUser checks that the user's email is valid.
// Returns a structured error for the caller to capture via httpError.
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
