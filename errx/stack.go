package errx

import "log/slog"

func (e *Error) StackTrace() []StackFrame {
	return e.stack
}

func stackAttrs(stack []StackFrame) slog.Value {
	values := make([]slog.Value, 0, len(stack))

	for _, frame := range stack {
		values = append(values,
			slog.GroupValue(
				slog.String("function", frame.Function),
				slog.String("file", frame.File),
				slog.Int("line", frame.Line),
			),
		)
	}

	return slog.AnyValue(values)
}