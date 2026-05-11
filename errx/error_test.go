package errx

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
)

func Test_New(t *testing.T) {
	tests := []struct {
		name      string
		msg       string
		args      []any
		wantMsg   string
		wantAttrs []slog.Attr
	}{
		{
			name:      "message only",
			msg:       "something failed",
			wantMsg:   "something failed",
			wantAttrs: []slog.Attr{},
		},
		{
			name:      "message with attrs",
			msg:       "validation failed",
			args:      []any{"field", "email", "value", "bad"},
			wantMsg:   "validation failed",
			wantAttrs: []slog.Attr{slog.Any("field", "email"), slog.Any("value", "bad")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.msg, tt.args...)

			if err.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.wantMsg)
			}

			e, ok := err.(*Error)
			if !ok {
				t.Fatal("expected *Error")
			}

			assertAttrs(t, e.Attrs(), tt.wantAttrs)
		})
	}
}

func Test_Wrap(t *testing.T) {
	base := errors.New("base error")

	tests := []struct {
		name      string
		err       error
		msg       string
		args      []any
		wantNil   bool
		wantMsg   string
		wantAttrs []slog.Attr
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			msg:     "ignored",
			wantNil: true,
		},
		{
			name:      "wraps with message prefix",
			err:       base,
			msg:       "operation failed",
			wantMsg:   "operation failed: base error",
			wantAttrs: []slog.Attr{},
		},
		{
			name:      "wraps with attrs",
			err:       base,
			msg:       "operation failed",
			args:      []any{"component", "db"},
			wantMsg:   "operation failed: base error",
			wantAttrs: []slog.Attr{slog.Any("component", "db")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.msg, tt.args...)

			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}

			if got.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), tt.wantMsg)
			}

			e, ok := got.(*Error)
			if !ok {
				t.Fatal("expected *Error")
			}

			assertAttrs(t, e.Attrs(), tt.wantAttrs)
		})
	}
}

func Test_With(t *testing.T) {
	base := errors.New("original message")

	tests := []struct {
		name      string
		err       error
		args      []any
		wantNil   bool
		wantMsg   string
		wantAttrs []slog.Attr
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			wantNil: true,
		},
		{
			name:      "preserves original message",
			err:       base,
			args:      []any{"user_id", "u-1"},
			wantMsg:   "original message",
			wantAttrs: []slog.Attr{slog.Any("user_id", "u-1")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := With(tt.err, tt.args...)

			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}

			if got.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), tt.wantMsg)
			}

			e, ok := got.(*Error)
			if !ok {
				t.Fatal("expected *Error")
			}

			assertAttrs(t, e.Attrs(), tt.wantAttrs)
		})
	}
}

func Test_Attrs(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantAttrs []slog.Attr
	}{
		{
			name:      "nil error",
			err:       nil,
			wantAttrs: nil,
		},
		{
			name:      "plain error has no attrs",
			err:       errors.New("plain"),
			wantAttrs: nil,
		},
		{
			name: "single errx.Error",
			err:  New("fail", "k", "v"),
			wantAttrs: []slog.Attr{
				slog.Any("k", "v"),
			},
		},
		{
			name: "wrapped chain: outer attrs take precedence on duplicate keys",
			err: Wrap(
				With(errors.New("base"), "k", "inner", "only_inner", "yes"),
				"outer",
				"k", "outer",
			),
			wantAttrs: []slog.Attr{
				slog.Any("k", "outer"),
				slog.Any("only_inner", "yes"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Attrs(tt.err)
			assertAttrs(t, got, tt.wantAttrs)
		})
	}
}

func Test_LogValue_causeHasNoStackTrace(t *testing.T) {
	inner := New("inner error", "detail", "x")
	outer := Wrap(inner, "outer error", "component", "db")

	val := outer.(*Error).LogValue()
	group := val.Group()

	// Locate the "cause" attr in the outermost group.
	var causeVal *slog.Value
	for _, a := range group {
		if a.Key == "cause" {
			v := a.Value
			causeVal = &v
			break
		}
	}

	if causeVal == nil {
		t.Fatal("expected 'cause' attr in LogValue output")
	}

	// Cause group must not contain a "stack_trace" key.
	for _, a := range causeVal.Group() {
		if a.Key == "stack_trace" {
			t.Error("cause group must not contain stack_trace; only the outermost error includes a stack trace")
		}
	}
}

func Test_LogValue_outerHasStackTrace(t *testing.T) {
	err := New("something failed", "k", "v")

	val := err.(*Error).LogValue()

	var found bool
	for _, a := range val.Group() {
		if a.Key == "stack_trace" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected outermost error to have stack_trace in LogValue output")
	}
}

func Test_Wrap_singleStackPerChain(t *testing.T) {
	inner := New("root cause", "k", "v")
	middle := Wrap(inner, "middle context", "layer", "service")
	outer := Wrap(middle, "outer context", "component", "handler")

	innerE := inner.(*Error)
	middleE := middle.(*Error)
	outerE := outer.(*Error)

	if len(innerE.stack) == 0 {
		t.Error("inner (first errx.Error) should have a stack")
	}
	if len(middleE.stack) != 0 {
		t.Error("middle Wrap should not capture a new stack when inner already has one")
	}
	if len(outerE.stack) != 0 {
		t.Error("outer Wrap should not capture a new stack when inner already has one")
	}

	// LogValue should still surface the innermost stack_trace.
	val := outerE.LogValue()
	var found bool
	for _, a := range val.Group() {
		if a.Key == "stack_trace" {
			found = true
			break
		}
	}
	if !found {
		t.Error("LogValue should show stack_trace from innermost errx.Error in the chain")
	}
}

func Test_With_singleStackPerChain(t *testing.T) {
	inner := New("root cause")
	enriched := With(inner, "ctx", "yes")

	innerE := inner.(*Error)
	enrichedE := enriched.(*Error)

	if len(innerE.stack) == 0 {
		t.Error("inner (first errx.Error) should have a stack")
	}
	if len(enrichedE.stack) != 0 {
		t.Error("With should not capture a new stack when inner already has one")
	}
}

func Test_hasStack_externalProvider(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// Layer 1: stackProvider (StackFrames() []StackFrame)
		{
			name: "stackProvider: non-empty frames detected",
			err:  &fakeProvider{frames: []StackFrame{{Function: "f", File: "f.go", Line: 1}}},
			want: true,
		},
		{
			name: "stackProvider: empty frames not detected",
			err:  &fakeProvider{frames: nil},
			want: false,
		},
		// Layer 2: stackTracer (StackTrace() any)
		{
			name: "stackTracer: non-empty slice detected",
			err:  &fakeTracer{stack: []string{"frame1"}},
			want: true,
		},
		{
			name: "stackTracer: nil slice not detected",
			err:  &fakeTracer{stack: nil},
			want: false,
		},
		// Layer 3: reflect fallback (concrete StackTrace() return type, e.g. pkg/errors)
		{
			name: "reflect fallback: non-empty []uintptr detected",
			err:  &fakeStackErr{frames: []uintptr{1, 2, 3}},
			want: true,
		},
		{
			name: "reflect fallback: nil []uintptr not detected",
			err:  &fakeStackErr{frames: nil},
			want: false,
		},
		// Chain traversal
		{
			name: "plain error has no stack",
			err:  errors.New("plain"),
			want: false,
		},
		{
			name: "reflect fallback detected in wrapped chain",
			err:  fmt.Errorf("wrapping: %w", &fakeStackErr{frames: []uintptr{1}}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasStack(tt.err); got != tt.want {
				t.Errorf("hasStack() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Wrap_skipsStackForExternalProvider(t *testing.T) {
	ext := &fakeStackErr{frames: []uintptr{1, 2, 3}}
	wrapped := Wrap(ext, "service layer", "component", "api")

	e := wrapped.(*Error)
	if len(e.stack) != 0 {
		t.Error("Wrap should not capture a stack when inner error has external StackTrace()")
	}
}

// fakeProvider satisfies stackProvider — layer 1.
type fakeProvider struct {
	frames []StackFrame
}

func (e *fakeProvider) Error() string             { return "fake provider error" }
func (e *fakeProvider) StackFrames() []StackFrame { return e.frames }

// fakeTracer satisfies stackTracer (StackTrace() any) — layer 2.
type fakeTracer struct {
	stack []string
}

func (e *fakeTracer) Error() string { return "fake tracer error" }
func (e *fakeTracer) StackTrace() any {
	if e.stack == nil {
		return ([]string)(nil)
	}
	return e.stack
}

// fakeStackErr simulates pkg/errors: StackTrace() returns a concrete type — layer 3 (reflect).
type fakeStackErr struct {
	frames []uintptr
}

func (e *fakeStackErr) Error() string         { return "fake external error" }
func (e *fakeStackErr) StackTrace() []uintptr { return e.frames }

func assertAttrs(t *testing.T, got, want []slog.Attr) {
	if len(got) != len(want) {
		t.Fatalf("attrs len = %d, want %d; got %v", len(got), len(want), got)
	}

	for i := range got {
		if !got[i].Equal(want[i]) {
			t.Errorf("attr[%d] got %v, want %v", i, got[i], want[i])
		}
	}
}
