package errx

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
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
	if !hasStack(err) {
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
	if !hasStack(err) {
		stack = callers(3)
	}

	return &Error{
		err:   err,
		attrs: attr.Args(args...),
		stack: stack,
	}
}

// hasStack reports whether the error chain already carries a stack trace.
// Detects *errx.Error stacks natively, and external stacks from any error that
// implements a StackTrace() method (pkg/errors, cockroachdb/errors, etc.).
func hasStack(err error) bool {
	for err != nil {
		if ee, ok := err.(*Error); ok && len(ee.stack) > 0 {
			return true
		}
		if externalStack(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// externalStack detects stack traces from external error packages by checking for
// a StackTrace() method via reflection. This avoids importing any external package
// while remaining compatible with pkg/errors, cockroachdb/errors, and others.
func externalStack(err error) bool {
	rv := reflect.ValueOf(err)
	if !rv.IsValid() {
		return false
	}
	m := rv.MethodByName("StackTrace")
	if !m.IsValid() {
		return false
	}
	mt := m.Type()
	if mt.NumIn() != 0 || mt.NumOut() != 1 {
		return false
	}
	result := m.Call(nil)[0]
	switch result.Kind() {
	case reflect.Slice:
		return result.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !result.IsNil()
	default:
		return result.IsValid()
	}
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