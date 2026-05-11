package attr

import (
	"log/slog"

	"github.com/DaniDeer/go-logx/internal/dedup"
)

// Merge merges multiple attr slices. Outermost attrs take precedence.
// For example, if the same key appears in multiple groups, the value from the first group will be used.
func Merge(groups ...[]slog.Attr) []slog.Attr {
	var merged []slog.Attr

	for _, group := range groups {
		merged = append(merged, group...)
	}

	return dedup.Attrs(merged)
}