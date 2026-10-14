package parser

import (
	"strings"
	"testing"
)

func TestParse_Errors(t *testing.T) {
	t.Run("spec_error_rendering", func(t *testing.T) {
		src := `@lkal "0.1"










port =    tcp
`
		file, err := Parse("web.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected Parse to fail, but succeeded")
		}
		if file != nil {
			t.Errorf("expected nil File on error, got %v", file)
		}
		want := "web.lkal:12:11: L102 bare word in value position\n  hint: write \"tcp\" for a string or .tcp for an enum"
		if got := err.Error(); got != want {
			t.Errorf("err.Error() mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("syntax_and_semantic_codes", func(t *testing.T) {
		tests := []struct {
			name     string
			src      string
			wantCode string
			wantLine int
			wantCol  int
		}{
			{
				name:     "L101_heading_newline",
				src:      "@lkal \"0.1\"\nservice \"web\"\n{ replicas = 1 }",
				wantCode: "L101",
				wantLine: 2,
				wantCol:  14,
			},
			{
				name:     "L102_bare_word",
				src:      "@lkal \"0.1\"\nval = bare",
				wantCode: "L102",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L103_unclosed_brace",
				src:      "@lkal \"0.1\"\nrec {",
				wantCode: "L103",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L103_unclosed_bracket",
				src:      "@lkal \"0.1\"\nlst = [1, 2",
				wantCode: "L103",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L104_raw_gap",
				src:      "@lkal \"0.1\"\na =\n  \\\\line1\n\n  \\\\line2",
				wantCode: "L104",
				wantLine: 5,
				wantCol:  3,
			},
			{
				name:     "L105_missing_version",
				src:      "a = 1",
				wantCode: "L105",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L105_bad_version_token",
				src:      "@lkal 123\n",
				wantCode: "L105",
				wantLine: 1,
				wantCol:  7,
			},
			{
				name:     "L105_duplicate_version",
				src:      "@lkal \"0.1\"\n@lkal \"0.1\"\n",
				wantCode: "L105",
				wantLine: 2,
				wantCol:  1,
			},
			{
				name:     "L106_unexpected_token",
				src:      "@lkal \"0.1\"\na = 1 b = 2",
				wantCode: "L106",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L106_brace_after_equal",
				src:      "@lkal \"0.1\"\na = { b = 1 }",
				wantCode: "L106",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L201_duplicate_key",
				src:      "@lkal \"0.1\"\na = 1\na = 2",
				wantCode: "L201",
				wantLine: 3,
				wantCol:  1,
			},
			{
				name:     "L201_duplicate_label",
				src:      "@lkal \"0.1\"\ns \"item\" {}\ns \"item\" {}",
				wantCode: "L201",
				wantLine: 3,
				wantCol:  3,
			},
			{
				name:     "L202_conflicting_kind",
				src:      "@lkal \"0.1\"\na = 1\na {}",
				wantCode: "L202",
				wantLine: 3,
				wantCol:  1,
			},
			{
				name:     "L203_discontinuous_collection",
				src:      "@lkal \"0.1\"\ns \"a\" {}\nx = 1\ns \"b\" {}",
				wantCode: "L203",
				wantLine: 4,
				wantCol:  1,
			},
			{
				name:     "L204_mixed_list",
				src:      "@lkal \"0.1\"\nlst = [1, \"two\"]",
				wantCode: "L204",
				wantLine: 2,
				wantCol:  11,
			},
			{
				name:     "L205_dollar_key",
				src:      "@lkal \"0.1\"\n\"$key\" = 1",
				wantCode: "L205",
				wantLine: 2,
				wantCol:  1,
			},
			{
				name:     "L206_assign_to_collection_elem",
				src:      "@lkal \"0.1\"\ns \"a\" = 1",
				wantCode: "L206",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L207_unsupported_version",
				src:      "@lkal \"0.2\"\n",
				wantCode: "L207",
				wantLine: 1,
				wantCol:  7,
			},
			{
				name:     "L001_bad_comment",
				src:      "# comment\n@lkal \"0.1\"\n",
				wantCode: "L001",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L002_inline_comment",
				src:      "@lkal \"0.1\"\na = 1 # bad\n",
				wantCode: "L002",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L002_list_comment",
				src:      "@lkal \"0.1\"\na = [1, # bad\n 2]\n",
				wantCode: "L002",
				wantLine: 2,
				wantCol:  9,
			},
			{
				name:     "L002_list_comment_line",
				src:      "@lkal \"0.1\"\na = [\n  ### c\n  1,\n]\n",
				wantCode: "L002",
				wantLine: 3,
				wantCol:  3,
			},
			{
				name:     "L003_bad_escape",
				src:      "@lkal \"0.1\"\na = \"\\q\"\n",
				wantCode: "L003",
				wantLine: 2,
				wantCol:  6,
			},
			{
				name:     "L004_digit_after_unit",
				src:      "@lkal \"0.1\"\na = 1h30m\n",
				wantCode: "L004",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L005_integer_overflow",
				src:      "@lkal \"0.1\"\na = 99999999999999999999999999999999999999999\n",
				wantCode: "L005",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L006_unsupported_directive",
				src:      "@custom \"0.1\"\n",
				wantCode: "L006",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L007_unclosed_string",
				src:      "@lkal \"0.1\"\na = \"unclosed\n",
				wantCode: "L007",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L008_illegal_character",
				src:      "@lkal \"0.1\"\na = 1 ;\n",
				wantCode: "L008",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L009_malformed_number",
				src:      "@lkal \"0.1\"\na = 01\n",
				wantCode: "L009",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L009_malformed_ref",
				src:      "@lkal \"0.1\"\na = &a/\n",
				wantCode: "L009",
				wantLine: 2,
				wantCol:  5,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				errList := parserMustFail(t, tc.name+".lkal", tc.src)
				if len(errList.Errors) == 0 {
					t.Fatalf("errList.Errors is empty")
				}
				first := errList.Errors[0]
				if first.Code != tc.wantCode {
					t.Errorf("error code mismatch: got %q, want %q", first.Code, tc.wantCode)
				}
				if first.Pos.Line != tc.wantLine || first.Pos.Col != tc.wantCol {
					t.Errorf("error pos mismatch: got %v, want %d:%d", first.Pos, tc.wantLine, tc.wantCol)
				}
				if first.Msg == "" {
					t.Errorf("error Msg should not be empty")
				}
				if first.Hint == "" {
					t.Errorf("error Hint should not be empty")
				}
			})
		}
	})

	t.Run("error_sorting_and_truncation", func(t *testing.T) {
		src := "@lkal \"0.1\"\n" + strings.Repeat("?\n", 25)
		errList := parserMustFail(t, "truncation.lkal", src)
		if len(errList.Errors) != 20 {
			t.Errorf("len(errList.Errors) = %d, want 20 (truncated)", len(errList.Errors))
		}
		for i := 1; i < len(errList.Errors); i++ {
			prev := errList.Errors[i-1]
			curr := errList.Errors[i]
			if prev.Pos.Line > curr.Pos.Line ||
				(prev.Pos.Line == curr.Pos.Line && prev.Pos.Col > curr.Pos.Col) ||
				(prev.Pos.Line == curr.Pos.Line && prev.Pos.Col == curr.Pos.Col && prev.Code > curr.Code) {
				t.Errorf("errors not sorted: [%d]=%v:%s vs [%d]=%v:%s", i-1, prev.Pos, prev.Code, i, curr.Pos, curr.Code)
			}
		}
	})
}
