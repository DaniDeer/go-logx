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

// stackProvider is implemented by errors that carry normalized stack frames.
// *errx.Error satisfies this natively. External error types may implement
// StackFrames() []StackFrame to integrate fully with errx's display logic.
type stackProvider interface {
	StackFrames() []StackFrame
}

// stackTracer is a generic interface for external errors that carry a stack trace
// and can express it as an untyped value. Errors that implement StackTrace() any
// satisfy this interface and are detected before the reflect fallback.
// Note: pkg/errors errors do NOT satisfy this — their StackTrace() returns a
// concrete type, not any. Those are handled by the reflect fallback.
type stackTracer interface {
	StackTrace() any
}

// hasStack reports whether the error chain already carries a stack trace.
// Detection is layered: (1) stackProvider (errx-native), (2) stackTracer (explicit
// external contract), (3) reflect fallback for pkg/errors and similar libraries.
func hasStack(err error) bool {
	for err != nil {
		if sp, ok := err.(stackProvider); ok && len(sp.StackFrames()) > 0 {
			return true
		}
		if externalStack(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// externalStack detects stack traces from external error packages.
// It tries the stackTracer interface first (explicit contract), then falls back
// to reflection for libraries like pkg/errors whose StackTrace() returns a
// concrete type that cannot satisfy a generic interface.
func externalStack(err error) bool {
	var st stackTracer
	if errors.As(err, &st) {
		return stackValueNonEmpty(reflect.ValueOf(st.StackTrace()))
	}
	return reflectHasStack(err)
}

// reflectHasStack detects a StackTrace() method on err via reflection.
// Used as a last resort for libraries (e.g. pkg/errors) that define StackTrace()
// with a concrete return type, which prevents interface matching.
func reflectHasStack(err error) bool {
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
	return stackValueNonEmpty(m.Call(nil)[0])
}

// stackValueNonEmpty reports whether rv represents a non-empty stack value.
// Handles all nilable kinds to avoid reflect.Value.IsNil panics.
func stackValueNonEmpty(rv reflect.Value) bool {
	if !rv.IsValid() {
		return false
	}
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		return rv.Len() > 0
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Chan, reflect.Func:
		return !rv.IsNil()
	default:
		return true
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

// StackFrames returns the stack frames captured for this error.
// It satisfies the stackProvider interface.
func (e *Error) StackFrames() []StackFrame {
	return e.stack
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
