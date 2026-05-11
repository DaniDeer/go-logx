package attr

import (
	"log/slog"
	"testing"
)

func Test_Merge(t *testing.T) {
	tests := []struct {
		name   string
		groups [][]slog.Attr
		want   []slog.Attr
	}{
		{
			name:   "empty input",
			groups: nil,
			want:   nil,
		},
		{
			name: "single group",
			groups: [][]slog.Attr{
				{slog.String("a", "1"), slog.String("b", "2")},
			},
			want: []slog.Attr{slog.String("a", "1"), slog.String("b", "2")},
		},
		{
			name: "two groups no overlap",
			groups: [][]slog.Attr{
				{slog.String("a", "1")},
				{slog.String("b", "2")},
			},
			want: []slog.Attr{slog.String("a", "1"), slog.String("b", "2")},
		},
		{
			name: "overlapping keys: first group wins",
			groups: [][]slog.Attr{
				{slog.String("key", "first")},
				{slog.String("key", "second")},
			},
			want: []slog.Attr{slog.String("key", "first")},
		},
		{
			name: "partial overlap: unique keys preserved",
			groups: [][]slog.Attr{
				{slog.String("a", "1"), slog.String("shared", "outer")},
				{slog.String("shared", "inner"), slog.String("b", "2")},
			},
			want: []slog.Attr{
				slog.String("a", "1"),
				slog.String("shared", "outer"),
				slog.String("b", "2"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.groups...)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}

			for i := range got {
				if !got[i].Equal(tt.want[i]) {
					t.Errorf("[%d] got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
