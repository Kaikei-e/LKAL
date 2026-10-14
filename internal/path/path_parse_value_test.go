package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"fmt"
	"testing"
)

func TestParseValue(t *testing.T) {
	validTests := []struct {
		name string
		src  string
		want Value
	}{
		{
			name: "scalar_string",
			src:  `"hello"`,
			want: String("hello"),
		},
		{
			name: "scalar_int_positive",
			src:  "42",
			want: Int(42),
		},
		{
			name: "scalar_int_negative",
			src:  "-17",
			want: Int(-17),
		},
		{
			name: "scalar_decimal",
			src:  "0.25",
			want: Decimal("0.25"),
		},
		{
			name: "scalar_decimal_normalized",
			src:  "1.50",
			want: Decimal("1.5"),
		},
		{
			name: "scalar_bool_true",
			src:  "true",
			want: Bool(true),
		},
		{
			name: "scalar_bool_false",
			src:  "false",
			want: Bool(false),
		},
		{
			name: "scalar_enum",
			src:  ".tcp",
			want: Enum("tcp"),
		},
		{
			name: "scalar_quantity",
			src:  "512Mi",
			want: Quantity{Number: "512", Unit: "Mi"},
		},
		{
			name: "scalar_quantity_decimal_normalized",
			src:  "1.50Gi",
			want: Quantity{Number: "1.5", Unit: "Gi"},
		},
		{
			name: "scalar_ref",
			src:  "&secret/db-creds",
			want: Ref{"secret", "db-creds"},
		},
		{
			name: "list_scalars",
			src:  "[1, 2, 3]",
			want: List{Int(1), Int(2), Int(3)},
		},
		{
			name: "list_empty",
			src:  "[]",
			want: List{},
		},
		{
			name: "list_multiline",
			src: `[
  "a",
  "b"
]`,
			want: List{String("a"), String("b")},
		},
		{
			name: "raw_string_lines",
			src:  "\\\\one\n\\\\two",
			want: String("one\ntwo"),
		},
		{
			name: "raw_string_surrounded_by_comments",
			src: `### comment before
\\line 1
\\line 2
### comment after
`,
			want: String("line 1\nline 2"),
		},
		{
			name: "record_oneline",
			src:  "{ a = 1, b { c = 2 } }",
			want: Record{Fields: []Field{
				{Key: "a", Value: Int(1)},
				{Key: "b", Value: Record{Fields: []Field{{Key: "c", Value: Int(2)}}}},
			}},
		},
		{
			name: "record_multiline_with_comments",
			src: `{
  ### note
  a = 1
  ### another note
  b = 2
}`,
			want: Record{Fields: []Field{
				{Key: "a", Value: Int(1)},
				{Key: "b", Value: Int(2)},
			}},
		},
		{
			name: "collection_comma_separated",
			src:  `"a" { x = 1 }, "b" {}`,
			want: Collection{Elems: []Elem{
				{Label: "a", Body: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}},
				{Label: "b", Body: Record{}},
			}},
		},
		{
			name: "collection_newline_separated",
			src:  "\"a\" { x = 1 }\n\"b\" {}",
			want: Collection{Elems: []Elem{
				{Label: "a", Body: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}},
				{Label: "b", Body: Record{}},
			}},
		},
		{
			name: "collection_trailing_comma",
			src:  `"a" {},`,
			want: Collection{Elems: []Elem{
				{Label: "a", Body: Record{}},
			}},
		},
		{
			name: "collection_trailing_blank_and_comment_lines",
			src:  "\"a\" {}\n\n### trailing comment\n",
			want: Collection{Elems: []Elem{
				{Label: "a", Body: Record{}},
			}},
		},
		{
			name: "leading_and_trailing_comments_and_blank_lines",
			src: `
### header

42

### footer
`,
			want: Int(42),
		},
	}

	for _, tc := range validTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err != nil {
				t.Fatalf("ParseValue(%q) unexpected error: %v", tc.src, err)
			}
			if val == nil {
				t.Fatalf("ParseValue(%q) returned nil Value", tc.src)
			}
			pathTCheckEqualAndOrder(t, val, tc.want)
		})
	}

	errorTests := []struct {
		name     string
		src      string
		wantCode string
		wantTag  string
		wantHint string
	}{
		{
			name:     "bare_word",
			src:      "tcp",
			wantCode: "L102",
			wantTag:  "L102 1:1",
			wantHint: `write "tcp" for a string or .tcp for an enum`,
		},
		{
			name:     "empty_input",
			src:      "",
			wantCode: "L106",
			wantTag:  "L106 1:1",
			wantHint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
		},
		{
			name:     "extra_token",
			src:      "1 2",
			wantCode: "L106",
			wantTag:  "L106 1:3",
			wantHint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
		},
		{
			name:     "duplicate_key_in_record",
			src:      "{ a = 1, a = 2 }",
			wantCode: "L201",
			wantTag:  "L201 1:10",
			wantHint: "remove one of them",
		},
		{
			name:     "list_mixes_types",
			src:      `[1, "x"]`,
			wantCode: "L204",
			wantTag:  "L204 1:5",
			wantHint: "use one type for all elements",
		},
		{
			name:     "duplicate_label_in_collection",
			src:      "\"a\" {}\n\"a\" {}",
			wantCode: "L201",
			wantTag:  "L201 2:1",
			wantHint: "remove one of them",
		},
		{
			name:     "duplicate_label_in_comma_separated_collection",
			src:      `"a" { x = 1 }, "a" { x = 2 }`,
			wantCode: "L201",
			wantTag:  "L201 1:16",
			wantHint: "remove one of them",
		},
		{
			name:     "unclosed_bracket_brace",
			src:      "{",
			wantCode: "L103",
			wantTag:  "L103 1:1",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "unclosed_bracket_list",
			src:      "[",
			wantCode: "L103",
			wantTag:  "L103 1:1",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "dollar_key_inside_braces",
			src:      "{ \"$x\" = 1 }",
			wantCode: "L205",
			wantTag:  "L205 1:3",
			wantHint: "rename the key",
		},
		{
			name:     "collection_missing_separator_two_blocks",
			src:      `"a" {} "b" {}`,
			wantCode: "L106",
			wantTag:  "L106 1:8",
			wantHint: "separate entries with a newline or a comma",
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err == nil {
				t.Fatalf("ParseValue(%q) succeeded unexpectedly, got %#v", tc.src, val)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if len(errList.Errors) == 0 {
				t.Fatalf("expected at least 1 error in list, got 0")
			}
			e := errList.Errors[0]
			if e.File != "" {
				t.Errorf("File = %q, want empty", e.File)
			}
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if tc.wantHint != "" && e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			gotTag := fmt.Sprintf("%s %d:%d", e.Code, e.Pos.Line, e.Pos.Col)
			if gotTag != tc.wantTag {
				t.Errorf("got %q, want %q", gotTag, tc.wantTag)
			}
		})
	}
}
