// Package main demonstrates logx.Config.ConsoleJSON.
//
// When ConsoleJSON is false (default), console output uses the human-readable
// slog text format. When ConsoleJSON is true, console output switches to JSON —
// useful in environments where logs are ingested by a structured log aggregator
// (e.g. Cloud Logging, Datadog, Loki) that parses JSON directly.
//
// This example initializes two loggers with identical settings except for
// ConsoleJSON and logs the same event through both, making the format
// difference immediately visible.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

func main() {
	textLogger, cleanupText, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		Console:     true,
		ConsoleJSON: false, // human-readable text (default)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize text logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := cleanupText(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up text logger: %v\n", err)
		}
	}()

	jsonLogger, cleanupJSON, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		Console:     true,
		ConsoleJSON: true, // structured JSON — for log aggregators
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize json logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := cleanupJSON(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up json logger: %v\n", err)
		}
	}()

	// Build a sample error so the output includes nested errx fields.
	svcErr := errx.Wrap(
		errx.New("connection refused", "host", "db.internal", "port", 5432),
		"query failed",
		"table", "users",
		"user_id", "u-99",
	)

	fmt.Fprintln(os.Stderr, "--- ConsoleJSON: false (text) ---")
	textLogger.Error("request failed",
		slog.String("method", "GET"),
		slog.String("path", "/users"),
		slog.Any("error", svcErr),
	)

	fmt.Fprintln(os.Stderr, "\n--- ConsoleJSON: true (JSON) ---")
	jsonLogger.Error("request failed",
		slog.String("method", "GET"),
		slog.String("path", "/users"),
		slog.Any("error", svcErr),
	)
}
