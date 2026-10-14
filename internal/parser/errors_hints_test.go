package parser_test

import "testing"

func TestDirectiveOutOfPlace(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "directive_in_value_position",
			src:  "@lkal \"0.1\"\na =\n@lkal \"0.1\"\n",
			want: []expectedError{
				{code: "L105", line: 3, col: 1},
			},
		},
		{
			name: "directive_in_list_position",
			src:  "@lkal \"0.1\"\na = [\n@lkal\n]\n",
			want: []expectedError{
				{code: "L105", line: 3, col: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestStrayRawStrings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "gap_between_raw_lines_resumes_valid_entry",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n\n\\\\y\nb =\n[1,\n2]\n",
			want: []expectedError{
				{code: "L104", line: 5, col: 1},
			},
		},
		{
			name: "gap_between_raw_lines_preserves_block_semantic_error",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n\n\\\\y\nb {\nc = 1\nc = 2\n}\n",
			want: []expectedError{
				{code: "L104", line: 5, col: 1},
				{code: "L201", line: 8, col: 1},
			},
		},
		{
			name: "duplicate_key_precedes_raw_gap_error",
			src:  "@lkal \"0.1\"\na = 1\na =\n\\\\x\n\n\\\\y\n",
			want: []expectedError{
				{code: "L201", line: 3, col: 1},
				{code: "L104", line: 6, col: 1},
			},
		},
		{
			name: "top_level_raw_run_without_assign",
			src:  "@lkal \"0.1\"\n\\\\x\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 2, col: 1},
			},
		},
		{
			name: "stray_raw_run_after_brace_recovery",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n}\n\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 4, col: 1},
				{code: "L106", line: 6, col: 1},
			},
		},
		{
			name: "comma_line_between_raw_values_yields_l106",
			src:  "@lkal \"0.1\"\na =\n\\\\x\n,\n\\\\y\n",
			want: []expectedError{
				{code: "L106", line: 5, col: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestBOMRecovery(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "bom_before_head_comment",
			src:  "\ufeff### c\n@lkal \"0.1\"\na = 1\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
			},
		},
		{
			name: "bom_missing_version",
			src:  "\ufeffa = 1\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L105", line: 1, col: 2},
			},
		},
		{
			name: "bom_unsupported_version",
			src:  "\ufeff@lkal \"0.2\"\n",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L207", line: 1, col: 8},
			},
		},
		{
			name: "bom_empty_document",
			src:  "\ufeff",
			want: []expectedError{
				{code: "L008", line: 1, col: 1},
				{code: "L105", line: 1, col: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestHeaderRecovery(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "missing_version_string_resumes_multiline_entry",
			src:  "@lkal\na =\n[1,\n2]\n",
			want: []expectedError{
				{code: "L105", line: 1, col: 6},
			},
		},
		{
			name: "missing_version_string_resumes_subsequent_entry",
			src:  "@lkal\na = b\n",
			want: []expectedError{
				{code: "L105", line: 1, col: 6},
				{code: "L102", line: 2, col: 5},
			},
		},
		{
			name: "unexpected_brace_on_header_line_recovers_to_next_line",
			src:  "@lkal \"0.1\" {\nb = c\n}\nd = e\n",
			want: []expectedError{
				{code: "L106", line: 1, col: 13},
				{code: "L102", line: 4, col: 5},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestEOFInsideBrackets(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []expectedError
	}{
		{
			name: "eof_inside_list",
			src:  "@lkal \"0.1\"\na = [1, \"x\"",
			want: []expectedError{
				{code: "L103", line: 2, col: 5},
			},
		},
		{
			name: "eof_after_equal_in_record",
			src:  "@lkal \"0.1\"\na {\n b =",
			want: []expectedError{
				{code: "L103", line: 2, col: 3},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExactErrors(t, tc.name+".lkal", tc.src, tc.want)
		})
	}
}

func TestUnexpectedTokenHints(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		line     int
		col      int
		wantHint string
	}{
		{
			name:     "entry_start_closing_brace",
			src:      "@lkal \"0.1\"\n}\n",
			line:     2,
			col:      1,
			wantHint: `start an entry with key = value, key { ... } or key "label" { ... }`,
		},
		{
			name:     "entry_start_raw_line",
			src:      "@lkal \"0.1\"\n\\\\x\n",
			line:     2,
			col:      1,
			wantHint: `start an entry with key = value, key { ... } or key "label" { ... }`,
		},
		{
			name:     "keyword_as_key",
			src:      "@lkal \"0.1\"\ntrue = 1\n",
			line:     2,
			col:      1,
			wantHint: "quote a reserved word to use it as a key",
		},
		{
			name:     "after_key",
			src:      "@lkal \"0.1\"\na 1\n",
			line:     2,
			col:      3,
			wantHint: `follow the key with =, { or "label"`,
		},
		{
			name:     "after_label",
			src:      "@lkal \"0.1\"\na \"x\" 1\n",
			line:     2,
			col:      7,
			wantHint: "follow the label with {",
		},
		{
			name:     "after_value",
			src:      "@lkal \"0.1\"\na = 1 b = 2\n",
			line:     2,
			col:      7,
			wantHint: "separate entries with a newline or a comma",
		},
		{
			name:     "brace_after_equal",
			src:      "@lkal \"0.1\"\na = { b = 1 }\n",
			line:     2,
			col:      5,
			wantHint: "write a record without =: key { ... }",
		},
		{
			name:     "nested_list_in_list",
			src:      "@lkal \"0.1\"\na = [[1]]\n",
			line:     2,
			col:      6,
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "record_in_list",
			src:      "@lkal \"0.1\"\na = [{}]\n",
			line:     2,
			col:      6,
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "missing_comma_in_list",
			src:      "@lkal \"0.1\"\na = [1 2]\n",
			line:     2,
			col:      8,
			wantHint: "separate list elements with ,",
		},
		{
			name:     "unexpected_closing_bracket_in_assign",
			src:      "@lkal \"0.1\"\na = ]\n",
			line:     2,
			col:      5,
			wantHint: `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`,
		},
		{
			name:     "unexpected_equal_in_list",
			src:      "@lkal \"0.1\"\na = [=]\n",
			line:     2,
			col:      6,
			wantHint: `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`,
		},
		{
			name:     "comment_between_assign_and_value",
			src:      "@lkal \"0.1\"\na =\n### c\n",
			line:     3,
			col:      1,
			wantHint: "move the comment above the entry",
		},
		{
			name:     "version_trailing_character",
			src:      "@lkal \"0.1\" a = 1\n",
			line:     1,
			col:      13,
			wantHint: "end the line after the version declaration",
		},
		{
			name:     "closing_brace_after_top_level_value",
			src:      "@lkal \"0.1\"\na = [1, \"x\"] }\n",
			line:     2,
			col:      14,
			wantHint: "separate entries with a newline or a comma",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertSingleErrorHint(t, tc.name+".lkal", tc.src, "L106", tc.line, tc.col, tc.wantHint)
		})
	}
}
