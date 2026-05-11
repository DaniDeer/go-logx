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


func (s *server) handleUsers(w http.ResponseWriter, r *http.Request) {
	
	// Extract the custom context from the response writer instead of using r.Context().
	// This demonstrates accessing framework-specific data (like the logger with request attributes)
	// without inflating the request's context. The middleware wraps the ResponseWriter with a
	// custom type that carries its own context.
	var ctx context.Context
	if ctxWriter, ok := w.(interface{ Context() context.Context }); ok {
		ctx = ctxWriter.Context()
	} else {
		// Fallback to request context if not wrapped (e.g., during testing)
		ctx = r.Context()
	}

	logger := logx.FromContext(ctx)

	logger.Debug("fetching users")

	// Business Logic...
	// Fetch users from the database, validate them, and return an error if any step fails.
	// Pass the context to fetchUsers so that it can log with the same request-specific information that was added by the middleware.
	users, err := fetchUsers(ctx)
	if err != nil {
		// Wrap the error with additional context about the failure, including structured attributes for the component that failed. This will help with debugging and understanding the error in the logs.
		err = errx.Wrap(
			err,
			"failed to fetch users",
			"component", "user_service",
		)
		// Log the error with the request-specific logger, which will include the request ID and other relevant information in the log output.
		logger.Error(
			"request failed",
			slog.Any("error", err),
		)
		// Return a generic error response to the client without exposing internal details. The error has already been logged with all the necessary context for debugging.
		http.Error(
			w,
			"internal server error",
			http.StatusInternalServerError,
		)
		return
	}

	logger.Info("users fetched successfully",
		slog.Int("user_count", len(users)),
	)

	// Validate each user and return an error if validation fails. The error will include structured attributes for the user ID and email, which will help with debugging.
	// This time we do not pass the context to validateUser, because it does not need to log with the request-specific information. 
	// Instead, it returns an error with structured attributes that can be wrapped and logged by the caller.
	for _, user := range users {
		if err := validateUser(user); err != nil {
			err = errx.With(
				fmt.Errorf("user validation failed: %w", err),
				"user_id", user.ID,
				"user_email", user.Email,
			)
			logger.Error(
				"request failed",
				slog.Any("error", err),
			)
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
			return
		}
	}

	logger.Info("all users validated successfully")

	_, err = w.Write([]byte("ok\n"))
	if err != nil {
		logger.Error(
			"failed to write response",
			slog.Any("error", err),
		)
	}
}

// fetchUsers simulates a database query to fetch users. 
// It logs the start and completion of the database query, and returns a list of users or an error if the query fails.
// It accesses the logger from the context, which allows it to log with the same request-specific information that was added by the middleware.
func fetchUsers(ctx context.Context) ([]User, error) {
	
	// The logger is of type *slog.Logger, which is the standard logger type used by the slog package.
	// By using dependency injection to pass the logger to the application that implements the business logic, 
	// we can ensure that all parts of the application have access to the same logger instance and can log with consistent structured attributes.
	logger := logx.FromContext(ctx)

	logger.Debug("querying database")

	time.Sleep(150 * time.Millisecond)

	logger.Debug("database query completed")

	return []User{
		{
			ID:    "u-100",
			Email: "alice@example.com",
		},
		{
			ID:    "u-200",
			Email: "invalid-email",
		},
	}, nil
}

// validateUser checks if the user's email is valid. If the email is empty or has an invalid format.
// It returns an error with structured attributes that provide context about the validation failure.
// This error can then be wrapped and logged by the caller, which will include the user ID and email in the log output for easier debugging.
func validateUser(user User) error {

	if user.Email == "" {

		return errx.New(
			"email is empty",
			"validation", "email_required",
		)
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