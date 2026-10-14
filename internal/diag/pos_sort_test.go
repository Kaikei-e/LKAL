package diag

import (
	"slices"
	"testing"
)

func TestPosString(t *testing.T) {
	tests := []struct {
		pos  Pos
		want string
	}{
		{Pos{Line: 1, Col: 1}, "1:1"},
		{Pos{Line: 42, Col: 108}, "42:108"},
		{Pos{Line: 0, Col: 0}, "0:0"},
	}
	for _, tt := range tests {
		if got := tt.pos.String(); got != tt.want {
			t.Errorf("Pos%+v.String() = %q, want %q", tt.pos, got, tt.want)
		}
	}
}

func TestLineCols(t *testing.T) {
	t.Run("EmptyLine", func(t *testing.T) {
		got := LineCols(nil, 1)
		want := []int{1}
		if !slices.Equal(got, want) {
			t.Errorf("LineCols(nil, 1) = %v, want %v", got, want)
		}
	})

	t.Run("ASCIIWithOffsetStartCol", func(t *testing.T) {
		got := LineCols([]byte("abc"), 5)
		want := []int{5, 6, 7, 8}
		if !slices.Equal(got, want) {
			t.Errorf("LineCols(\"abc\", 5) = %v, want %v", got, want)
		}
	})

	t.Run("MultibyteUTF8", func(t *testing.T) {
		// "あa" -> 'あ' is 3 bytes (col 1), 'a' is 1 byte (col 2), end is col 3.
		line := []byte("あa")
		got := LineCols(line, 1)
		want := []int{1, 1, 1, 2, 3}
		if !slices.Equal(got, want) {
			t.Errorf("LineCols(%q, 1) = %v, want %v", line, got, want)
		}
	})
}

func TestSortErrorsPureAndCapped(t *testing.T) {
	e1 := NewError("a.lkal", Pos{Line: 3, Col: 2}, "L102", "")
	e2 := NewError("a.lkal", Pos{Line: 1, Col: 5}, "L201", "")
	e3 := NewError("a.lkal", Pos{Line: 1, Col: 5}, "L101", "")
	e4 := NewError("a.lkal", Pos{Line: 1, Col: 2}, "L106", "")

	input := []*Error{e1, e2, e3, e4}
	snapshot := slices.Clone(input)

	sorted := SortErrors(input)

	// Verify purity: input slice is not mutated.
	if !slices.Equal(input, snapshot) {
		t.Fatalf("SortErrors mutated input slice: got %v, want %v", input, snapshot)
	}

	wantOrder := []*Error{e4, e3, e2, e1}
	if !slices.Equal(sorted, wantOrder) {
		t.Errorf("SortErrors order mismatch:\ngot:  %v\nwant: %v", sorted, wantOrder)
	}

	// Verify cap at MaxErrors (20).
	many := make([]*Error, 25)
	for i := range many {
		many[i] = NewError("a.lkal", Pos{Line: 25 - i, Col: 1}, "L106", "")
	}
	capped := SortErrors(many)
	if len(capped) != MaxErrors {
		t.Fatalf("len(SortErrors(25)) = %d, want %d", len(capped), MaxErrors)
	}
	if capped[0].Pos.Line != 1 || capped[MaxErrors-1].Pos.Line != MaxErrors {
		t.Errorf("unexpected capped range: first line=%d, last line=%d", capped[0].Pos.Line, capped[MaxErrors-1].Pos.Line)
	}
}
