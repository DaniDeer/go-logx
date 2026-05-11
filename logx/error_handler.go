package logx

import (
	"context"
	"log/slog"

	"github.com/DaniDeer/go-logx/errx"
)

type ErrorHandler struct {
	next slog.Handler
}

func NewErrorHandler(next slog.Handler) slog.Handler {
	return &ErrorHandler{
		next: next,
	}
}

func (h *ErrorHandler) Enabled(
	ctx context.Context,
	level slog.Level,
) bool {
	return h.next.Enabled(ctx, level)
}

func (h *ErrorHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {

	newRecord := slog.NewRecord(
		record.Time,
		record.Level,
		record.Message,
		record.PC,
	)

	record.Attrs(func(a slog.Attr) bool {

		newRecord.AddAttrs(a)

		if err, ok := a.Value.Any().(error); ok {
			newRecord.AddAttrs(errx.Attrs(err)...)
		}

		return true
	})

	return h.next.Handle(ctx, newRecord)
}

func (h *ErrorHandler) WithAttrs(
	attrs []slog.Attr,
) slog.Handler {
	return &ErrorHandler{
		next: h.next.WithAttrs(attrs),
	}
}

func (h *ErrorHandler) WithGroup(
	name string,
) slog.Handler {
	return &ErrorHandler{
		next: h.next.WithGroup(name),
	}
}