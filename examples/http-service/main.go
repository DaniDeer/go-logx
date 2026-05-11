package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func main() {

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	status := run(ctx, cancel, *httpPort)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int) int {

	// Initialize the logger with the desired configuration.
	// If initialization fails, print the error to stderr and exit with a non-zero status code.
	logger, cleanup, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		AddSource:   true,
		Console:     true,
		ConsoleJSON: false,
		File:        "service.log",
		FileLevel:   slog.LevelInfo,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		return 1
	}

	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up logger: %v\n", err)
		}
	}()

	srv := NewServer(httpPort, cancel, logger)

	var serverErr error

	go func() {
		logger.Debug("starting http server")
		serverErr = srv.Start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.Debug("shutting down http server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", slog.Any("error", err))
		return 1
	}
	if serverErr != nil {
		logger.Error("http server failed", slog.Any("error", serverErr))
		return 1
	}
	logger.Debug("http server shutdown complete")
	return 0
}

func handleUsers(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
) error {

	logger := logx.FromContext(ctx)

	logger.Debug("fetching users")

	users, err := fetchUsers(ctx)

	if err != nil {
		return errx.Wrap(
			err,
			"failed to fetch users",
			"component", "user_service",
		)
	}

	logger.Info(
		"users fetched",
		slog.Int("count", len(users)),
	)

	for _, user := range users {

		if err := validateUser(user); err != nil {

			return errx.With(
				fmt.Errorf(
					"user validation failed: %w",
					err,
				),
				"user_id", user.ID,
				"user_email", user.Email,
			)
		}
	}

	_, err = w.Write([]byte("ok\n"))

	if err != nil {
		return errx.Wrap(
			err,
			"failed to write response",
		)
	}

	return nil
}
