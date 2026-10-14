package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONRoundTrip(t *testing.T) {
	fixtureFiles, err := filepath.Glob(filepath.Join("../../testdata", "conformance", "valid", "*.lkal"))
	if err != nil {
		t.Fatalf("failed to glob valid fixtures: %v", err)
	}
	if len(fixtureFiles) == 0 {
		t.Fatal("no valid fixtures found in testdata/conformance/valid")
	}

	for _, path := range fixtureFiles {
		name := filepath.Base(path)
		t.Run("fixture_"+name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", path, err)
			}
			f, err := Parse(name, src)
			if err != nil {
				t.Fatalf("Parse(%s) failed: %v", name, err)
			}
			if f == nil {
				t.Fatalf("Parse(%s) returned nil File", name)
			}

			j, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}

			g, err := FromJSON(j)
			if err != nil {
				t.Fatalf("FromJSON failed: %v", err)
			}
			if g == nil {
				t.Fatalf("FromJSON returned nil File")
			}

			if !Equal(g.Data(), f.Data()) {
				t.Errorf("Equal(g.Data(), f.Data()) = false")
			}
			if gotVer, wantVer := g.Version(), f.Version(); gotVer != wantVer {
				t.Errorf("Version mismatch: got %q, want %q", gotVer, wantVer)
			}

			j2, err := ToJSON(g, false)
			if err != nil {
				t.Fatalf("second ToJSON failed: %v", err)
			}
			if !bytes.Equal(j2, j) {
				t.Errorf("roundtrip ToJSON mismatch:\ngot:\n%s\nwant:\n%s", j2, j)
			}
		})
	}

	inlineDocs := []struct {
		name string
		src  string
	}{
		{
			name: "empty_document",
			src:  "@lkal \"0.1\"\n",
		},
		{
			name: "simple_scalars",
			src:  "@lkal \"0.1\"\na = 1\nb = \"hello\"\nc = 0.25\nd = true\ne = .prod\nq = 100m\nr = &ref/path\n",
		},
		{
			name: "record_and_collection",
			src:  "@lkal \"0.1\"\nrec { x = 1 }\ncoll \"elem\" { y = 2 }\n",
		},
	}

	for _, doc := range inlineDocs {
		t.Run("inline_"+doc.name, func(t *testing.T) {
			f, err := Parse(doc.name+".lkal", []byte(doc.src))
			if err != nil {
				t.Fatalf("Parse(%s) failed: %v", doc.name, err)
			}
			if f == nil {
				t.Fatalf("Parse(%s) returned nil File", doc.name)
			}

			j, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}

			g, err := FromJSON(j)
			if err != nil {
				t.Fatalf("FromJSON failed: %v", err)
			}
			if g == nil {
				t.Fatalf("FromJSON returned nil File")
			}

			if !Equal(g.Data(), f.Data()) {
				t.Errorf("Equal(g.Data(), f.Data()) = false")
			}
			if gotVer, wantVer := g.Version(), f.Version(); gotVer != wantVer {
				t.Errorf("Version mismatch: got %q, want %q", gotVer, wantVer)
			}

			j2, err := ToJSON(g, false)
			if err != nil {
				t.Fatalf("second ToJSON failed: %v", err)
			}
			if !bytes.Equal(j2, j) {
				t.Errorf("roundtrip ToJSON mismatch:\ngot:\n%s\nwant:\n%s", j2, j)
			}
		})
	}
}

func FuzzFromJSON(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"$lkal": "0.1"}`),
		[]byte(`{"$lkal": "0.1", "a": 1}`),
		[]byte(`{"$lkal": "0.1", "str": "hello", "dec": 0.25, "b": true, "proto": {"$enum": "tcp"}, "mem": {"$qty": "512Mi"}, "ref": {"$ref": "secret/db-creds"}}`),
		[]byte(`{"$lkal": "0.1", "list": [1, 2, 3]}`),
		[]byte(`{"$lkal": "0.1", "rec": {"x": 1}}`),
		[]byte(`{"$lkal": "0.1", "coll": {"$coll": {"elem": {"x": 1}}}}`),
		[]byte(``),
		[]byte(`{}`),
		[]byte(`{"a": 1}`),
		[]byte(`{"$lkal": "0.2"}`),
		[]byte(`{"$lkal": "0.1", "a": null}`),
		[]byte(`{"$lkal": "0.1", "a": 1e5}`),
		[]byte(`{"$lkal": "0.1", "a": [[]]}`),
		[]byte(`{"$lkal": "0.1", "a": {"$enum": 123}}`),
		[]byte(`{"$lkal": "0.1", "a": 1, "a": 2}`),
		[]byte(`{"$lkal": "0.1", "$key": 1}`),
		[]byte(`{"$lkal": "0.1", "a": [1, "two"]}`),
		[]byte("\xff\xfe\xfd\x00\x01"),
	}

	validJSONs, _ := filepath.Glob(filepath.Join("../../testdata", "conformance", "valid", "*.json"))
	for _, p := range validJSONs {
		data, err := os.ReadFile(p)
		if err == nil {
			seeds = append(seeds, data)
		}
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g, err := FromJSON(data)
		if err == nil {
			if g == nil {
				t.Fatalf("FromJSON returned nil File with nil error")
			}
			j, err := ToJSON(g, false)
			if err != nil {
				t.Fatalf("ToJSON failed on parsed file: %v", err)
			}
			g2, err := FromJSON(j)
			if err != nil {
				t.Fatalf("FromJSON failed on roundtrip JSON: %v", err)
			}
			if g2 == nil {
				t.Fatalf("FromJSON returned nil File on roundtrip")
			}
			if !Equal(g2.Data(), g.Data()) {
				t.Errorf("Equal(g2.Data(), g.Data()) = false")
			}
			j2, err := ToJSON(g2, false)
			if err != nil {
				t.Fatalf("second ToJSON failed: %v", err)
			}
			if !bytes.Equal(j2, j) {
				t.Errorf("roundtrip ToJSON is not byte-stable")
			}
			return
		}

		if g != nil {
			t.Errorf("FromJSON returned non-nil File on error: %v", g)
		}
		errList, ok := err.(*ErrorList)
		if !ok {
			t.Fatalf("expected *ErrorList, got %T: %v", err, err)
		}
		if errList == nil || len(errList.Errors) == 0 {
			t.Fatalf("expected non-empty ErrorList")
		}
		if len(errList.Errors) > 20 {
			t.Errorf("error count %d not in [1, 20]", len(errList.Errors))
		}
		for i, e := range errList.Errors {
			if e == nil {
				t.Fatalf("nil error at index %d", i)
			}
			if e.File != "" {
				t.Errorf("error File: got %q, want %q", e.File, "")
			}
			if !jsonTIsKnownJSONErrorCode(e.Code) {
				t.Errorf("unknown error code: %q", e.Code)
			}
			if e.Msg == "" {
				t.Errorf("error Msg should not be empty for code %q", e.Code)
			}
			if e.Pos.Line < 1 {
				t.Errorf("invalid error Line %d, want >= 1", e.Pos.Line)
			}
			if e.Pos.Col < 1 {
				t.Errorf("invalid error Col %d, want >= 1", e.Pos.Col)
			}
			if i > 0 {
				prev := errList.Errors[i-1]
				if prev != nil {
					if prev.Pos.Line > e.Pos.Line ||
						(prev.Pos.Line == e.Pos.Line && prev.Pos.Col > e.Pos.Col) ||
						(prev.Pos.Line == e.Pos.Line && prev.Pos.Col == e.Pos.Col && prev.Code > e.Code) {
						t.Errorf("errors not sorted: [%d]=%v:%s vs [%d]=%v:%s", i-1, prev.Pos, prev.Code, i, e.Pos, e.Code)
					}
				}
			}
		}
	})
}
