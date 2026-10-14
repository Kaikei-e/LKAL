package lkal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type confFixture struct {
	name     string
	srcPath  string
	dataPath string
}

func confFindPairFixtures(t *testing.T, dir, srcExt, dataExt string) []confFixture {
	t.Helper()
	var fixtures []confFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		if strings.HasSuffix(path, srcExt) {
			base := strings.TrimSuffix(path, srcExt)
			dataPath := base + dataExt
			if _, statErr := os.Stat(dataPath); statErr != nil {
				t.Fatalf("missing fixture pair for %s: expected %s (%v)", path, dataPath, statErr)
			}
			rel, relErr := filepath.Rel(dir, base)
			if relErr != nil {
				rel = base
			}
			fixtures = append(fixtures, confFixture{
				name:     filepath.ToSlash(rel),
				srcPath:  path,
				dataPath: dataPath,
			})
		} else if strings.HasSuffix(path, dataExt) {
			srcPath := strings.TrimSuffix(path, dataExt) + srcExt
			if _, statErr := os.Stat(srcPath); statErr != nil {
				t.Fatalf("missing fixture pair for %s: expected %s (%v)", path, srcPath, statErr)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("confFindPairFixtures failed for %s: %v", dir, err)
	}
	return fixtures
}

func confDecodeTokens(data []byte) ([]json.Token, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var tokens []json.Token
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

func confDiffJSON(gotBytes, wantBytes []byte) []string {
	gotTokens, err := confDecodeTokens(gotBytes)
	if err != nil {
		return []string{fmt.Sprintf("decoding got: %v", err)}
	}
	wantTokens, err := confDecodeTokens(wantBytes)
	if err != nil {
		return []string{fmt.Sprintf("decoding want: %v", err)}
	}
	var diffs []string
	minLen := min(len(gotTokens), len(wantTokens))
	for i := range minLen {
		gt, wt := gotTokens[i], wantTokens[i]
		if reflect.TypeOf(gt) != reflect.TypeOf(wt) || gt != wt {
			diffs = append(diffs, fmt.Sprintf("token %d: got %#v, want %#v", i, gt, wt))
		}
	}
	if len(gotTokens) != len(wantTokens) {
		diffs = append(diffs, fmt.Sprintf("token count mismatch: got %d, want %d", len(gotTokens), len(wantTokens)))
	}
	return diffs
}

func confParseErrLines(data []byte) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func confExtractErrors(el *ErrorList) []string {
	var res []string
	for _, e := range el.Errors {
		if e != nil {
			res = append(res, fmt.Sprintf("%s %d:%d", e.Code, e.Pos.Line, e.Pos.Col))
		}
	}
	return res
}

func TestConformanceValidAndFromJSON(t *testing.T) {
	fixtures := confFindPairFixtures(t, filepath.Join("testdata", "conformance", "valid"), ".lkal", ".json")
	if len(fixtures) == 0 {
		t.Fatal("no valid fixtures found")
	}
	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.srcPath)
			if err != nil {
				t.Fatal(err)
			}
			wantBytes, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(fix.name, src)
			if err != nil || f == nil {
				t.Fatalf("%s: Parse failed: %v", fix.name, err)
			}
			if f.Version() != "0.1" {
				t.Errorf("%s: Version() = %q, want 0.1", fix.name, f.Version())
			}
			gotBytes, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("%s: ToJSON failed: %v", fix.name, err)
			}
			if diffs := confDiffJSON(gotBytes, wantBytes); len(diffs) > 0 {
				t.Errorf("%s: ToJSON diff:\n%s", fix.name, strings.Join(diffs, "\n"))
			}

			jsonFile, err := FromJSON(wantBytes)
			if err != nil || jsonFile == nil {
				t.Fatalf("%s: FromJSON failed: %v", fix.name, err)
			}
			if !Equal(jsonFile.Data(), f.Data()) {
				t.Errorf("%s: FromJSON Data != Parse Data", fix.name)
			}
		})
	}
}

func TestConformancePlain(t *testing.T) {
	fixtures := confFindPairFixtures(t, filepath.Join("testdata", "conformance", "plain"), ".lkal", ".json")
	if len(fixtures) == 0 {
		t.Fatal("no plain fixtures found")
	}
	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.srcPath)
			if err != nil {
				t.Fatal(err)
			}
			wantBytes, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(fix.name, src)
			if err != nil {
				t.Fatalf("%s: Parse failed: %v", fix.name, err)
			}
			gotBytes, err := ToJSON(f, true)
			if err != nil {
				t.Fatalf("%s: ToJSON(plain) failed: %v", fix.name, err)
			}
			if diffs := confDiffJSON(gotBytes, wantBytes); len(diffs) > 0 {
				t.Errorf("%s: plain diff:\n%s", fix.name, strings.Join(diffs, "\n"))
			}
		})
	}
}

func TestConformanceInvalidAndJSONInvalid(t *testing.T) {
	lkalInvalid := confFindPairFixtures(t, filepath.Join("testdata", "conformance", "invalid"), ".lkal", ".err")
	for _, fix := range lkalInvalid {
		t.Run("lkal/"+fix.name, func(t *testing.T) {
			src, _ := os.ReadFile(fix.srcPath)
			errData, _ := os.ReadFile(fix.dataPath)
			_, parseErr := Parse(fix.name, src)
			el, ok := parseErr.(*ErrorList)
			if !ok {
				t.Fatalf("%s: expected *ErrorList, got %v", fix.name, parseErr)
			}
			if got, want := confExtractErrors(el), confParseErrLines(errData); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: got %v, want %v", fix.name, got, want)
			}
		})
	}

	jsonInvalid := confFindPairFixtures(t, filepath.Join("testdata", "conformance", "json-invalid"), ".json", ".err")
	for _, fix := range jsonInvalid {
		t.Run("json/"+fix.name, func(t *testing.T) {
			src, _ := os.ReadFile(fix.srcPath)
			errData, _ := os.ReadFile(fix.dataPath)
			_, fromErr := FromJSON(src)
			el, ok := fromErr.(*ErrorList)
			if !ok {
				t.Fatalf("%s: expected *ErrorList, got %v", fix.name, fromErr)
			}
			if got, want := confExtractErrors(el), confParseErrLines(errData); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: got %v, want %v", fix.name, got, want)
			}
		})
	}
}

func TestPublicFacadeOperations(t *testing.T) {
	var nilFile *File
	if nilFile.Version() != "" || len(nilFile.Data().Fields) != 0 || nilFile.Format() != nil {
		t.Fatalf("unexpected non-empty results on nil *File")
	}

	f, err := Parse("app.lkal", []byte("@lkal \"0.1\"\n\nport = 8080\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	v, err := ParseValue([]byte("9090"))
	if err != nil {
		t.Fatalf("ParseValue failed: %v", err)
	}
	if err := f.Set("port", v); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	got, err := f.Get("port")
	if err != nil || !Equal(got, Int(9090)) {
		t.Fatalf("Get(port) = %v, %v", got, err)
	}
	if string(FormatValue(got)) != "9090" {
		t.Fatalf("FormatValue = %q", FormatValue(got))
	}
	if err := f.Delete("port"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	out, err := Format(f.Format())
	if err != nil || string(out) != "@lkal \"0.1\"\n" {
		t.Fatalf("Format = %q, %v", out, err)
	}
}
