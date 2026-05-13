# http-service-logctx example

A minimal HTTP service demonstrating a **LogContext** approach to request logging: a mutable struct stored in `r.Context()` that middlewares and handlers enrich during request processing. The outer logging middleware emits **one consolidated log line per request** after the entire handler chain returns.

This is a deliberate contrast to the [`http-service`](../http-service/) example, which propagates an enriched `*slog.Logger` through the `ResponseWriter` context and emits separate log lines at each layer.

## Files

| File | Role |
|---|---|
| `main.go` | Entry point — logger init, graceful shutdown |
| `server.go` | HTTP server, middleware chain wiring, route registration |
| `requestid_middleware.go` | `requestIDMiddleware`, `requestIDFrom` helper |
| `log_context.go` | `LogContext` struct, `logContextKey`, `httpError` helper |
| `logging_middleware.go` | `requestLogger` middleware, spy wrappers |
| `auth_middleware.go` | Bearer token extraction, sets `logCtx.Username` |
| `handlers.go` | `UserService` (own logger only), `handleUsers`, `validateUser` |

## Middleware chain

```
Incoming request
      │
      ▼
requestIDMiddleware      reads X-Request-ID header; generates UUID if absent
      │                  stores ID in r.Context(); echoes ID in response header
      ▼
requestLogger            reads request_id from context into LogContext
      │                  starts spy wrappers (body bytes, status, response bytes)
      ▼
authMiddleware           sets logCtx.Username  (returns 401 + captures error if missing)
      │
      ▼
handleUsers / UserService  calls httpError on failure → captures error in LogContext
      │
      ▼ (handler chain returns)
requestLogger            reads final LogContext, emits ONE log line:
                         request_id, method, path, client_ip, duration, bytes, status,
                         user (if set), error (if set)
```

Each middleware and handler writes to the shared `LogContext` — no layer needs to know about the others.

## Three loggers in play

All three loggers are derived from the same root `*slog.Logger` created by `logx.New` in `main.go`, but each serves a distinct role:

| Logger | Where | Component tag | Purpose |
|--------|-------|--------------|---------|
| Request logger | `logger` passed to `requestLogger` | none | Emits one "served request" line per request — the access log |
| Server logger | `s.logger` (`component=http_server`) | `http_server` | Lifecycle events: server start, stop, shutdown; fallback for I/O errors after response headers are sent |
| Service logger | `s.userService.logger` (`component=user_service`) | `user_service` | Operational events inside `UserService`: queries, cache hits, background work |

```
logx.New(Config)
    │
    ├─► requestLogger(logger) ─────────────────── emits "served request" (no component)
    │       reads request_id from context set by requestIDMiddleware
    │
    ├─► s.logger = logger.With("component","http_server") ── start/stop/shutdown
    │
    └─► s.userService.logger = logger.With("component","user_service") ── queries etc.
```

The request logger (inside `requestLogger`) does not carry a component tag by design — the `request_id`, `method`, `path`, and other request fields provide enough context to distinguish these lines in a log stream.

`request_id` itself is set by `requestIDMiddleware` (outermost), which reads `X-Request-ID` from the incoming request header or generates a UUID, echoes it in the response header, and stores it in `r.Context()`. `requestLogger` reads it via `requestIDFrom(r.Context())` and stores it in `LogContext.RequestID` so it appears in the consolidated log line.

The server logger is also used as a **last-resort fallback** in `handleUsers` when `http.ResponseWriter.Write` fails after headers have already been sent — at that point `httpError` cannot be used because the status code is already committed.

## Key principles

### 1. LogContext in r.Context()

The middleware creates a `*LogContext` and stores it via `context.WithValue`:

```go
logCtx := &LogContext{}
r = r.WithContext(context.WithValue(r.Context(), logContextKey, logCtx))
```

Handlers and middlewares retrieve and mutate it directly — no `ResponseWriter` type assertions needed:

```go
if logCtx, ok := r.Context().Value(logContextKey).(*LogContext); ok {
    logCtx.Username = token
}
```

After the handler chain returns, `requestLogger` reads the final state and builds the log record.

### 2. httpError — capture before respond

`httpError` stashes the error in `LogContext` before calling `http.Error`, so the consolidated log always carries the error without extra log calls in handlers:

```go
func httpError(ctx context.Context, w http.ResponseWriter, status int, err error) {
    if logCtx, ok := ctx.Value(logContextKey).(*LogContext); ok {
        logCtx.Error = err
    }
    http.Error(w, err.Error(), status)
}
```

Handler usage:

```go
if err := s.userService.FetchUsers(ctx); err != nil {
    httpError(ctx, w, http.StatusInternalServerError,
        errx.Wrap(err, "failed to fetch users"))
    return
}
```

### 3. Service logger is independent

`UserService` has only one logger: the injected `*slog.Logger` enriched with `component=user_service`. It does not receive request-scoped fields. Service logs are operational events (queries, cache hits); request telemetry is the middleware's concern.

```go
func (s *UserService) FetchUsers(ctx context.Context) ([]User, error) {
    s.logger.Debug("querying database")  // component=user_service only
    // ...
}
```

This contrasts with the `http-service` example, where `logx.FromContext(ctx)` gives the service the fully-enriched request logger (carrying `request_id`, `user_id`, etc.).

### 4. Single consolidated log line

`requestLogger` logs once, after the handler chain returns, with all accumulated metadata:

```go
logger.Info("served request",
    slog.String("request_id", logCtx.RequestID),
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
    slog.String("client_ip", r.RemoteAddr),
    slog.Duration("duration", duration),
    slog.Int("request_body_bytes", sr.bytesRead),
    slog.Int("response_status", sw.statusCode),
    slog.Int("response_body_bytes", sw.bytesWritten),
    // + "user" if logCtx.Username != ""
    // + "error" if logCtx.Error != nil
)
```

## Comparison with http-service

| Aspect | http-service | http-service-logctx |
|--------|-------------|---------------------|
| Enrichment vehicle | `*slog.Logger` in `ResponseWriter` context | `*LogContext` in `r.Context()` |
| Log lines per request | 2+ ("request started", "request completed", debug lines from service) | 1 consolidated line from middleware, plus service debug lines |
| Auth enrichment | Creates new logger with `user_id` added | Mutates `logCtx.Username` |
| Error capture | Handler logs error, calls `http.Error` | `httpError()` captures error in `logCtx.Error` |
| Service logger | `logx.FromContext(ctx)` (inherits all request fields) | Own injected logger (no request fields) |
| Request fields on service logs? | Yes — `request_id`, `user_id`, etc. on every line | No — service logs are independent |
| Context retrieval in handlers | Type assert on `ResponseWriter` | `r.Context().Value(logContextKey)` |

**When to prefer http-service (enriched logger):**
- You want every log line from every layer to carry request correlation fields (`request_id`, `user_id`).
- Log search tools or alerting pipelines rely on per-line fields for filtering.
- Multiple layers emit independent logs that must be correlated.

**When to prefer http-service-logctx (LogContext):**
- You want a single structured summary line per request (access-log style).
- You prefer less log volume and simpler log pipelines.
- Service logs and request logs are consumed separately.
- Handlers and middlewares accumulate state (counters, errors) across steps before committing to the log.

## How to run

```bash
go run ./examples/http-service-logctx/
```

In another terminal:

```bash
# Authenticated request (validation fails → 500 + error in log)
curl -H "Authorization: Bearer alice" http://localhost:8080/users

# Missing token → 401
curl http://localhost:8080/users
```

## Sample output

Authenticated request:

```
# UserService debug logs — own logger, component=user_service, no request fields
time=... level=DEBUG msg="querying database"        component=user_service
time=... level=DEBUG msg="database query completed" component=user_service

# requestLogger — single consolidated line after handler returns
time=... level=INFO  msg="served request"
    request_id=3f2a...  method=GET  path=/users  client_ip=127.0.0.1:54321
    duration=151ms  request_body_bytes=0
    response_status=500  response_body_bytes=41
    user=alice
    error.message="user validation failed: invalid email format"
    error.validation=email_format  error.email=invalid-email
```

Rejected request (missing token):

```
# requestLogger — captures the auth rejection in a single line
time=... level=INFO  msg="served request"
    request_id=9c1b...  method=GET  path=/users  client_ip=127.0.0.1:54322
    duration=0s  request_body_bytes=0
    response_status=401  response_body_bytes=35
    error.message="missing or invalid authorization"
```

Note: the rejection log carries no `user` field because `authMiddleware` never set `logCtx.Username` — the token was absent.
