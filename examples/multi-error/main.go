// Package main demonstrates errx.Join for logging multiple errors as a single
// structured event — one log line per operation, not one per error.
//
// Two scenarios are shown:
//
//  1. Batch processing: N items processed in a loop; individual failures are
//     collected and logged together at the end.
//
//  2. Multi-field validation: a struct is validated against several rules;
//     all violations are collected and logged as one event.
//
// In both cases the caller decides when to log, not the inner logic.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

type Order struct {
	ID     string
	Amount int // cents
	Email  string
}

func main() {
	logger, cleanup, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		Console:     true,
		ConsoleJSON: false,
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

	runBatch(logger)
	fmt.Fprintln(os.Stderr)
	runValidation(logger)
}

// runBatch processes a list of orders. Errors from individual items are
// collected and logged once at the end — one log event for the whole batch.
func runBatch(logger *slog.Logger) {
	orders := []Order{
		{ID: "o-1", Amount: 1999, Email: "alice@example.com"},
		{ID: "o-2", Amount: -50, Email: "bob@example.com"},    // negative amount
		{ID: "o-3", Amount: 500, Email: "charlie@example.com"},
		{ID: "o-4", Amount: 750, Email: ""},                   // missing email
	}

	logger.Info("starting batch", slog.Int("total", len(orders)))

	var errs []error
	for _, order := range orders {
		if err := chargeOrder(order); err != nil {
			// Collect the failure — do NOT log here.
			errs = append(errs, err)
		}
	}

	succeeded := len(orders) - len(errs)

	// errx.Join returns nil when all inputs are nil, so this check is safe
	// even when every order succeeds.
	if joined := errx.Join(errs...); joined != nil {
		// One log event for the entire batch, with every failure structured
		// under errors.0.*, errors.1.*, etc.
		logger.Error("batch completed with failures",
			slog.Any("errors", joined),
			slog.Int("succeeded", succeeded),
			slog.Int("failed", len(errs)),
			slog.Int("total", len(orders)),
		)
		return
	}

	logger.Info("batch completed successfully", slog.Int("total", len(orders)))
}

func chargeOrder(order Order) error {
	if order.Amount <= 0 {
		return errx.New("invalid amount",
			"order_id", order.ID,
			"amount_cents", order.Amount,
		)
	}
	if order.Email == "" {
		return errx.New("missing email",
			"order_id", order.ID,
		)
	}
	return nil
}

// runValidation validates a struct against multiple rules and logs all
// violations as a single event — not one log per violation.
type UserInput struct {
	Name     string
	Email    string
	Password string
}

func runValidation(logger *slog.Logger) {
	input := UserInput{
		Name:     "Al",              // too short
		Email:    "not-an-email",    // invalid format
		Password: "secret",          // too short
	}

	// validateUser returns a *MultiError (via errx.Join) or nil.
	if err := validateUser(input); err != nil {
		// One log event with all validation failures structured as a list.
		logger.Warn("user input invalid", slog.Any("errors", err))
		return
	}

	logger.Info("user input valid")
}

func validateUser(u UserInput) error {
	var errs []error

	if len(u.Name) < 3 {
		errs = append(errs, errx.New("name too short",
			"field", "name",
			"min_length", 3,
			"got_length", len(u.Name),
		))
	}
	if !strings.Contains(u.Email, "@") {
		errs = append(errs, errx.New("invalid email",
			"field", "email",
			"value", u.Email,
		))
	}
	if len(u.Password) < 8 {
		errs = append(errs, errx.New("password too short",
			"field", "password",
			"min_length", 8,
			"got_length", len(u.Password),
		))
	}

	// errx.Join filters nils automatically and returns nil when errs is empty.
	return errx.Join(errs...)
}

// Ensure *errx.MultiError participates in errors.Is / errors.As chains.
// This is a compile-time demonstration — not needed in production code.
var _ = func() bool {
	e1 := errx.New("sentinel")
	joined := errx.Join(e1, errors.New("other"))
	return errors.Is(joined, e1) // always true
}
