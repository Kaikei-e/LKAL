package parser_test

import . "github.com/Kaikei-e/LKAL"

import (
	"strconv"
	"strings"
	"testing"
)

func TestLexicalSuppression(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "L001_HashComment",
			src:      "@lkal \"0.1\"\n# bad comment\na = 1\n",
			wantCode: "L001",
			wantPos:  errPos(2, 1),
			wantMsg:  "comment starts with # or ##",
			wantHint: "start the comment with ###",
		},
		{
			name:     "L002_TrailingComment",
			src:      "@lkal \"0.1\"\na = 1 # bad comment\n",
			wantCode: "L002",
			wantPos:  errPos(2, 7),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L002_ListCommentLineContinuation",
			src:      "@lkal \"0.1\"\na = [\n  ### c\n  1,\n]\n",
			wantCode: "L002",
			wantPos:  errPos(3, 3),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L002_ListInlineCommentContinuation",
			src:      "@lkal \"0.1\"\na = [1, # bad\n 2]\n",
			wantCode: "L002",
			wantPos:  errPos(2, 9),
			wantMsg:  "comment at the end of a line or inside a list",
			wantHint: "move the comment to its own line outside the list",
		},
		{
			name:     "L003_BadEscape",
			src:      "@lkal \"0.1\"\na = \"bad \\x escape\"\n",
			wantCode: "L003",
			wantPos:  errPos(2, 10),
			wantMsg:  "invalid escape sequence",
			wantHint: `use \u{...}`,
		},
		{
			name:     "L004_QuantityBadUnit",
			src:      "@lkal \"0.1\"\na = 1h30m\n",
			wantCode: "L004",
			wantPos:  errPos(2, 5),
			wantMsg:  "digits after a unit",
			wantHint: "use a single unit (for example 90m)",
		},
		{
			name:     "L005_NumberTooLong",
			src:      "@lkal \"0.1\"\na = 99999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999\n",
			wantCode: "L005",
			wantPos:  errPos(2, 5),
			wantMsg:  "number out of range",
			wantHint: "keep integers within int64 and numbers within 100 digits",
		},
		{
			name:     "L006_UnknownDirective",
			src:      "@unknown \"0.1\"\na = 1\n",
			wantCode: "L006",
			wantPos:  errPos(1, 1),
			wantMsg:  "unknown directive",
			wantHint: "v0.1 supports only @lkal",
		},
		{
			name:     "L007_UnclosedString",
			src:      "@lkal \"0.1\"\na = \"unclosed\n",
			wantCode: "L007",
			wantPos:  errPos(2, 5),
			wantMsg:  "unterminated string",
			wantHint: `close the string on the same line; write multi-line strings as \\ lines`,
		},
		{
			name:     "L008_IllegalChar",
			src:      "@lkal \"0.1\"\na = %\n",
			wantCode: "L008",
			wantPos:  errPos(2, 5),
			wantMsg:  "invalid character",
			wantHint: `remove the character; write control characters as \u{...}`,
		},
		{
			name:     "L009_MalformedToken",
			src:      "@lkal \"0.1\"\na = .123\n",
			wantCode: "L009",
			wantPos:  errPos(2, 5),
			wantMsg:  "malformed token",
			wantHint: "follow the syntax for numbers, enums, references and directives",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}
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

func TestErrorLimitAndOrdering(t *testing.T) {
	t.Run("TruncateTo20Errors", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("@lkal \"0.1\"\n")
		for i := 1; i <= 25; i++ {
			b.WriteString("entry")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" = bad\n")
		}

		file, err := Parse("limit.lkal", []byte(b.String()))
		if err == nil {
			t.Fatalf("expected error from Parse, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 20 {
			t.Fatalf("expected 20 errors, got %d", len(errList.Errors))
		}
		if got := errList.Errors[0].Pos.Line; got != 2 {
			t.Errorf("first error line mismatch: got %d, want 2", got)
		}
		if got := errList.Errors[19].Pos.Line; got != 21 {
			t.Errorf("20th error line mismatch: got %d, want 21", got)
		}
	})

	t.Run("OrderingAcrossLines", func(t *testing.T) {
		src := "@lkal \"0.1\"\ne1 = bad1\ne2 = bad2\ne3 = bad3\n"
		file, err := Parse("order_lines.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 3 {
			t.Fatalf("expected 3 errors, got %d", len(errList.Errors))
		}
		for i := 0; i < len(errList.Errors)-1; i++ {
			curr, next := errList.Errors[i], errList.Errors[i+1]
			if curr.Pos.Line > next.Pos.Line {
				t.Errorf("line out of order: [%d]=%d > [%d]=%d", i, curr.Pos.Line, i+1, next.Pos.Line)
			}
		}
	})

	t.Run("OrderingAcrossColumnsOnSameLine", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\"$a\" = 1, \"$b\" = 2\n"
		file, err := Parse("order_cols.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(errList.Errors))
		}
		if errList.Errors[0].Pos.Col >= errList.Errors[1].Pos.Col {
			t.Errorf("columns out of order: first col %d >= second col %d", errList.Errors[0].Pos.Col, errList.Errors[1].Pos.Col)
		}
	})
}

func TestUnicodeColumns(t *testing.T) {
	tests := []errTestCase{
		{
			name:     "CJKKey",
			src:      "@lkal \"0.1\"\n\"日本語キー\" = bad\n",
			wantCode: "L102",
			wantPos:  errPos(2, 11),
			wantMsg:  "bare word in value position",
			wantHint: `write "bad" for a string or .bad for an enum`,
		},
		{
			name:     "EmojiInStringKey",
			src:      "@lkal \"0.1\"\n\"🚀🎉\" = bad\n",
			wantCode: "L102",
			wantPos:  errPos(2, 8),
			wantMsg:  "bare word in value position",
			wantHint: `write "bad" for a string or .bad for an enum`,
		},
		{
			name:     "TabCountedAsOne",
			src:      "@lkal \"0.1\"\nk = \t\tbare\n",
			wantCode: "L102",
			wantPos:  errPos(2, 7),
			wantMsg:  "bare word in value position",
			wantHint: `write "bare" for a string or .bare for an enum`,
		},
		{
			name:              "TabIndentation",
			src:               "@lkal \"0.1\"\n\t\t= 1\n",
			wantCode:          "L106",
			wantPos:           errPos(2, 3),
			wantMsg:           "unexpected token",
			checkNonEmptyHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := Parse(tt.name, []byte(tt.src))
			if err == nil {
				t.Fatalf("expected error, got file: %v", file)
			}
			errCheckSingle(t, err, tt.wantCode, tt.wantPos, tt.wantMsg, tt.wantHint, tt.checkNonEmptyHint)
		})
	}

	t.Run("InvalidUTF8BytesCountedAsOneCol", func(t *testing.T) {
		src := "@lkal \"0.1\"\n\xff\xfe%\n"
		file, err := Parse("invalid_utf8.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected error, got file: %v", file)
		}
		errList := errRequireErrorList(t, err)
		if len(errList.Errors) != 3 {
			t.Fatalf("expected exactly 3 errors, got %d: %v", len(errList.Errors), errList.Error())
		}
		wantPositions := []Pos{errPos(2, 1), errPos(2, 2), errPos(2, 3)}
		for i, wantP := range wantPositions {
			errAssertError(t, errList.Errors[i], "L008", wantP, "invalid character", `remove the character; write control characters as \u{...}`, false)
		}
	})
}
