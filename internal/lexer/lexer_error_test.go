package lexer

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/token"
)

func TestLexErrors(t *testing.T) {
	cases := []lexCase{
		{name: "L001_SingleHash", src: "# comment\n", tokens: []token.Token{tok(token.Comment, 1, 1, "# comment", "comment"), tok(token.Newline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L001", 1, 1)}},
		{name: "L001_DoubleHash", src: "## comment\n", tokens: []token.Token{tok(token.Comment, 1, 1, "## comment", "comment"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L001", 1, 1)}},
		{name: "L001_IndentedHash", src: "  # indent\n", tokens: []token.Token{tok(token.Comment, 1, 3, "# indent", "indent"), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L001", 1, 3)}},
		{name: "L001_TabIndentedHash", src: "\t# tab\n", tokens: []token.Token{tok(token.Comment, 1, 2, "# tab", "tab"), tok(token.Newline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L001", 1, 2)}},
		{name: "L002_InlineAfterToken", src: "a = 1 # bad\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Assign, 1, 3, "=", ""), tok(token.Int, 1, 5, "1", "1"), tok(token.Newline, 1, 12, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L002", 1, 7)}},
		{name: "L002_InlineAtEOF", src: "a = 1 # bad", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Assign, 1, 3, "=", ""), tok(token.Int, 1, 5, "1", "1"), eofTok(1, 12)}, errs: []*diag.Error{errExp("L002", 1, 7)}},
		{name: "L002_InsideList", src: "[1, # inside\n2]\n", tokens: []token.Token{tok(token.LBrack, 1, 1, "[", ""), tok(token.Int, 1, 2, "1", "1"), tok(token.Comma, 1, 3, ",", ""), tok(token.Newline, 1, 13, "\n", ""), tok(token.Int, 2, 1, "2", "2"), tok(token.RBrack, 2, 2, "]", ""), tok(token.Newline, 2, 3, "\n", ""), eofTok(3, 1)}, errs: []*diag.Error{errExp("L002", 1, 5)}},
		{name: "L002_DirectlyAfterSymbol", src: "={#\n}\n", tokens: []token.Token{tok(token.Assign, 1, 1, "=", ""), tok(token.LBrace, 1, 2, "{", ""), tok(token.Newline, 1, 4, "\n", ""), tok(token.RBrace, 2, 1, "}", ""), tok(token.Newline, 2, 2, "\n", ""), eofTok(3, 1)}, errs: []*diag.Error{errExp("L002", 1, 3)}},
		{name: "L003_InvalidEscapeX", src: "\"bad \\x escape\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"bad \\x escape\"", ""), tok(token.Newline, 1, 16, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 6)}},
		{name: "L003_InvalidEscapeA", src: "\"\\a\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\a\"", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeEmpty", src: "\"\\u{}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{}\"", ""), tok(token.Newline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeOutOfRange", src: "\"\\u{110000}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{110000}\"", ""), tok(token.Newline, 1, 13, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeSurrogateHigh", src: "\"\\u{D800}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{D800}\"", ""), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeSurrogateLow", src: "\"\\u{DFFF}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{DFFF}\"", ""), tok(token.Newline, 1, 11, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeTooLong", src: "\"\\u{1234567}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{1234567}\"", ""), tok(token.Newline, 1, 14, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_UnicodeNonHex", src: "\"\\u{xyz}\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\u{xyz}\"", ""), tok(token.Newline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L003_RefQuotedEscape", src: "&secret/\"bad \\x\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "&secret/\"bad \\x\"", ""), tok(token.Newline, 1, 17, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 14)}},
		{name: "L003_FirstStringErrorOnly", src: "\"\\x\\y\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"\\x\\y\"", ""), tok(token.Newline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L003", 1, 2)}},
		{name: "L004_QuantityDigitsAfterUnit", src: "1h30m\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1h30m", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityScientific", src: "1e5\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1e5", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityHex", src: "0x1f\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "0x1f", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L004", 1, 1)}},
		{name: "L004_QuantityDecimalWithDigits", src: "1.5m2\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1.5m2", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L004", 1, 1)}},
		{name: "L005_NumberOver100Digits", src: "11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", ""), tok(token.Newline, 1, 102, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L005", 1, 1)}},
		{name: "L005_DecimalOver100Digits", src: "0.11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "0.11111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", ""), tok(token.Newline, 1, 104, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L005", 1, 1)}},
		{name: "L005_Int64PositiveOverflow", src: "9223372036854775808\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "9223372036854775808", ""), tok(token.Newline, 1, 20, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L005", 1, 1)}},
		{name: "L005_Int64NegativeOverflow", src: "-9223372036854775809\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "-9223372036854775809", ""), tok(token.Newline, 1, 21, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L005", 1, 1)}},
		{name: "L006_UnknownDirectiveUse", src: "@use\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "@use", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L006", 1, 1)}},
		{name: "L006_UnknownDirectiveConfig", src: "@config\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "@config", ""), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L006", 1, 1)}},
		{name: "L007_UnclosedStringEOL", src: "\"unclosed\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"unclosed", ""), tok(token.Newline, 1, 10, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedStringEOF", src: "\"unclosed", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"unclosed", ""), eofTok(1, 10)}, errs: []*diag.Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedEmpty", src: "\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L007", 1, 1)}},
		{name: "L007_UnclosedSuppressesL003", src: "\"bad \\x escape unclosed\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"bad \\x escape unclosed", ""), tok(token.Newline, 1, 24, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L007", 1, 1)}},
		{name: "L008_BareCR", src: "a\rb\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Illegal, 1, 2, "\r", ""), tok(token.Name, 1, 3, "b", "b"), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 2)}},
		{name: "L008_BareCRAtEOF", src: "a\r", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Illegal, 1, 2, "\r", ""), eofTok(1, 3)}, errs: []*diag.Error{errExp("L008", 1, 2)}},
		{name: "L008_BOM", src: "\xef\xbb\xbf@lkal\n", tokens: []token.Token{tok(token.Directive, 1, 2, "@lkal", "lkal"), tok(token.Newline, 1, 7, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1)}},
		{name: "L008_RawNullInString", src: "\"a\x00b\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\x00b\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "L008_RawControlInString", src: "\"a\x1fb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\x1fb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "L008_DelInString", src: "\"a\x7fb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\x7fb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "L008_InvalidUTF8InString", src: "\"a\xffb\"\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\"a\xffb\"", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "L008_InvalidUTF8OutsideString", src: "\xff\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "\xff", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1)}},
		{name: "L008_InvalidUTF8InComment", src: "### a\xffb\n", tokens: []token.Token{tok(token.Comment, 1, 1, "### a\xffb", "a\xffb"), tok(token.Newline, 1, 8, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 6)}},
		{name: "L008_InvalidUTF8InRawLine", src: "\\\\a\xffb\n", tokens: []token.Token{tok(token.RawLine, 1, 1, "\\\\a\xffb", "a\xffb"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 4)}},
		{name: "L008_DisallowedChars", src: "% ; *\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "%", ""), tok(token.Illegal, 1, 3, ";", ""), tok(token.Illegal, 1, 5, "*", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 1), errExp("L008", 1, 3), errExp("L008", 1, 5)}},
		{name: "L008_MidLineAt", src: "a = @\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Assign, 1, 3, "=", ""), tok(token.Illegal, 1, 5, "@", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 5)}},
		{name: "L008_BackslashMidLine", src: "a \\ b\n", tokens: []token.Token{tok(token.Name, 1, 1, "a", "a"), tok(token.Illegal, 1, 3, "\\", ""), tok(token.Name, 1, 5, "b", "b"), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L008", 1, 3)}},
		{name: "L009_LeadingZeroInt", src: "01\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "01", ""), tok(token.Newline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_LeadingZeroNegative", src: "-01\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "-01", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_TrailingDotDecimal", src: "1.\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1.", ""), tok(token.Newline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_BareMinus", src: "-\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "-", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_MinusNonDigit", src: "-a\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "-a", ""), tok(token.Newline, 1, 3, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_UnderscoreInNumber", src: "1_000\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1_000", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_MultipleDotsInNumber", src: "1.2.3\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "1.2.3", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_UnitTrailingDot", src: "5m.\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "5m.", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_DirectiveNotNameStart", src: "@123\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "@123", ""), tok(token.Newline, 1, 5, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_DirectiveBareAt", src: "@\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "@", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_EnumBareDot", src: ".\n", tokens: []token.Token{tok(token.Illegal, 1, 1, ".", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_EnumNonNameStart", src: ".-x\n", tokens: []token.Token{tok(token.Illegal, 1, 1, ".-x", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_RefBareAmpersand", src: "&\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "&", ""), tok(token.Newline, 1, 2, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_RefLeadingSlash", src: "&/a\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "&/a", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_RefTrailingSlash", src: "&a/\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "&a/", ""), tok(token.Newline, 1, 4, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
		{name: "L009_RefDoubleSlash", src: "&a//b\n", tokens: []token.Token{tok(token.Illegal, 1, 1, "&a//b", ""), tok(token.Newline, 1, 6, "\n", ""), eofTok(2, 1)}, errs: []*diag.Error{errExp("L009", 1, 1)}},
	}
	runLexCases(t, cases)
}

func TestLexMalformedUnicodeEscapeRecovery(t *testing.T) {
	cases := []lexCase{
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote",
			src:  "\"\\u{\\\"}\"\n",
			tokens: []token.Token{
				tok(token.Illegal, 1, 1, "\"\\u{\\\"}\"", ""),
				tok(token.Newline, 1, 9, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L003", 1, 2)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_FollowingToken",
			src:  "\"\\u{\\\"}\" a\n",
			tokens: []token.Token{
				tok(token.Illegal, 1, 1, "\"\\u{\\\"}\"", ""),
				tok(token.Name, 1, 10, "a", "a"),
				tok(token.Newline, 1, 11, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L003", 1, 2)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_Unclosed",
			src:  "\"\\u{\\\"}\n",
			tokens: []token.Token{
				tok(token.Illegal, 1, 1, "\"\\u{\\\"}", ""),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L007", 1, 1)},
		},
		{
			name: "MalformedUnicodeEscapeWithEscapedQuote_QuotedRefSegment",
			src:  "&a/\"\\u{\\\"}\" b\n",
			tokens: []token.Token{
				tok(token.Illegal, 1, 1, "&a/\"\\u{\\\"}\"", ""),
				tok(token.Name, 1, 13, "b", "b"),
				tok(token.Newline, 1, 14, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L003", 1, 5)},
		},
	}
	runLexCases(t, cases)
}

func TestLexEnumAndRefTrailingInvalidChars(t *testing.T) {
	cases := []lexCase{
		{
			name: "EnumFollowedByPercent",
			src:  ".tcp%\n",
			tokens: []token.Token{
				tok(token.Enum, 1, 1, ".tcp", "tcp"),
				tok(token.Illegal, 1, 5, "%", ""),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumFollowedByBareCR",
			src:  ".tcp\r",
			tokens: []token.Token{
				tok(token.Enum, 1, 1, ".tcp", "tcp"),
				tok(token.Illegal, 1, 5, "\r", ""),
				eofTok(1, 6),
			},
			errs: []*diag.Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumFollowedByInvalidUTF8",
			src:  ".tcp\xff\n",
			tokens: []token.Token{
				tok(token.Enum, 1, 1, ".tcp", "tcp"),
				tok(token.Illegal, 1, 5, "\xff", ""),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 5)},
		},
		{
			name: "EnumChainedDotName",
			src:  ".a.b\n",
			tokens: []token.Token{
				tok(token.Enum, 1, 1, ".a", "a"),
				tok(token.Enum, 1, 3, ".b", "b"),
				tok(token.Newline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "RefFollowedByPercent",
			src:  "&a%\n",
			tokens: []token.Token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(token.Illegal, 1, 3, "%", ""),
				tok(token.Newline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByPercentAndName",
			src:  "&a%b\n",
			tokens: []token.Token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(token.Illegal, 1, 3, "%", ""),
				tok(token.Name, 1, 4, "b", "b"),
				tok(token.Newline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByBareCR",
			src:  "&a\r",
			tokens: []token.Token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(token.Illegal, 1, 3, "\r", ""),
				eofTok(1, 4),
			},
			errs: []*diag.Error{errExp("L008", 1, 3)},
		},
		{
			name: "RefFollowedByInvalidUTF8",
			src:  "&a\xff\n",
			tokens: []token.Token{
				refTok(1, 1, "&a", "a", []string{"a"}),
				tok(token.Illegal, 1, 3, "\xff", ""),
				tok(token.Newline, 1, 4, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 3)},
		},
	}
	runLexCases(t, cases)
}
