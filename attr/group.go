package attr

import "log/slog"

// Group creates a slog.Attr with a group value. The group value is a collection of attributes that can be nested within other groups.
// For example, Group("user", slog.String("id", "123"), slog.String("name", "Alice")) would create an attribute with the key "user" and a group value containing the attributes "id" and "name".
func Group(name string, attrs ...slog.Attr) slog.Attr {
	//return slog.Group(name, attrs)
	return slog.Attr{
		Key:   name,
		Value: slog.GroupValue(attrs...),
	}
}
