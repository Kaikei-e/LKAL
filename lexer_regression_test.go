package lkal

import "testing"

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
