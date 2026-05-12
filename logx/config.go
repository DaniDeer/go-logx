package logx

import "log/slog"

// BuildInfo holds build-time metadata injected via -ldflags.
// When set in Config.Build, all log lines carry a "build" group containing
// these fields plus the Go runtime version (build.go) added automatically.
//
// Typical ldflags usage:
//
//	go build -ldflags "-X main.version=v1.2.3 -X main.commit=abc123 -X main.date=2024-01-15T10:00:00Z"
type BuildInfo struct {
	Version string // semantic version, e.g. "v1.2.3"
	Commit  string // git commit SHA, e.g. "abc123def"
	Date    string // build timestamp, e.g. "2024-01-15T10:00:00Z"
}

type Config struct {
	Level       slog.Level // The minimum log level to output. For example, if set to slog.LevelInfo, then only logs with level Info and above (e.g., Warning, Error) will be output.
	AddSource   bool       // Whether to include the source file and line number in the log output.
	Console     bool       // Whether to output logs to the console.
	ConsoleJSON bool       // Whether to output logs in JSON format to the console.

	File      string     // The file to output logs to. If empty, logs will not be written to a file.
	FileLevel slog.Level // The minimum log level to output to the file.

	// DefaultAttrs are attached to every log line emitted by the returned logger.
	// Use this for static, process-wide fields such as service name, region, or environment.
	// Example: []slog.Attr{slog.String("service", "order-api"), slog.String("region", "eu-west-1")}
	DefaultAttrs []slog.Attr

	// Build, when non-nil, adds a "build" group to every log line containing
	// version, commit, date, and the Go runtime version (build.go).
	Build *BuildInfo
}
