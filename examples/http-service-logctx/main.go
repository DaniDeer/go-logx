package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniDeer/go-logx/logx"
)

// User is the domain type returned by UserService.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	status := run(ctx, cancel, httpPort)
	cancel()
	os.Exit(status)
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort *int) int {
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

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
