package errx

import (
	"errors"
	"log/slog"
)

func (e *Error) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("message", e.Error()),
	}

	attrs = append(attrs, Attrs(e)...)

	if len(e.stack) > 0 {
		attrs = append(attrs,
			slog.Attr{
				Key:   "stack_trace",
				Value: stackAttrs(e.stack),
			},
		)
	}

	if cause := errors.Unwrap(e.err); cause != nil {
		attrs = append(attrs,
			slog.Any("cause", cause),
		)
	}

	return slog.GroupValue(attrs...)
}