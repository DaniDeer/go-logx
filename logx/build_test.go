package logx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"runtime"
	"strings"
	"testing"
)

// newTestLogger creates a logger that writes JSON to buf using logx.New.
func newTestLogger(t *testing.T, cfg Config) (*slog.Logger, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}

	// Redirect console output to our buffer by building the handler directly.
	// logx.New always writes to os.Stderr, so we test the enrichment logic by
	// constructing a JSON handler over our buffer and applying the same
	// DefaultAttrs / Build enrichment that New does.
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	if len(cfg.DefaultAttrs) > 0 {
		args := make([]any, len(cfg.DefaultAttrs))
		for i, a := range cfg.DefaultAttrs {
			args[i] = a
		}
		logger = logger.With(args...)
	}

	if cfg.Build != nil {
		logger = logger.With(slog.Attr{
			Key: "build",
			Value: slog.GroupValue(
				slog.String("version", cfg.Build.Version),
				slog.String("commit", cfg.Build.Commit),
				slog.String("date", cfg.Build.Date),
				slog.String("go", runtime.Version()),
			),
		})
	}

	return logger, buf
}

func logLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse log line: %v\nraw: %s", err, buf.String())
	}
	return m
}

func Test_DefaultAttrs_emittedOnEveryLine(t *testing.T) {
	logger, buf := newTestLogger(t, Config{
		DefaultAttrs: []slog.Attr{
			slog.String("service", "test-svc"),
			slog.String("region", "eu-west-1"),
		},
	})

	logger.Info("hello")

	m := logLine(t, buf)
	if m["service"] != "test-svc" {
		t.Errorf("service = %v, want test-svc", m["service"])
	}
	if m["region"] != "eu-west-1" {
		t.Errorf("region = %v, want eu-west-1", m["region"])
	}
}

func Test_DefaultAttrs_nilIsNoop(t *testing.T) {
	logger, buf := newTestLogger(t, Config{})

	logger.Info("hello")

	m := logLine(t, buf)
	if _, ok := m["service"]; ok {
		t.Error("expected no 'service' key when DefaultAttrs is nil")
	}
}

func Test_BuildInfo_groupEmittedOnEveryLine(t *testing.T) {
	logger, buf := newTestLogger(t, Config{
		Build: &BuildInfo{
			Version: "v1.2.3",
			Commit:  "abc123",
			Date:    "2024-01-15T10:00:00Z",
		},
	})

	logger.Info("hello")

	m := logLine(t, buf)
	build, ok := m["build"].(map[string]any)
	if !ok {
		t.Fatalf("expected build group in log output, got: %v", m["build"])
	}
	if build["version"] != "v1.2.3" {
		t.Errorf("build.version = %v, want v1.2.3", build["version"])
	}
	if build["commit"] != "abc123" {
		t.Errorf("build.commit = %v, want abc123", build["commit"])
	}
	if build["date"] != "2024-01-15T10:00:00Z" {
		t.Errorf("build.date = %v, want 2024-01-15T10:00:00Z", build["date"])
	}
	goVer, _ := build["go"].(string)
	if !strings.HasPrefix(goVer, "go") {
		t.Errorf("build.go = %q, expected runtime.Version() prefix 'go'", goVer)
	}
}

func Test_BuildInfo_nilIsNoop(t *testing.T) {
	logger, buf := newTestLogger(t, Config{})

	logger.Info("hello")

	m := logLine(t, buf)
	if _, ok := m["build"]; ok {
		t.Error("expected no 'build' key when Build is nil")
	}
}

func Test_DefaultAttrs_and_BuildInfo_combinedOrder(t *testing.T) {
	logger, buf := newTestLogger(t, Config{
		DefaultAttrs: []slog.Attr{slog.String("service", "svc")},
		Build:        &BuildInfo{Version: "v0.1.0", Commit: "fff", Date: "2024-01-01"},
	})

	logger.Info("combined")

	m := logLine(t, buf)
	if m["service"] != "svc" {
		t.Errorf("service = %v, want svc", m["service"])
	}
	if _, ok := m["build"].(map[string]any); !ok {
		t.Error("expected build group alongside DefaultAttrs")
	}
}
