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

if stack := innermostStack(e); len(stack) > 0 {
attrs = append(attrs,
slog.Attr{
Key:   "stack_trace",
Value: slog.AnyValue(stackAttrs(stack)),
},
)
}

if cause := errors.Unwrap(e.err); cause != nil {
if ce, ok := cause.(*Error); ok {
attrs = append(attrs, slog.Attr{Key: "cause", Value: ce.compact()})
} else {
attrs = append(attrs, slog.String("cause", cause.Error()))
}
}

return slog.GroupValue(attrs...)
}

// innermostStack walks the error chain and returns the stack from the deepest
// *Error that has one. This is the most useful stack — closest to the error origin.
func innermostStack(e *Error) []StackFrame {
	var deepest []StackFrame
	if len(e.stack) > 0 {
		deepest = e.stack
	}

	err := errors.Unwrap(e.err)
	for err != nil {
		if ee, ok := err.(*Error); ok {
			if len(ee.stack) > 0 {
				deepest = ee.stack
			}
			err = errors.Unwrap(ee.err)
		} else {
			err = errors.Unwrap(err)
		}
	}

	return deepest
}

// compact serializes the error without a stack trace. Used for cause chain entries
// to keep log output concise — only the outermost error includes a stack trace.
func (e *Error) compact() slog.Value {
attrs := []slog.Attr{
slog.String("message", e.Error()),
}

attrs = append(attrs, Attrs(e)...)

if cause := errors.Unwrap(e.err); cause != nil {
if ce, ok := cause.(*Error); ok {
attrs = append(attrs, slog.Attr{Key: "cause", Value: ce.compact()})
} else {
attrs = append(attrs, slog.String("cause", cause.Error()))
}
}

return slog.GroupValue(attrs...)
}
