package errx

import (
	"errors"
	"log/slog"
	"testing"
)

func Test_Join_allNilReturnsNil(t *testing.T) {
	if got := Join(nil, nil, nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func Test_Join_emptyReturnsNil(t *testing.T) {
	if got := Join(); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func Test_Join_filtersNils(t *testing.T) {
	err := New("only one")
	got := Join(nil, err, nil)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	m, ok := got.(*MultiError)
	if !ok {
		t.Fatal("expected *MultiError")
	}
	if len(m.errs) != 1 {
		t.Errorf("errs len = %d, want 1", len(m.errs))
	}
}

func Test_Join_Error_joinsMsgs(t *testing.T) {
	tests := []struct {
		name    string
		errs    []error
		wantMsg string
	}{
		{
			name:    "single error",
			errs:    []error{New("alpha")},
			wantMsg: "alpha",
		},
		{
			name:    "two errors",
			errs:    []error{New("alpha"), New("beta")},
			wantMsg: "alpha; beta",
		},
		{
			name:    "three errors",
			errs:    []error{New("a"), New("b"), New("c")},
			wantMsg: "a; b; c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Join(tt.errs...)
			if got == nil {
				t.Fatal("expected non-nil")
			}
			if got.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), tt.wantMsg)
			}
		})
	}
}

func Test_Join_Unwrap_errorsIs(t *testing.T) {
	sentinel := New("sentinel")
	other := New("other")
	joined := Join(sentinel, other)

	if !errors.Is(joined, sentinel) {
		t.Error("errors.Is should find sentinel in joined error")
	}
	if !errors.Is(joined, other) {
		t.Error("errors.Is should find other in joined error")
	}
}

func Test_Join_Unwrap_errorsAs(t *testing.T) {
	e1 := New("first", "k", "v")
	joined := Join(e1, errors.New("plain"))

	var target *Error
	if !errors.As(joined, &target) {
		t.Error("errors.As should find *Error in joined error")
	}
}

func Test_Join_LogValue_indexedGroups(t *testing.T) {
	e0 := New("zero", "field", "email")
	e1 := New("one", "field", "name")
	joined := Join(e0, e1)

	lv, ok := joined.(slog.LogValuer)
	if !ok {
		t.Fatal("*MultiError must implement slog.LogValuer")
	}

	val := lv.LogValue()
	group := val.Group()

	if len(group) != 2 {
		t.Fatalf("expected 2 attrs in group, got %d", len(group))
	}
	if group[0].Key != "0" {
		t.Errorf("group[0].Key = %q, want %q", group[0].Key, "0")
	}
	if group[1].Key != "1" {
		t.Errorf("group[1].Key = %q, want %q", group[1].Key, "1")
	}
}

func Test_Join_LogValue_plainErrorRendersAsString(t *testing.T) {
	plain := errors.New("plain error")
	joined := Join(plain)

	lv := joined.(slog.LogValuer)
	val := lv.LogValue()
	group := val.Group()

	if len(group) != 1 {
		t.Fatalf("expected 1 attr, got %d", len(group))
	}
	// Plain errors are not LogValuer; slog renders them via their Error() string.
	// The Value kind will be KindAny wrapping the error itself.
	if group[0].Value.Any() != plain {
		t.Errorf("expected plain error as Any value, got %v", group[0].Value)
	}
}
