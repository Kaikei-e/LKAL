package lkal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

func confIsSimpleRefSeg(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

func confQuoteRefSeg(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if (r < 0x20 && r != '\t') || r == 0x7f {
				fmt.Fprintf(&b, "\\u{%x}", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func confWriteIndent(buf *bytes.Buffer, indent int) {
	for i := 0; i < indent; i++ {
		buf.WriteString("  ")
	}
}

func confWriteValue(buf *bytes.Buffer, v Value, indent int) {
	switch val := v.(type) {
	case String:
		b, _ := json.Marshal(string(val))
		buf.Write(b)
	case Int:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case Decimal:
		buf.WriteString(string(val))
	case Bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case Enum:
		buf.WriteString(`{"$enum": `)
		b, _ := json.Marshal(string(val))
		buf.Write(b)
		buf.WriteString(`}`)
	case Quantity:
		buf.WriteString(`{"$qty": `)
		b, _ := json.Marshal(val.Number + val.Unit)
		buf.Write(b)
		buf.WriteString(`}`)
	case Ref:
		var segs []string
		for _, seg := range val {
			if confIsSimpleRefSeg(seg) {
				segs = append(segs, seg)
			} else {
				segs = append(segs, confQuoteRefSeg(seg))
			}
		}
		buf.WriteString(`{"$ref": `)
		b, _ := json.Marshal(strings.Join(segs, "/"))
		buf.Write(b)
		buf.WriteString(`}`)
	case List:
		if len(val) == 0 {
			buf.WriteString("[]")
			return
		}
		buf.WriteString("[\n")
		for i, elem := range val {
			confWriteIndent(buf, indent+1)
			confWriteValue(buf, elem, indent+1)
			if i < len(val)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		confWriteIndent(buf, indent)
		buf.WriteString("]")
	case Record:
		if len(val.Fields) == 0 {
			buf.WriteString("{}")
			return
		}
		buf.WriteString("{\n")
		for i, f := range val.Fields {
			confWriteIndent(buf, indent+1)
			kb, _ := json.Marshal(f.Key)
			buf.Write(kb)
			buf.WriteString(": ")
			confWriteValue(buf, f.Value, indent+1)
			if i < len(val.Fields)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		confWriteIndent(buf, indent)
		buf.WriteString("}")
	case Collection:
		if len(val.Elems) == 0 {
			buf.WriteString(`{"$coll": {}}`)
			return
		}
		buf.WriteString("{\n")
		confWriteIndent(buf, indent+1)
		buf.WriteString(`"$coll": {` + "\n")
		for i, elem := range val.Elems {
			confWriteIndent(buf, indent+2)
			lb, _ := json.Marshal(elem.Label)
			buf.Write(lb)
			buf.WriteString(": ")
			confWriteValue(buf, elem.Body, indent+2)
			if i < len(val.Elems)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		confWriteIndent(buf, indent+1)
		buf.WriteString("}\n")
		confWriteIndent(buf, indent)
		buf.WriteString("}")
	default:
		buf.WriteString("null")
	}
}

func confRecordToLKALJSON(version string, rec Record) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	confWriteIndent(&buf, 1)
	buf.WriteString(`"$lkal": `)
	b, _ := json.Marshal(version)
	buf.Write(b)
	if len(rec.Fields) > 0 {
		buf.WriteString(",\n")
		for i, f := range rec.Fields {
			confWriteIndent(&buf, 1)
			kb, _ := json.Marshal(f.Key)
			buf.Write(kb)
			buf.WriteString(": ")
			confWriteValue(&buf, f.Value, 1)
			if i < len(rec.Fields)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
	} else {
		buf.WriteString("\n")
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

func confFileToJSON(f *File) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("nil file")
	}
	return confRecordToLKALJSON(f.Version(), f.Data())
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

			gotBytes, err := confFileToJSON(f)
			if err != nil {
				t.Fatalf("%s: confFileToJSON failed: %v", fix.name, err)
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

func TestConformanceConverter(t *testing.T) {
	tests := []struct {
		name     string
		rec      Record
		wantJSON string
	}{
		{
			name: "string",
			rec: Record{
				Fields: []Field{
					{Key: "str", Value: String("hello")},
				},
			},
			wantJSON: `{"$lkal": "0.1", "str": "hello"}`,
		},
		{
			name: "int",
			rec: Record{
				Fields: []Field{
					{Key: "num", Value: Int(42)},
				},
			},
			wantJSON: `{"$lkal": "0.1", "num": 42}`,
		},
		{
			name: "decimal",
			rec: Record{
				Fields: []Field{
					{Key: "dec", Value: Decimal("0.25")},
				},
			},
			wantJSON: `{"$lkal": "0.1", "dec": 0.25}`,
		},
		{
			name: "bool",
			rec: Record{
				Fields: []Field{
					{Key: "b", Value: Bool(true)},
				},
			},
			wantJSON: `{"$lkal": "0.1", "b": true}`,
		},
		{
			name: "enum",
			rec: Record{
				Fields: []Field{
					{Key: "proto", Value: Enum("tcp")},
				},
			},
			wantJSON: `{"$lkal": "0.1", "proto": {"$enum": "tcp"}}`,
		},
		{
			name: "quantity",
			rec: Record{
				Fields: []Field{
					{Key: "mem", Value: Quantity{Number: "512", Unit: "Mi"}},
				},
			},
			wantJSON: `{"$lkal": "0.1", "mem": {"$qty": "512Mi"}}`,
		},
		{
			name: "ref",
			rec: Record{
				Fields: []Field{
					{Key: "target", Value: Ref{"secret", "db-creds"}},
				},
			},
			wantJSON: `{"$lkal": "0.1", "target": {"$ref": "secret/db-creds"}}`,
		},
		{
			name: "list",
			rec: Record{
				Fields: []Field{
					{Key: "items", Value: List{Int(1), Int(2)}},
				},
			},
			wantJSON: `{"$lkal": "0.1", "items": [1, 2]}`,
		},
		{
			name: "record",
			rec: Record{
				Fields: []Field{
					{
						Key: "sub",
						Value: Record{
							Fields: []Field{
								{Key: "leaf", Value: String("ok")},
							},
						},
					},
				},
			},
			wantJSON: `{"$lkal": "0.1", "sub": {"leaf": "ok"}}`,
		},
		{
			name: "collection",
			rec: Record{
				Fields: []Field{
					{
						Key: "port",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "http",
									Body: Record{
										Fields: []Field{
											{Key: "number", Value: Int(80)},
										},
									},
								},
							},
						},
					},
				},
			},
			wantJSON: `{"$lkal": "0.1", "port": {"$coll": {"http": {"number": 80}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := confRecordToLKALJSON("0.1", tt.rec)
			if err != nil {
				t.Fatalf("confRecordToLKALJSON failed: %v", err)
			}
			diffs := confDiffJSON(gotBytes, []byte(tt.wantJSON))
			if len(diffs) > 0 {
				t.Errorf("differences found:\n%s\ngot:\n%s\nwant:\n%s",
					strings.Join(diffs, "\n"), string(gotBytes), tt.wantJSON)
			}
		})
	}
}
