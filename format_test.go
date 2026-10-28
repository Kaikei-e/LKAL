package lkal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fmtTRuleCase struct {
	name string
	src  string
	want string
}

func fmtTEqualErrorLists(a, b *ErrorList) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a.Errors) != len(b.Errors) {
		return false
	}
	for i := range a.Errors {
		ea, eb := a.Errors[i], b.Errors[i]
		if ea == nil && eb == nil {
			continue
		}
		if ea == nil || eb == nil {
			return false
		}
		if ea.Code != eb.Code || ea.Pos != eb.Pos || ea.File != eb.File || ea.Msg != eb.Msg || ea.Hint != eb.Hint {
			return false
		}
	}
	return true
}

func fmtTFindFixtures(t *testing.T, dir, srcExt, wantExt string) []struct{ name, srcPath, wantPath string } {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read fixture dir %s: %v", dir, err)
	}
	var res []struct{ name, srcPath, wantPath string }
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, srcExt) {
			base := strings.TrimSuffix(name, srcExt)
			srcPath := filepath.Join(dir, name)
			wantPath := filepath.Join(dir, base+wantExt)
			if _, err := os.Stat(wantPath); err != nil {
				t.Fatalf("missing pair for %s: expected %s (%v)", srcPath, wantPath, err)
			}
			res = append(res, struct{ name, srcPath, wantPath string }{
				name:     base,
				srcPath:  srcPath,
				wantPath: wantPath,
			})
		}
	}
	return res
}

func TestFormatRules(t *testing.T) {
	cases := []fmtTRuleCase{
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

func TestFormatGolden(t *testing.T) {
	pairs := fmtTFindFixtures(t, filepath.Join("testdata", "format"), ".lkal", ".golden")
	if len(pairs) < 6 {
		t.Fatalf("expected at least 6 format fixtures, found %d", len(pairs))
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			src, err := os.ReadFile(p.srcPath)
			if err != nil {
				t.Fatalf("failed reading %s: %v", p.srcPath, err)
			}
			want, err := os.ReadFile(p.wantPath)
			if err != nil {
				t.Fatalf("failed reading %s: %v", p.wantPath, err)
			}

			got, err := Format(src)
			if err != nil {
				t.Fatalf("Format(%s) returned unexpected error: %v", p.name, err)
			}
			if got == nil {
				t.Fatalf("Format(%s) returned nil bytes", p.name)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("Format(%s) output mismatch:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
					p.name, string(got), string(want), string(got), string(want))
			}

			// Format of golden must be byte-for-byte identical to golden.
			goldenAgain, err := Format(want)
			if err != nil {
				t.Fatalf("Format(golden) returned unexpected error for %s: %v", p.name, err)
			}
			if !bytes.Equal(goldenAgain, want) {
				t.Errorf("Format(golden) not idempotent for %s:\ngot:\n%s\nwant:\n%s",
					p.name, string(goldenAgain), string(want))
			}
		})
	}
}

func TestFormatSpecExample(t *testing.T) {
	specPath := filepath.Join("testdata", "conformance", "valid", "spec_example.lkal")
	src, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", specPath, err)
	}

	got, err := Format(src)
	if err != nil {
		t.Fatalf("Format(spec_example) failed unexpectedly: %v", err)
	}
	if got == nil {
		t.Fatalf("Format(spec_example) returned nil bytes")
	}
	if !bytes.Equal(got, src) {
		t.Errorf("Format(spec_example) modified canonical file:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
			string(got), string(src), string(got), string(src))
	}
}

func TestFormatErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "EmptyDocument",
			src:  "",
		},
		{
			name: "MissingVersion",
			src:  "a = 1\n",
		},
		{
			name: "UnsupportedVersion",
			src:  "@lkal \"0.2\"\n",
		},
		{
			name: "DuplicateKey",
			src:  "@lkal \"0.1\"\na = 1\na = 2\n",
		},
		{
			name: "BareWordInValue",
			src:  "@lkal \"0.1\"\na = tcp\n",
		},
		{
			name: "L008ControlInComment",
			src:  "@lkal \"0.1\"\n### a\x01b\n",
		},
		{
			name: "L008ControlInRawLine",
			src:  "@lkal \"0.1\"\nx =\n  \\\\a\x7fb\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotBytes, fmtErr := Format([]byte(tc.src))
			if fmtErr == nil {
				t.Fatalf("Format(%s) expected error, got nil error and output %q", tc.name, string(gotBytes))
			}
			if gotBytes != nil {
				t.Errorf("Format(%s) returned non-nil bytes on error: %q", tc.name, string(gotBytes))
			}

			fmtList, ok := fmtErr.(*ErrorList)
			if !ok {
				t.Fatalf("Format(%s) error is %T, want *ErrorList: %v", tc.name, fmtErr, fmtErr)
			}

			_, parseErr := Parse("", []byte(tc.src))
			if parseErr == nil {
				t.Fatalf("Parse(\"\", %s) unexpectedly succeeded", tc.name)
			}
			parseList, ok := parseErr.(*ErrorList)
			if !ok {
				t.Fatalf("Parse(\"\", %s) error is %T, want *ErrorList: %v", tc.name, parseErr, parseErr)
			}

			if !fmtTEqualErrorLists(fmtList, parseList) {
				t.Errorf("Format(%s) error list mismatch:\ngot:  %v\nwant: %v", tc.name, fmtList, parseList)
			}
		})
	}
}

func TestFileFormatFromJSON(t *testing.T) {
	pairs := fmtTFindFixtures(t, filepath.Join("testdata", "fromjson"), ".json", ".golden")
	if len(pairs) < 4 {
		t.Fatalf("expected at least 4 fromjson fixtures, found %d", len(pairs))
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			jsonBytes, err := os.ReadFile(p.srcPath)
			if err != nil {
				t.Fatalf("failed reading %s: %v", p.srcPath, err)
			}
			wantBytes, err := os.ReadFile(p.wantPath)
			if err != nil {
				t.Fatalf("failed reading %s: %v", p.wantPath, err)
			}

			f, err := FromJSON(jsonBytes)
			if err != nil {
				t.Fatalf("FromJSON(%s) failed: %v", p.name, err)
			}
			if f == nil {
				t.Fatalf("FromJSON(%s) returned nil File", p.name)
			}

			got := f.Format()
			if got == nil {
				t.Fatalf("f.Format() returned nil bytes for %s", p.name)
			}
			if !bytes.Equal(got, wantBytes) {
				t.Errorf("FromJSON(%s).Format() mismatch:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
					p.name, string(got), string(wantBytes), string(got), string(wantBytes))
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	cases := []struct {
		name string
		val  Value
		want []byte
	}{
		{
			name: "Int",
			val:  Int(42),
			want: []byte("42"),
		},
		{
			name: "Decimal",
			val:  Decimal("1.5"),
			want: []byte("1.5"),
		},
		{
			name: "BoolTrue",
			val:  Bool(true),
			want: []byte("true"),
		},
		{
			name: "BoolFalse",
			val:  Bool(false),
			want: []byte("false"),
		},
		{
			name: "Enum",
			val:  Enum("tcp"),
			want: []byte(".tcp"),
		},
		{
			name: "Quantity",
			val:  Quantity{Number: "512", Unit: "Mi"},
			want: []byte("512Mi"),
		},
		{
			name: "RefSimple",
			val:  Ref{"secret", "db-creds"},
			want: []byte("&secret/db-creds"),
		},
		{
			name: "RefQuotedSegment",
			val:  Ref{"a", "b c"},
			want: []byte("&a/\"b c\""),
		},
		{
			name: "StringSimple",
			val:  String("hello"),
			want: []byte("\"hello\""),
		},
		{
			name: "StringRawCandidate",
			val:  String("one\ntwo"),
			want: []byte("\\\\one\n\\\\two"),
		},
		{
			name: "StringQuotedWithCR",
			val:  String("one\rtwo"),
			want: []byte("\"one\\rtwo\""),
		},
		{
			name: "ListEmpty",
			val:  List{},
			want: []byte("[]"),
		},
		{
			name: "ListScalars",
			val:  List{Int(1), Int(2)},
			want: []byte("[1, 2]"),
		},
		{
			name: "RecordEmpty",
			val:  Record{},
			want: []byte("{}"),
		},
		{
			name: "RecordWithNested",
			val: Record{
				Fields: []Field{
					{Key: "scalar", Value: Int(1)},
					{
						Key: "nested",
						Value: Record{
							Fields: []Field{
								{Key: "x", Value: String("ok")},
							},
						},
					},
					{
						Key: "service",
						Value: Record{
							Fields: []Field{
								{
									Key: "web",
									Value: Record{
										Fields: []Field{
											{Key: "port", Value: Int(8080)},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []byte("{\n  scalar = 1\n  nested {\n    x = \"ok\"\n  }\n  service {\n    web {\n      port = 8080\n    }\n  }\n}"),
		},
		{
			name: "RecordWithTwoNestedOneEmpty",
			val: Record{
				Fields: []Field{
					{
						Key: "first",
						Value: Record{
							Fields: []Field{
								{Key: "x", Value: Int(1)},
							},
						},
					},
					{
						Key:   "second",
						Value: Record{},
					},
				},
			},
			want: []byte("{\n  first {\n    x = 1\n  }\n  second {}\n}"),
		},
		{
			name: "RecordWithRawCandidateString",
			val: Record{
				Fields: []Field{
					{Key: "script", Value: String("line1\nline2")},
				},
			},
			want: []byte("{\n  script =\n    \\\\line1\n    \\\\line2\n}"),
		},
		{
			name: "RecordWithList",
			val: Record{
				Fields: []Field{
					{Key: "items", Value: List{Int(1), Int(2)}},
				},
			},
			want: []byte("{\n  items = [1, 2]\n}"),
		},
		{
			name: "DecimalStoredText",
			val:  Decimal("2.0"),
			want: []byte("2.0"),
		},
		{
			name: "NilValue",
			val:  nil,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatValue(tc.val)
			if tc.want == nil {
				if got != nil {
					t.Errorf("FormatValue(%s) got %q, want nil", tc.name, string(got))
				}
				return
			}
			if got == nil {
				t.Fatalf("FormatValue(%s) got nil, want %q", tc.name, string(tc.want))
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("FormatValue(%s) mismatch:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
					tc.name, string(got), string(tc.want), string(got), string(tc.want))
			}
		})
	}
}

func TestFormatProperties(t *testing.T) {
	type fmtTPropSource struct {
		name string
		src  []byte
	}

	var cases []fmtTPropSource

	confFiles, err := filepath.Glob(filepath.Join("testdata", "conformance", "valid", "*.lkal"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	for _, p := range confFiles {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("failed reading %s: %v", p, err)
		}
		cases = append(cases, fmtTPropSource{
			name: "Conformance_" + filepath.Base(p),
			src:  data,
		})
	}

	formatFiles, err := filepath.Glob(filepath.Join("testdata", "format", "*.lkal"))
	if err != nil {
		t.Fatalf("filepath.Glob failed: %v", err)
	}
	for _, p := range formatFiles {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("failed reading %s: %v", p, err)
		}
		cases = append(cases, fmtTPropSource{
			name: "Format_" + filepath.Base(p),
			src:  data,
		})
	}

	inlines := []struct {
		name string
		src  string
	}{
		{name: "Inline_Commas", src: "@lkal \"0.1\"\na = 1, b = 2\n"},
		{name: "Inline_RawString", src: "@lkal \"0.1\"\nx = \"a\\nb\"\n"},
		{name: "Inline_Collection", src: "@lkal \"0.1\"\ns \"w\" { port = 80 }\n"},
		{name: "Inline_EmptyRecord", src: "@lkal \"0.1\"\nempty {}\n"},
	}
	for _, in := range inlines {
		cases = append(cases, fmtTPropSource{
			name: in.name,
			src:  []byte(in.src),
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.src
			fOrig, err := Parse("", src)
			if err != nil {
				t.Fatalf("Parse(src) failed: %v", err)
			}
			if fOrig == nil {
				t.Fatalf("Parse(src) returned nil File")
			}

			out, err := Format(src)
			if err != nil {
				t.Fatalf("Format(src) failed: %v", err)
			}
			if out == nil {
				t.Fatalf("Format(src) returned nil bytes")
			}

			out2, err := Format(out)
			if err != nil {
				t.Fatalf("Format(out) failed: %v", err)
			}
			if !bytes.Equal(out, out2) {
				t.Fatalf("Format not idempotent:\nfirst:\n%s\nsecond:\n%s", string(out), string(out2))
			}

			fFmt, err := Parse("", out)
			if err != nil {
				t.Fatalf("Parse(out) failed: %v", err)
			}
			if fFmt == nil {
				t.Fatalf("Parse(out) returned nil File")
			}

			if fFmt.Version() != fOrig.Version() {
				t.Errorf("Version mismatch: got %q, want %q", fFmt.Version(), fOrig.Version())
			}
			if !Equal(fFmt.Data(), fOrig.Data()) {
				t.Errorf("Data mismatch after format:\ngot:  %+v\nwant: %+v", fFmt.Data(), fOrig.Data())
			}

			jOrig, err := ToJSON(fOrig, false)
			if err != nil {
				t.Fatalf("ToJSON(fOrig) failed: %v", err)
			}
			jFmt, err := ToJSON(fFmt, false)
			if err != nil {
				t.Fatalf("ToJSON(fFmt) failed: %v", err)
			}
			if !bytes.Equal(jOrig, jFmt) {
				t.Errorf("ToJSON mismatch:\ngot:\n%s\nwant:\n%s", string(jFmt), string(jOrig))
			}

			fFromJSON, err := FromJSON(jOrig)
			if err != nil {
				t.Fatalf("FromJSON(jOrig) failed: %v", err)
			}
			if fFromJSON == nil {
				t.Fatalf("FromJSON(jOrig) returned nil File")
			}
			fromJSONFmt := fFromJSON.Format()
			if fromJSONFmt == nil {
				t.Fatalf("fFromJSON.Format() returned nil bytes")
			}
			fromJSONFmt2, err := Format(fromJSONFmt)
			if err != nil {
				t.Fatalf("Format(fromJSONFmt) failed: %v", err)
			}
			if !bytes.Equal(fromJSONFmt, fromJSONFmt2) {
				t.Errorf("FromJSON.Format() is not fixed point of Format:\nfirst:\n%s\nsecond:\n%s",
					string(fromJSONFmt), string(fromJSONFmt2))
			}
		})
	}
}

func FuzzFormat(f *testing.F) {
	seeds := []string{
		"@lkal \"0.1\"\n",
		"@lkal \"0.1\"\na = 1\n",
		"@lkal \"0.1\"\na = 1, b = [1,\n  2]\nc { x = \"p\\nq\" }\n",
		"@lkal \"0.1\"\nservice \"web\" { port = 8080 }\n",
		"@lkal \"0.1\"\nroute \"GET\" \"/users\" { handler = \"list\", auth = true }\nenv \"LOG_LEVEL\" = \"info\"\n",
		"@lkal \"0.1\"\n### comment\na = \"hello\"\n",
		"@lkal \"0.1\"\na = 1\na = 2\n",
		"@lkal \"0.1\"\n### a\x01b\n",
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	for _, b := range fuzzLoadSeedFiles(
		filepath.Join("testdata", "conformance", "valid", "*.lkal"),
		filepath.Join("testdata", "conformance", "plain", "*.lkal"),
		filepath.Join("testdata", "format", "*.lkal"),
		filepath.Join("testdata", "conformance", "invalid", "*.lkal"),
	) {
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		fOrig, parseErr := Parse("", src)
		out, fmtErr := Format(src)

		if parseErr != nil {
			if fmtErr == nil {
				t.Fatalf("Format succeeded but Parse failed for %q", src)
			}
			if out != nil {
				t.Fatalf("Format returned non-nil bytes on parse error")
			}
			pList, ok1 := parseErr.(*ErrorList)
			fList, ok2 := fmtErr.(*ErrorList)
			if !ok1 || !ok2 {
				t.Fatalf("errors are not *ErrorList: parse=%T fmt=%T", parseErr, fmtErr)
			}
			if !fmtTEqualErrorLists(fList, pList) {
				t.Fatalf("ErrorList mismatch: got %v want %v", fList, pList)
			}
			return
		}

		if fmtErr != nil {
			t.Fatalf("Format failed unexpectedly on valid input: %v", fmtErr)
		}
		if out == nil {
			t.Fatalf("Format returned nil bytes on valid input")
		}
		if !bytes.Equal(out, fOrig.Format()) {
			t.Fatalf("Format(src) != fOrig.Format()")
		}
		if reason := fuzzCanonicalLint(out); reason != "" {
			t.Fatalf("Format output failed canonical lint (%s):\n%s", reason, out)
		}

		out2, err := Format(out)
		if err != nil {
			t.Fatalf("Format(out) failed: %v", err)
		}
		if !bytes.Equal(out, out2) {
			t.Fatalf("Format not idempotent")
		}

		fFmt, err := Parse("", out)
		if err != nil {
			t.Fatalf("Parse(out) failed: %v", err)
		}
		if fFmt == nil {
			t.Fatalf("Parse(out) returned nil File")
		}

		if fFmt.Version() != fOrig.Version() {
			t.Fatalf("Version mismatch: got %q, want %q", fFmt.Version(), fOrig.Version())
		}
		if !Equal(fFmt.Data(), fOrig.Data()) {
			t.Fatalf("Data mismatch between original and formatted")
		}
		if !fuzzSameValue(fFmt.Data(), fOrig.Data()) {
			t.Fatalf("strict Data() field-order mismatch between original and formatted")
		}

		jOrig, err := ToJSON(fOrig, false)
		if err != nil {
			t.Fatalf("ToJSON(fOrig) failed: %v", err)
		}
		jFmt, err := ToJSON(fFmt, false)
		if err != nil {
			t.Fatalf("ToJSON(fFmt) failed: %v", err)
		}
		if !bytes.Equal(jOrig, jFmt) {
			t.Fatalf("ToJSON mismatch")
		}

		fFromJSON, err := FromJSON(jOrig)
		if err != nil {
			t.Fatalf("FromJSON failed: %v", err)
		}
		if fFromJSON == nil {
			t.Fatalf("FromJSON returned nil File")
		}
		if !Equal(fFromJSON.Data(), fOrig.Data()) {
			t.Fatalf("Equal(fFromJSON.Data(), fOrig.Data()) = false")
		}
		if !fuzzSameValue(fFromJSON.Data(), fOrig.Data()) {
			t.Fatalf("strict Data() mismatch after FromJSON")
		}
		fuzzCheckEqualLaws(t, fOrig.Data(), fFmt.Data(), fFromJSON.Data())

		fromJSONFmt := fFromJSON.Format()
		if fromJSONFmt == nil {
			t.Fatalf("fFromJSON.Format() returned nil bytes")
		}
		if reason := fuzzCanonicalLint(fromJSONFmt); reason != "" {
			t.Fatalf("fFromJSON.Format() failed canonical lint (%s):\n%s", reason, fromJSONFmt)
		}
		fromJSONFmt2, err := Format(fromJSONFmt)
		if err != nil {
			t.Fatalf("Format(fromJSONFmt) failed: %v", err)
		}
		if !bytes.Equal(fromJSONFmt, fromJSONFmt2) {
			t.Fatalf("FromJSON.Format() is not fixed point of Format")
		}
	})
}

func TestFormatDeepNesting(t *testing.T) {
	type nestTFormatCase struct {
		name string
		src  string
	}

	tests := []nestTFormatCase{
		{
			name: "1000_nested_records",
			src:  "@lkal \"0.1\"\n" + strings.Repeat("a { ", 1000) + "x = 1" + strings.Repeat(" }", 1000) + "\n",
		},
		{
			name: "500_nested_labeled_blocks",
			src:  "@lkal \"0.1\"\n" + strings.Repeat("a \"l\" { ", 500) + "x = 1" + strings.Repeat(" }", 500) + "\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fOrig, err := Parse("", []byte(tc.src))
			if err != nil {
				t.Fatalf("Parse(%q) original failed: got error %v, want nil", tc.name, err)
			}
			if fOrig == nil {
				t.Fatalf("Parse(%q) original returned nil File: got nil, want non-nil *File", tc.name)
			}

			out, err := Format([]byte(tc.src))
			if err != nil {
				t.Fatalf("Format(%q) failed: got error %v, want nil", tc.name, err)
			}
			if out == nil {
				t.Fatalf("Format(%q) returned nil bytes: got nil, want non-nil []byte", tc.name)
			}

			out2, err := Format(out)
			if err != nil {
				t.Fatalf("Format(Format(%q)) failed: got error %v, want nil", tc.name, err)
			}
			if out2 == nil {
				t.Fatalf("Format(Format(%q)) returned nil bytes: got nil, want non-nil []byte", tc.name)
			}
			if !bytes.Equal(out, out2) {
				t.Fatalf("Format(%q) not idempotent: got Format(Format(x)) != Format(x), want equal bytes", tc.name)
			}

			fFmt, err := Parse("", out)
			if err != nil {
				t.Fatalf("Parse(%q) formatted failed: got error %v, want nil", tc.name, err)
			}
			if fFmt == nil {
				t.Fatalf("Parse(%q) formatted returned nil File: got nil, want non-nil *File", tc.name)
			}

			if !Equal(fFmt.Data(), fOrig.Data()) {
				t.Fatalf("Parse(%q, out).Data() mismatch: got %v, want %v", tc.name, fFmt.Data(), fOrig.Data())
			}
		})
	}
}

func TestFormatLabeledEntries(t *testing.T) {
	src := `@lkal "0.1"

route   "GET"   "/users"  { handler = "list", auth = true }
env  "LOG_LEVEL"   =   "info"
block { env "DEBUG" = "1" }
`
	want := `@lkal "0.1"

route "GET" "/users" { handler = "list", auth = true }
env "LOG_LEVEL" = "info"
block {
  env "DEBUG" = "1"
}
`
	got, err := Format([]byte(src))
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	if string(got) != want {
		t.Errorf("Format mismatch:\ngot:\n%s\nwant:\n%s", string(got), want)
	}
}
