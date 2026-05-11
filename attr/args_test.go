package attr

import (
	"log/slog"
	"testing"
)

func Test_Args(t *testing.T) {
	tests := []struct {
		name string
		args []any
		want []slog.Attr
	}{
		{
			name: "empty",
			args: nil,
			want: []slog.Attr{},
		},
		{
			name: "single key-value pair",
			args: []any{"user_id", "u-42"},
			want: []slog.Attr{slog.Any("user_id", "u-42")},
		},
		{
			name: "multiple key-value pairs",
			args: []any{"user_id", "u-42", "method", "GET"},
			want: []slog.Attr{
				slog.Any("user_id", "u-42"),
				slog.Any("method", "GET"),
			},
		},
		{
			name: "slog.Attr passed directly",
			args: []any{slog.String("host", "localhost")},
			want: []slog.Attr{slog.String("host", "localhost")},
		},
		{
			name: "unpaired trailing key becomes !BADKEY",
			args: []any{"orphan"},
			want: []slog.Attr{slog.Any("!BADKEY", "orphan")},
		},
		{
			name: "non-string key becomes !BADKEY",
			args: []any{42, "value"},
			want: []slog.Attr{
				slog.Any("!BADKEY", 42),
				slog.Any("!BADKEY", "value"),
			},
		},
		{
			name: "mixed: valid pair then slog.Attr then bad key",
			args: []any{"k", "v", slog.Int("n", 1), 99},
			want: []slog.Attr{
				slog.Any("k", "v"),
				slog.Int("n", 1),
				slog.Any("!BADKEY", 99),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Args(tt.args...)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if !got[i].Equal(tt.want[i]) {
					t.Errorf("[%d] got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
