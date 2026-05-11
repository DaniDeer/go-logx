package logx

import "log/slog"

type Config struct {
	Level       slog.Level  // The minimum log level to output. For example, if set to slog.LevelInfo, then only logs with level Info and above (e.g., Warning, Error) will be output.
	AddSource   bool        // Whether to include the source file and line number in the log output.
	Console     bool        // Whether to output logs to the console.
	ConsoleJSON bool        // Whether to output logs in JSON format to the console.

	File      string      // The file to output logs to. If empty, logs will not be written to a file.
	FileLevel slog.Level  // The minimum log level to output to the file.
}