package parser

import "testing"

func TestNestedBlockFirstItemBlankBefore(t *testing.T) {
	t.Run("entry_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  child = 1
}
`
		file := parserMustParse(t, "nested_entry_no_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil || len(parent.Body.Entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.Body)
		}
		child := parent.Body.Entries[0]
		if child.BlankBefore != false {
			t.Errorf("child.BlankBefore = %v, want false", child.BlankBefore)
		}
	})

	t.Run("entry_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  child = 1
}
`
		file := parserMustParse(t, "nested_entry_with_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil || len(parent.Body.Entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.Body)
		}
		child := parent.Body.Entries[0]
		if child.BlankBefore != true {
			t.Errorf("child.BlankBefore = %v, want true", child.BlankBefore)
		}
	})

	t.Run("comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### nested comment
  child = 1
}
`
		file := parserMustParse(t, "nested_comment_no_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil || len(parent.Body.Entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.Body)
		}
		child := parent.Body.Entries[0]
		if len(child.Comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.Comments))
		}
		if child.Comments[0].BlankBefore != false {
			t.Errorf("child comment blankBefore = %v, want false", child.Comments[0].BlankBefore)
		}
	})

	t.Run("comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### nested comment
  child = 1
}
`
		file := parserMustParse(t, "nested_comment_with_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil || len(parent.Body.Entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.Body)
		}
		child := parent.Body.Entries[0]
		if len(child.Comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.Comments))
		}
		if child.Comments[0].BlankBefore != true {
			t.Errorf("child comment blankBefore = %v, want true", child.Comments[0].BlankBefore)
		}
	})

	t.Run("trailing_comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### trailing comment
}
`
		file := parserMustParse(t, "nested_trailing_no_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.Body.Trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.Body.Trailing))
		}
		if parent.Body.Trailing[0].BlankBefore != false {
			t.Errorf("trailing comment blankBefore = %v, want false", parent.Body.Trailing[0].BlankBefore)
		}
	})

	t.Run("trailing_comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### trailing comment
}
`
		file := parserMustParse(t, "nested_trailing_with_blank.lkal", src)
		if file.Body == nil || len(file.Body.Entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.Body)
		}
		parent := file.Body.Entries[0]
		if parent.Body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.Body.Trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.Body.Trailing))
		}
		if parent.Body.Trailing[0].BlankBefore != true {
			t.Errorf("trailing comment blankBefore = %v, want true", parent.Body.Trailing[0].BlankBefore)
		}
	})
}

func TestBlankBeforeAroundCommas(t *testing.T) {
	t.Run("blank_line_before_leading_comma", func(t *testing.T) {
		src := "@lkal \"0.1\"\na = 1\n\n, b = 2\n"
		f := parserMustParse(t, "blank_line_before_leading_comma.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 2 {
			t.Fatalf("expected 2 entries in body, got %+v", f.Body)
		}
		if f.Body.Entries[1].Key != "b" {
			t.Fatalf("f.Body.Entries[1].Key = %q, want %q", f.Body.Entries[1].Key, "b")
		}
		if got := f.Body.Entries[1].BlankBefore; got != true {
			t.Errorf("f.Body.Entries[1].BlankBefore = %v, want true", got)
		}
	})

	t.Run("comma_only_line", func(t *testing.T) {
		src := "@lkal \"0.1\"\na = 1\n,\nb = 2\n"
		f := parserMustParse(t, "comma_only_line.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 2 {
			t.Fatalf("expected 2 entries in body, got %+v", f.Body)
		}
		if f.Body.Entries[1].Key != "b" {
			t.Fatalf("f.Body.Entries[1].Key = %q, want %q", f.Body.Entries[1].Key, "b")
		}
		if got := f.Body.Entries[1].BlankBefore; got != false {
			t.Errorf("f.Body.Entries[1].BlankBefore = %v, want false", got)
		}
	})

	t.Run("same_line_comma", func(t *testing.T) {
		src := "@lkal \"0.1\"\na = 1, b = 2\n"
		f := parserMustParse(t, "same_line_comma.lkal", src)
		if f.Body == nil || len(f.Body.Entries) != 2 {
			t.Fatalf("expected 2 entries in body, got %+v", f.Body)
		}
		if f.Body.Entries[1].Key != "b" {
			t.Fatalf("f.Body.Entries[1].Key = %q, want %q", f.Body.Entries[1].Key, "b")
		}
		if got := f.Body.Entries[1].BlankBefore; got != false {
			t.Errorf("f.Body.Entries[1].BlankBefore = %v, want false", got)
		}
	})
}
