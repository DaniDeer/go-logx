package errx

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/DaniDeer/go-logx/attr"
)

type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type Error struct {
	err   error
	attrs []slog.Attr
	stack []StackFrame
}

func New(msg string, args ...any) error {
	return &Error{
		err:   errors.New(msg),
		attrs: attr.Args(args...),
		stack: callers(3),
	}
}

func Wrap(err error, msg string, args ...any) error {
	if err == nil {
		return nil
	}

	var stack []StackFrame
	if !hasErrxStack(err) {
		stack = callers(3)
	}

	return &Error{
		err:   fmt.Errorf("%s: %w", msg, err),
		attrs: attr.Args(args...),
		stack: stack,
	}
}

func With(err error, args ...any) error {
	if err == nil {
		return nil
	}

	var stack []StackFrame
	if !hasErrxStack(err) {
		stack = callers(3)
	}

	return &Error{
		err:   err,
		attrs: attr.Args(args...),
		stack: stack,
	}
}

// hasErrxStack reports whether any *Error in the chain has a non-empty stack.
func hasErrxStack(err error) bool {
	for err != nil {
		if ee, ok := err.(*Error); ok && len(ee.stack) > 0 {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

func (e *Error) Error() string {
	return e.err.Error()
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) Attrs() []slog.Attr {
	return e.attrs
}

func callers(skip int) []StackFrame {
	pcs := make([]uintptr, 32)

	n := runtime.Callers(skip, pcs)

	frames := runtime.CallersFrames(pcs[:n])

	var out []StackFrame

	for {
		frame, more := frames.Next()

		out = append(out, StackFrame{
			Function: frame.Function,
			File:     frame.File,
			Line:     frame.Line,
		})

		if !more {
			break
		}
	}

	return out
}