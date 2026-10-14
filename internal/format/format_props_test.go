package format_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatProperties(t *testing.T) {
	type fmtTPropSource struct {
		name string
		src  []byte
	}

	var cases []fmtTPropSource

	confFiles, err := filepath.Glob(filepath.Join("../../testdata", "conformance", "valid", "*.lkal"))
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

	formatFiles, err := filepath.Glob(filepath.Join("../../testdata", "format", "*.lkal"))
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
		"@lkal \"0.1\"\n### comment\na = \"hello\"\n",
		"@lkal \"0.1\"\na = 1\na = 2\n",
		"@lkal \"0.1\"\n### a\x01b\n",
		"",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
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
		fromJSONFmt := fFromJSON.Format()
		if fromJSONFmt == nil {
			t.Fatalf("fFromJSON.Format() returned nil bytes")
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
			name: "1000_nested_elements",
			src:  "@lkal \"0.1\"\n" + strings.Repeat("a \"l\" { ", 1000) + "x = 1" + strings.Repeat(" }", 1000) + "\n",
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
