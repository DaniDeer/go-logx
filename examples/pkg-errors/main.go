// Package main demonstrates errx integration with pkg/errors.
//
// When an error from pkg/errors (or any library that implements StackTrace())
// is passed to errx.Wrap or errx.With, errx detects the existing stack via
// reflection and skips capturing a new one. This prevents redundant stack
// captures when integrating with codebases already using pkg/errors.
package main

import (
	"log/slog"
	"os"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
	pkgerrors "github.com/pkg/errors"
)

func queryDB() error {
	// Simulates a pkg/errors-style error from a library or legacy codebase.
	return pkgerrors.New("connection refused")
}

func fetchUser(id string) error {
	err := queryDB()
	if err != nil {
		// pkg/errors already captured a stack in queryDB.
		// errx.Wrap detects it via StackTrace() and skips a redundant capture.
		return errx.Wrap(err, "failed to fetch user", "user_id", id)
	}
	return nil
}

func main() {
	logger, cleanup, err := logx.New(logx.Config{
		Console: true,
	})
	if err != nil {
		slog.Error("failed to create logger", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := fetchUser("u-42"); err != nil {
		logger.Error("request failed", slog.Any("error", err))
	}
}
