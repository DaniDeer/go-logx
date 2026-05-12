package errx

import (
	"fmt"
	"log/slog"
	"strings"
)

// MultiError holds a flat list of non-nil errors collected from a batch
// operation or multi-field validation. It implements error, slog.LogValuer,
// and Unwrap() []error so that errors.Is and errors.As traverse all children.
type MultiError struct {
	errs []error
}

// Join collects non-nil errors into a single MultiError.
// Returns nil if every input is nil, so callers can do:
//
//	if joined := errx.Join(errs...); joined != nil { ... }
func Join(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	return &MultiError{errs: nonNil}
}

// Error returns all child messages joined by "; ".
func (m *MultiError) Error() string {
	msgs := make([]string, len(m.errs))
	for i, err := range m.errs {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "; ")
}

// Unwrap returns the list of child errors so that errors.Is and errors.As
// can traverse all of them (Go 1.20+ multi-error unwrap contract).
func (m *MultiError) Unwrap() []error {
	return m.errs
}

// LogValue serializes the error list as an indexed slog group.
// Each child appears under a numeric key ("0", "1", ...).
// Children that implement slog.LogValuer (e.g. *errx.Error) are rendered
// with their full structured output; plain errors render as their message string.
func (m *MultiError) LogValue() slog.Value {
	attrs := make([]slog.Attr, len(m.errs))
	for i, err := range m.errs {
		attrs[i] = slog.Attr{
			Key:   fmt.Sprintf("%d", i),
			Value: slog.AnyValue(err),
		}
	}
	return slog.GroupValue(attrs...)
}
