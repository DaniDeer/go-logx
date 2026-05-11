package attr

import "log/slog"

// Args converts alternating key/value pairs into slog attrs.
// If an odd number of args is provided, the last key will be logged with a "!BADKEY" prefix and a value of the key itself.
// If a non-string key is provided, it will also be logged with a "!BADKEY" prefix and the value of the key.
func Args(args ...any) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(args)/2)

	for i := 0; i < len(args); {
		switch v := args[i].(type) {

		case slog.Attr:
			attrs = append(attrs, v)
			i++

		case string:
			if i+1 >= len(args) {
				attrs = append(attrs,
					slog.Any("!BADKEY", v),
				)
				i++
				continue
			}

			attrs = append(attrs,
				slog.Any(v, args[i+1]),
			)

			i += 2

		default:
			attrs = append(attrs,
				slog.Any("!BADKEY", v),
			)
			i++
		}
	}

	return attrs
}