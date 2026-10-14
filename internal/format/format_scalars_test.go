package format_test

var fmtTScalarRuleCases = []fmtTRuleCase{
	{
		name: "StringRule9TwoLines",
		src:  "@lkal \"0.1\"\ns = \"a\\nb\"\n",
		want: "@lkal \"0.1\"\n\ns =\n  \\\\a\n  \\\\b\n",
	},
	{
		name: "StringRule9TrailingNewline",
		src:  "@lkal \"0.1\"\ns = \"line\\n\"\n",
		want: "@lkal \"0.1\"\n\ns =\n  \\\\line\n  \\\\\n",
	},
	{
		name: "StringRule9WithCRStaysQuoted",
		src:  "@lkal \"0.1\"\ns = \"a\\r\\nb\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\\r\\nb\"\n",
	},
	{
		name: "StringRule9WithControlStaysQuoted",
		src:  "@lkal \"0.1\"\ns = \"a\\u{1}\\nb\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\\u{1}\\nb\"\n",
	},
	{
		name: "StringSingleRawLineBecomesQuoted",
		src:  "@lkal \"0.1\"\ns =\n  \\\\single\n",
		want: "@lkal \"0.1\"\n\ns = \"single\"\n",
	},
	{
		name: "StringTabKeptRaw",
		src:  "@lkal \"0.1\"\ns = \"a\\tb\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\tb\"\n",
	},
	{
		name: "EscapeMinimalASCII",
		src:  "@lkal \"0.1\"\ns = \"\\u{41}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"A\"\n",
	},
	{
		name: "EscapeMinimalTab",
		src:  "@lkal \"0.1\"\ns = \"\\t\"\n",
		want: "@lkal \"0.1\"\n\ns = \"\t\"\n",
	},
	{
		name: "EscapeMinimalU007F",
		src:  "@lkal \"0.1\"\ns = \"\\u{7f}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"\\u{7f}\"\n",
	},
	{
		name: "EscapeMinimalEmoji",
		src:  "@lkal \"0.1\"\ns = \"\\u{1F600}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"😀\"\n",
	},
	{
		name: "NumberNormalization",
		src:  "@lkal \"0.1\"\na = 1.50\nb = 2.00\nc = -0\nd = -0.0\ne = 1.50Gi\nf = -0Mi\n",
		want: "@lkal \"0.1\"\n\na = 1.5\nb = 2.0\nc = 0\nd = 0.0\ne = 1.5Gi\nf = 0Mi\n",
	},
	{
		name: "KeyNormalization",
		src:  "@lkal \"0.1\"\n\"abc\" = 1\n\"true\" = 2\n\"false\" = 3\n\"a b\" = 4\n\"app.kubernetes.io/name\" = 5\n",
		want: "@lkal \"0.1\"\n\nabc = 1\n\"true\" = 2\n\"false\" = 3\n\"a b\" = 4\n\"app.kubernetes.io/name\" = 5\n",
	},
	{
		name: "LabelsAlwaysQuoted",
		src:  "@lkal \"0.1\"\nsvc \"web\" {}\n",
		want: "@lkal \"0.1\"\n\nsvc \"web\" {}\n",
	},
	{
		name: "RefNormalization",
		src:  "@lkal \"0.1\"\nr1 = &a/\"b\"\nr2 = &a/\"b c\"\nr3 = &a/\"\"\n",
		want: "@lkal \"0.1\"\n\nr1 = &a/b\nr2 = &a/\"b c\"\nr3 = &a/\"\"\n",
	},
	{
		name: "CRLFToLF",
		src:  "@lkal \"0.1\"\r\na = 1\r\n",
		want: "@lkal \"0.1\"\n\na = 1\n",
	},
	{
		name: "MissingFinalNewlineAdded",
		src:  "@lkal \"0.1\"\na = 1",
		want: "@lkal \"0.1\"\n\na = 1\n",
	},
	{
		name: "EmptyBodyJustDeclaration",
		src:  "@lkal \"0.1\"\n",
		want: "@lkal \"0.1\"\n",
	},
	{
		name: "BodyOnlyTrailingComments",
		src:  "@lkal \"0.1\"\n### only trailing\n",
		want: "@lkal \"0.1\"\n\n### only trailing\n",
	},
	{
		name: "BlankBeforeEntryFirstCommentPreserved",
		src:  "@lkal \"0.1\"\n\na = 1\n\n### doc\nb = 2\n",
		want: "@lkal \"0.1\"\n\na = 1\n\n### doc\nb = 2\n",
	},
	{
		name: "BlankBetweenCommentsOfOneEntryPreserved",
		src:  "@lkal \"0.1\"\n\n### first\n\n### second\na = 1\n",
		want: "@lkal \"0.1\"\n\n### first\n\n### second\na = 1\n",
	},
	{
		name: "BlankBetweenHeadCommentsPreserved",
		src:  "### head 1\n\n### head 2\n@lkal \"0.1\"\n",
		want: "### head 1\n\n### head 2\n@lkal \"0.1\"\n",
	},
	{
		name: "HeadCommentsFollowedByBody",
		src:  "### c\n@lkal \"0.1\"\na = 1\n",
		want: "### c\n@lkal \"0.1\"\n\na = 1\n",
	},
	{
		name: "QuoteStringDoubleQuote",
		src:  "@lkal \"0.1\"\ns = \"a\\\"b\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\\\"b\"\n",
	},
	{
		name: "QuoteStringBackslash",
		src:  "@lkal \"0.1\"\ns = \"a\\\\b\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\\\\b\"\n",
	},
	{
		name: "QuoteStringLoneCR",
		src:  "@lkal \"0.1\"\ns = \"a\\rb\"\n",
		want: "@lkal \"0.1\"\n\ns = \"a\\rb\"\n",
	},
	{
		name: "QuoteStringU001B",
		src:  "@lkal \"0.1\"\ns = \"\\u{1b}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"\\u{1b}\"\n",
	},
	{
		name: "QuoteStringU0000",
		src:  "@lkal \"0.1\"\ns = \"\\u{0}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"\\u{0}\"\n",
	},
	{
		name: "QuoteStringU2028Raw",
		src:  "@lkal \"0.1\"\ns = \"\\u{2028}\"\n",
		want: "@lkal \"0.1\"\n\ns = \"\u2028\"\n",
	},
	{
		name: "RawLineContentVerbatim",
		src:  "@lkal \"0.1\"\ns =\n  \\\\  leading spaces\n  \\\\trailing spaces  \n  \\\\### comment inside\n  \\\\\"quotes\" and \\\\backslash\\\\\n",
		want: "@lkal \"0.1\"\n\ns =\n  \\\\  leading spaces\n  \\\\trailing spaces  \n  \\\\### comment inside\n  \\\\\"quotes\" and \\\\backslash\\\\\n",
	},
	{
		name: "KeysBareAndQuoted",
		src:  "@lkal \"0.1\"\n\"a-b\" = 1\n\"_x\" = 2\n\"1a\" = 3\n\"-a\" = 4\n\"\" = 5\n\"キー\" = 6\n",
		want: "@lkal \"0.1\"\n\na-b = 1\n_x = 2\n\"1a\" = 3\n\"-a\" = 4\n\"\" = 5\n\"キー\" = 6\n",
	},
	{
		name: "RefBareAndQuotedSegments",
		src:  "@lkal \"0.1\"\nr1 = &a/v1.2\nr2 = &a/\"パス\"\n",
		want: "@lkal \"0.1\"\n\nr1 = &a/v1.2\nr2 = &a/\"パス\"\n",
	},
	{
		name: "OneLineBlockVariedValues",
		src:  "@lkal \"0.1\"\nrec { \"needs quote\" = 1, proto = .tcp, mem = 512Mi, target = &a/b, tags = [], esc = \"a\\r\\nb\" }\n",
		want: "@lkal \"0.1\"\n\nrec { \"needs quote\" = 1, proto = .tcp, mem = 512Mi, target = &a/b, tags = [], esc = \"a\\r\\nb\" }\n",
	},
}
