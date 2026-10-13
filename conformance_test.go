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
	lkalPath string
	dataPath string
}

func confFindFixtures(t *testing.T, dir, dataExt string) []confFixture {
	t.Helper()
	var fixtures []confFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".lkal") {
			base := strings.TrimSuffix(path, ".lkal")
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
				lkalPath: path,
				dataPath: dataPath,
			})
		} else if strings.HasSuffix(path, dataExt) {
			lkalPath := strings.TrimSuffix(path, dataExt) + ".lkal"
			if _, statErr := os.Stat(lkalPath); statErr != nil {
				t.Fatalf("missing fixture pair for %s: expected %s (%v)", path, lkalPath, statErr)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("confFindFixtures failed for %s: %v", dir, err)
	}
	return fixtures
}

func confFindJSONFixtures(t *testing.T, dir, dataExt string) []confFixture {
	t.Helper()
	var fixtures []confFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			base := strings.TrimSuffix(path, ".json")
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
				lkalPath: path,
				dataPath: dataPath,
			})
		} else if strings.HasSuffix(path, dataExt) {
			jsonPath := strings.TrimSuffix(path, dataExt) + ".json"
			if _, statErr := os.Stat(jsonPath); statErr != nil {
				t.Fatalf("missing fixture pair for %s: expected %s (%v)", path, jsonPath, statErr)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("confFindJSONFixtures failed for %s: %v", dir, err)
	}
	return fixtures
}

func confDiffFieldOrder(a, b Value) string {
	switch va := a.(type) {
	case Record:
		vb, ok := b.(Record)
		if !ok {
			return fmt.Sprintf("type mismatch: %T vs %T", a, b)
		}
		if len(va.Fields) != len(vb.Fields) {
			return fmt.Sprintf("record field count: got %d, want %d", len(va.Fields), len(vb.Fields))
		}
		for i := range va.Fields {
			if va.Fields[i].Key != vb.Fields[i].Key {
				return fmt.Sprintf("field %d key: got %q, want %q", i, va.Fields[i].Key, vb.Fields[i].Key)
			}
			if d := confDiffFieldOrder(va.Fields[i].Value, vb.Fields[i].Value); d != "" {
				return fmt.Sprintf("field %q -> %s", va.Fields[i].Key, d)
			}
		}
	case Collection:
		vb, ok := b.(Collection)
		if !ok {
			return fmt.Sprintf("type mismatch: %T vs %T", a, b)
		}
		if len(va.Elems) != len(vb.Elems) {
			return fmt.Sprintf("collection elem count: got %d, want %d", len(va.Elems), len(vb.Elems))
		}
		for i := range va.Elems {
			if va.Elems[i].Label != vb.Elems[i].Label {
				return fmt.Sprintf("elem %d label: got %q, want %q", i, va.Elems[i].Label, vb.Elems[i].Label)
			}
			if d := confDiffFieldOrder(va.Elems[i].Body, vb.Elems[i].Body); d != "" {
				return fmt.Sprintf("elem %q -> %s", va.Elems[i].Label, d)
			}
		}
	case List:
		vb, ok := b.(List)
		if !ok {
			return fmt.Sprintf("type mismatch: %T vs %T", a, b)
		}
		if len(va) != len(vb) {
			return fmt.Sprintf("list len: got %d, want %d", len(va), len(vb))
		}
		for i := range va {
			if d := confDiffFieldOrder(va[i], vb[i]); d != "" {
				return fmt.Sprintf("list[%d] -> %s", i, d)
			}
		}
	}
	return ""
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

func confFormatToken(t json.Token) string {
	if t == nil {
		return "null"
	}
	switch v := t.(type) {
	case string:
		return fmt.Sprintf("%T(%q)", v, v)
	case json.Number:
		return fmt.Sprintf("Number(%s)", v.String())
	case json.Delim:
		return fmt.Sprintf("Delim(%c)", rune(v))
	default:
		return fmt.Sprintf("%T(%v)", v, v)
	}
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
	minLen := len(gotTokens)
	if len(wantTokens) < minLen {
		minLen = len(wantTokens)
	}
	for i := 0; i < minLen; i++ {
		gt, wt := gotTokens[i], wantTokens[i]
		if reflect.TypeOf(gt) != reflect.TypeOf(wt) || gt != wt {
			diffs = append(diffs, fmt.Sprintf("token %d: got %s, want %s", i, confFormatToken(gt), confFormatToken(wt)))
		}
	}
	if len(gotTokens) != len(wantTokens) {
		diffs = append(diffs, fmt.Sprintf("token count mismatch: got %d, want %d", len(gotTokens), len(wantTokens)))
	}
	return diffs
}

func confParseErrFile(data []byte) ([]string, error) {
	var lines []string
	raw := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines, nil
}

func confExtractErrors(el *ErrorList) []string {
	if el == nil {
		return nil
	}
	var res []string
	for _, e := range el.Errors {
		if e != nil {
			res = append(res, fmt.Sprintf("%s %d:%d", e.Code, e.Pos.Line, e.Pos.Col))
		}
	}
	return res
}

func TestConformanceValid(t *testing.T) {
	fixtures := confFindFixtures(t, filepath.Join("testdata", "conformance", "valid"), ".json")
	if len(fixtures) == 0 {
		t.Fatal("no valid fixtures found in testdata/conformance/valid")
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.lkalPath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", fix.lkalPath, err)
			}

			wantBytes, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("failed to read expected json %s: %v", fix.dataPath, err)
			}

			f, err := Parse(fix.name, src)
			if err != nil {
				t.Fatalf("%s: Parse returned error: %v", fix.name, err)
			}
			if f == nil {
				t.Fatalf("%s: Parse returned nil File", fix.name)
			}
			if gotVer := f.Version(); gotVer != "0.1" {
				t.Errorf("%s: f.Version() = %q, want %q", fix.name, gotVer, "0.1")
			}

			gotBytes, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("%s: ToJSON failed: %v", fix.name, err)
			}

			diffs := confDiffJSON(gotBytes, wantBytes)
			if len(diffs) > 0 {
				t.Errorf("%s: differences found:\n%s\ngot:\n%s\nwant:\n%s",
					fix.name, strings.Join(diffs, "\n"), string(gotBytes), string(wantBytes))
			}
		})
	}
}

func TestConformanceInvalid(t *testing.T) {
	fixtures := confFindFixtures(t, filepath.Join("testdata", "conformance", "invalid"), ".err")
	if len(fixtures) == 0 {
		t.Fatal("no invalid fixtures found in testdata/conformance/invalid")
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.lkalPath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", fix.lkalPath, err)
			}

			errData, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("failed to read expected err file %s: %v", fix.dataPath, err)
			}

			wantErrors, err := confParseErrFile(errData)
			if err != nil {
				t.Fatalf("%s: failed to parse .err file: %v", fix.name, err)
			}

			f, parseErr := Parse(fix.name, src)
			if parseErr == nil {
				ver := ""
				if f != nil {
					ver = f.Version()
				}
				t.Fatalf("%s: Parse succeeded unexpectedly with version %q", fix.name, ver)
			}
			if f != nil {
				t.Errorf("%s: Parse returned non-nil File on error: %v", fix.name, f)
			}

			errList, ok := parseErr.(*ErrorList)
			if !ok {
				t.Fatalf("%s: Parse returned error of type %T, want *ErrorList: %v", fix.name, parseErr, parseErr)
			}

			gotErrors := confExtractErrors(errList)
			if !reflect.DeepEqual(gotErrors, wantErrors) {
				t.Errorf("%s: error mismatch:\ngot:  %v\nwant: %v", fix.name, gotErrors, wantErrors)
			}
		})
	}
}

func TestConformanceFromJSON(t *testing.T) {
	fixtures := confFindFixtures(t, filepath.Join("testdata", "conformance", "valid"), ".json")
	if len(fixtures) == 0 {
		t.Fatal("no valid fixtures found in testdata/conformance/valid")
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			jsonData, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("failed to read json fixture %s: %v", fix.dataPath, err)
			}
			lkalData, err := os.ReadFile(fix.lkalPath)
			if err != nil {
				t.Fatalf("failed to read lkal fixture %s: %v", fix.lkalPath, err)
			}

			parsedFile, err := Parse(fix.name, lkalData)
			if err != nil {
				t.Fatalf("%s: Parse returned error: %v", fix.name, err)
			}
			if parsedFile == nil {
				t.Fatalf("%s: Parse returned nil File", fix.name)
			}

			jsonFile, err := FromJSON(jsonData)
			if err != nil {
				t.Fatalf("%s: FromJSON returned error: %v", fix.name, err)
			}
			if jsonFile == nil {
				t.Fatalf("%s: FromJSON returned nil File", fix.name)
			}
			if gotVer := jsonFile.Version(); gotVer != "0.1" {
				t.Errorf("%s: jsonFile.Version() = %q, want %q", fix.name, gotVer, "0.1")
			}
			if !Equal(jsonFile.Data(), parsedFile.Data()) {
				t.Errorf("%s: Data mismatch between FromJSON and Parse", fix.name)
			}
			if diff := confDiffFieldOrder(jsonFile.Data(), parsedFile.Data()); diff != "" {
				t.Errorf("%s: field order mismatch: %s", fix.name, diff)
			}
		})
	}
}

func TestConformancePlain(t *testing.T) {
	fixtures := confFindFixtures(t, filepath.Join("testdata", "conformance", "plain"), ".json")
	if len(fixtures) == 0 {
		t.Fatal("no plain fixtures found in testdata/conformance/plain")
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.lkalPath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", fix.lkalPath, err)
			}

			wantBytes, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("failed to read expected json %s: %v", fix.dataPath, err)
			}

			f, err := Parse(fix.name, src)
			if err != nil {
				t.Fatalf("%s: Parse returned error: %v", fix.name, err)
			}
			if f == nil {
				t.Fatalf("%s: Parse returned nil File", fix.name)
			}

			gotBytes, err := ToJSON(f, true)
			if err != nil {
				t.Fatalf("%s: ToJSON(plain=true) failed: %v", fix.name, err)
			}

			diffs := confDiffJSON(gotBytes, wantBytes)
			if len(diffs) > 0 {
				t.Errorf("%s: differences found:\n%s\ngot:\n%s\nwant:\n%s",
					fix.name, strings.Join(diffs, "\n"), string(gotBytes), string(wantBytes))
			}
		})
	}
}

func TestConformanceJSONInvalid(t *testing.T) {
	fixtures := confFindJSONFixtures(t, filepath.Join("testdata", "conformance", "json-invalid"), ".err")
	if len(fixtures) == 0 {
		t.Fatal("no json-invalid fixtures found in testdata/conformance/json-invalid")
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			src, err := os.ReadFile(fix.lkalPath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", fix.lkalPath, err)
			}

			errData, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("failed to read expected err file %s: %v", fix.dataPath, err)
			}

			wantErrors, err := confParseErrFile(errData)
			if err != nil {
				t.Fatalf("%s: failed to parse .err file: %v", fix.name, err)
			}

			f, fromErr := FromJSON(src)
			if fromErr == nil {
				ver := ""
				if f != nil {
					ver = f.Version()
				}
				t.Fatalf("%s: FromJSON succeeded unexpectedly with version %q", fix.name, ver)
			}
			if f != nil {
				t.Errorf("%s: FromJSON returned non-nil File on error: %v", fix.name, f)
			}

			errList, ok := fromErr.(*ErrorList)
			if !ok {
				t.Fatalf("%s: FromJSON returned error of type %T, want *ErrorList: %v", fix.name, fromErr, fromErr)
			}

			gotErrors := confExtractErrors(errList)
			if !reflect.DeepEqual(gotErrors, wantErrors) {
				t.Errorf("%s: error mismatch:\ngot:  %v\nwant: %v", fix.name, gotErrors, wantErrors)
			}
		})
	}
}
