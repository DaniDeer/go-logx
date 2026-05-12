// Package main demonstrates how to embed build metadata and static service
// attributes into every log line using logx.Config.DefaultAttrs and logx.Config.Build.
//
// This pattern is essential for canary and rolling deployments where multiple
// versions of a service run simultaneously — without build info on every log
// line it is impossible to tell which instance emitted a given event.
//
// # Injecting build info at compile time
//
// Set the three vars below via -ldflags when building for production:
//
//	go build -ldflags "
//	  -X main.version=v1.2.3
//	  -X main.commit=$(git rev-parse --short HEAD)
//	  -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
//	" ./examples/build-info/
//
// Running without -ldflags uses the "dev"/"none"/"unknown" defaults and falls
// back to VCS info embedded by the Go toolchain (vcs.revision, vcs.time).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

// Build vars — overridden at compile time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Fill in commit and date from embedded VCS info when ldflags were not used.
	// The Go toolchain embeds vcs.revision and vcs.time automatically when
	// building from a git-tracked directory (Go 1.18+).
	if commit == "none" || date == "unknown" {
		vcsCommit, vcsDate := readVCSInfo()
		if commit == "none" && vcsCommit != "" {
			commit = vcsCommit
		}
		if date == "unknown" && vcsDate != "" {
			date = vcsDate
		}
	}

	logger, cleanup, err := logx.New(logx.Config{
		Level:   slog.LevelDebug,
		Console: true,

		// DefaultAttrs are attached to every log line.
		// Use this for static, process-wide fields: service identity, deployment
		// topology, environment. These appear at the top level of each log record.
		DefaultAttrs: []slog.Attr{
			slog.String("service", "order-api"),
			slog.String("env", "production"),
			slog.String("region", "eu-west-1"),
		},

		// Build adds a "build" group to every log line.
		// build.go (Go runtime version) is always included automatically.
		Build: &logx.BuildInfo{
			Version: version,
			Commit:  commit,
			Date:    date,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up logger: %v\n", err)
		}
	}()

	run(ctx, logger)
}

func run(ctx context.Context, logger *slog.Logger) {
	// Every log line below carries:
	//   service=order-api env=production region=eu-west-1
	//   build.version=v1.2.3 build.commit=abc123 build.date=... build.go=go1.26.2
	logger.Info("service started", slog.String("listen", ":8080"))

	// Simulate a request handled successfully.
	logger.Debug("processing order", slog.String("order_id", "o-1"))
	logger.Info("order processed", slog.String("order_id", "o-1"))

	// Simulate a failure — build info identifies which version is misbehaving.
	err := errx.Wrap(
		errx.New("payment gateway timeout", "gateway", "stripe", "timeout_ms", 3000),
		"charge failed",
		"order_id", "o-2",
		"user_id", "u-99",
	)
	logger.Error("request failed", slog.Any("error", err))

	// Propagate the enriched logger through context for downstream callers.
	ctx = logx.WithLogger(ctx, logger)
	handleShutdown(ctx)
}

func handleShutdown(ctx context.Context) {
	// logx.FromContext retrieves the logger that already carries all default
	// attrs and build info — no need to thread them through function arguments.
	logger := logx.FromContext(ctx)
	logger.Info("service stopped")
}

// readVCSInfo reads vcs.revision and vcs.time from the Go toolchain's embedded
// build settings. These are populated automatically when building from a
// git-tracked source tree without any extra flags.
func readVCSInfo() (commit, date string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 12 {
				commit = s.Value[:12] // short SHA
			} else {
				commit = s.Value
			}
		case "vcs.time":
			date = s.Value
		}
	}
	return commit, date
}
