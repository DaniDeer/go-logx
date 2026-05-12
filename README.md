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

| Field          | Type          | Description                                                                                                |
| -------------- | ------------- | ---------------------------------------------------------------------------------------------------------- |
| `Level`        | `slog.Level`  | Minimum log level for console output                                                                       |
| `AddSource`    | `bool`        | Include source file and line number                                                                        |
| `Console`      | `bool`        | Enable console output (writes to `os.Stderr`)                                                              |
| `ConsoleJSON`  | `bool`        | Use JSON format for console (default: text)                                                                |
| `File`         | `string`      | Log file path; empty disables file output                                                                  |
| `FileLevel`    | `slog.Level`  | Minimum log level for file output                                                                          |
| `DefaultAttrs` | `[]slog.Attr` | Attrs attached to every log line (service name, region, env, …)                                            |
| `Build`        | `*BuildInfo`  | Adds a `build.*` group to every log line; `build.go` (Go runtime version) is always included automatically |

`BuildInfo` fields: `Version`, `Commit`, `Date` — typically set via `-ldflags` at build time.

File output is always JSON, buffered (8 KiB), and rotated automatically:

- **Max size:** 100 MB per file
- **Max backups:** 5 compressed files
- **Max age:** 30 days

#### Static Attrs and Build Info

Use `DefaultAttrs` and `Build` to stamp every log line with process-wide context. This is essential in canary and rolling deployments where multiple versions run simultaneously:

```go
var (
    version   string // set via -ldflags "-X main.version=v1.2.3"
    commit    string // set via -ldflags "-X main.commit=$(git rev-parse --short HEAD)"
    buildDate string // set via -ldflags "-X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
)

// In main() or init():
logger, cleanup, err := logx.New(logx.Config{
    Level:   slog.LevelInfo,
    Console: true,
    DefaultAttrs: []slog.Attr{
        slog.String("service", "order-api"),
        slog.String("env",     "production"),
        slog.String("region",  "eu-west-1"),
    },
    Build: &logx.BuildInfo{
        Version: version,   // set via -ldflags "-X main.version=v1.2.3"
        Commit:  commit,    // set via -ldflags "-X main.commit=$(git rev-parse --short HEAD)"
        Date:    buildDate, // set via -ldflags "-X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    },
})
```

Use `-ldflags` to inject build info at compile time:

```BASH
go build -ldflags "-X my/package/build.GitSHA=$(git rev-parse HEAD) -X my/package/build.BuildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
```

Where `my/package/build.GitSHA` and `my/package/build.BuildTime` are `string` variables in your code in the respective package. In case you initialize the logger in the `main` package, you can set them as `main.GitSHA` and `main.BuildTime`.

If you build with Go 1.20+, you can also use the `debug.ReadBuildInfo` API to read build info from the binary at runtime and populate the `BuildInfo` struct without needing `-ldflags`. See [`examples/build-info`](examples/build-info/main.go) for a runnable example.

If you build with Docker, you can use `ARG` and `--build-arg` to pass build metadata from the Docker build context into your Go binary via `-ldflags`. In Docker builds commit hashes are not available by default, but you can use `git rev-parse` in your build script to capture it and pass it as a build arg or forward it from a GH Action step into the Docker build.

Every log line carries:

```
service=order-api env=production region=eu-west-1 build.version=v1.2.3 build.commit=abc123 build.date=2024-01-15T10:00:00Z build.go=go1.26.2
```

`build.go` (Go runtime version) is added automatically — no ldflags needed for it.

##### What to Log?

- Runtime environment: `production`, `staging`, `development`
- Host information: `hostname`, `ip_address`, `instance_id`, `OS`, `arch`, `kernel_version`, etc.
- Service name: `order-api`, `user-service`, etc.
- Region: `eu-west-1`, `us-east-1`, etc.
- Timezone: `UTC`, `America/New_York`, etc.
- Node/Container or pod name (for Kubernetes): `node-42`, `pod-abc123`, etc.
- Build info: `build.version`, `build.commit`, `build.date`, `build.go`

##### What to Log - HTTP Requests

- Request ID: `request_id` (generated per request, e.g. via middleware)
- Request duration: `duration_ms` (in milliseconds, e.g. via middleware)
- Request headers: `User-Agent`, `Content-Type`, etc. (be mindful of PII and sensitive data)
- Relevant cookies for the request: `session_id`, `auth_token` (presence), etc. (again, be mindful of PII and sensitive data)
- Number of bytes received in the request body: `request_size_bytes`
- User ID: `user_id` (if authenticated)
- Response headers: `Content-Type`, `Content-Length`, etc. (be mindful of PII and sensitive data)
- Response status code: `status_code` (e.g., 200, 404, 500)
- Number of bytes sent in the response: `response_size_bytes`

> Rule of Thumb: Don´t log everything in every app, add fields as they become useful!

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

| Function                       | Description                                                         |
| ------------------------------ | ------------------------------------------------------------------- |
| `errx.New(msg, args...)`       | New error with message, attrs, and stack                            |
| `errx.Wrap(err, msg, args...)` | Wrap an error with a new message, attrs, and stack                  |
| `errx.With(err, args...)`      | Attach attrs and stack to an existing error                         |
| `errx.Join(errs...)`           | Collect multiple errors into one; returns nil if all inputs are nil |

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

#### Collecting Multiple Errors (Batch / Validation)

Use `errx.Join` when you want to log all failures from a batch operation or multi-field validation as **one log event**:

```go
var errs []error
for _, item := range batch {
    if err := process(item); err != nil {
        errs = append(errs, errx.Wrap(err, "item failed", "item_id", item.ID))
    }
}
if joined := errx.Join(errs...); joined != nil {
    logger.Error("batch failed",
        slog.Any("errors", joined),
        slog.Int("failed", len(errs)),
        slog.Int("total", len(batch)),
    )
}
```

`errx.Join` returns nil if every input is nil, so the `if joined != nil` check is safe even when the batch fully succeeds. Each child error is serialized under an indexed key (`errors.0.*`, `errors.1.*`, ...) via `slog.LogValuer`. `errors.Is` and `errors.As` traverse all children.

```json
{
	"level": "ERROR",
	"msg": "batch failed",
	"failed": 2,
	"total": 10,
	"errors": {
		"0": { "message": "item failed: connection refused", "item_id": "i-1" },
		"1": {
			"message": "item failed: timeout",
			"item_id": "i-5",
			"timeout_ms": 5000
		}
	}
}
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

| Layer | Interface / Method                             | Works for                                                                                                                                                                                                         |
| ----- | ---------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | `stackProvider` — `StackFrames() []StackFrame` | `*errx.Error` and custom types that normalize to errx frames                                                                                                                                                      |
| 2     | `stackTracer` — `StackTrace() any`             | New code that explicitly adopts this contract                                                                                                                                                                     |
| 3     | Reflect fallback                               | [`pkg/errors`](https://pkg.go.dev/github.com/pkg/errors), [`cockroachdb/errors`](https://pkg.go.dev/github.com/cockroachdb/errors), and similar (concrete `StackTrace()` return type prevents interface matching) |

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
- [`examples/attr-group`](examples/attr-group/main.go) — `attr.Group` for nested attribute grouping (`request.*`, `db.*`)
- [`examples/console-json`](examples/console-json/main.go) — text vs. JSON console output side-by-side (`ConsoleJSON: false` vs `ConsoleJSON: true`)
- [`examples/context-logger`](examples/context-logger/main.go) — `logx.WithLogger` / `logx.FromContext` through a call chain without threading a logger argument
- [`examples/errx-attrs`](examples/errx-attrs/main.go) — `errx.Attrs(err)` + `attr.Args` + `attr.Merge` for building custom error-reporting pipelines
- [`examples/multi-error`](examples/multi-error/main.go) — `errx.Join` for batch processing and multi-field validation (one log event per operation)
- [`examples/build-info`](examples/build-info/main.go) — `Config.DefaultAttrs` + `Config.Build` for stamping every log line with service identity and build metadata
