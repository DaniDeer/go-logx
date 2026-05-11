package logx

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Cleanup func() error

func New(config Config) (*slog.Logger, Cleanup, error) {

	var handlers []slog.Handler
	var closers []Cleanup

	if config.Console {

		var console slog.Handler

		// If ConsoleJSON is true, use a JSON handler for console output. Otherwise, use a text handler.
		if config.ConsoleJSON {
			console = slog.NewJSONHandler(
				os.Stderr,
				&slog.HandlerOptions{
					Level:     config.Level,
					AddSource: config.AddSource,
				},
			)
		} else {
			console = slog.NewTextHandler(
				os.Stderr,
				&slog.HandlerOptions{
					Level:     config.Level,
					AddSource: config.AddSource,
				},
			)
		}

		handlers = append(handlers, console)
	}

	// If a file is specified, set up a lumberjack logger for log rotation and buffering for performance.
	// Otherwise, if no file is specified, logs will only be output to the console (if enabled).
	if config.File != "" {

		rotating := &lumberjack.Logger{
			Filename:   config.File,
			MaxSize:    100,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}

		buffered := bufio.NewWriterSize(rotating, 8192)

		fileHandler := slog.NewJSONHandler(
			buffered,
			&slog.HandlerOptions{
				Level:     config.FileLevel,
				AddSource: config.AddSource,
			},
		)

		handlers = append(handlers, fileHandler)

		// Add a closer to flush the buffer and close the rotating logger when cleaning up.
		closers = append(closers,
			func() error {
				if err := buffered.Flush(); err != nil {
					return fmt.Errorf(
						"flush log buffer: %w",
						err,
					)
				}

				return rotating.Close()
			},
		)
	}

	// Combine all handlers into a MultiHandler, which will dispatch log records to all handlers.
	handler := NewMultiHandler(handlers...)

	// Create a new slog.Logger with the combined handler. The logger will use the MultiHandler to output logs to all configured destinations (console and/or file).
	logger := slog.New(handler)

	// Define a cleanup function that will be returned to the caller. This function will be responsible for flushing buffers and closing any resources when the logger is no longer needed.
	cleanup := func() error {
		for _, close := range closers {
			if err := close(); err != nil {
				return err
			}
		}

		return nil
	}

	return logger, cleanup, nil
}
