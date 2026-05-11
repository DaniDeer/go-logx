package dedup

import "log/slog"

// Attrs removes duplicate attr keys. The first occurrence wins.
// For example, if the input contains slog.String("key", "value1") and slog.String("key", "value2"),
// the output will contain slog.String("key", "value1") and the second occurrence will be removed.
func Attrs(attrs []slog.Attr) []slog.Attr {
	seen := map[string]struct{}{}
	out := make([]slog.Attr, 0, len(attrs))

	for _, attr := range attrs {

		if _, ok := seen[attr.Key]; ok {
			continue
		}

		seen[attr.Key] = struct{}{}

		out = append(out, attr)
	}

	return out
}
