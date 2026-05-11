package logx

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func Test_FromContext_noLogger(t *testing.T) {
	logger := FromContext(context.Background())

	if logger != slog.Default() {
		t.Error("expected slog.Default() when no logger in context")
	}
}

func Test_WithLogger_FromContext(t *testing.T) {
	buf := &bytes.Buffer{}
	stored := slog.New(slog.NewJSONHandler(buf, nil))

	ctx := WithLogger(context.Background(), stored)
	got := FromContext(ctx)

	if got != stored {
		t.Error("FromContext returned a different logger than the one stored")
	}
}

func Test_FromContext_nested(t *testing.T) {
	outer := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	inner := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	outerCtx := WithLogger(context.Background(), outer)
	innerCtx := WithLogger(outerCtx, inner)

	if FromContext(outerCtx) != outer {
		t.Error("outer context returned wrong logger")
	}

	if FromContext(innerCtx) != inner {
		t.Error("inner context returned wrong logger")
	}
}
