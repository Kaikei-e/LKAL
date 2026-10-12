package lkal

import "testing"

type lexCase struct {
	name   string
	src    string
	tokens []token
	errs   []*Error
}

func tok(k tokKind, l, c int, lit, text string) token {
	return token{kind: k, pos: Pos{Line: l, Col: c}, lit: lit, text: text}
}

func refTok(l, c int, lit, text string, segs []string) token {
	return token{kind: tokRef, pos: Pos{Line: l, Col: c}, lit: lit, text: text, segs: segs}
}

func eofTok(l, c int) token {
	return token{kind: tokEOF, pos: Pos{Line: l, Col: c}}
}

type lexTestExpectedErr struct {
	msg  string
	hint string
}

var lexTestExpectedErrTable = map[string]lexTestExpectedErr{
	"L001": {
		msg:  "# か ## で始まるコメントは書けない",
		hint: "### で始める",
	},
	"L002": {
		msg:  "行末やリストの内側にはコメントを書けない",
		hint: "リストの外の、独立した行に移す",
	},
	"L003": {
		msg:  "使えないエスケープ",
		hint: `\u{…} を使う`,
	},
	"L004": {
		msg:  "単位の後ろに数字が続いている",
		hint: "単位を1つにする（例: 90m）",
	},
	"L005": {
		msg:  "数値が範囲を超えている",
		hint: "整数は int64 の範囲、数値は100桁以内に収める",
	},
	"L006": {
		msg:  "使えないディレクティブ",
		hint: "v0.1で使えるのは @lkal だけ",
	},
	"L007": {
		msg:  "閉じていない文字列",
		hint: `同じ行で " を閉じる。複数行の文字列は \\ 行で書く`,
	},
	"L008": {
		msg:  "使えない文字",
		hint: `文字を消す。制御文字は \u{…} で書く`,
	},
	"L009": {
		msg:  "トークンの形が正しくない",
		hint: "数値・列挙子・参照・ディレクティブの書き方に合わせる",
	},
}

func errExp(code string, l, c int) *Error {
	entry, ok := lexTestExpectedErrTable[code]
	if !ok {
		panic("unknown lex error code: " + code)
	}
	return &Error{
		File: "test.lkal",
		Pos:  Pos{Line: l, Col: c},
		Code: code,
		Msg:  entry.msg,
		Hint: entry.hint,
	}
}

func equalSegs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b != nil || a != nil && b == nil {
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
			tokens, errs := lex("test.lkal", []byte(tc.src))
			if len(tokens) != len(tc.tokens) {
				t.Fatalf("token count mismatch: got %d, want %d\ngot:  %+v\nwant: %+v", len(tokens), len(tc.tokens), tokens, tc.tokens)
			}
			for i, want := range tc.tokens {
				got := tokens[i]
				if got.kind != want.kind || got.pos != want.pos || got.lit != want.lit || got.text != want.text || !equalSegs(got.segs, want.segs) {
					t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q segs:%v}, want {kind:%v pos:%v lit:%q text:%q segs:%v}",
						i, got.kind, got.pos, got.lit, got.text, got.segs, want.kind, want.pos, want.lit, want.text, want.segs)
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
		{name: "Empty", src: "", tokens: []token{eofTok(1, 1)}},
		{name: "NewlineLF", src: "\n", tokens: []token{tok(tokNewline, 1, 1, "\n", ""), eofTok(2, 1)}},
		{name: "NewlineCRLF", src: "\r\n", tokens: []token{tok(tokNewline, 1, 1, "\r\n", ""), eofTok(2, 1)}},
		{name: "BlankLines", src: "\n\n", tokens: []token{tok(tokNewline, 1, 1, "\n", ""), tok(tokNewline, 2, 1, "\n", ""), eofTok(3, 1)}},
		{name: "SpaceBlankLine", src: "  \n", tokens: []token{tok(tokNewline, 1, 3, "\n", ""), eofTok(2, 1)}},
		{name: "NoTrailingNewline", src: "a", tokens: []token{tok(tokName, 1, 1, "a", "a"), eofTok(1, 2)}},
		{name: "CommentStandard", src: "### hello\n", tokens: []token{tok(tokComment, 1, 1, "### hello", "hello"), tok(tokNewline, 1, 10, "\n", ""), eofTok(2, 1)}},
		{name: "CommentExtraHashes", src: "####x\n", tokens: []token{tok(tokComment, 1, 1, "####x", "#x"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "CommentIndentAndWhitespace", src: "  ###  hi \n", tokens: []token{tok(tokComment, 1, 3, "###  hi ", " hi"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "CommentCRLF", src: "### x\r\n", tokens: []token{tok(tokComment, 1, 1, "### x", "x"), tok(tokNewline, 1, 6, "\r\n", ""), eofTok(2, 1)}},
		{name: "RawLineBasic", src: "\\\\hello\n", tokens: []token{tok(tokRawLine, 1, 1, "\\\\hello", "hello"), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineVerbatim", src: "  \\\\  hi ### \n", tokens: []token{tok(tokRawLine, 1, 3, "\\\\  hi ### ", "  hi ### "), tok(tokNewline, 1, 14, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineEmpty", src: "\\\\\n", tokens: []token{tok(tokRawLine, 1, 1, "\\\\", ""), tok(tokNewline, 1, 3, "\n", ""), eofTok(2, 1)}},
		{name: "RawLineCRLF", src: "\\\\a \r\n", tokens: []token{tok(tokRawLine, 1, 1, "\\\\a ", "a "), tok(tokNewline, 1, 5, "\r\n", ""), eofTok(2, 1)}},
		{name: "DirectiveLkal", src: "@lkal\n", tokens: []token{tok(tokDirective, 1, 1, "@lkal", "lkal"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "DirectiveIndented", src: "  @lkal\n", tokens: []token{tok(tokDirective, 1, 3, "@lkal", "lkal"), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "NamesAndKeywords", src: "foo _bar-123\n", tokens: []token{tok(tokName, 1, 1, "foo", "foo"), tok(tokName, 1, 5, "_bar-123", "_bar-123"), tok(tokNewline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "Bools", src: "true false\n", tokens: []token{tok(tokBool, 1, 1, "true", "true"), tok(tokBool, 1, 6, "false", "false"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "StringsSimple", src: "\"\" \"hello\"\n", tokens: []token{tok(tokString, 1, 1, "\"\"", ""), tok(tokString, 1, 4, "\"hello\"", "hello"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "StringsEscapes", src: "\"\\\"\\\\\\n\\r\\t\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\\"\\\\\\n\\r\\t\"", "\"\\\n\r\t"), tok(tokNewline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "StringsUnicodeScalar", src: "\"\\u{41}\\u{1f600}\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\u{41}\\u{1f600}\"", "A😀"), tok(tokNewline, 1, 18, "\n", ""), eofTok(2, 1)}},
		{name: "StringsValidUFFFD", src: "\"\ufffd\"\n", tokens: []token{tok(tokString, 1, 1, "\"\ufffd\"", "\ufffd"), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}},
		{name: "Ints", src: "0 -0 42 -100\n", tokens: []token{tok(tokInt, 1, 1, "0", "0"), tok(tokInt, 1, 3, "-0", "-0"), tok(tokInt, 1, 6, "42", "42"), tok(tokInt, 1, 9, "-100", "-100"), tok(tokNewline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "IntsInt64Limits", src: "9223372036854775807 -9223372036854775808\n", tokens: []token{tok(tokInt, 1, 1, "9223372036854775807", "9223372036854775807"), tok(tokInt, 1, 21, "-9223372036854775808", "-9223372036854775808"), tok(tokNewline, 1, 41, "\n", ""), eofTok(2, 1)}},
		{name: "Decimals", src: "0.0 0.25 -1.5 123.4560\n", tokens: []token{tok(tokDecimal, 1, 1, "0.0", "0.0"), tok(tokDecimal, 1, 5, "0.25", "0.25"), tok(tokDecimal, 1, 10, "-1.5", "-1.5"), tok(tokDecimal, 1, 15, "123.4560", "123.4560"), tok(tokNewline, 1, 23, "\n", ""), eofTok(2, 1)}},
		{name: "Quantities", src: "512Mi 250m 30s 1.5Gi -10px\n", tokens: []token{tok(tokQuantity, 1, 1, "512Mi", "512Mi"), tok(tokQuantity, 1, 7, "250m", "250m"), tok(tokQuantity, 1, 12, "30s", "30s"), tok(tokQuantity, 1, 16, "1.5Gi", "1.5Gi"), tok(tokQuantity, 1, 22, "-10px", "-10px"), tok(tokNewline, 1, 27, "\n", ""), eofTok(2, 1)}},
		{name: "Enums", src: ".tcp .active ._foo .bar-baz\n", tokens: []token{tok(tokEnum, 1, 1, ".tcp", "tcp"), tok(tokEnum, 1, 6, ".active", "active"), tok(tokEnum, 1, 14, "._foo", "_foo"), tok(tokEnum, 1, 20, ".bar-baz", "bar-baz"), tok(tokNewline, 1, 28, "\n", ""), eofTok(2, 1)}},
		{name: "RefsSimple", src: "&secret/db-creds &a &a/b/c\n", tokens: []token{refTok(1, 1, "&secret/db-creds", "secret/db-creds", []string{"secret", "db-creds"}), refTok(1, 18, "&a", "a", []string{"a"}), refTok(1, 21, "&a/b/c", "a/b/c", []string{"a", "b", "c"}), tok(tokNewline, 1, 27, "\n", ""), eofTok(2, 1)}},
		{name: "RefsQuotedSegment", src: "&secret/\"db creds\" &\"a\\tb\"/c\n", tokens: []token{refTok(1, 1, "&secret/\"db creds\"", "secret/\"db creds\"", []string{"secret", "db creds"}), refTok(1, 20, "&\"a\\tb\"/c", "\"a\\tb\"/c", []string{"a\tb", "c"}), tok(tokNewline, 1, 29, "\n", ""), eofTok(2, 1)}},
		{name: "Symbols", src: "= { } [ ] ,\n", tokens: []token{tok(tokAssign, 1, 1, "=", ""), tok(tokLBrace, 1, 3, "{", ""), tok(tokRBrace, 1, 5, "}", ""), tok(tokLBrack, 1, 7, "[", ""), tok(tokRBrack, 1, 9, "]", ""), tok(tokComma, 1, 11, ",", ""), tok(tokNewline, 1, 12, "\n", ""), eofTok(2, 1)}},
		{name: "DocumentFragment", src: "web \"app\" { port = 8080 }\n", tokens: []token{tok(tokName, 1, 1, "web", "web"), tok(tokString, 1, 5, "\"app\"", "app"), tok(tokLBrace, 1, 11, "{", ""), tok(tokName, 1, 13, "port", "port"), tok(tokAssign, 1, 18, "=", ""), tok(tokInt, 1, 20, "8080", "8080"), tok(tokRBrace, 1, 25, "}", ""), tok(tokNewline, 1, 26, "\n", ""), eofTok(2, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexErrors(t *testing.T) {
	cases := []lexCase{
		{name: "L001_SingleHash", src: "# comment\n", tokens: []token{tok(tokComment, 1, 1, "# comment", "comment"), tok(tokNewline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L001", 1, 1)}},
		{name: "L001_DoubleHash", src: "## comment\n", tokens: []token{tok(tokComment, 1, 1, "## comment", "comment"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L001", 1, 1)}},
		{name: "L001_IndentedHash", src: "  # indent\n", tokens: []token{tok(tokComment, 1, 3, "# indent", "indent"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L001", 1, 3)}},
		{name: "L001_TabIndentedHash", src: "\t# tab\n", tokens: []token{tok(tokComment, 1, 2, "# tab", "tab"), tok(tokNewline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L001", 1, 2)}},
		{name: "L002_InlineAfterToken", src: "a = 1 # bad\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokAssign, 1, 3, "=", ""), tok(tokInt, 1, 5, "1", "1"), tok(tokNewline, 1, 12, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L002", 1, 7)}},
		{name: "L002_InlineAtEOF", src: "a = 1 # bad", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokAssign, 1, 3, "=", ""), tok(tokInt, 1, 5, "1", "1"), eofTok(1, 12)}, errs: []*Error{errExp("L002", 1, 7)}},
		{name: "L002_InsideList", src: "[1, # inside\n2]\n", tokens: []token{tok(tokLBrack, 1, 1, "[", ""), tok(tokInt, 1, 2, "1", "1"), tok(tokComma, 1, 3, ",", ""), tok(tokNewline, 1, 13, "\n", ""), tok(tokInt, 2, 1, "2", "2"), tok(tokRBrack, 2, 2, "]", ""), tok(tokNewline, 2, 3, "\n", ""), eofTok(3, 1)}, errs: []*Error{errExp("L002", 1, 5)}},
		{name: "L002_DirectlyAfterSymbol", src: "={#\n}\n", tokens: []token{tok(tokAssign, 1, 1, "=", ""), tok(tokLBrace, 1, 2, "{", ""), tok(tokNewline, 1, 4, "\n", ""), tok(tokRBrace, 2, 1, "}", ""), tok(tokNewline, 2, 2, "\n", ""), eofTok(3, 1)}, errs: []*Error{errExp("L002", 1, 3)}},
		{name: "L003_InvalidEscapeX", src: "\"bad \\x escape\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"bad \\x escape\"", ""), tok(tokNewline, 1, 16, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 6)}},
		{name: "L003_InvalidEscapeA", src: "\"\\a\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\a\"", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeEmpty", src: "\"\\u{}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{}\"", ""), tok(tokNewline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeOutOfRange", src: "\"\\u{110000}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{110000}\"", ""), tok(tokNewline, 1, 13, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeSurrogateHigh", src: "\"\\u{D800}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{D800}\"", ""), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeSurrogateLow", src: "\"\\u{DFFF}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{DFFF}\"", ""), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeTooLong", src: "\"\\u{1234567}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{1234567}\"", ""), tok(tokNewline, 1, 14, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeNonHex", src: "\"\\u{xyz}\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\u{xyz}\"", ""), tok(tokNewline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L003_RefQuotedEscape", src: "&secret/\"bad \\x\"\n", tokens: []token{tok(tokIllegal, 1, 1, "&secret/\"bad \\x\"", ""), tok(tokNewline, 1, 17, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 14)}},
		{name: "L003_FirstStringErrorOnly", src: "\"\\x\\y\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\x\\y\"", ""), tok(tokNewline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "L004_QuantityDigitsAfterUnit", src: "1h30m\n", tokens: []token{tok(tokIllegal, 1, 1, "1h30m", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityScientific", src: "1e5\n", tokens: []token{tok(tokIllegal, 1, 1, "1e5", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityHex", src: "0x1f\n", tokens: []token{tok(tokIllegal, 1, 1, "0x1f", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityDecimalWithDigits", src: "1.5m2\n", tokens: []token{tok(tokIllegal, 1, 1, "1.5m2", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L004", 1, 1)}},
		{name: "L005_NumberOver100Digits", src: "11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111\n", tokens: []token{tok(tokIllegal, 1, 1, "11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", ""), tok(tokNewline, 1, 102, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L005", 1, 1)}},
		{name: "L005_DecimalOver100Digits", src: "0.11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111\n", tokens: []token{tok(tokIllegal, 1, 1, "0.11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", ""), tok(tokNewline, 1, 104, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L005", 1, 1)}},
		{name: "L005_Int64PositiveOverflow", src: "9223372036854775808\n", tokens: []token{tok(tokIllegal, 1, 1, "9223372036854775808", ""), tok(tokNewline, 1, 20, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L005", 1, 1)}},
		{name: "L005_Int64NegativeOverflow", src: "-9223372036854775809\n", tokens: []token{tok(tokIllegal, 1, 1, "-9223372036854775809", ""), tok(tokNewline, 1, 21, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L005", 1, 1)}},
		{name: "L006_UnknownDirectiveUse", src: "@use\n", tokens: []token{tok(tokIllegal, 1, 1, "@use", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L006", 1, 1)}},
		{name: "L006_UnknownDirectiveConfig", src: "@config\n", tokens: []token{tok(tokIllegal, 1, 1, "@config", ""), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L006", 1, 1)}},
		{name: "L007_UnclosedStringEOL", src: "\"unclosed\n", tokens: []token{tok(tokIllegal, 1, 1, "\"unclosed", ""), tok(tokNewline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedStringEOF", src: "\"unclosed", tokens: []token{tok(tokIllegal, 1, 1, "\"unclosed", ""), eofTok(1, 10)}, errs: []*Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedEmpty", src: "\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedSuppressesL003", src: "\"bad \\x escape unclosed\n", tokens: []token{tok(tokIllegal, 1, 1, "\"bad \\x escape unclosed", ""), tok(tokNewline, 1, 24, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L007", 1, 1)}},
		{name: "L008_BareCR", src: "a\rb\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokIllegal, 1, 2, "\r", ""), tok(tokName, 1, 3, "b", "b"), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 2)}},
		{name: "L008_BareCRAtEOF", src: "a\r", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokIllegal, 1, 2, "\r", ""), eofTok(1, 3)}, errs: []*Error{errExp("L008", 1, 2)}},
		{name: "L008_BOM", src: "\xef\xbb\xbf@lkal\n", tokens: []token{tok(tokDirective, 1, 2, "@lkal", "lkal"), tok(tokNewline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1)}},
		{name: "L008_RawNullInString", src: "\"a\x00b\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\x00b\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "L008_RawControlInString", src: "\"a\x1fb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\x1fb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "L008_DelInString", src: "\"a\x7fb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\x7fb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "L008_InvalidUTF8InString", src: "\"a\xffb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\xffb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "L008_InvalidUTF8OutsideString", src: "\xff\n", tokens: []token{tok(tokIllegal, 1, 1, "\xff", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1)}},
		{name: "L008_InvalidUTF8InComment", src: "### a\xffb\n", tokens: []token{tok(tokComment, 1, 1, "### a\xffb", "a\xffb"), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 6)}},
		{name: "L008_InvalidUTF8InRawLine", src: "\\\\a\xffb\n", tokens: []token{tok(tokRawLine, 1, 1, "\\\\a\xffb", "a\xffb"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 4)}},
		{name: "L008_DisallowedChars", src: "% ; *\n", tokens: []token{tok(tokIllegal, 1, 1, "%", ""), tok(tokIllegal, 1, 3, ";", ""), tok(tokIllegal, 1, 5, "*", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1), errExp("L008", 1, 3), errExp("L008", 1, 5)}},
		{name: "L008_MidLineAt", src: "a = @\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokAssign, 1, 3, "=", ""), tok(tokIllegal, 1, 5, "@", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 5)}},
		{name: "L008_BackslashMidLine", src: "a \\ b\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokIllegal, 1, 3, "\\", ""), tok(tokName, 1, 5, "b", "b"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "L009_LeadingZeroInt", src: "01\n", tokens: []token{tok(tokIllegal, 1, 1, "01", ""), tok(tokNewline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_LeadingZeroNegative", src: "-01\n", tokens: []token{tok(tokIllegal, 1, 1, "-01", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_TrailingDotDecimal", src: "1.\n", tokens: []token{tok(tokIllegal, 1, 1, "1.", ""), tok(tokNewline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_BareMinus", src: "-\n", tokens: []token{tok(tokIllegal, 1, 1, "-", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_MinusNonDigit", src: "-a\n", tokens: []token{tok(tokIllegal, 1, 1, "-a", ""), tok(tokNewline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_UnderscoreInNumber", src: "1_000\n", tokens: []token{tok(tokIllegal, 1, 1, "1_000", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_MultipleDotsInNumber", src: "1.2.3\n", tokens: []token{tok(tokIllegal, 1, 1, "1.2.3", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_UnitTrailingDot", src: "5m.\n", tokens: []token{tok(tokIllegal, 1, 1, "5m.", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_DirectiveNotNameStart", src: "@123\n", tokens: []token{tok(tokIllegal, 1, 1, "@123", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_DirectiveBareAt", src: "@\n", tokens: []token{tok(tokIllegal, 1, 1, "@", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_EnumBareDot", src: ".\n", tokens: []token{tok(tokIllegal, 1, 1, ".", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_EnumNonNameStart", src: ".-x\n", tokens: []token{tok(tokIllegal, 1, 1, ".-x", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_RefBareAmpersand", src: "&\n", tokens: []token{tok(tokIllegal, 1, 1, "&", ""), tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_RefLeadingSlash", src: "&/a\n", tokens: []token{tok(tokIllegal, 1, 1, "&/a", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_RefTrailingSlash", src: "&a/\n", tokens: []token{tok(tokIllegal, 1, 1, "&a/", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
		{name: "L009_RefDoubleSlash", src: "&a//b\n", tokens: []token{tok(tokIllegal, 1, 1, "&a//b", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L009", 1, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexBoundariesAndUnicode(t *testing.T) {
	cases := []lexCase{
		{name: "JapaneseKeyAssignInt", src: "\"日本\" = 1\n", tokens: []token{tok(tokString, 1, 1, "\"日本\"", "日本"), tok(tokAssign, 1, 6, "=", ""), tok(tokInt, 1, 8, "1", "1"), tok(tokNewline, 1, 9, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseComment", src: "### 日本語\n", tokens: []token{tok(tokComment, 1, 1, "### 日本語", "日本語"), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseRawLine", src: "\\\\日本語\n", tokens: []token{tok(tokRawLine, 1, 1, "\\\\日本語", "日本語"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "JapaneseRef", src: "&\"日本語\"/path\n", tokens: []token{refTok(1, 1, "&\"日本語\"/path", "\"日本語\"/path", []string{"日本語", "path"}), tok(tokNewline, 1, 12, "\n", ""), eofTok(2, 1)}},
		{name: "TabAtLineStart", src: "\ta = 1\n", tokens: []token{tok(tokName, 1, 2, "a", "a"), tok(tokAssign, 1, 4, "=", ""), tok(tokInt, 1, 6, "1", "1"), tok(tokNewline, 1, 7, "\n", ""), eofTok(2, 1)}},
		{name: "TabBetweenTokens", src: "a\t=\t1\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokAssign, 1, 3, "=", ""), tok(tokInt, 1, 5, "1", "1"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "InvalidUTF8ByteOutside", src: "\xff a\n", tokens: []token{tok(tokIllegal, 1, 1, "\xff", ""), tok(tokName, 1, 3, "a", "a"), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1)}},
		{name: "InvalidUTF8ByteInsideString", src: "\"a\xffb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\xffb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "RawTabAllowedInString", src: "\"a\tb\"\n", tokens: []token{tok(tokString, 1, 1, "\"a\tb\"", "a\tb"), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}},
		{name: "RawDelInString", src: "\"a\x7fb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\x7fb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "RawCrInString", src: "\"a\rb\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"a\rb\"", ""), tok(tokNewline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 3)}},
		{name: "UnicodeScalarMin", src: "\"\\u{0}\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\u{0}\"", "\x00"), tok(tokNewline, 1, 8, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarMax", src: "\"\\u{10ffff}\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\u{10ffff}\"", "\U0010ffff"), tok(tokNewline, 1, 13, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarJustBelowSurrogate", src: "\"\\u{D7FF}\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\u{D7FF}\"", "\uD7FF"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "UnicodeScalarJustAboveSurrogate", src: "\"\\u{E000}\"\n", tokens: []token{tok(tokString, 1, 1, "\"\\u{E000}\"", "\uE000"), tok(tokNewline, 1, 11, "\n", ""), eofTok(2, 1)}},
		{name: "InvalidEscapeSingleQuote", src: "\"\\'\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\'\"", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "InvalidEscapeQuestion", src: "\"\\?\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\?\"", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "InvalidEscapeFormFeed", src: "\"\\f\"\n", tokens: []token{tok(tokIllegal, 1, 1, "\"\\f\"", ""), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L003", 1, 2)}},
		{name: "BOMFollowedByNewline", src: "\xef\xbb\xbf\n", tokens: []token{tok(tokNewline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1)}},
		{name: "BOMFollowedBySpaces", src: "\xef\xbb\xbf  a\n", tokens: []token{tok(tokName, 1, 4, "a", "a"), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 1)}},
		{name: "BareCRMultiple", src: "a\r\rb\n", tokens: []token{tok(tokName, 1, 1, "a", "a"), tok(tokIllegal, 1, 2, "\r", ""), tok(tokIllegal, 1, 3, "\r", ""), tok(tokName, 1, 4, "b", "b"), tok(tokNewline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*Error{errExp("L008", 1, 2), errExp("L008", 1, 3)}},
		{name: "DelimitedTokensNoSpaces", src: "[1,2]\nk=1\n", tokens: []token{tok(tokLBrack, 1, 1, "[", ""), tok(tokInt, 1, 2, "1", "1"), tok(tokComma, 1, 3, ",", ""), tok(tokInt, 1, 4, "2", "2"), tok(tokRBrack, 1, 5, "]", ""), tok(tokNewline, 1, 6, "\n", ""), tok(tokName, 2, 1, "k", "k"), tok(tokAssign, 2, 2, "=", ""), tok(tokInt, 2, 3, "1", "1"), tok(tokNewline, 2, 4, "\n", ""), eofTok(3, 1)}},
		{name: "DelimitedIntQuote", src: "[1\"str\"]\n", tokens: []token{tok(tokLBrack, 1, 1, "[", ""), tok(tokInt, 1, 2, "1", "1"), tok(tokString, 1, 3, "\"str\"", "str"), tok(tokRBrack, 1, 8, "]", ""), tok(tokNewline, 1, 9, "\n", ""), eofTok(2, 1)}},
		{name: "DelimitedBraces", src: "{1}\n", tokens: []token{tok(tokLBrace, 1, 1, "{", ""), tok(tokInt, 1, 2, "1", "1"), tok(tokRBrace, 1, 3, "}", ""), tok(tokNewline, 1, 4, "\n", ""), eofTok(2, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexNumbersAndDelimiters(t *testing.T) {
	cases := []lexCase{
		{
			name: "Exact100DigitsInt",
			src:  "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\n",
			tokens: []token{
				tok(tokIllegal, 1, 1, "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000", ""),
				tok(tokNewline, 1, 101, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L005", 1, 1)},
		},
		{
			name: "Exact100DigitsDecimal",
			src:  "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999\n",
			tokens: []token{
				tok(tokDecimal, 1, 1, "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999", "-0.999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999"),
				tok(tokNewline, 1, 103, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "Exact100DigitsQuantity",
			src:  "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms\n",
			tokens: []token{
				tok(tokQuantity, 1, 1, "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms", "1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ms"),
				tok(tokNewline, 1, 103, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "NegativeZeroDecimal",
			src:  "-0.0\n",
			tokens: []token{
				tok(tokDecimal, 1, 1, "-0.0", "-0.0"),
				tok(tokNewline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "NegativeZeroQuantity",
			src:  "-0ms\n",
			tokens: []token{
				tok(tokQuantity, 1, 1, "-0ms", "-0ms"),
				tok(tokNewline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "DelimitersInComplexAssignment",
			src:  "a={b=1,c=2.5}\n",
			tokens: []token{
				tok(tokName, 1, 1, "a", "a"),
				tok(tokAssign, 1, 2, "=", ""),
				tok(tokLBrace, 1, 3, "{", ""),
				tok(tokName, 1, 4, "b", "b"),
				tok(tokAssign, 1, 5, "=", ""),
				tok(tokInt, 1, 6, "1", "1"),
				tok(tokComma, 1, 7, ",", ""),
				tok(tokName, 1, 8, "c", "c"),
				tok(tokAssign, 1, 9, "=", ""),
				tok(tokDecimal, 1, 10, "2.5", "2.5"),
				tok(tokRBrace, 1, 13, "}", ""),
				tok(tokNewline, 1, 14, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "MalformedNumberDelimiterRecovery",
			src:  "[01,1.]\n",
			tokens: []token{
				tok(tokLBrack, 1, 1, "[", ""),
				tok(tokIllegal, 1, 2, "01", ""),
				tok(tokComma, 1, 4, ",", ""),
				tok(tokIllegal, 1, 5, "1.", ""),
				tok(tokRBrack, 1, 7, "]", ""),
				tok(tokNewline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L009", 1, 2), errExp("L009", 1, 5)},
		},
		{
			name: "NumberDirectlyFollowedByComment",
			src:  "1#x\n",
			tokens: []token{
				tok(tokInt, 1, 1, "1", "1"),
				tok(tokNewline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L002", 1, 2)},
		},
	}
	runLexCases(t, cases)
}

func TestLexMultiLineAndRecovery(t *testing.T) {
	t.Run("MultiLineDocumentWithRawAndComments", func(t *testing.T) {
		src := "@lkal \"0.1\"\n### comment\nservice \"web\" {\n  startup =\n    \\\\line1\n    \\\\\n    \\\\line2\n}\n"
		tokens, errs := lex("test.lkal", []byte(src))
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
		}
		wantTokens := []token{
			tok(tokDirective, 1, 1, "@lkal", "lkal"),
			tok(tokString, 1, 7, "\"0.1\"", "0.1"),
			tok(tokNewline, 1, 12, "\n", ""),
			tok(tokComment, 2, 1, "### comment", "comment"),
			tok(tokNewline, 2, 12, "\n", ""),
			tok(tokName, 3, 1, "service", "service"),
			tok(tokString, 3, 9, "\"web\"", "web"),
			tok(tokLBrace, 3, 15, "{", ""),
			tok(tokNewline, 3, 16, "\n", ""),
			tok(tokName, 4, 3, "startup", "startup"),
			tok(tokAssign, 4, 11, "=", ""),
			tok(tokNewline, 4, 12, "\n", ""),
			tok(tokRawLine, 5, 5, "\\\\line1", "line1"),
			tok(tokNewline, 5, 12, "\n", ""),
			tok(tokRawLine, 6, 5, "\\\\", ""),
			tok(tokNewline, 6, 7, "\n", ""),
			tok(tokRawLine, 7, 5, "\\\\line2", "line2"),
			tok(tokNewline, 7, 12, "\n", ""),
			tok(tokRBrace, 8, 1, "}", ""),
			tok(tokNewline, 8, 2, "\n", ""),
			eofTok(9, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.kind != want.kind || got.pos != want.pos || got.lit != want.lit || got.text != want.text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.kind, got.pos, got.lit, got.text, want.kind, want.pos, want.lit, want.text)
			}
		}
	})

	t.Run("MultiLineIndependentLineErrors", func(t *testing.T) {
		src := "# bad1\na = 1\n\"unclosed\nb = 2\n"
		tokens, errs := lex("test.lkal", []byte(src))
		if len(errs) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(errs))
		}
		if errs[0].Code != "L001" || errs[0].Pos != (Pos{Line: 1, Col: 1}) {
			t.Errorf("error 0 mismatch: %v", errs[0])
		}
		if errs[1].Code != "L007" || errs[1].Pos != (Pos{Line: 3, Col: 1}) {
			t.Errorf("error 1 mismatch: %v", errs[1])
		}
		wantTokens := []token{
			tok(tokComment, 1, 1, "# bad1", "bad1"),
			tok(tokNewline, 1, 7, "\n", ""),
			tok(tokName, 2, 1, "a", "a"),
			tok(tokAssign, 2, 3, "=", ""),
			tok(tokInt, 2, 5, "1", "1"),
			tok(tokNewline, 2, 6, "\n", ""),
			tok(tokIllegal, 3, 1, "\"unclosed", ""),
			tok(tokNewline, 3, 10, "\n", ""),
			tok(tokName, 4, 1, "b", "b"),
			tok(tokAssign, 4, 3, "=", ""),
			tok(tokInt, 4, 5, "2", "2"),
			tok(tokNewline, 4, 6, "\n", ""),
			eofTok(5, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.kind != want.kind || got.pos != want.pos || got.lit != want.lit || got.text != want.text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.kind, got.pos, got.lit, got.text, want.kind, want.pos, want.lit, want.text)
			}
		}
	})

	t.Run("MultiLineInlineCommentSuppression", func(t *testing.T) {
		src := "a = 1 # trailing comment\nb = 2\n"
		tokens, errs := lex("test.lkal", []byte(src))
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if errs[0].Code != "L002" || errs[0].Pos != (Pos{Line: 1, Col: 7}) {
			t.Errorf("error mismatch: %v", errs[0])
		}
		wantTokens := []token{
			tok(tokName, 1, 1, "a", "a"),
			tok(tokAssign, 1, 3, "=", ""),
			tok(tokInt, 1, 5, "1", "1"),
			tok(tokNewline, 1, 25, "\n", ""),
			tok(tokName, 2, 1, "b", "b"),
			tok(tokAssign, 2, 3, "=", ""),
			tok(tokInt, 2, 5, "2", "2"),
			tok(tokNewline, 2, 6, "\n", ""),
			eofTok(3, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.kind != want.kind || got.pos != want.pos || got.lit != want.lit || got.text != want.text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.kind, got.pos, got.lit, got.text, want.kind, want.pos, want.lit, want.text)
			}
		}
	})
}

func TestLexMalformedUnicodeEscapeRecovery(t *testing.T) {
	cases := []lexCase{
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote",
			src:  "\"\\u{\\\"}\"\n",
			tokens: []token{
				tok(tokIllegal, 1, 1, "\"\\u{\\\"}\"", ""),
				tok(tokNewline, 1, 9, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L003", 1, 2)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_FollowingToken",
			src:  "\"\\u{\\\"}\" a\n",
			tokens: []token{
				tok(tokIllegal, 1, 1, "\"\\u{\\\"}\"", ""),
				tok(tokName, 1, 10, "a", "a"),
				tok(tokNewline, 1, 11, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L003", 1, 2)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_Unclosed",
			src:  "\"\\u{\\\"}\n",
			tokens: []token{
				tok(tokIllegal, 1, 1, "\"\\u{\\\"}", ""),
				tok(tokNewline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L007", 1, 1)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_QuotedRefSegment",
			src:  "&a/\"\\u{\\\"}\" b\n",
			tokens: []token{
				tok(tokIllegal, 1, 1, "&a/\"\\u{\\\"}\"", ""),
				tok(tokName, 1, 13, "b", "b"),
				tok(tokNewline, 1, 14, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L003", 1, 5)},
		},
	}
	runLexCases(t, cases)
}

func TestLexEnumAndRefTrailingInvalidChars(t *testing.T) {
	cases := []lexCase{
		{
			name: "EnumFollowedByPercent",
			src:  ".tcp%\n",
			tokens: []token{
				tok(tokEnum, 1, 1, ".tcp", "tcp"),
				tok(tokIllegal, 1, 5, "%", ""),
				tok(tokNewline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumFollowedByBareCR",
			src:  ".tcp\r",
			tokens: []token{
				tok(tokEnum, 1, 1, ".tcp", "tcp"),
				tok(tokIllegal, 1, 5, "\r", ""),
				eofTok(1, 6),
			},
			errs: []*Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumFollowedByInvalidUTF8",
			src:  ".tcp\xff\n",
			tokens: []token{
				tok(tokEnum, 1, 1, ".tcp", "tcp"),
				tok(tokIllegal, 1, 5, "\xff", ""),
				tok(tokNewline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumChainedDotName",
			src:  ".a.b\n",
			tokens: []token{
				tok(tokEnum, 1, 1, ".a", "a"),
				tok(tokEnum, 1, 3, ".b", "b"),
				tok(tokNewline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "RefFollowedByPercent",
			src:  "&a%\n",
			tokens: []token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(tokIllegal, 1, 3, "%", ""),
				tok(tokNewline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByPercentAndName",
			src:  "&a%b\n",
			tokens: []token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(tokIllegal, 1, 3, "%", ""),
				tok(tokName, 1, 4, "b", "b"),
				tok(tokNewline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByBareCR",
			src:  "&a\r",
			tokens: []token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(tokIllegal, 1, 3, "\r", ""),
				eofTok(1, 4),
			},
			errs: []*Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByInvalidUTF8",
			src:  "&a\xff\n",
			tokens: []token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(tokIllegal, 1, 3, "\xff", ""),
				tok(tokNewline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*Error{errExp("L008", 1, 3)},
		},
	}
	runLexCases(t, cases)
}
