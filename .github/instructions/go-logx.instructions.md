---
description: 'Project-specific instructions for go-logx: a structured logging library for Go built on log/slog'
applyTo: '**/*.go,**/go.mod,**/go.sum'
---

# go-logx Development Instructions

`go-logx` is a structured logging library for Go (1.26+) built on top of the standard `log/slog` package. It provides a logger factory with multi-sink output, structured error types with automatic attribute extraction, and context-based logger propagation.

## Package Overview

| Package | Role |
|---|---|
| `logx` | Logger factory, multi-handler fan-out, context helpers |
| `errx` | Structured errors with `[]slog.Attr`, stack trace capture, and `slog.LogValuer` |
| `attr` | `slog.Attr` construction helpers: `Args`, `Group`, `Merge` |
| `internal/dedup` | Deduplicates `[]slog.Attr` by key; first occurrence wins (internal use only) |

## Examples

| Directory | Demonstrates |
|---|---|
| `examples/basic` | `logx.New` with console + file output and `errx.Wrap` |
| `examples/http-service` | HTTP server with request-ID middleware, context logger propagation, structured error handling |
| `examples/pkg-errors` | `errx` integration with `pkg/errors` (standalone module) |
| `examples/attr-group` | `attr.Group` for nested attribute grouping (`request.*`, `db.*`) |
| `examples/console-json` | Text vs. JSON console output (`ConsoleJSON: false` vs `ConsoleJSON: true`) |
| `examples/context-logger` | `logx.WithLogger` / `logx.FromContext` through a call chain |
| `examples/errx-attrs` | `errx.Attrs(err)`, `attr.Args`, `attr.Merge` for custom error-reporting pipelines |

## Logger Initialization

Always initialize via `logx.New(Config)`. It returns a `*slog.Logger`, a `Cleanup` function, and an error.

- Call `Cleanup` explicitly in a deferred function and handle its error; do not use `defer cleanup()` directly.
- If `logx.New` fails, print to `os.Stderr` and exit — never use the logger before it is successfully initialized.

```go
logger, cleanup, err := logx.New(logx.Config{
    Level:       slog.LevelDebug,
    AddSource:   true,
    Console:     true,
    ConsoleJSON: false,
    File:        "service.log",
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
```

### Config Reference

| Field | Type | Description |
|---|---|---|
| `Level` | `slog.Level` | Minimum level for console output |
| `AddSource` | `bool` | Include source file and line in output |
| `Console` | `bool` | Enable console output (to `os.Stderr`) |
| `ConsoleJSON` | `bool` | Use JSON format for console (default: text) |
| `File` | `string` | Log file path; empty disables file output |
| `FileLevel` | `slog.Level` | Minimum level for file output |

File output is always JSON, buffered (8 KiB), and rotated via lumberjack (100 MB / 5 backups / 30 days / compressed).

## Structured Errors (`errx`)

Use `errx` to attach structured `slog.Attr` values and stack traces to errors. When logged via `slog.Any("error", err)`, all error context appears **nested inside the `"error"` group** — nothing is promoted to the top level of the log record.

This is by design: each error is self-contained. Attrs, stack trace, and the cause chain all live under `"error"`, so they never collide with attrs set on the logger itself (e.g. request-scoped fields from middleware). `*errx.Error` implements `slog.LogValuer` which handles the nested serialization automatically.

### Error Creation

| Function | When to use |
|---|---|
| `errx.New(msg, args...)` | Create a new error with message, attrs, and stack trace |
| `errx.Wrap(err, msg, args...)` | Wrap an existing error with a new message, attrs, and stack trace |
| `errx.With(err, args...)` | Attach attrs and stack trace to an existing error without changing its message |

```go
// New error with structured context
err := errx.New("database connection failed",
    "host", "db.example.com",
    "port", 5432,
)

// Wrap an upstream error with domain context
err = errx.Wrap(err, "purchase failed",
    "purchase_id", "p-123",
    "user_id", "u-42",
)

// Attach context to an error from a third-party or stdlib
err = errx.With(
    fmt.Errorf("user validation failed: %w", validationErr),
    "user_id", user.ID,
    "user_email", user.Email,
)
```

### Logging Errors

Pass `errx` errors via `slog.Any("error", err)`. The `*errx.Error` value implements `slog.LogValuer` and emits a self-contained group with `message`, attrs, `stack_trace`, and `cause`. Attrs from the entire error chain are merged; the outermost error's attrs take precedence (first key wins).

**Only one `stack_trace` per error chain.** `errx.Wrap` and `errx.With` call `runtime.Callers` only when no existing stack is detected. Detection is layered: (1) `stackProvider interface { StackFrames() []StackFrame }` — native errx contract, also used by `innermostStack` for display; (2) `stackTracer interface { StackTrace() any }` — explicit external contract checked via `errors.As`; (3) reflect fallback for `pkg/errors`, `cockroachdb/errors` etc. whose `StackTrace()` returns a concrete type. The displayed stack is always from the innermost `stackProvider` in the chain — closest to error origin. Cause chain entries contain `message` and attrs only.

```go
logger.Error("request failed", slog.Any("error", err))
```

Avoid logging the same error at multiple levels in the same call stack — log once at the point where handling occurs.

### Extracting Attrs

Use `errx.Attrs(err)` to manually extract `[]slog.Attr` from an error chain when building custom handlers or tooling.

## Attr Helpers (`attr`)

Use `attr.Args` to convert alternating key/value pairs into `[]slog.Attr`. This is the same conversion `errx.New/Wrap/With` use internally.

- Passing an `slog.Attr` directly in args is supported.
- A non-string key or an unpaired trailing key is logged with the `!BADKEY` prefix.

```go
attrs := attr.Args("user_id", "u-42", "method", "POST")

group := attr.Group("request",
    slog.String("method", "GET"),
    slog.String("path", "/users"),
)

merged := attr.Merge(outerAttrs, innerAttrs) // first key wins
```

## Context-Based Logger Propagation

Attach a logger to a context using `logx.WithLogger` and retrieve it with `logx.FromContext`. If no logger is stored, `FromContext` returns `slog.Default()`.

```go
// In middleware — enrich logger with request-scoped fields
reqLogger := logger.With(
    slog.String("request_id", uuid.NewString()),
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
)
ctx := logx.WithLogger(r.Context(), reqLogger)

// In handler or downstream function
logger := logx.FromContext(ctx)
logger.Debug("processing request")
```

Use dependency injection (`*slog.Logger` as a function or struct parameter) for top-level wiring. Use `logx.FromContext` for request-scoped propagation within a call chain.

## Common Patterns

### Good — Structured error with context, logged once

```go
if err := db.Query(ctx, query); err != nil {
    err = errx.Wrap(err, "query failed", "query", query, "component", "user_repo")
    logger.Error("request failed", slog.Any("error", err))
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
}
```

### Bad — Bare error, no context, logged multiple times

```go
if err := db.Query(ctx, query); err != nil {
    log.Println("error:", err)           // no structure
    logger.Error("query failed")         // logged again, no attrs
    return fmt.Errorf("query failed: %w", err) // caller logs a third time
}
```

### Good — Cleanup handled explicitly

```go
defer func() {
    if err := cleanup(); err != nil {
        fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
    }
}()
```

### Bad — Cleanup error silently dropped

```go
defer cleanup() // error is ignored
```

## MultiHandler

`MultiHandler` is internal to `logx.New` and managed automatically. Do not instantiate it manually in application code. Add custom `slog.Handler` implementations by composing them outside `logx.New` if additional sinks are needed.

## Testing

Use table-driven tests. Keep test coverage focused on pure logic — avoid testing file I/O or handler wiring in unit tests.

### Test file placement

Place test files next to the code they test, using the same package name (white-box):

```
attr/args_test.go      → package attr
attr/merge_test.go     → package attr
errx/error_test.go     → package errx
```

### Table-driven pattern

```go
func Test_Merge(t *testing.T) {
    tests := []struct {
        name   string
        groups [][]slog.Attr
        want   []slog.Attr
    }{
        {name: "empty input", groups: nil, want: nil},
        {name: "overlapping keys: first wins", groups: [][]slog.Attr{
            {slog.String("k", "first")},
            {slog.String("k", "second")},
        }, want: []slog.Attr{slog.String("k", "first")}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Merge(tt.groups...)
            // assert...
        })
    }
}
```

### What to test

- `attr.Args` — k/v pairs, raw `slog.Attr`, unpaired key → `!BADKEY`, non-string key → `!BADKEY`
- `attr.Merge` — dedup first-wins across groups
- `errx.New/Wrap/With` — message, nil guards, attrs preserved
- `errx.Attrs` — chain extraction, dedup precedence

### What NOT to test

- `logx.New` — requires file I/O; test only in integration scenarios
- `MultiHandler` / `ErrorHandler` — covered indirectly through `errx` and `attr` unit tests
- Example programs

## Validation

```bash
go build ./...
go vet ./...
go test ./...
```

When the `logx`, `errx`, or `attr` public API changes, also run all examples to verify they still compile and produce expected output:

```bash
go run ./examples/basic/
go run ./examples/attr-group/
go run ./examples/console-json/
go run ./examples/context-logger/
go run ./examples/errx-attrs/
go run ./examples/http-service/
```

The `examples/pkg-errors/` example is a standalone module — run it separately from its own directory:

```bash
cd examples/pkg-errors && go run .
```
