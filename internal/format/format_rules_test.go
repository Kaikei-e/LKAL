package format_test

import . "github.com/Kaikei-e/LKAL"

import "testing"

type fmtTRuleCase struct {
	name string
	src  string
	want string
}

var fmtTLayoutRuleCases = []fmtTRuleCase{
	{
		name: "AnchorExample",
		src:  "@lkal \"0.1\"\na = 1, b = [1,\n  2]\nc { x = \"p\\nq\" }\n",
		want: "@lkal \"0.1\"\n\na = 1\nb = [\n  1,\n  2,\n]\nc {\n  x =\n    \\\\p\n    \\\\q\n}\n",
	},
	{
		name: "IndentationTabsToSpaces",
		src:  "@lkal \"0.1\"\nblock {\n\tsub {\n\t\tval = 1\n\t}\n}\n",
		want: "@lkal \"0.1\"\n\nblock {\n  sub {\n    val = 1\n  }\n}\n",
	},
	{
		name: "AssignSpacing",
		src:  "@lkal \"0.1\"\na=1\nb   =   2\n",
		want: "@lkal \"0.1\"\n\na = 1\nb = 2\n",
	},
	{
		name: "BracesHeaderAndAlone",
		src:  "@lkal \"0.1\"\nrec   {   \n  x = 1\n  }\n",
		want: "@lkal \"0.1\"\n\nrec {\n  x = 1\n}\n",
	},
	{
		name: "EmptyBlockOneLine",
		src:  "@lkal \"0.1\"\nrec {}\n",
		want: "@lkal \"0.1\"\n\nrec {}\n",
	},
	{
		name: "EmptyBlockMultiLine",
		src:  "@lkal \"0.1\"\nrec {\n\n}\n",
		want: "@lkal \"0.1\"\n\nrec {}\n",
	},
	{
		name: "EmptyElementBlock",
		src:  "@lkal \"0.1\"\nsvc \"web\" {\n}\n",
		want: "@lkal \"0.1\"\n\nsvc \"web\" {}\n",
	},
	{
		name: "OneLineBlockKept",
		src:  "@lkal \"0.1\"\nrec { a = 1, b = 2 }\n",
		want: "@lkal \"0.1\"\n\nrec { a = 1, b = 2 }\n",
	},
	{
		name: "OneLineBlockWithOneLineList",
		src:  "@lkal \"0.1\"\nrec { a = 1, b = [1, 2] }\n",
		want: "@lkal \"0.1\"\n\nrec { a = 1, b = [1, 2] }\n",
	},
	{
		name: "OneLineBlockTrailingCommaRemoved",
		src:  "@lkal \"0.1\"\nrec { a = 1, }\n",
		want: "@lkal \"0.1\"\n\nrec { a = 1 }\n",
	},
	{
		name: "MultiLineBlockWithOneAssignmentStaysMultiLine",
		src:  "@lkal \"0.1\"\nrec {\n  a = 1\n}\n",
		want: "@lkal \"0.1\"\n\nrec {\n  a = 1\n}\n",
	},
	{
		name: "OneLineBlockExpandsOnNestedBlock",
		src:  "@lkal \"0.1\"\nouter { inner { x = 1 } }\n",
		want: "@lkal \"0.1\"\n\nouter {\n  inner { x = 1 }\n}\n",
	},
	{
		name: "OneLineBlockExpandsOnRawCandidateString",
		src:  "@lkal \"0.1\"\nrec { s = \"line1\\nline2\" }\n",
		want: "@lkal \"0.1\"\n\nrec {\n  s =\n    \\\\line1\n    \\\\line2\n}\n",
	},
	{
		name: "MultiLineBlockWithMultiLineList",
		src:  "@lkal \"0.1\"\nrec { xs = [\n  1,\n  2\n] }\n",
		want: "@lkal \"0.1\"\n\nrec {\n  xs = [\n    1,\n    2,\n  ]\n}\n",
	},
	{
		name: "ListOneLine",
		src:  "@lkal \"0.1\"\nlst = [ 1 , 2 , 3 ]\n",
		want: "@lkal \"0.1\"\n\nlst = [1, 2, 3]\n",
	},
	{
		name: "ListMultiLineTrailingCommas",
		src:  "@lkal \"0.1\"\nlst = [\n1,\n2\n]\n",
		want: "@lkal \"0.1\"\n\nlst = [\n  1,\n  2,\n]\n",
	},
	{
		name: "ListMultiLineEmptyToBrackets",
		src:  "@lkal \"0.1\"\nlst = [\n\n]\n",
		want: "@lkal \"0.1\"\n\nlst = []\n",
	},
	{
		name: "ListBlankLinesDropped",
		src:  "@lkal \"0.1\"\nlst = [\n  1,\n\n  2,\n]\n",
		want: "@lkal \"0.1\"\n\nlst = [\n  1,\n  2,\n]\n",
	},
	{
		name: "ListStringsAlwaysQuotedWithLF",
		src:  "@lkal \"0.1\"\nlst = [\"line1\\nline2\"]\n",
		want: "@lkal \"0.1\"\n\nlst = [\"line1\\nline2\"]\n",
	},
	{
		name: "CommasBetweenEntriesRemoved",
		src:  "@lkal \"0.1\"\na = 1, b = 2, c = 3\n",
		want: "@lkal \"0.1\"\n\na = 1\nb = 2\nc = 3\n",
	},
	{
		name: "BlankLinesCollapsedToOne",
		src:  "@lkal \"0.1\"\n\n\n\na = 1\n\n\n\nb = 2\n",
		want: "@lkal \"0.1\"\n\na = 1\n\nb = 2\n",
	},
	{
		name: "BlankLinesRemovedAtBlockStartAndEnd",
		src:  "@lkal \"0.1\"\nrec {\n\n  a = 1\n\n}\n",
		want: "@lkal \"0.1\"\n\nrec {\n  a = 1\n}\n",
	},
	{
		name: "DeclarationFollowedByExactlyOneBlank",
		src:  "@lkal \"0.1\"\na = 1\n",
		want: "@lkal \"0.1\"\n\na = 1\n",
	},
	{
		name: "HeadCommentsBlankBeforeLkalPreserved",
		src:  "### comment\n\n@lkal \"0.1\"\n",
		want: "### comment\n\n@lkal \"0.1\"\n",
	},
	{
		name: "HeadCommentsNoBlankBeforeLkal",
		src:  "### comment\n@lkal \"0.1\"\n",
		want: "### comment\n@lkal \"0.1\"\n",
	},
	{
		name: "HeadCommentsLeadingBlankDropped",
		src:  "\n\n### comment\n@lkal \"0.1\"\n",
		want: "### comment\n@lkal \"0.1\"\n",
	},
	{
		name: "BlankBetweenCommentAndKeyPreserved",
		src:  "@lkal \"0.1\"\n### doc\n\na = 1\n",
		want: "@lkal \"0.1\"\n\n### doc\n\na = 1\n",
	},
	{
		name: "BlankBeforeTrailingCommentPreserved",
		src:  "@lkal \"0.1\"\nrec {\n  a = 1\n\n  ### trailing\n}\n",
		want: "@lkal \"0.1\"\n\nrec {\n  a = 1\n\n  ### trailing\n}\n",
	},
	{
		name: "CommentEmpty",
		src:  "@lkal \"0.1\"\n###\na = 1\n",
		want: "@lkal \"0.1\"\n\n###\na = 1\n",
	},
	{
		name: "CommentTextSpacingKept",
		src:  "@lkal \"0.1\"\n###   x\na = 1\n",
		want: "@lkal \"0.1\"\n\n###   x\na = 1\n",
	},
	{
		name: "CommentExtraHashes",
		src:  "@lkal \"0.1\"\n####x\na = 1\n",
		want: "@lkal \"0.1\"\n\n### #x\na = 1\n",
	},
	{
		name: "CommentTrailingWhitespaceRemoved",
		src:  "@lkal \"0.1\"\n### text   \na = 1\n",
		want: "@lkal \"0.1\"\n\n### text\na = 1\n",
	},
	{
		name: "CommentIndentationMatched",
		src:  "@lkal \"0.1\"\nrec {\n### comment\n  x = 1\n}\n",
		want: "@lkal \"0.1\"\n\nrec {\n  ### comment\n  x = 1\n}\n",
	},
	{
		name: "TrailingCommentInnerLevel",
		src:  "@lkal \"0.1\"\nrec {\n  x = 1\n### trailing\n}\n",
		want: "@lkal \"0.1\"\n\nrec {\n  x = 1\n  ### trailing\n}\n",
	},
}

func TestFormatRules(t *testing.T) {
	cases := append(append([]fmtTRuleCase(nil), fmtTLayoutRuleCases...), fmtTScalarRuleCases...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Format([]byte(tc.src))
			if err != nil {
				t.Fatalf("Format failed unexpectedly for %s: %v", tc.name, err)
			}
			if got == nil {
				t.Fatalf("Format returned nil bytes for %s", tc.name)
			}
			if string(got) != tc.want {
				t.Errorf("Format mismatch for %s:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
					tc.name, string(got), tc.want, string(got), tc.want)
			}
		})
	}
}
