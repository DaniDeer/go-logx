# go-logx

A structured logging library for Go (1.26+) built on the standard `log/slog` package. Provides a logger factory with console and rotating file output, structured errors with automatic attribute extraction, and context-based logger propagation — with no custom logging framework required.

## Installation

```bash
go get github.com/DaniDeer/go-logx
```

## Quick Start

```go
package main

import (
    "fmt"
    "log/slog"
    "os"

    "github.com/DaniDeer/go-logx/errx"
    "github.com/DaniDeer/go-logx/logx"
)

func main() {
    logger, cleanup, err := logx.New(logx.Config{
        Level:       slog.LevelDebug,
        AddSource:   true,
        Console:     true,
        ConsoleJSON: false,
        File:        "app.log",
        FileLevel:   slog.LevelInfo,
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

    err = errx.Wrap(
        fmt.Errorf("database timeout"),
        "purchase failed",
        "purchase_id", "p-123",
        "user_id", "u-42",
    )

    logger.Error("request failed", slog.Any("error", err))
}
```

## Packages

### `logx` — Logger Factory

`logx.New(Config)` returns a `*slog.Logger`, a `Cleanup` func, and an error. The logger fans out to all configured sinks.

#### Config

| Field | Type | Description |
|---|---|---|
| `Level` | `slog.Level` | Minimum log level for console output |
| `AddSource` | `bool` | Include source file and line number |
| `Console` | `bool` | Enable console output (writes to `os.Stderr`) |
| `ConsoleJSON` | `bool` | Use JSON format for console (default: text) |
| `File` | `string` | Log file path; empty disables file output |
| `FileLevel` | `slog.Level` | Minimum log level for file output |

File output is always JSON, buffered (8 KiB), and rotated automatically:
- **Max size:** 100 MB per file
- **Max backups:** 5 compressed files
- **Max age:** 30 days

#### Context Helpers

```go
// Store a logger in a context (e.g., in HTTP middleware)
ctx = logx.WithLogger(ctx, logger.With(slog.String("request_id", id)))

// Retrieve the logger anywhere in the call chain
logger := logx.FromContext(ctx) // falls back to slog.Default() if not set
```

#### Cleanup

Always handle the `Cleanup` error explicitly:

```go
defer func() {
    if err := cleanup(); err != nil {
        fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
    }
}()
```

`Cleanup` flushes the write buffer and closes the rotating log file.

---

### `errx` — Structured Errors

Errors created with `errx` carry structured `slog.Attr` values and a stack trace. When logged via `slog.Any("error", err)`, all error context appears **nested inside the `"error"` group** — nothing is promoted to the top level of the log record.

This is by design: each error is self-contained. Attrs, stack trace, and the cause chain all live under `"error"`, so they never collide with attrs set on the logger itself (e.g. request-scoped fields from middleware).

#### Creating Errors

| Function | Description |
|---|---|
| `errx.New(msg, args...)` | New error with message, attrs, and stack |
| `errx.Wrap(err, msg, args...)` | Wrap an error with a new message, attrs, and stack |
| `errx.With(err, args...)` | Attach attrs and stack to an existing error |

`args` accepts alternating `key, value` pairs or `slog.Attr` values directly.

```go
// New error
err := errx.New("connection refused", "host", "db.example.com")

// Wrap an upstream error
err = errx.Wrap(err, "purchase failed", "purchase_id", "p-123")

// Attach context to a stdlib or third-party error
err = errx.With(
    fmt.Errorf("user validation failed: %w", validationErr),
    "user_id", user.ID,
    "user_email", user.Email,
)
```

#### Log Output

`*errx.Error` implements `slog.LogValuer` and serializes as a self-contained group. Attrs from the entire error chain are merged; the outermost error's attrs take precedence (first key wins).

**Only one `stack_trace` per error chain.** The stack is captured once — at the first `errx.New`, `errx.Wrap`, or `errx.With` call in the chain. Subsequent wraps enrich with context only (no additional `runtime.Callers`). The captured stack is from the innermost `*errx.Error` — the point closest to the error origin — and appears at the top level of the serialized group. Cause chain entries show `message` and attrs only.

```json
{
  "time": "...",
  "level": "ERROR",
  "msg": "request failed",
  "request_id": "abc-123",
  "error": {
    "message": "purchase failed: connection refused",
    "purchase_id": "p-123",
    "stack_trace": [...],
    "cause": {
      "message": "connection refused",
      "host": "db.example.com"
    }
  }
}
```

---

### `attr` — Attribute Helpers

```go
// Convert k/v pairs to []slog.Attr (bad keys get !BADKEY prefix)
attrs := attr.Args("user_id", "u-42", "method", "GET")

// Create a named attribute group
group := attr.Group("request",
    slog.String("method", "GET"),
    slog.String("path", "/users"),
)

// Merge multiple []slog.Attr slices; first key wins
merged := attr.Merge(outerAttrs, innerAttrs)
```

## External Stack Providers

`errx.Wrap` and `errx.With` detect existing stack traces before calling `runtime.Callers`. Detection uses three layers:

| Layer | Interface / Method | Works for |
|---|---|---|
| 1 | `stackProvider` — `StackFrames() []StackFrame` | `*errx.Error` and custom types that normalize to errx frames |
| 2 | `stackTracer` — `StackTrace() any` | New code that explicitly adopts this contract |
| 3 | Reflect fallback | [`pkg/errors`](https://pkg.go.dev/github.com/pkg/errors), [`cockroachdb/errors`](https://pkg.go.dev/github.com/cockroachdb/errors), and similar (concrete `StackTrace()` return type prevents interface matching) |

```go
import pkgerrors "github.com/pkg/errors"

func fetchUser(id string) error {
    err := queryDB()                          // pkg/errors error — has a stack (reflect layer)
    return errx.Wrap(err,                     // detects existing stack; skips runtime.Callers
        "failed to fetch user",
        "user_id", id,
    )
}
```

The errx attrs are attached normally. Only one stack allocation occurs per chain.

See [`examples/pkg-errors`](examples/pkg-errors/main.go) for a runnable example.

---

## Examples

- [`examples/basic`](examples/basic/main.go) — `logx.New` with console + file output and `errx.Wrap`
- [`examples/http-service`](examples/http-service/) — HTTP server with request-ID middleware, context logger propagation, and structured error handling per request
- [`examples/pkg-errors`](examples/pkg-errors/main.go) — integrating `pkg/errors` with `errx` (standalone module)
