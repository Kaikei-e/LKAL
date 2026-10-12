package lkal

import (
	"testing"
)

type expectedError struct {
	code string
	line int
	col  int
}

func assertExactErrors(t *testing.T, name, src string, want []expectedError) {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err == nil {
		t.Fatalf("Parse(%q) succeeded unexpectedly, want %d error(s)", name, len(want))
	}
	if file != nil {
		t.Errorf("Parse(%q) returned non-nil File on error: %v", name, file)
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("Parse(%q) returned error of type %T, want *ErrorList", name, err)
	}
	if len(errList.Errors) != len(want) {
		t.Fatalf("Parse(%q) returned %d error(s), want %d:\ngot:\n%s", name, len(errList.Errors), len(want), errList.Error())
	}
	for i, w := range want {
		got := errList.Errors[i]
		if got.Code != w.code || got.Pos.Line != w.line || got.Pos.Col != w.col {
			t.Errorf("Parse(%q) error[%d] mismatch: got %s at %d:%d, want %s at %d:%d",
				name, i, got.Code, got.Pos.Line, got.Pos.Col, w.code, w.line, w.col)
		}
	}
}

func parseSuccessful(t *testing.T, name, src string) *File {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("Parse(%q) failed unexpectedly: %v", name, err)
	}
	if file == nil {
		t.Fatalf("Parse(%q) returned nil File with nil error", name)
	}
	return file
}

func TestInitialBOMProducesOnlyLexicalError(t *testing.T) {
	src := "\xEF\xBB\xBF@lkal \"0.1\"\na = 1\n"
	assertExactErrors(t, "initial_bom.lkal", src, []expectedError{
		{code: "L008", line: 1, col: 1},
	})
}

func TestIllegalTokenSuppressesParserErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "version_value_illegal_character",
			src:  "@lkal ;\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 7},
			},
		},
		{
			name: "version_value_unclosed_string",
			src:  "@lkal \"0.1\n",
			want: []expectedError{
				{code: "L007", line: 1, col: 7},
			},
		},
		{
			name: "version_trailer",
			src:  "@lkal \"0.1\" ;\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 13},
			},
		},
		{
			name: "key_trailer",
			src:  "@lkal \"0.1\"\na ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 3},
			},
		},
		{
			name: "label_trailer",
			src:  "@lkal \"0.1\"\na \"lbl\" ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 9},
			},
		},
		{
			name: "value_separator",
			src:  "@lkal \"0.1\"\na = 1 ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 7},
			},
		},
		{
			name: "block_separator",
			src:  "@lkal \"0.1\"\na {} ;\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 6},
			},
		},
		{
			name: "list_separator",
			src:  "@lkal \"0.1\"\na = [1 ;]\n",
			want: []expectedError{
				{code: "L008", line: 2, col: 8},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestMultilineAssignBlockRecoveryReportsSingleError(t *testing.T) {
	src := `@lkal "0.1"
a = {
  b = 1
}
`
	assertExactErrors(t, "multiline_assign_block.lkal", src, []expectedError{
		{code: "L106", line: 2, col: 5},
	})
}

func TestNestedBlockFirstItemBlankBefore(t *testing.T) {
	t.Run("entry_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  child = 1
}
`
		file := parseSuccessful(t, "nested_entry_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if child.blankBefore != false {
			t.Errorf("child.blankBefore = %v, want false", child.blankBefore)
		}
	})

	t.Run("entry_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  child = 1
}
`
		file := parseSuccessful(t, "nested_entry_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if child.blankBefore != true {
			t.Errorf("child.blankBefore = %v, want true", child.blankBefore)
		}
	})

	t.Run("comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### nested comment
  child = 1
}
`
		file := parseSuccessful(t, "nested_comment_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if len(child.comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.comments))
		}
		if child.comments[0].blankBefore != false {
			t.Errorf("child comment blankBefore = %v, want false", child.comments[0].blankBefore)
		}
	})

	t.Run("comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### nested comment
  child = 1
}
`
		file := parseSuccessful(t, "nested_comment_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if len(child.comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.comments))
		}
		if child.comments[0].blankBefore != true {
			t.Errorf("child comment blankBefore = %v, want true", child.comments[0].blankBefore)
		}
	})

	t.Run("trailing_comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### trailing comment
}
`
		file := parseSuccessful(t, "nested_trailing_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.body.trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.body.trailing))
		}
		if parent.body.trailing[0].blankBefore != false {
			t.Errorf("trailing comment blankBefore = %v, want false", parent.body.trailing[0].blankBefore)
		}
	})

	t.Run("trailing_comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### trailing comment
}
`
		file := parseSuccessful(t, "nested_trailing_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.body.trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.body.trailing))
		}
		if parent.body.trailing[0].blankBefore != true {
			t.Errorf("trailing comment blankBefore = %v, want true", parent.body.trailing[0].blankBefore)
		}
	})
}

func TestInterruptedMultilineRawStringReportsSingleError(t *testing.T) {
	src := `@lkal "0.1"
a =
  \\one

  \\two
  \\three
`
	assertExactErrors(t, "interrupted_multiline_raw.lkal", src, []expectedError{
		{code: "L104", line: 5, col: 3},
	})
}

func TestNestedBlockRecoveryResumesTopLevel(t *testing.T) {
	src := `@lkal "0.1"
a {
  b = 1 2
}
c = x
`
	assertExactErrors(t, "nested_recovery_resume.lkal", src, []expectedError{
		{code: "L106", line: 3, col: 9},
		{code: "L102", line: 5, col: 5},
	})
}

func TestSemanticErrorsPreservedBeforeUnclosedBlock(t *testing.T) {
	src := `@lkal "0.1"
x = 1
x = 2
a {
`
	assertExactErrors(t, "unclosed_block_semantic_preserved.lkal", src, []expectedError{
		{code: "L201", line: 3, col: 1},
		{code: "L103", line: 4, col: 3},
	})
}

func TestListRecoveryPreservesSubsequentEntry(t *testing.T) {
	src := `@lkal "0.1"
a = [1 2]
b = x
`
	assertExactErrors(t, "list_recovery_preserves_subsequent.lkal", src, []expectedError{
		{code: "L106", line: 2, col: 8},
		{code: "L102", line: 3, col: 5},
	})
}

func TestKindMixingRepeatedReportsConflictingKind(t *testing.T) {
	t.Run("element_assign_element", func(t *testing.T) {
		src := `@lkal "0.1"
a "x" {}
a = 1
a "y" {}
`
		assertExactErrors(t, "kind_mixing_elem_assign_elem.lkal", src, []expectedError{
			{code: "L202", line: 3, col: 1},
			{code: "L202", line: 4, col: 1},
		})
	})

	t.Run("record_element_record", func(t *testing.T) {
		src := `@lkal "0.1"
a {}
a "x" {}
a {}
`
		assertExactErrors(t, "kind_mixing_record_elem_record.lkal", src, []expectedError{
			{code: "L202", line: 3, col: 1},
			{code: "L202", line: 4, col: 1},
		})
	})
}
