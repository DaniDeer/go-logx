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

	return &Error{
		err: fmt.Errorf("%s: %w", msg, err),
		attrs: attr.Args(args...),
		stack: callers(3),
	}
}

func With(err error, args ...any) error {
	if err == nil {
		return nil
	}

	return &Error{
		err:   err,
		attrs: attr.Args(args...),
		stack: callers(3),
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