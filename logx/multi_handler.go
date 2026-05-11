package logx

import (
	"context"
	"log/slog"
)

type MultiHandler struct {
	handlers []slog.Handler
}

func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &MultiHandler{
		handlers: handlers,
	}
}

func (h *MultiHandler) Enabled(
	ctx context.Context,
	level slog.Level,
) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

func (h *MultiHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, record); err != nil {
			return err
		}
	}

	return nil
}

func (h *MultiHandler) WithAttrs(
	attrs []slog.Attr,
) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))

	for _, handler := range h.handlers {
		handlers = append(handlers,
			handler.WithAttrs(attrs),
		)
	}

	return &MultiHandler{
		handlers: handlers,
	}
}

func (h *MultiHandler) WithGroup(
	name string,
) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))

	for _, handler := range h.handlers {
		handlers = append(handlers,
			handler.WithGroup(name),
		)
	}

	return &MultiHandler{
		handlers: handlers,
	}
}
