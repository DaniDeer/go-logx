package errx

import (
	"errors"
	"log/slog"

	"github.com/DaniDeer/go-logx/attr"
)

type attrError interface {
	Attrs() []slog.Attr
}

func Attrs(err error) []slog.Attr {
	var groups [][]slog.Attr

	for err != nil {
		if ae, ok := err.(attrError); ok {
			groups = append(groups, ae.Attrs())
		}

		err = errors.Unwrap(err)
	}

	return attr.Merge(groups...)
}