package format_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestFormatGolden(t *testing.T) {
	pairs := fmtTFindFixtures(t, filepath.Join("../../testdata", "format"), ".lkal", ".golden")
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
	specPath := filepath.Join("../../testdata", "conformance", "valid", "spec_example.lkal")
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
	pairs := fmtTFindFixtures(t, filepath.Join("../../testdata", "fromjson"), ".json", ".golden")
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
