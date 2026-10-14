package lexer

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/token"
)

type lexCase struct {
	name   string
	src    string
	tokens []token.Token
	errs   []*diag.Error
}

func tok(k token.Kind, l, c int, lit, text string) token.Token {
	return token.Token{Kind: k, Pos: diag.Pos{Line: l, Col: c}, Lit: lit, Text: text}
}

func refTok(l, c int, lit, text string, segs []string) token.Token {
	return token.Token{Kind: token.Ref, Pos: diag.Pos{Line: l, Col: c}, Lit: lit, Text: text, Segs: segs}
}

func eofTok(l, c int) token.Token {
	return token.Token{Kind: token.EOF, Pos: diag.Pos{Line: l, Col: c}}
}

func errExp(code string, l, c int) *diag.Error {
	return diag.NewError("test.lkal", diag.Pos{Line: l, Col: c}, code, "")
}

func equalSegs(a, b []string) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runLexCases(t *testing.T, cases []lexCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, errs := Lex("test.lkal", []byte(tc.src))
			if len(tokens) != len(tc.tokens) {
				t.Fatalf("token count mismatch: got %d, want %d\ngot:  %+v\nwant: %+v", len(tokens), len(tc.tokens), tokens, tc.tokens)
			}
			for i, want := range tc.tokens {
				got := tokens[i]
				if got.Kind != want.Kind || got.Pos != want.Pos || got.Lit != want.Lit || got.Text != want.Text || !equalSegs(got.Segs, want.Segs) {
					t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q segs:%v}, want {kind:%v pos:%v lit:%q text:%q segs:%v}",
						i, got.Kind, got.Pos, got.Lit, got.Text, got.Segs, want.Kind, want.Pos, want.Lit, want.Text, want.Segs)
				}
			}
			if len(errs) != len(tc.errs) {
				t.Fatalf("error count mismatch: got %d, want %d\ngot:  %+v\nwant: %+v", len(errs), len(tc.errs), errs, tc.errs)
			}
			for i, want := range tc.errs {
				got := errs[i]
				if got == nil {
					t.Fatalf("err[%d] is nil", i)
				}
				if got.Code != want.Code || got.Pos != want.Pos || got.Msg != want.Msg || got.Hint != want.Hint || got.File != want.File {
					t.Errorf("error[%d] mismatch: got {code:%q pos:%v msg:%q hint:%q file:%q}, want {code:%q pos:%v msg:%q hint:%q file:%q}",
						i, got.Code, got.Pos, got.Msg, got.Hint, got.File, want.Code, want.Pos, want.Msg, want.Hint, want.File)
				}
			}
		})
	}
}

func TestLexValidTokens(t *testing.T) {
	cases := []lexCase{
		{name: "Empty", src: "", tokens: []token.Token{eofTok(1, 1)}},
		{name: "NewlineLF", src: "\n", tokens: []token.Token{tok(token.Newline, 1, 1, "\n", ""), eofTok(2, 1)}},
		{name: "NewlineCRLF", src: "\r\n", tokens: []token.Token{tok(token.Newline, 1, 1, "\r\n", ""), eofTok(2, 1)}},
		{name: "BlankLines", src: "\n\n", tokens: []token.Token{tok(token.Newline, 1, 1, "\n", ""), tok(token.Newline, 2, 1, "\n", ""), eofTok(3, 1)}},
		{name: "SpaceBlankLine", src: "  \n", tokens: []token.Token{tok(token.Newline, 1, 3, "\n", ""), eofTok(2, 1)}},
		{name: "NoTrailingNewline", src: "a", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), eofTok(1, 2)}},
		{name: "CommentStandard", src: "### hello\n", tokens: []token.Token{tok(token.Comment, 1, 1, "### hello", "hello"), tok(token.Newline, 1, 10, "\n", ""), eofTok(2, 1)}},
		{name: "CommentExtraHashes", src: "####x\n", tokens: []token.Token{tok(token.Comment, 1, 1, "####x", "#x"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "CommentIndentAndWhitespace", src: "  ###  hi \n", tokens: []token.Token{tok(token.Comment, 1, 3, "###  hi ", " hi"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "CommentCRLF", src: "### x\r\n", tokens: []token.Token{tok(token.Comment, 1, 1, "### x", "x"), tok(token.Newline, 1, 6, "\r\n", ""), eofTok(2, 1)}},
		{name: "RawLineBasic", src: "\\\\hello\n", tokens: []token.Token{tok(token.RawLine, 1, 1, "\\\\hello", "hello"), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineVerbatim", src: "  \\\\  hi ### \n", tokens: []token.Token{tok(token.RawLine, 1, 3, "\\\\  hi ### ", "  hi ### "), tok(token.Newline, 1, 14, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineEmpty", src: "\\\\\n", tokens: []token.Token{tok(token.RawLine, 1, 1, "\\\\", ""), tok(token.Newline, 1, 3, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineCRLF", src: "\\\\a \r\n", tokens: []token.Token{tok(token.RawLine, 1, 1, "\\\\a ", "a "), tok(token.Newline, 1, 5, "\r\n", ""), eofTok(2, 1)}},
		{name: "DirectiveLkal", src: "@lkal\n", tokens: []token.Token{tok(token.Directive, 1, 1, "@lkal", "lkal"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "DirectiveIndented", src: "  @lkal\n", tokens: []token.Token{tok(token.Directive, 1, 3, "@lkal", "lkal"), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "NamesAndKeywords", src: "foo _bar-123\n", tokens: []token.Token{tok(token.Name, 1, 1, "foo", "foo"), tok(token.Name, 1, 5, "_bar-123", "_bar-123"), tok(token.Newline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "Bools", src: "true false\n", tokens: []token.Token{tok(token.Bool, 1, 1, "true", "true"), tok(token.Bool, 1, 6, "false", "false"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "StringsSimple", src: "\"\" \"hello\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\"", ""), tok(token.String, 1, 4, "\"hello\"", "hello"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "StringsEscapes", src: "\"\\\"\\\\\\n\\r\\t\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\\"\\\\\\n\\r\\t\"", "\"\\\n\r\t"), tok(token.Newline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "StringsUnicodeScalar", src: "\"\\u{41}\\u{1f600}\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\u{41}\\u{1f600}\"", "A😀"), tok(token.Newline, 1, 18, "\n", ""), eofTok(2, 1)}},
		{name: "StringsValidUFFFD", src: "\"\ufffd\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\ufffd\"", "\ufffd"), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}},
		{name: "Ints", src: "0 -0 42 -100\n", tokens: []token.Token{tok(token.Int, 1, 1, "0", "0"), tok(token.Int, 1, 3, "-0", "-0"), tok(token.Int, 1, 6, "42", "42"), tok(token.Int, 1, 9, "-100", "-100"), tok(token.Newline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "IntsInt64Limits", src: "9223372036854775807 -9223372036854775808\n", tokens: []token.Token{tok(token.Int, 1, 1, "9223372036854775807", "9223372036854775807"), tok(token.Int, 1, 21, "-9223372036854775808", "-9223372036854775808"), tok(token.Newline, 1, 41, "\n", ""), eofTok(2, 1)}},
		{name: "Decimals", src: "0.0 0.25 -1.5 123.4560\n", tokens: []token.Token{tok(token.Decimal, 1, 1, "0.0", "0.0"), tok(token.Decimal, 1, 5, "0.25", "0.25"), tok(token.Decimal, 1, 10, "-1.5", "-1.5"), tok(token.Decimal, 1, 15, "123.4560", "123.4560"), tok(token.Newline, 1, 23, "\n", ""), eofTok(2, 1)}},
		{name: "Quantities", src: "512Mi 250m 30s 1.5Gi -10px\n", tokens: []token.Token{tok(token.Quantity, 1, 1, "512Mi", "512Mi"), tok(token.Quantity, 1, 7, "250m", "250m"), tok(token.Quantity, 1, 12, "30s", "30s"), tok(token.Quantity, 1, 16, "1.5Gi", "1.5Gi"), tok(token.Quantity, 1, 22, "-10px", "-10px"), tok(token.Newline, 1, 27, "\n", ""), eofTok(2, 1)}},
		{name: "Enums", src: ".tcp .active ._foo .bar-baz\n", tokens: []token.Token{tok(token.Enum, 1, 1, ".tcp", "tcp"), tok(token.Enum, 1, 6, ".active", "active"), tok(token.Enum, 1, 14, "._foo", "_foo"), tok(token.Enum, 1, 20, ".bar-baz", "bar-baz"), tok(token.Newline, 1, 28, "\n", ""), eofTok(2, 1)}},
		{name: "RefsSimple", src: "&secret/db-creds &a &a/b/c\n", tokens: []token.Token{refTok(1, 1, "&secret/db-creds", "secret/db-creds", []string{"secret", "db-creds"}), refTok(1, 18, "&a", "a", []string{"a"}), refTok(1, 21, "&a/b/c", "a/b/c", []string{"a", "b", "c"}), tok(token.Newline, 1, 27, "\n", ""), eofTok(2, 1)}},
		{name: "RefsQuotedSegment", src: "&secret/\"db creds\" &\"a\\tb\"/c\n", tokens: []token.Token{refTok(1, 1, "&secret/\"db creds\"", "secret/\"db creds\"", []string{"secret", "db creds"}), refTok(1, 20, "&\"a\\tb\"/c", "\"a\\tb\"/c", []string{"a\tb", "c"}), tok(token.Newline, 1, 29, "\n", ""), eofTok(2, 1)}},
		{name: "Symbols", src: "= { } [ ] ,\n", tokens: []token.Token{tok(token.Assign, 1, 1, "=", ""), tok(token.LBrace, 1, 3, "{", ""), tok(token.RBrace, 1, 5, "}", ""), tok(token.LBrack, 1, 7, "[", ""), tok(token.RBrack, 1, 9, "]", ""), tok(token.Comma, 1, 11, ",", ""), tok(token.Newline, 1, 12, "\n", ""), eofTok(2, 1)}},
		{name: "DocumentFragment", src: "web \"app\" { port = 8080 }\n", tokens: []token.Token{tok(token.Name, 1, 1, "web", "web"), tok(token.String, 1, 5, "\"app\"", "app"), tok(token.LBrace, 1, 11, "{", ""), tok(token.Name, 1, 13, "port", "port"), tok(token.Assign, 1, 18, "=", ""), tok(token.Int, 1, 20, "8080", "8080"), tok(token.RBrace, 1, 25, "}", ""), tok(token.Newline, 1, 26, "\n", ""), eofTok(2, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexBoundariesAndUnicode(t *testing.T) {
	cases := []lexCase{
		{name: "JapaneseKeyAssignInt", src: "\"日本\" = 1\n", tokens: []token.Token{tok(token.String, 1, 1, "\"日本\"", "日本"), tok(token.Assign, 1, 6, "=", ""), tok(token.Int, 1, 8, "1", "1"), tok(token.Newline, 1, 9, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseComment", src: "### 日本語\n", tokens: []token.Token{tok(token.Comment, 1, 1, "### 日本語", "日本語"), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseRawLine", src: "\\\\日本語\n", tokens: []token.Token{tok(token.RawLine, 1, 1, "\\\\日本語", "日本語"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseRef", src: "&\"日本語\"/path\n", tokens: []token.Token{refTok(1, 1, "&\"日本語\"/path", "\"日本語\"/path", []string{"日本語", "path"}), tok(token.Newline, 1, 12, "\n", ""), eofTok(2, 1)}},
		{name: "TabAtLineStart", src: "\ta = 1\n", tokens: []token.Token{tok(token.Name, 1, 2, "a", "a"), tok(token.Assign, 1, 4, "=", ""), tok(token.Int, 1, 6, "1", "1"), tok(token.Newline, 1, 7, "\n", ""), eofTok(2, 1)}},
		{name: "TabBetweenTokens", src: "a\t=\t1\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Assign, 1, 3, "=", ""), tok(token.Int, 1, 5, "1", "1"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "InvalidUTF8ByteOutside", src: "\xff a\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\xff", ""), tok(token.Name, 1, 3, "a", "a"), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1)}},
		{name: "InvalidUTF8ByteInsideString", src: "\"a\xffb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\xffb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "RawTabAllowedInString", src: "\"a\tb\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"a\tb\"", "a\tb"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "RawDelInString", src: "\"a\x7fb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\x7fb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "RawCrInString", src: "\"a\rb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\rb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "UnicodeScalarMin", src: "\"\\u{0}\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\u{0}\"", "\x00"), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarMax", src: "\"\\u{10ffff}\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\u{10ffff}\"", "\U0010ffff"), tok(token.Newline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarJustBelowSurrogate", src: "\"\\u{D7FF}\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\u{D7FF}\"", "\uD7FF"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarJustAboveSurrogate", src: "\"\\u{E000}\"\n", tokens: []token.Token{tok(token.String, 1, 1, "\"\\u{E000}\"", "\uE000"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "InvalidEscapeSingleQuote", src: "\"\\'\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\'\"", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "InvalidEscapeQuestion", src: "\"\\?\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\?\"", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "InvalidEscapeFormFeed", src: "\"\\f\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\f\"", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "BOMFollowedByNewline", src: "\xef\xbb\xbf\n", tokens: []token.Token{tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1)}},
		{name: "BOMFollowedBySpaces", src: "\xef\xbb\xbf  a\n", tokens: []token.Token{tok(token.Name, 1, 4, "a", "a"), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1)}},
		{name: "BareCRMultiple", src: "a\r\rb\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Illegal, 1, 2, "\r", ""), tok(token.Illegal, 1, 3, "\r", ""), tok(token.Name, 1, 4, "b", "b"), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 2), errExp("L008", 1, 3)}},
		{name: "DelimitedTokensNoSpaces", src: "[1,2]\nk=1\n", tokens: []token.Token{tok(token.LBrack, 1, 1, "[", ""), tok(token.Int, 1, 2, "1", "1"), tok(token.Comma, 1, 3, ",", ""), tok(token.Int, 1, 4, "2", "2"), tok(token.RBrack, 1, 5, "]", ""), tok(token.Newline, 1, 6, "\n", ""), tok(token.Name, 2, 1, "k", "k"), tok(token.Assign, 2, 2, "=", ""), tok(token.Int, 2, 3, "1", "1"), tok(token.Newline, 2, 4, "\n", ""), eofTok(3, 1)}},
		{name: "DelimitedIntQuote", src: "[1\"str\"]\n", tokens: []token.Token{tok(token.LBrack, 1, 1, "[", ""), tok(token.Int, 1, 2, "1", "1"), tok(token.String, 1, 3, "\"str\"", "str"), tok(token.RBrack, 1, 8, "]", ""), tok(token.Newline, 1, 9, "\n", ""), eofTok(2, 1)}},
		{name: "DelimitedBraces", src: "{1}\n", tokens: []token.Token{tok(token.LBrace, 1, 1, "{", ""), tok(token.Int, 1, 2, "1", "1"), tok(token.RBrace, 1, 3, "}", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexNumbersAndDelimiters(t *testing.T) {
	cases := []lexCase{
		{
			name: "Exact100DigitsInt",
			src:  "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\n",
			tokens: []token.Token{
				tok(token.Illegal, 1, 1, "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", ""),
				tok(token.Newline, 1, 101, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L005", 1, 1)},
		},
		{
			name: "Exact100DigitsDecimal",
			src:  "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999\n",
			tokens: []token.Token{
				tok(token.Decimal, 1, 1, "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999", "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999"),
				tok(token.Newline, 1, 103, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "Exact100DigitsQuantity",
			src:  "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms\n",
			tokens: []token.Token{
				tok(token.Quantity, 1, 1, "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms", "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms"),
				tok(token.Newline, 1, 103, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "NegativeZeroDecimal",
			src:  "-0.0\n",
			tokens: []token.Token{
				tok(token.Decimal, 1, 1, "-0.0", "-0.0"),
				tok(token.Newline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "NegativeZeroQuantity",
			src:  "-0ms\n",
			tokens: []token.Token{
				tok(token.Quantity, 1, 1, "-0ms", "-0ms"),
				tok(token.Newline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "DelimitersInComplexAssignment",
			src:  "a={b=1,c=2.5}\n",
			tokens: []token.Token{
				tok(token.Name, 1, 1, "a", "a"),
				tok(token.Assign, 1, 2, "=", ""),
				tok(token.LBrace, 1, 3, "{", ""),
				tok(token.Name, 1, 4, "b", "b"),
				tok(token.Assign, 1, 5, "=", ""),
				tok(token.Int, 1, 6, "1", "1"),
				tok(token.Comma, 1, 7, ",", ""),
				tok(token.Name, 1, 8, "c", "c"),
				tok(token.Assign, 1, 9, "=", ""),
				tok(token.Decimal, 1, 10, "2.5", "2.5"),
				tok(token.RBrace, 1, 13, "}", ""),
				tok(token.Newline, 1, 14, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "MalformedNumberDelimiterRecovery",
			src:  "[01,1.]\n",
			tokens: []token.Token{
				tok(token.LBrack, 1, 1, "[", ""),
				tok(token.Illegal, 1, 2, "01", ""),
				tok(token.Comma, 1, 4, ",", ""),
				tok(token.Illegal, 1, 5, "1.", ""),
				tok(token.RBrack, 1, 7, "]", ""),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L009", 1, 2), errExp("L009", 1, 5)},
		},
		{
			name: "NumberDirectlyFollowedByComment",
			src:  "1#x\n",
			tokens: []token.Token{
				tok(token.Int, 1, 1, "1", "1"),
				tok(token.Newline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L002", 1, 2)},
		},
	}
	runLexCases(t, cases)
}
