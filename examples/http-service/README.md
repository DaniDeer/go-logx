# http-service example

A minimal HTTP service demonstrating how to wire `go-logx` into a real service: two middleware layers that progressively enrich a structured logger, a service type that uses the context logger, and request/response telemetry captured without polluting handler code.

## Files

| File | Role |
|---|---|
| `main.go` | Entry point — logger init, graceful shutdown |
| `server.go` | HTTP server, middleware chain wiring, route registration |
| `logging_middleware.go` | `reqLoggerMiddleware`, spy wrappers, `responseWriterWithContext` |
| `auth_middleware.go` | `authMiddleware`, Bearer token extraction |
| `handlers.go` | `UserService` (dual-logger pattern), request handler, validation |

## Middleware chain

```
Incoming request
      │
      ▼
reqLoggerMiddleware          adds: request_id, method, path, remote_addr
      │                      logs: "request started"
      ▼
authMiddleware               adds: user_id  (returns 401 if token missing)
      │
      ▼
handler / UserService        reads: logx.FromContext(ctx)
                             all fields present automatically
      │
      ▼ (after handler returns)
reqLoggerMiddleware          logs: "request completed" with duration,
                             request_body_bytes, response_status, response_body_bytes
```

Each middleware layer enriches the logger stored in the `ResponseWriter` context — no layer needs to know about the others.

## Key principles

### 1. ResponseWriter context pattern

The enriched logger lives on the `ResponseWriter`, not on `r.Context()`:

```go
next.ServeHTTP(&responseWriterWithContext{ResponseWriter: w, ctx: ctx}, r)
```

Handlers retrieve it via type assertion:

```go
if cw, ok := w.(interface{ Context() context.Context }); ok {
    ctx = cw.Context()
}
logger := logx.FromContext(ctx)
```

This keeps `r.Context()` clean — it carries only request lifecycle signals (cancellation, deadlines), not framework data.

### 2. Composable middleware enrichment

Each middleware reads the current logger, adds its own fields, and writes an updated logger back to the context. Layers compose without coupling — `authMiddleware` doesn't need to know what `reqLoggerMiddleware` added:

```go
// authMiddleware — reads whatever is in context, adds user_id on top
logger := logx.FromContext(ctx)
authedCtx := logx.WithLogger(ctx, logger.With(slog.String("user_id", token)))
```

### 3. Service logger propagation

`UserService` has two loggers in play:

- **`s.logger`** (injected at construction, enriched with `component=user_service`) — for lifecycle events that run outside any request: startup, warmup, background jobs.
- **`logx.FromContext(ctx)`** — for request-scoped methods. Returns the fully-enriched request logger when called from a handler; falls back to `slog.Default()` when called from a background goroutine or test. No branching needed.

```go
func (s *UserService) FetchUsers(ctx context.Context) ([]User, error) {
    logger := logx.FromContext(ctx) // request logger in handlers, slog.Default() elsewhere
    logger.Debug("querying database")
    // ...
}
```

### 4. Spy wrappers for telemetry

`http.ResponseWriter` and `io.ReadCloser` don't expose bytes-written or status code after the fact. The spy wrappers intercept `Write` and `WriteHeader` to capture these values without any changes to handler code:

```go
sr := &spyReadCloser{ReadCloser: r.Body}
r.Body = sr
sw := &spyResponseWriter{ResponseWriter: w}
// ... call handler ...
logger.Info("request completed",
    slog.Int("request_body_bytes", sr.bytesRead),
    slog.Int("response_status", sw.statusCode),
    slog.Int("response_body_bytes", sw.bytesWritten),
)
```

## How to run

```bash
go run ./examples/http-service/
```

In another terminal:

```bash
# Authenticated request
curl -H "Authorization: Bearer user-42" http://localhost:8080/users

# Missing token → 401
curl http://localhost:8080/users
```

## Sample output

Successful request (annotated):

```
# reqLoggerMiddleware — request starts
time=... level=INFO  msg="request started"
    request_id=3f2a...  method=GET  path=/users  remote_addr=127.0.0.1:54321

# authMiddleware — user identified (user_id added to all subsequent logs)
# handler — UserService.FetchUsers
time=... level=DEBUG msg="querying database"
    request_id=3f2a...  method=GET  path=/users  remote_addr=...  user_id=user-42

time=... level=DEBUG msg="database query completed"
    request_id=3f2a...  ...  user_id=user-42

time=... level=INFO  msg="users fetched successfully"
    request_id=3f2a...  ...  user_id=user-42  user_count=2

# reqLoggerMiddleware — request completes
time=... level=INFO  msg="request completed"
    request_id=3f2a...  method=GET  path=/users  remote_addr=...
    duration=151ms  request_body_bytes=0  response_status=500  response_body_bytes=22
```

Rejected request (missing token):

```
time=... level=WARN  msg="request rejected: missing or invalid authorization"
    request_id=9c1b...  method=GET  path=/users  remote_addr=127.0.0.1:54322

time=... level=INFO  msg="request completed"
    request_id=9c1b...  duration=0s  response_status=401  response_body_bytes=13
```

Note that even the rejection log carries `request_id`, `method`, and `path` — because `authMiddleware` runs inside `reqLoggerMiddleware` and inherits its enriched logger.
