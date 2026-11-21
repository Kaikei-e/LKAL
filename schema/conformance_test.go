package schema

import (
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Kaikei-e/LKAL"
)

type schemaValidFixture struct {
	name       string
	schemaPath string
	dataPath   string
}

type schemaInvalidFixture struct {
	name       string
	schemaPath string
	dataPath   string
	errPath    string
}

type schemaCompileInvalidFixture struct {
	name       string
	schemaPath string
	errPath    string
}

type codePathPair struct {
	code string
	path string
}

func schemaConfFindValid(t *testing.T, dir string) []schemaValidFixture {
	t.Helper()
	var fixtures []schemaValidFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".schema.lkal"):
			base := strings.TrimSuffix(path, ".schema.lkal")
			dataPath := base + ".lkal"
			if _, statErr := os.Stat(dataPath); statErr != nil {
				t.Fatalf("missing data fixture for %s: expected %s (%v)", path, dataPath, statErr)
			}
			rel, relErr := filepath.Rel(dir, base)
			if relErr != nil {
				rel = base
			}
			fixtures = append(fixtures, schemaValidFixture{
				name:       filepath.ToSlash(rel),
				schemaPath: path,
				dataPath:   dataPath,
			})
		case strings.HasSuffix(path, ".lkal"):
			base := strings.TrimSuffix(path, ".lkal")
			schemaPath := base + ".schema.lkal"
			if _, statErr := os.Stat(schemaPath); statErr != nil {
				t.Fatalf("missing schema fixture for %s: expected %s (%v)", path, schemaPath, statErr)
			}
		default:
			t.Fatalf("unexpected file in %s: %s", dir, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("schemaConfFindValid failed for %s: %v", dir, err)
	}
	return fixtures
}

func schemaConfFindInvalid(t *testing.T, dir string) []schemaInvalidFixture {
	t.Helper()
	var fixtures []schemaInvalidFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".schema.lkal"):
			base := strings.TrimSuffix(path, ".schema.lkal")
			dataPath := base + ".lkal"
			errPath := base + ".err"
			if _, statErr := os.Stat(dataPath); statErr != nil {
				t.Fatalf("missing data fixture for %s: expected %s (%v)", path, dataPath, statErr)
			}
			if _, statErr := os.Stat(errPath); statErr != nil {
				t.Fatalf("missing err fixture for %s: expected %s (%v)", path, errPath, statErr)
			}
			rel, relErr := filepath.Rel(dir, base)
			if relErr != nil {
				rel = base
			}
			fixtures = append(fixtures, schemaInvalidFixture{
				name:       filepath.ToSlash(rel),
				schemaPath: path,
				dataPath:   dataPath,
				errPath:    errPath,
			})
		case strings.HasSuffix(path, ".lkal"):
			base := strings.TrimSuffix(path, ".lkal")
			if _, statErr := os.Stat(base + ".schema.lkal"); statErr != nil {
				t.Fatalf("orphan data fixture %s: missing .schema.lkal (%v)", path, statErr)
			}
			if _, statErr := os.Stat(base + ".err"); statErr != nil {
				t.Fatalf("orphan data fixture %s: missing .err (%v)", path, statErr)
			}
		case strings.HasSuffix(path, ".err"):
			base := strings.TrimSuffix(path, ".err")
			if _, statErr := os.Stat(base + ".schema.lkal"); statErr != nil {
				t.Fatalf("orphan err fixture %s: missing .schema.lkal (%v)", path, statErr)
			}
			if _, statErr := os.Stat(base + ".lkal"); statErr != nil {
				t.Fatalf("orphan err fixture %s: missing .lkal (%v)", path, statErr)
			}
		default:
			t.Fatalf("unexpected file in %s: %s", dir, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("schemaConfFindInvalid failed for %s: %v", dir, err)
	}
	return fixtures
}

func schemaConfFindSchemaInvalid(t *testing.T, dir string) []schemaCompileInvalidFixture {
	t.Helper()
	var fixtures []schemaCompileInvalidFixture
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".schema.lkal"):
			base := strings.TrimSuffix(path, ".schema.lkal")
			errPath := base + ".err"
			if _, statErr := os.Stat(errPath); statErr != nil {
				t.Fatalf("missing err fixture for %s: expected %s (%v)", path, errPath, statErr)
			}
			rel, relErr := filepath.Rel(dir, base)
			if relErr != nil {
				rel = base
			}
			fixtures = append(fixtures, schemaCompileInvalidFixture{
				name:       filepath.ToSlash(rel),
				schemaPath: path,
				errPath:    errPath,
			})
		case strings.HasSuffix(path, ".err"):
			base := strings.TrimSuffix(path, ".err")
			schemaPath := base + ".schema.lkal"
			if _, statErr := os.Stat(schemaPath); statErr != nil {
				t.Fatalf("orphan err fixture %s: missing %s (%v)", path, schemaPath, statErr)
			}
		default:
			t.Fatalf("unexpected file in %s: %s", dir, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("schemaConfFindSchemaInvalid failed for %s: %v", dir, err)
	}
	return fixtures
}

func schemaConfParseErrFile(data []byte) []string {
	var lines []string
	raw := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func schemaConfExtractErrors(el *lkal.ErrorList) []string {
	if el == nil {
		return nil
	}
	res := make([]string, 0, len(el.Errors))
	for _, e := range el.Errors {
		if e == nil {
			continue
		}
		if e.Path != "" {
			res = append(res, fmt.Sprintf("%s %d:%d %s", e.Code, e.Pos.Line, e.Pos.Col, e.Path))
		} else {
			res = append(res, fmt.Sprintf("%s %d:%d", e.Code, e.Pos.Line, e.Pos.Col))
		}
	}
	return res
}

func extractCodePathSlice(t *testing.T, err error) []codePathPair {
	t.Helper()
	if err == nil {
		return nil
	}
	var el *lkal.ErrorList
	if !errors.As(err, &el) || el == nil {
		t.Fatalf("expected *lkal.ErrorList, got %T: %v", err, err)
	}
	out := make([]codePathPair, 0, len(el.Errors))
	for _, e := range el.Errors {
		if e == nil {
			t.Fatalf("nil *lkal.Error in ErrorList")
		}
		out = append(out, codePathPair{code: e.Code, path: e.Path})
	}
	return out
}

func extractCodePathSorted(t *testing.T, err error) []codePathPair {
	t.Helper()
	out := extractCodePathSlice(t, err)
	sort.Slice(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}
		return out[i].code < out[j].code
	})
	return out
}

func TestSchemaConformanceValid(t *testing.T) {
	dir := filepath.Join("..", "testdata", "schema", "valid")
	fixtures := schemaConfFindValid(t, dir)
	if len(fixtures) == 0 {
		t.Fatalf("no valid schema fixtures found in %s", dir)
	}

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema %s: %v", fix.schemaPath, err)
			}
			dataSrc, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("read data %s: %v", fix.dataPath, err)
			}

			// Canonical format invariant on all valid fixtures.
			fmtSchema, err := lkal.Format(schemaSrc)
			if err != nil {
				t.Fatalf("%s: lkal.Format(schema) failed: %v", fix.name, err)
			}
			if !bytes.Equal(fmtSchema, schemaSrc) {
				t.Errorf("%s: schema fixture is not in canonical lkal.Format form\n--- got ---\n%s\n--- want ---\n%s",
					fix.name, schemaSrc, fmtSchema)
			}

			fmtData, err := lkal.Format(dataSrc)
			if err != nil {
				t.Fatalf("%s: lkal.Format(data) failed: %v", fix.name, err)
			}
			if !bytes.Equal(fmtData, dataSrc) {
				t.Errorf("%s: data fixture is not in canonical lkal.Format form\n--- got ---\n%s\n--- want ---\n%s",
					fix.name, dataSrc, fmtData)
			}

			s, err := Compile(fix.name+".schema.lkal", schemaSrc)
			if err != nil {
				t.Fatalf("%s: Compile failed:\n%v", fix.name, err)
			}
			if s == nil {
				t.Fatalf("%s: Compile returned nil Schema", fix.name)
			}

			f, err := lkal.Parse(fix.name+".lkal", dataSrc)
			if err != nil {
				t.Fatalf("%s: Parse failed:\n%v", fix.name, err)
			}
			if err := s.Validate(f); err != nil {
				t.Errorf("%s: Validate returned unexpected error:\n%v", fix.name, err)
			}
			if err := s.ValidateValue(f.Data()); err != nil {
				t.Errorf("%s: ValidateValue returned unexpected error:\n%v", fix.name, err)
			}
		})
	}
}

func TestSchemaConformanceInvalid(t *testing.T) {
	dir := filepath.Join("..", "testdata", "schema", "invalid")
	fixtures := schemaConfFindInvalid(t, dir)
	if len(fixtures) == 0 {
		t.Fatalf("no invalid schema fixtures found in %s", dir)
	}

	seenCodes := make(map[string]bool)

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema %s: %v", fix.schemaPath, err)
			}
			dataSrc, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("read data %s: %v", fix.dataPath, err)
			}
			errSrc, err := os.ReadFile(fix.errPath)
			if err != nil {
				t.Fatalf("read err %s: %v", fix.errPath, err)
			}

			wantErrors := schemaConfParseErrFile(errSrc)
			if len(wantErrors) == 0 {
				t.Fatalf("%s: .err file has no expected errors", fix.name)
			}

			s, err := Compile(fix.name+".schema.lkal", schemaSrc)
			if err != nil {
				t.Fatalf("%s: Compile failed unexpectedly:\n%v", fix.name, err)
			}

			f, err := lkal.Parse(fix.name+".lkal", dataSrc)
			if err != nil {
				t.Fatalf("%s: Parse failed unexpectedly:\n%v", fix.name, err)
			}

			valErr := s.Validate(f)
			if valErr == nil {
				t.Fatalf("%s: Validate succeeded unexpectedly; want %v", fix.name, wantErrors)
			}
			var errList *lkal.ErrorList
			if !errors.As(valErr, &errList) || errList == nil {
				t.Fatalf("%s: Validate returned %T, want *lkal.ErrorList: %v", fix.name, valErr, valErr)
			}

			for i, e := range errList.Errors {
				if e == nil {
					t.Fatalf("%s: nil error at index %d", fix.name, i)
				}
				if e.File != fix.name+".lkal" {
					t.Errorf("%s: error[%d].File = %q, want %q", fix.name, i, e.File, fix.name+".lkal")
				}
				if e.Msg == "" || e.Hint == "" {
					t.Errorf("%s: error[%d] missing Msg or Hint: %+v", fix.name, i, e)
				}
				seenCodes[e.Code] = true
			}

			gotErrors := schemaConfExtractErrors(errList)
			if !reflect.DeepEqual(gotErrors, wantErrors) {
				t.Errorf("%s: error mismatch:\ngot:\n  %s\nwant:\n  %s",
					fix.name, strings.Join(gotErrors, "\n  "), strings.Join(wantErrors, "\n  "))
			}

			if fix.name == "multi_five_errors" {
				if len(errList.Errors) != 5 {
					t.Errorf("multi_five_errors: expected 5 errors in one pass, got %d", len(errList.Errors))
				}
				foundDidYouMean := false
				for _, e := range errList.Errors {
					if e.Code == "L503" && e.Path == "replica" && strings.Contains(e.Hint, `did you mean "replicas"?`) {
						foundDidYouMean = true
					}
				}
				if !foundDidYouMean {
					t.Errorf("multi_five_errors: expected did-you-mean hint for replica, got %v", errList)
				}
			}
		})
	}

	for code := 501; code <= 510; code++ {
		c := fmt.Sprintf("L%d", code)
		if !seenCodes[c] {
			t.Errorf("data validation error code %s is not covered by any fixture in %s", c, dir)
		}
	}
}

func TestSchemaConformanceSchemaInvalid(t *testing.T) {
	dir := filepath.Join("..", "testdata", "schema", "schema-invalid")
	fixtures := schemaConfFindSchemaInvalid(t, dir)
	if len(fixtures) == 0 {
		t.Fatalf("no schema-invalid fixtures found in %s", dir)
	}

	seenCodes := make(map[string]bool)

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema %s: %v", fix.schemaPath, err)
			}
			errSrc, err := os.ReadFile(fix.errPath)
			if err != nil {
				t.Fatalf("read err %s: %v", fix.errPath, err)
			}

			wantErrors := schemaConfParseErrFile(errSrc)
			if len(wantErrors) == 0 {
				t.Fatalf("%s: .err file has no expected errors", fix.name)
			}

			s, compErr := Compile(fix.name+".schema.lkal", schemaSrc)
			if compErr == nil {
				t.Fatalf("%s: Compile succeeded unexpectedly; want %v", fix.name, wantErrors)
			}
			if s != nil {
				t.Errorf("%s: Compile returned non-nil *Schema on error", fix.name)
			}
			var errList *lkal.ErrorList
			if !errors.As(compErr, &errList) || errList == nil {
				t.Fatalf("%s: Compile returned %T, want *lkal.ErrorList: %v", fix.name, compErr, compErr)
			}

			for i, e := range errList.Errors {
				if e == nil {
					t.Fatalf("%s: nil error at index %d", fix.name, i)
				}
				if e.File != fix.name+".schema.lkal" {
					t.Errorf("%s: error[%d].File = %q, want %q", fix.name, i, e.File, fix.name+".schema.lkal")
				}
				if e.Msg == "" || e.Hint == "" {
					t.Errorf("%s: error[%d] missing Msg or Hint: %+v", fix.name, i, e)
				}
				seenCodes[e.Code] = true
			}

			gotErrors := schemaConfExtractErrors(errList)
			if !reflect.DeepEqual(gotErrors, wantErrors) {
				t.Errorf("%s: error mismatch:\ngot:\n  %s\nwant:\n  %s",
					fix.name, strings.Join(gotErrors, "\n  "), strings.Join(wantErrors, "\n  "))
			}
		})
	}

	for code := 601; code <= 607; code++ {
		c := fmt.Sprintf("L%d", code)
		if !seenCodes[c] {
			t.Errorf("schema compile error code %s is not covered by any fixture in %s", c, dir)
		}
	}
}

type fixturePair struct {
	name       string
	schemaPath string
	dataPath   string
}

func allSchemaDataFixtures(t *testing.T) []fixturePair {
	t.Helper()
	var all []fixturePair
	for _, vf := range schemaConfFindValid(t, filepath.Join("..", "testdata", "schema", "valid")) {
		all = append(all, fixturePair{name: "valid/" + vf.name, schemaPath: vf.schemaPath, dataPath: vf.dataPath})
	}
	for _, iv := range schemaConfFindInvalid(t, filepath.Join("..", "testdata", "schema", "invalid")) {
		all = append(all, fixturePair{name: "invalid/" + iv.name, schemaPath: iv.schemaPath, dataPath: iv.dataPath})
	}
	return all
}

func TestPropertyFormattingInvariance(t *testing.T) {
	for _, fix := range allSchemaDataFixtures(t) {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			dataSrc, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("read data: %v", err)
			}

			s, err := Compile(fix.name+".schema.lkal", schemaSrc)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			f, err := lkal.Parse(fix.name+".lkal", dataSrc)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			want := extractCodePathSlice(t, s.Validate(f))

			fFmt, err := lkal.Parse(fix.name+".lkal", f.Format())
			if err != nil {
				t.Fatalf("Parse(Format) failed: %v", err)
			}
			got := extractCodePathSlice(t, s.Validate(fFmt))

			if !slices.Equal(got, want) {
				t.Errorf("Validate(Parse(Format)) != Validate(f):\ngot:  %v\nwant: %v", got, want)
			}
		})
	}

	// Non-canonical inputs (inline blocks, extra blank lines, CRLF, comments).
	t.Run("non_canonical_inputs", func(t *testing.T) {
		s := mustCompile(t, `@lkal "0.1"
lkal-schema = "0.2"
root {
  field "name" { required = true, type = .string }
  field "replicas" { type = .int, min = 1, max = 10 }
  field "tags" { type = .list, unique = true, items { type = .string } }
}
`)
		raw := "### Header\r\n@lkal \"0.1\"\r\n\r\n\r\nreplicas = 20, tags = [\"a\", \"a\",], extra = 1\r\n"
		f, err := lkal.Parse("messy.lkal", []byte(raw))
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		fFmt, err := lkal.Parse("messy.lkal", f.Format())
		if err != nil {
			t.Fatalf("Parse(Format) failed: %v", err)
		}
		want := extractCodePathSlice(t, s.Validate(f))
		got := extractCodePathSlice(t, s.Validate(fFmt))
		if !slices.Equal(got, want) {
			t.Errorf("non-canonical Validate(Parse(Format)) != Validate(f):\ngot:  %v\nwant: %v", got, want)
		}
	})
}

func TestPropertyJSONRoundTripInvariance(t *testing.T) {
	for _, fix := range allSchemaDataFixtures(t) {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			dataSrc, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("read data: %v", err)
			}

			s, err := Compile(fix.name+".schema.lkal", schemaSrc)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			f, err := lkal.Parse(fix.name+".lkal", dataSrc)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			wantSet := extractCodePathSorted(t, s.Validate(f))

			jsonBytes, err := lkal.ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}
			jf, err := lkal.FromJSON(jsonBytes)
			if err != nil {
				t.Fatalf("FromJSON failed: %v", err)
			}

			gotValSet := extractCodePathSorted(t, s.ValidateValue(jf.Data()))
			if !slices.Equal(gotValSet, wantSet) {
				t.Errorf("ValidateValue(FromJSON(ToJSON).Data()) != Validate(f):\ngot:  %v\nwant: %v", gotValSet, wantSet)
			}

			gotFileSlice := extractCodePathSlice(t, s.Validate(jf))
			gotValSlice := extractCodePathSlice(t, s.ValidateValue(jf.Data()))
			if !slices.Equal(gotFileSlice, gotValSlice) {
				t.Errorf("Validate(FromJSON) != ValidateValue(FromJSON.Data()):\ngot:  %v\nwant: %v", gotFileSlice, gotValSlice)
			}
		})
	}
}

func TestPropertyLabelSugarInvariance(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"

lkal-schema = "0.2"

root {
  field "service" {
    type = .map
    key { pattern = "[a-z][a-z0-9-]*" }
    value {
      type = .record
      field "port" {
        type = .map
        key { pattern = "[a-z]+" }
        value {
          type = .record
          field "number" { required = true, type = .int, min = 1, max = 65535 }
          field "protocol" { type = .enum, values = [.tcp, .udp], default = .tcp }
        }
      }
      field "labels" {
        type = .map
        value { type = .string }
      }
      field "plugin" {
        type = .map
        unique-by = ["order"]
        value {
          type = .record
          field "order" { required = true, type = .int }
        }
      }
    }
  }
}
`)

	cases := []struct {
		name    string
		sugar   string
		nested  string
		wantErr bool
	}{
		{
			name: "valid_service_port",
			sugar: `@lkal "0.1"
service "web" {
  port "http" {
    number = 8080
  }
}
`,
			nested: `@lkal "0.1"
service {
  web {
    port {
      http {
        number = 8080
      }
    }
  }
}
`,
			wantErr: false,
		},
		{
			name: "valid_label_assign",
			sugar: `@lkal "0.1"
service "web" {
  labels "env" = "prod"
  labels "tier" = "edge"
}
`,
			nested: `@lkal "0.1"
service {
  web {
    labels {
      env = "prod"
      tier = "edge"
    }
  }
}
`,
			wantErr: false,
		},
		{
			name: "invalid_multiple_codes",
			sugar: `@lkal "0.1"
service "web" {
  port "http" {
    number = 70000
    protocol = .http
    extra = true
  }
  port "BAD" {}
  plugin "a" { order = 1 }
  plugin "b" { order = 1 }
}
`,
			nested: `@lkal "0.1"
service {
  web {
    port {
      http {
        number = 70000
        protocol = .http
        extra = true
      }
      BAD {}
    }
    plugin {
      a { order = 1 }
      b { order = 1 }
    }
  }
}
`,
			wantErr: true,
		},
		{
			name: "invalid_multi_label_compressed",
			sugar: `@lkal "0.1"
service "BAD_SVC" "port" "http" {
  number = "not-int"
}
`,
			nested: `@lkal "0.1"
service {
  BAD_SVC {
    port {
      http {
        number = "not-int"
      }
    }
  }
}
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fSugar := mustParse(t, "sugar.lkal", tc.sugar)
			fNested := mustParse(t, "nested.lkal", tc.nested)

			if !lkal.Equal(fSugar.Data(), fNested.Data()) {
				t.Fatalf("data models differ between label-sugar and nested braces")
			}

			errSugar := s.Validate(fSugar)
			errNested := s.Validate(fNested)
			if tc.wantErr {
				if errSugar == nil || errNested == nil {
					t.Fatalf("expected errors for both forms, got sugar=%v nested=%v", errSugar, errNested)
				}
			} else if errSugar != nil || errNested != nil {
				t.Fatalf("expected nil errors, got sugar=%v nested=%v", errSugar, errNested)
			}

			gotSugar := extractCodePathSlice(t, errSugar)
			gotNested := extractCodePathSlice(t, errNested)
			if !slices.Equal(gotSugar, gotNested) {
				t.Errorf("(Code, Path) mismatch between label-sugar and nested braces:\nsugar:  %v\nnested: %v",
					gotSugar, gotNested)
			}
		})
	}
}

func TestPropertyValidateNonMutation(t *testing.T) {
	for _, fix := range allSchemaDataFixtures(t) {
		t.Run(fix.name, func(t *testing.T) {
			schemaSrc, err := os.ReadFile(fix.schemaPath)
			if err != nil {
				t.Fatalf("read schema: %v", err)
			}
			dataSrc, err := os.ReadFile(fix.dataPath)
			if err != nil {
				t.Fatalf("read data: %v", err)
			}

			s, err := Compile(fix.name+".schema.lkal", schemaSrc)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			f, err := lkal.Parse(fix.name+".lkal", dataSrc)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			fmtBefore := slices.Clone(f.Format())
			jsonBefore, err := lkal.ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON before failed: %v", err)
			}
			dataBefore := f.Data()
			dataBeforeText := slices.Clone(lkal.FormatValue(dataBefore))

			_ = s.Validate(f)
			_ = s.ValidateValue(dataBefore)

			fmtAfter := f.Format()
			if !bytes.Equal(fmtAfter, fmtBefore) {
				t.Errorf("Validate mutated f.Format():\nbefore:\n%s\nafter:\n%s", fmtBefore, fmtAfter)
			}
			jsonAfter, err := lkal.ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON after failed: %v", err)
			}
			if !bytes.Equal(jsonAfter, jsonBefore) {
				t.Errorf("Validate mutated ToJSON(f, false):\nbefore:\n%s\nafter:\n%s", jsonBefore, jsonAfter)
			}
			if !bytes.Equal(lkal.FormatValue(dataBefore), dataBeforeText) {
				t.Errorf("ValidateValue mutated input lkal.Value")
			}
		})
	}
}

func TestCoreDoesNotImportSchema(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "*.go"))
	if err != nil {
		t.Fatalf("glob ../*.go failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one .go file in root package lkal")
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		astFile, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) failed: %v", path, err)
		}
		for _, impSpec := range astFile.Imports {
			impPath, err := strconv.Unquote(impSpec.Path.Value)
			if err != nil {
				t.Fatalf("%s: invalid import literal %s: %v", path, impSpec.Path.Value, err)
			}
			if impPath == "github.com/Kaikei-e/LKAL/schema" || strings.HasPrefix(impPath, "github.com/Kaikei-e/LKAL/schema/") {
				t.Errorf("root package file %s illegally imports schema package %q", path, impPath)
			}
			firstSeg, _, _ := strings.Cut(impPath, "/")
			if strings.Contains(firstSeg, ".") {
				t.Errorf("root package file %s imports non-stdlib package %q", path, impPath)
			}
		}
	}
}

func TestPropertyDeclarativeDefaults(t *testing.T) {
	s := mustCompile(t, `@lkal "0.1"

lkal-schema = "0.2"

root {
  field "name" { required = true, type = .string }
  field "replicas" { type = .int, min = 1, max = 10, default = 1 }
  field "tags" {
    type = .list
    items { type = .string }
    default = ["default"]
  }
  field "service" {
    type = .map
    value {
      type = .record
      field "image" { required = true, type = .string }
      field "timeout" { type = .quantity, units = ["s", "m"], default = 30s }
      field "protocol" { type = .enum, values = [.tcp, .udp], default = .tcp }
      field "tls" {
        type = .record
        field "enabled" { type = .bool, default = true }
      }
    }
  }
}
`)

	// Default(path) returns deep-cloned values.
	defTags, ok := s.Default("tags")
	if !ok {
		t.Fatalf("Default(tags) returned false")
	}
	if lst, isList := defTags.(lkal.List); !isList || len(lst) != 1 || lst[0] != lkal.String("default") {
		t.Fatalf("Default(tags) = %#v, want [\"default\"]", defTags)
	} else {
		lst[0] = lkal.String("mutated")
	}
	defTags2, ok := s.Default("tags")
	if !ok || !lkal.Equal(defTags2, lkal.List{lkal.String("default")}) {
		t.Fatalf("mutating Default(tags) return value polluted schema default: %#v", defTags2)
	}

	// Incomplete record: omits replicas, tags, service/web/timeout, service/web/protocol, and absent parent service/web/tls.
	incomplete := lkal.Record{
		Fields: []lkal.Field{
			{Key: "name", Value: lkal.String("demo")},
			{
				Key: "service",
				Value: lkal.Record{
					Fields: []lkal.Field{
						{
							Key: "web",
							Value: lkal.Record{
								Fields: []lkal.Field{
									{Key: "image", Value: lkal.String("app:1")},
								},
							},
						},
					},
				},
			},
		},
	}

	inputSnapshot := slices.Clone(lkal.FormatValue(incomplete))

	// Valid before Complete, and Equal(incomplete, completed) MUST be false.
	if err := s.ValidateValue(incomplete); err != nil {
		t.Fatalf("ValidateValue(incomplete) failed: %v", err)
	}

	c1, err := s.Complete(incomplete)
	if err != nil {
		t.Fatalf("Complete(incomplete) failed: %v", err)
	}

	// Input must not be mutated.
	if !bytes.Equal(lkal.FormatValue(incomplete), inputSnapshot) {
		t.Errorf("Complete mutated its input Value:\nbefore: %s\nafter:  %s", inputSnapshot, lkal.FormatValue(incomplete))
	}

	// Principle 6: schema-free Equal distinguishes omitted defaulted field from completed value.
	if lkal.Equal(incomplete, c1) {
		t.Errorf("lkal.Equal(incomplete, Complete(incomplete)) = true, want false")
	}

	// Completed value must satisfy ValidateValue.
	if err := s.ValidateValue(c1); err != nil {
		t.Errorf("ValidateValue(Complete(incomplete)) failed: %v", err)
	}

	// Missing parent record `tls` must NOT be implicitly created.
	c1Rec := c1.(lkal.Record)
	svcRec := findTestField(t, c1Rec, "service").(lkal.Record)
	webRec := findTestField(t, svcRec, "web").(lkal.Record)
	if _, hasTLS := findField(webRec, "tls"); hasTLS {
		t.Errorf("Complete implicitly created missing parent record service/web/tls")
	}

	// Idempotence: Complete(Complete(v)) == Complete(v).
	c2, err := s.Complete(c1)
	if err != nil {
		t.Fatalf("second Complete failed: %v", err)
	}
	if !lkal.Equal(c2, c1) {
		t.Errorf("Complete is not idempotent:\nc1: %s\nc2: %s", lkal.FormatValue(c1), lkal.FormatValue(c2))
	}

	// Mutating c1 in-place must not affect input or future Complete calls.
	c1Tags := findTestField(t, c1Rec, "tags").(lkal.List)
	c1Tags[0] = lkal.String("corrupted")
	c3, err := s.Complete(incomplete)
	if err != nil {
		t.Fatalf("third Complete failed: %v", err)
	}
	if !lkal.Equal(c3, c2) {
		t.Errorf("mutating Complete output affected subsequent Complete call")
	}
	if !bytes.Equal(lkal.FormatValue(incomplete), inputSnapshot) {
		t.Errorf("mutating Complete output aliased original input")
	}

	// Also verify declarative defaults property on service_full valid fixture.
	sfSchemaBytes, err := os.ReadFile(filepath.Join("..", "testdata", "schema", "valid", "service_full.schema.lkal"))
	if err != nil {
		t.Fatalf("read service_full schema: %v", err)
	}
	sfDataBytes, err := os.ReadFile(filepath.Join("..", "testdata", "schema", "valid", "service_full.lkal"))
	if err != nil {
		t.Fatalf("read service_full data: %v", err)
	}
	sfSchema, err := Compile("service_full.schema.lkal", sfSchemaBytes)
	if err != nil {
		t.Fatalf("Compile service_full failed: %v", err)
	}
	sfFile, err := lkal.Parse("service_full.lkal", sfDataBytes)
	if err != nil {
		t.Fatalf("Parse service_full failed: %v", err)
	}
	sfRaw := sfFile.Data()
	sfC1, err := sfSchema.Complete(sfRaw)
	if err != nil {
		t.Fatalf("Complete(service_full) failed: %v", err)
	}
	if lkal.Equal(sfRaw, sfC1) {
		t.Errorf("service_full worker omits defaulted fields, so Equal(raw, Complete(raw)) should be false")
	}
	if err := sfSchema.ValidateValue(sfC1); err != nil {
		t.Errorf("ValidateValue(Complete(service_full)) failed: %v", err)
	}
	sfC2, err := sfSchema.Complete(sfC1)
	if err != nil {
		t.Fatalf("second Complete(service_full) failed: %v", err)
	}
	if !lkal.Equal(sfC2, sfC1) {
		t.Errorf("Complete(service_full) is not idempotent")
	}
}

func findTestField(t *testing.T, rec lkal.Record, key string) lkal.Value {
	t.Helper()
	v, ok := findField(rec, key)
	if !ok {
		t.Fatalf("field %q not found in record", key)
	}
	return v
}
