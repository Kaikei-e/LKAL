package lkal

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

func parserFuzzIsKnownErrorCode(code string) bool {
	switch code {
	case "L001", "L002", "L003", "L004", "L005", "L006", "L007", "L008", "L009",
		"L101", "L102", "L103", "L104", "L105", "L106", "L107",
		"L201", "L204", "L205", "L207":
		return true
	default:
		return false
	}
}

func valueFuzzIsKnownErrorCode(code string) bool {
	switch code {
	case "L001", "L002", "L003", "L004", "L005", "L006", "L007", "L008", "L009",
		"L101", "L102", "L103", "L104", "L105", "L106", "L107",
		"L201", "L204", "L205":
		return true
	default:
		return false
	}
}

func fuzzSameValue(a, b Value) bool {
	switch x := a.(type) {
	case Record:
		y, ok := b.(Record)
		if !ok || len(x.Fields) != len(y.Fields) {
			return false
		}
		for i := range x.Fields {
			if x.Fields[i].Key != y.Fields[i].Key || !fuzzSameValue(x.Fields[i].Value, y.Fields[i].Value) {
				return false
			}
		}
		return true
	case List:
		y, ok := b.(List)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !fuzzSameValue(x[i], y[i]) {
				return false
			}
		}
		return true
	default:
		return Equal(a, b)
	}
}

func fuzzPermuteRecord(v Value, mode int) Value {
	switch x := v.(type) {
	case Record:
		n := len(x.Fields)
		fields := make([]Field, n)
		for i, f := range x.Fields {
			fields[i] = Field{Key: f.Key, Value: fuzzPermuteRecord(f.Value, mode)}
		}
		if n > 1 {
			switch mode % 3 {
			case 0:
				slices.Reverse(fields)
			case 1:
				first := fields[0]
				copy(fields, fields[1:])
				fields[n-1] = first
			case 2:
				for i := 0; i+1 < n; i += 2 {
					fields[i], fields[i+1] = fields[i+1], fields[i]
				}
			}
		}
		return Record{Fields: fields}
	case List:
		out := make(List, len(x))
		for i, el := range x {
			out[i] = fuzzPermuteRecord(el, mode)
		}
		return out
	case Ref:
		return slices.Clone(x)
	default:
		return v
	}
}

func fuzzCheckEqualLaws(t *testing.T, a, b, c Value) {
	t.Helper()
	if !Equal(a, a) {
		t.Errorf("Equal reflexivity failed on a: %#v", a)
	}
	if !Equal(b, b) {
		t.Errorf("Equal reflexivity failed on b: %#v", b)
	}
	if !Equal(c, c) {
		t.Errorf("Equal reflexivity failed on c: %#v", c)
	}

	ab := Equal(a, b)
	ba := Equal(b, a)
	if ab != ba {
		t.Errorf("Equal symmetry failed: Equal(a, b)=%v != Equal(b, a)=%v\na=%#v\nb=%#v", ab, ba, a, b)
	}

	bc := Equal(b, c)
	cb := Equal(c, b)
	if bc != cb {
		t.Errorf("Equal symmetry failed: Equal(b, c)=%v != Equal(c, b)=%v\nb=%#v\nc=%#v", bc, cb, b, c)
	}

	ac := Equal(a, c)
	ca := Equal(c, a)
	if ac != ca {
		t.Errorf("Equal symmetry failed: Equal(a, c)=%v != Equal(c, a)=%v\na=%#v\nc=%#v", ac, ca, a, c)
	}

	if ab && bc && !ac {
		t.Errorf("Equal transitivity failed: Equal(a, b) && Equal(b, c) but !Equal(a, c)\na=%#v\nb=%#v\nc=%#v", a, b, c)
	}
	if ab && !bc && ac {
		t.Errorf("Equal transitivity failed: Equal(a, b) && !Equal(b, c) but Equal(a, c)\na=%#v\nb=%#v\nc=%#v", a, b, c)
	}
	if !ab && bc && ac {
		t.Errorf("Equal transitivity failed: !Equal(a, b) && Equal(b, c) but Equal(a, c)\na=%#v\nb=%#v\nc=%#v", a, b, c)
	}
}

func fuzzDisjointPaths(p1, p2 []string) bool {
	if len(p1) <= len(p2) && slices.Equal(p1, p2[:len(p1)]) {
		return false
	}
	if len(p2) <= len(p1) && slices.Equal(p2, p1[:len(p2)]) {
		return false
	}
	return true
}

func fuzzValueRecordDepth(v Value) int {
	rec, ok := v.(Record)
	if !ok {
		return 0
	}
	maxChild := 0
	for _, f := range rec.Fields {
		if d := fuzzValueRecordDepth(f.Value); d > maxChild {
			maxChild = d
		}
	}
	return 1 + maxChild
}

func fuzzMutateValueInPlace(v Value) {
	switch val := v.(type) {
	case Ref:
		if len(val) > 0 {
			val[0] = "mutated_seg"
		}
	case List:
		if len(val) > 0 {
			fuzzMutateValueInPlace(val[0])
			val[0] = String("mutated_elem")
		}
	case Record:
		if len(val.Fields) > 0 {
			fuzzMutateValueInPlace(val.Fields[0].Value)
			val.Fields[0].Key = "mutated_key"
		}
	}
}

func fuzzStripStrings(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in {
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				in = false
				b.WriteByte('"')
			}
			continue
		}
		if c == '"' {
			in = true
		}
		b.WriteByte(c)
	}
	return b.String()
}

func fuzzCanonicalLint(out []byte) string {
	s := string(out)
	if strings.Contains(s, "\r") {
		return "contains CR"
	}
	if !strings.HasSuffix(s, "\n") {
		return "does not end with LF"
	}
	if strings.HasSuffix(s, "\n\n") {
		return "ends with more than one LF"
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	decl := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "@lkal") {
			decl = i
			break
		}
	}
	if decl < 0 {
		return "no @lkal line"
	}
	if lines[decl] != `@lkal "0.1"` {
		return fmt.Sprintf("declaration line is %q", lines[decl])
	}
	for i := 0; i < decl; i++ {
		if lines[i] == "" {
			if i == 0 {
				return "blank first line"
			}
			continue
		}
		if !strings.HasPrefix(lines[i], "#") {
			return fmt.Sprintf("line %d before @lkal is not a comment: %q", i+1, lines[i])
		}
	}
	if decl+1 < len(lines) {
		if lines[decl+1] != "" {
			return "no blank line after the @lkal line"
		}
		if decl+2 >= len(lines) {
			return "blank line after @lkal but nothing follows"
		}
	}
	depth := 0
	rawOpen, justOpened := false, false
	prevBlank, prevOpens := false, false
	for i, ln := range lines {
		no := i + 1
		if ln == "" {
			if prevBlank {
				return fmt.Sprintf("line %d: two consecutive blank lines", no)
			}
			if prevOpens {
				return fmt.Sprintf("line %d: blank line right after an opening line", no)
			}
			if rawOpen && justOpened {
				return fmt.Sprintf("line %d: blank line right after a ` =` line", no)
			}
			if rawOpen {
				depth--
				rawOpen = false
			}
			justOpened = false
			prevBlank, prevOpens = true, false
			continue
		}
		trimmed := strings.TrimLeft(ln, " ")
		if trimmed == "" {
			return fmt.Sprintf("line %d: whitespace-only line %q", no, ln)
		}
		indent := len(ln) - len(trimmed)
		isRaw := strings.HasPrefix(trimmed, `\\`)
		isComment := strings.HasPrefix(trimmed, "#")
		if rawOpen && !isRaw {
			if justOpened {
				return fmt.Sprintf("line %d: ` =` line not followed by a \\\\ line", no)
			}
			depth--
			rawOpen = false
		}
		if isRaw && !rawOpen {
			return fmt.Sprintf("line %d: \\\\ line without a preceding ` =` line", no)
		}
		justOpened = false
		if !isRaw && !isComment && (trimmed[0] == '}' || trimmed[0] == ']') {
			if trimmed != "}" && trimmed != "]" {
				return fmt.Sprintf("line %d: closing line with extra text %q", no, ln)
			}
			if prevBlank {
				return fmt.Sprintf("line %d: blank line right before a closing line", no)
			}
			depth--
			if depth < 0 {
				return fmt.Sprintf("line %d: unbalanced closing line", no)
			}
		}
		if indent != 2*depth {
			return fmt.Sprintf("line %d: indentation %d, want %d: %q", no, indent, 2*depth, ln)
		}
		if !isRaw && (strings.HasSuffix(ln, " ") || strings.HasSuffix(ln, "\t")) {
			return fmt.Sprintf("line %d: trailing whitespace: %q", no, ln)
		}
		if isComment && trimmed != "###" && !(strings.HasPrefix(trimmed, "### ") && len(trimmed) > 4) {
			return fmt.Sprintf("line %d: comment not in ### form: %q", no, ln)
		}
		prevOpens = false
		if !isRaw && !isComment {
			st := fuzzStripStrings(trimmed)
			if strings.Contains(st, "\t") {
				return fmt.Sprintf("line %d: tab outside a quoted string: %q", no, ln)
			}
			switch {
			case strings.HasSuffix(st, " {") || strings.HasSuffix(st, " ["):
				depth++
				prevOpens = true
			case strings.HasSuffix(st, " ="):
				depth++
				rawOpen, justOpened = true, true
			}
		}
		prevBlank = false
	}
	if rawOpen {
		if justOpened {
			return "last line is a ` =` line"
		}
		depth--
	}
	if depth != 0 {
		return fmt.Sprintf("unbalanced nesting at end (depth %d)", depth)
	}
	return ""
}

type fuzzKV struct {
	K string
	V any
}

type fuzzObj []fuzzKV

func fuzzWriteJSONTokens(enc *jsontext.Encoder, v any) error {
	switch x := v.(type) {
	case string:
		return enc.WriteToken(jsontext.String(x))
	case bool:
		return enc.WriteToken(jsontext.Bool(x))
	case json.Number:
		return enc.WriteValue(jsontext.Value(x))
	case []any:
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		for _, e := range x {
			if err := fuzzWriteJSONTokens(enc, e); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndArray)
	case fuzzObj:
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}
		for _, kv := range x {
			if err := enc.WriteToken(jsontext.String(kv.K)); err != nil {
				return err
			}
			if err := fuzzWriteJSONTokens(enc, kv.V); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndObject)
	}
	return fmt.Errorf("unexpected type %T", v)
}

func fuzzRecordToJSONObj(r Record, plain bool) fuzzObj {
	o := make(fuzzObj, 0, len(r.Fields))
	for _, f := range r.Fields {
		o = append(o, fuzzKV{K: f.Key, V: fuzzValueToJSONAny(f.Value, plain)})
	}
	return o
}

func fuzzValueToJSONAny(v Value, plain bool) any {
	switch x := v.(type) {
	case String:
		return string(x)
	case Int:
		return json.Number(strconv.FormatInt(int64(x), 10))
	case Decimal:
		return json.Number(string(x))
	case Bool:
		return bool(x)
	case Enum:
		if plain {
			return string(x)
		}
		return fuzzObj{{K: "$enum", V: string(x)}}
	case Quantity:
		s := x.Number + x.Unit
		if plain {
			return s
		}
		return fuzzObj{{K: "$qty", V: s}}
	case Ref:
		s := refBody(x)
		if plain {
			return s
		}
		return fuzzObj{{K: "$ref", V: s}}
	case List:
		out := make([]any, 0, len(x))
		for _, e := range x {
			out = append(out, fuzzValueToJSONAny(e, plain))
		}
		return out
	case Record:
		return fuzzRecordToJSONObj(x, plain)
	}
	return nil
}

func fuzzExpectedJSON(version string, data Record, plain bool) ([]byte, error) {
	top := make(fuzzObj, 0, len(data.Fields)+1)
	if !plain {
		top = append(top, fuzzKV{K: "$lkal", V: version})
	}
	top = append(top, fuzzRecordToJSONObj(data, plain)...)
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf, jsontext.WithIndent("  "))
	if err := fuzzWriteJSONTokens(enc, top); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fuzzHex4(src []byte, i int) (rune, bool) {
	if i < 0 || i+4 > len(src) {
		return 0, false
	}
	v, err := strconv.ParseUint(string(src[i:i+4]), 16, 32)
	if err != nil {
		return 0, false
	}
	return rune(v), true
}

func fuzzHasUnpairedSurrogate(src []byte) bool {
	in := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if !in {
			if c == '"' {
				in = true
			}
			continue
		}
		if c == '"' {
			in = false
			continue
		}
		if c != '\\' {
			continue
		}
		i++
		if i >= len(src) {
			return false
		}
		if src[i] != 'u' {
			continue
		}
		v, ok := fuzzHex4(src, i+1)
		if !ok {
			return false
		}
		i += 4
		if v >= 0xD800 && v <= 0xDBFF {
			if i+6 < len(src) && src[i+1] == '\\' && src[i+2] == 'u' {
				if w, ok := fuzzHex4(src, i+3); ok && w >= 0xDC00 && w <= 0xDFFF {
					i += 6
					continue
				}
			}
			return true
		}
		if v >= 0xDC00 && v <= 0xDFFF {
			return true
		}
	}
	return false
}

func fuzzCheckErrorList(t *testing.T, err error, wantFile string, src []byte, isKnownCode func(string) bool) *ErrorList {
	t.Helper()
	errList, ok := err.(*ErrorList)
	if !ok || errList == nil {
		t.Fatalf("expected non-nil *ErrorList, got %T: %v", err, err)
	}
	if len(errList.Errors) < 1 || len(errList.Errors) > 20 {
		t.Errorf("error count %d not in [1, 20]", len(errList.Errors))
	}
	if errList.Error() == "" {
		t.Errorf("ErrorList.Error() returned empty string")
	}
	maxLine := bytes.Count(src, []byte("\n")) + 1
	lines := bytes.Split(src, []byte("\n"))
	for i, e := range errList.Errors {
		if e == nil {
			t.Fatalf("nil error at index %d", i)
		}
		if e.File != wantFile {
			t.Errorf("error File: got %q, want %q", e.File, wantFile)
		}
		if !isKnownCode(e.Code) {
			t.Errorf("unknown error code: %q", e.Code)
		}
		if e.Msg == "" {
			t.Errorf("error Msg should not be empty for code %q", e.Code)
		}
		if e.Hint == "" {
			t.Errorf("error Hint should not be empty for code %q", e.Code)
		}
		if e.Error() == "" {
			t.Errorf("Error.Error() should not be empty for code %q", e.Code)
		}
		if e.Pos.Line < 1 || e.Pos.Line > maxLine {
			t.Errorf("invalid error Line %d, want in [1, %d]", e.Pos.Line, maxLine)
		}
		if e.Pos.Line >= 1 && e.Pos.Line <= len(lines) {
			maxCol := 1 + utf8.RuneCount(lines[e.Pos.Line-1])
			if e.Pos.Col < 1 || e.Pos.Col > maxCol {
				t.Errorf("invalid error Col %d on line %d, want in [1, %d]", e.Pos.Col, e.Pos.Line, maxCol)
			}
		} else if e.Pos.Col < 1 {
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
	return errList
}

func fuzzCheckSinglePathError(t *testing.T, err error, wantFile string, allowedCodes ...string) {
	t.Helper()
	errList, ok := err.(*ErrorList)
	if !ok || errList == nil {
		t.Fatalf("expected non-nil *ErrorList, got %T: %v", err, err)
	}
	if len(errList.Errors) != 1 {
		t.Fatalf("expected 1 path error, got %d: %v", len(errList.Errors), errList)
	}
	e := errList.Errors[0]
	if e == nil {
		t.Fatalf("nil *Error in ErrorList")
	}
	if e.File != wantFile {
		t.Errorf("path error File: got %q, want %q", e.File, wantFile)
	}
	if e.Pos != (Pos{}) {
		t.Errorf("path error Pos: got %v, want zero Pos", e.Pos)
	}
	if e.Msg == "" {
		t.Errorf("path error Msg is empty for code %q", e.Code)
	}
	if e.Hint == "" {
		t.Errorf("path error Hint is empty for code %q", e.Code)
	}
	if e.Error() == "" {
		t.Errorf("path error Error() is empty")
	}
	matched := false
	for _, c := range allowedCodes {
		if e.Code == c {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("unexpected path error code %q, want one of %v", e.Code, allowedCodes)
	}
}

func fuzzLoadSeedFiles(patterns ...string) [][]byte {
	var out [][]byte
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, p := range matches {
			data, err := os.ReadFile(p)
			if err == nil && len(data) <= 4096 {
				out = append(out, data)
			}
		}
	}
	return out
}

func FuzzParse(f *testing.F) {
	seeds := [][]byte{
		[]byte("@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\napp = \"web\"\n"),
		[]byte("### Comment\n@lkal \"0.1\"\ncount = 1\n"),
		[]byte("@lkal \"0.1\"\na =\n  \\\\one\n  \\\\two\n"),
		[]byte("@lkal \"0.1\"\nb =\n  \\\\one\n  \\\\\n  \\\\two\n"),
		[]byte("@lkal \"0.1\"\nc =\n  \\\\  indented\n"),
		[]byte("@lkal \"0.1\"\nd =\n  \\\\line\n  \\\\\n"),
		[]byte("@lkal \"0.1\"\ne =\n  \\\\### memo\n"),
		[]byte("@lkal \"0.1\"\ns = \"str\"\ni = 42\nd = 0.25\nb = true\ne = .prod\nq = 250m\nr = &ref/path\n"),
		[]byte("@lkal \"0.1\"\nempty = []\nnums = [1, 2, 3]\nstrs = [\"a\", \"b\",]\n"),
		[]byte("@lkal \"0.1\"\nrec { x = 1, y = 2 }\ncoll \"elem1\" { a = \"ok\" }\ncoll \"elem2\" { a = \"yes\" }\n"),
		[]byte("@lkal \"0.1\"\nroute \"GET\" \"/users\" { handler = \"list\", auth = true }\nenv \"LOG_LEVEL\" = \"info\"\n"),
		[]byte("@lkal \"0.1\"\n\"\" { \"\" = 1, \"true\" = false }\n"),
		[]byte("@lkal \"0.1\"\nlabels { app = \"web\", tier = \"frontend\" }\n"),
		[]byte("### Header\n\n@lkal \"0.1\"\n\nx = 1\n\n### Trailing\n"),
		[]byte("@lkal \"0.1\"\nservice \"web\" {\n  replicas = 3\n  port \"http\" { number = 8080, protocol = .tcp }\n}\n"),
		[]byte(""),
		[]byte("a = 1\n"),
		[]byte("@lkal \"0.2\"\n"),
		[]byte("@lkal \"0.1\"\nrec {\n"),
		[]byte("@lkal \"0.1\"\nlst = [1, 2\n"),
		[]byte("@lkal \"0.1\"\ns = \"unclosed\n"),
		[]byte("@lkal \"0.1\"\nval = bareword\n"),
		[]byte("# bad comment\n@lkal \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\na = 1 # inline\n"),
		[]byte("@lkal \"0.1\"\na = 1\na = 2\n"),
		[]byte("@lkal \"0.1\"\ns \"a\" {}\ns \"a\" {}\n"),
		[]byte("@lkal \"0.1\"\na = 1\na {}\n"),
		[]byte("@lkal \"0.1\"\ns \"a\" {}\nx = 1\ns \"b\" {}\n"),
		[]byte("@lkal \"0.1\"\nlst = [1, \"two\"]\n"),
		[]byte("@lkal \"0.1\"\n$key = 1\n"),
		[]byte("@lkal \"0.1\"\ns \"$a\" = 1\n"),
		[]byte("@unknown \"0.1\"\n"),
		[]byte("@lkal \"0.1\"\nqty = 1h30m\n"),
		[]byte("@lkal \"0.1\"\nhex = 0xFF\nbin = 0b10\noct = 0o77\nexp = 1e5\n"),
		[]byte("@lkal \"0.1\"\nbad_num = 01\n"),
		[]byte("@lkal \"0.1\"\nesc = \"\\q\"\n"),
		[]byte("@lkal \"0.1\"\nsurr = \"\\u{D800}\"\n"),
		[]byte("@lkal \"0.1\"\nsemi = 1;\n"),
		[]byte("@lkal \"0.1\"\nheading\n= 1\n"),
		[]byte("@lkal \"0.1\"\nraw =\n  \\\\line1\n\n  \\\\line2\n"),
		[]byte("\xef\xbb\xbf@lkal \"0.1\"\na = 1\n"),
		[]byte("\xff\xfe\xfd\x00\x01\x02"),
	}
	seeds = append(seeds, fuzzLoadSeedFiles(
		filepath.Join("testdata", "conformance", "valid", "*.lkal"),
		filepath.Join("testdata", "conformance", "plain", "*.lkal"),
		filepath.Join("testdata", "format", "*.lkal"),
		filepath.Join("testdata", "conformance", "invalid", "*.lkal"),
	)...)
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Parse("fuzz.lkal", data)
		if err == nil {
			if file == nil {
				t.Fatalf("Parse returned nil file with nil error")
			}
			if file.Version() != "0.1" {
				t.Errorf("Version() = %q, want \"0.1\"", file.Version())
			}
			d := file.Data()
			if !Equal(d, d) || !fuzzSameValue(d, d) {
				t.Errorf("Equal/Same(Data(), Data()) returned false")
			}
			dPerm0 := fuzzPermuteRecord(d, 0)
			dPerm1 := fuzzPermuteRecord(d, 1)
			fuzzCheckEqualLaws(t, d, dPerm0, dPerm1)

			// Format roundtrip & canonical lint
			fmtBytes := file.Format()
			if fmtBytes == nil {
				t.Fatalf("file.Format() returned nil")
			}
			fmtBytes2, fmtErr := Format(data)
			if fmtErr != nil {
				t.Fatalf("Format(data) failed on valid Parse input: %v", fmtErr)
			}
			if !bytes.Equal(fmtBytes, fmtBytes2) {
				t.Errorf("file.Format() != Format(data)")
			}
			if lintMsg := fuzzCanonicalLint(fmtBytes); lintMsg != "" {
				t.Errorf("canonical lint failed on Format output: %s\nout:\n%s", lintMsg, fmtBytes)
			}
			fFmt, err := Parse("fuzz.lkal", fmtBytes)
			if err != nil {
				t.Fatalf("Parse(Format) failed: %v", err)
			}
			if !Equal(fFmt.Data(), d) || !fuzzSameValue(fFmt.Data(), d) {
				t.Errorf("Parse(Format).Data() mismatch")
			}
			if !bytes.Equal(fFmt.Format(), fmtBytes) {
				t.Errorf("Format is not idempotent")
			}

			// JSON & plain JSON roundtrip + independent oracle check
			j, err := ToJSON(file, false)
			if err != nil {
				t.Fatalf("ToJSON(false) failed: %v", err)
			}
			if !json.Valid(j) {
				t.Errorf("ToJSON(false) produced invalid JSON: %s", j)
			}
			wantJ, err := fuzzExpectedJSON(file.Version(), d, false)
			if err != nil {
				t.Fatalf("fuzzExpectedJSON(false) failed: %v", err)
			}
			if !bytes.Equal(j, wantJ) {
				t.Errorf("ToJSON(false) mismatch with independent oracle:\ngot:\n%s\nwant:\n%s", j, wantJ)
			}

			plain, err := ToJSON(file, true)
			if err != nil {
				t.Fatalf("ToJSON(plain) failed: %v", err)
			}
			if !json.Valid(plain) {
				t.Errorf("ToJSON(plain) produced invalid JSON: %s", plain)
			}
			wantPlain, err := fuzzExpectedJSON(file.Version(), d, true)
			if err != nil {
				t.Fatalf("fuzzExpectedJSON(plain) failed: %v", err)
			}
			if !bytes.Equal(plain, wantPlain) {
				t.Errorf("ToJSON(plain) mismatch with independent oracle:\ngot:\n%s\nwant:\n%s", plain, wantPlain)
			}

			g, err := FromJSON(j)
			if err != nil {
				t.Fatalf("FromJSON(ToJSON) failed: %v", err)
			}
			if g == nil {
				t.Fatalf("FromJSON returned nil File")
			}
			if !Equal(g.Data(), d) || !fuzzSameValue(g.Data(), d) {
				t.Errorf("FromJSON(ToJSON).Data() mismatch")
			}
			fuzzCheckEqualLaws(t, d, fFmt.Data(), g.Data())

			j2, err := ToJSON(g, false)
			if err != nil {
				t.Fatalf("ToJSON(g, false) failed: %v", err)
			}
			if !bytes.Equal(j2, j) {
				t.Errorf("roundtrip ToJSON mismatch:\ngot:\n%s\nwant:\n%s", j2, j)
			}

			gFmt := g.Format()
			if lintMsg := fuzzCanonicalLint(gFmt); lintMsg != "" {
				t.Errorf("canonical lint failed on FromJSON.Format(): %s\nout:\n%s", lintMsg, gFmt)
			}
			return
		}

		if file != nil {
			t.Errorf("Parse returned non-nil file on error: %v", file)
		}
		fuzzCheckErrorList(t, err, "fuzz.lkal", data, parserFuzzIsKnownErrorCode)
	})
}

func FuzzEqual(f *testing.F) {
	seeds := [][]byte{
		{},
		{0, 1, 2, 3, 4, 5},
		{10, 20, 30, 40, 50, 60},
		[]byte("equal-equivalence-laws-seed-1"),
		[]byte("equal-equivalence-laws-seed-2"),
		[]byte("\xff\x00\x7f\x80\x01\x02\x03\x04"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		c := newFuzzConsumer(data)
		a := c.genValue(2)
		aPerm0 := fuzzPermuteRecord(a, 0)
		aPerm1 := fuzzPermuteRecord(a, 1)
		aPerm2 := fuzzPermuteRecord(a, 2)

		// Permuted Record versions must all be Equal and satisfy reflexivity, symmetry, transitivity
		if !Equal(a, aPerm0) || !Equal(aPerm0, aPerm1) || !Equal(aPerm1, aPerm2) {
			t.Fatalf("Record field permutation broke Equal:\na=%#v\naPerm0=%#v\naPerm1=%#v", a, aPerm0, aPerm1)
		}
		fuzzCheckEqualLaws(t, a, aPerm0, aPerm1)
		fuzzCheckEqualLaws(t, aPerm0, aPerm1, aPerm2)

		// Construct b and cVal (mix of equal permutations and mutated/distinct values)
		var b, cVal Value
		switch c.n(4) {
		case 0:
			b = aPerm0
		case 1:
			b = fuzzMutateCopy(c, a)
		default:
			b = c.genValue(2)
		}
		switch c.n(4) {
		case 0:
			cVal = aPerm1
		case 1:
			cVal = fuzzPermuteRecord(b, 0)
		case 2:
			cVal = fuzzMutateCopy(c, b)
		default:
			cVal = c.genValue(2)
		}

		fuzzCheckEqualLaws(t, a, b, cVal)
		fuzzCheckEqualLaws(t, aPerm0, b, cVal)

		// List element-order sensitivity
		if lst, ok := a.(List); ok && len(lst) >= 2 && !Equal(lst[0], lst[1]) {
			swapped := slices.Clone(lst)
			swapped[0], swapped[1] = swapped[1], swapped[0]
			if Equal(lst, swapped) || Equal(swapped, lst) {
				t.Errorf("List order sensitivity violated: %#v vs %#v", lst, swapped)
			}
			fuzzCheckEqualLaws(t, lst, aPerm0, swapped)
		}

		// Quantity non-conversion invariant (ADR 0010 / Roadmap Problem 2.2)
		q1 := Quantity{Number: "1", Unit: "Gi"}
		q2 := Quantity{Number: "1024", Unit: "Mi"}
		if Equal(q1, q2) || Equal(q2, q1) {
			t.Errorf("Quantity unit conversion must not occur in Equal")
		}
		fuzzCheckEqualLaws(t, q1, Quantity{Number: "1", Unit: "Gi"}, q2)
	})
}

func fuzzMutateCopy(c *fuzzConsumer, v Value) Value {
	switch x := v.(type) {
	case String:
		return String(string(x) + "!")
	case Int:
		return Int(x ^ 1)
	case Decimal:
		return Decimal(string(x) + "1")
	case Bool:
		return Bool(!x)
	case Enum:
		return Enum(string(x) + "_x")
	case Quantity:
		return Quantity{Number: x.Number, Unit: x.Unit + "s"}
	case Ref:
		cp := slices.Clone(x)
		if len(cp) > 0 {
			cp[len(cp)-1] += "_x"
		}
		return cp
	case List:
		cp := slices.Clone(x)
		if len(cp) >= 2 && !Equal(cp[0], cp[1]) {
			cp[0], cp[1] = cp[1], cp[0]
			return cp
		}
		return append(cp, Int(0))
	case Record:
		if len(x.Fields) == 0 {
			return Record{Fields: []Field{{Key: "added", Value: Int(1)}}}
		}
		fields := slices.Clone(x.Fields)
		idx := c.n(len(fields))
		if c.coin(2) {
			fields[idx].Value = fuzzMutateCopy(c, fields[idx].Value)
		} else {
			fields[idx].Key += "_mut"
		}
		return Record{Fields: fields}
	default:
		return Int(0)
	}
}

func FuzzParseValue(f *testing.F) {
	seeds := [][]byte{
		[]byte(`"hello"`),
		[]byte(`"line1\nline2"`),
		[]byte(`"tab\tand\rcr\u{0001}"`),
		[]byte("\\\\one\n\\\\two"),
		[]byte("### header\n\n\\\\line 1\n\\\\line 2\n\n### footer\n"),
		[]byte(`0`),
		[]byte(`-0`),
		[]byte(`42`),
		[]byte(`-9223372036854775808`),
		[]byte(`9223372036854775807`),
		[]byte(`0.25`),
		[]byte(`1.500`),
		[]byte(`-0.00`),
		[]byte(`true`),
		[]byte(`false`),
		[]byte(`.tcp`),
		[]byte(`.prod_1`),
		[]byte(`512Mi`),
		[]byte(`0.25s`),
		[]byte(`-100m`),
		[]byte(`99999999999999999999Mi`),
		[]byte(`&secret/db-creds`),
		[]byte(`&a/"b c"/""`),
		[]byte(`[]`),
		[]byte(`[1, 2, 3]`),
		[]byte("[\n  \"a\",\n  \"b\",\n]"),
		[]byte(`{}`),
		[]byte(`{ a = 1, b { c = 2 } }`),
		[]byte(`{ route "GET" "/users" { auth = true }, env "LOG_LEVEL" = "info" }`),
		[]byte(`{ "" { "" = 1 } }`),
		[]byte(``),
		[]byte(`   `),
		[]byte(`### only comment`),
		[]byte(`tcp`),
		[]byte(`1 2`),
		[]byte(`[1, "a"]`),
		[]byte(`[1, "a"] extra`),
		[]byte(`{ a = 1, a = 2 }`),
		[]byte(`{ "$x" = 1 }`),
		[]byte(`{ "$x" = 1 } extra`),
		[]byte(`{ a {} b {} }`),
		[]byte(`{`),
		[]byte(`[`),
		[]byte(`0xFF`),
		[]byte(`1e5`),
		[]byte("[\n### c\n1]"),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		val, err := ParseValue(data)
		if err == nil {
			if val == nil {
				t.Fatalf("ParseValue returned nil Value with nil error")
			}
			if !Equal(val, val) || !fuzzSameValue(val, val) {
				t.Errorf("Equal/Same(val, val) returned false")
			}

			formatted := FormatValue(val)
			if len(formatted) == 0 {
				t.Fatalf("FormatValue returned empty bytes for non-nil Value %#v", val)
			}
			if bytes.HasSuffix(formatted, []byte("\n")) {
				t.Errorf("FormatValue(%#v) has trailing newline: %q", val, formatted)
			}

			val2, err := ParseValue(formatted)
			if err != nil {
				t.Fatalf("ParseValue(FormatValue(%#v)) failed on %q: %v", val, formatted, err)
			}
			if !Equal(val2, val) || !fuzzSameValue(val2, val) {
				t.Errorf("ParseValue(FormatValue) mismatch:\ngot:  %#v\nwant: %#v\nformatted: %q", val2, val, formatted)
			}
			fuzzCheckEqualLaws(t, val, val2, fuzzPermuteRecord(val, 0))

			formatted2 := FormatValue(val2)
			if !bytes.Equal(formatted, formatted2) {
				t.Errorf("FormatValue is not idempotent:\nfirst:  %q\nsecond: %q", formatted, formatted2)
			}

			doc, err := Parse("val.lkal", []byte("@lkal \"0.1\"\n"))
			if err != nil {
				t.Fatalf("Parse empty doc failed: %v", err)
			}
			recDepth := fuzzValueRecordDepth(val)
			setErr := doc.Set("k", val)
			if recDepth < maxBlockDepth {
				if setErr != nil {
					t.Fatalf("Set(\"k\", val) failed at depth %d: %v", recDepth, setErr)
				}
				got, getErr := doc.Get("k")
				if getErr != nil {
					t.Fatalf("Get(\"k\") after Set failed: %v", getErr)
				}
				if !Equal(got, val) || !fuzzSameValue(got, val) {
					t.Errorf("Get(\"k\") after Set mismatch:\ngot:  %#v\nwant: %#v", got, val)
				}
				docFmt := doc.Format()
				if lintMsg := fuzzCanonicalLint(docFmt); lintMsg != "" {
					t.Errorf("canonical lint failed after Set(\"k\", val): %s\nout:\n%s", lintMsg, docFmt)
				}
			} else {
				fuzzCheckSinglePathError(t, setErr, "val.lkal", "L403")
			}
			return
		}

		if val != nil {
			t.Errorf("ParseValue returned non-nil Value on error: %#v", val)
		}
		fuzzCheckErrorList(t, err, "", data, valueFuzzIsKnownErrorCode)
	})
}

type fuzzReachablePath struct {
	segs []string
	val  Value
}

func fuzzCollectPaths(r Record, prefix []string, out *[]fuzzReachablePath, limit int) {
	for _, f := range r.Fields {
		if len(*out) >= limit {
			return
		}
		p := append(append([]string(nil), prefix...), f.Key)
		*out = append(*out, fuzzReachablePath{segs: p, val: f.Value})
		if sub, ok := f.Value.(Record); ok {
			fuzzCollectPaths(sub, p, out, limit)
		}
	}
}

func fuzzQuotedPath(segs []string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = quoteString(s)
	}
	return strings.Join(parts, "/")
}

func FuzzPath(f *testing.F) {
	specDoc, _ := os.ReadFile(filepath.Join("testdata", "conformance", "valid", "spec_example.lkal"))
	if len(specDoc) == 0 {
		specDoc = []byte("@lkal \"0.1\"\nservice \"web\" { replicas = 3, port \"http\" { number = 8080 } }\n")
	}
	seeds := []struct {
		doc    []byte
		path   string
		valSrc []byte
	}{
		{specDoc, "service/web/replicas", []byte("5")},
		{specDoc, "service/web/container/app/port/http/number", []byte("9090")},
		{specDoc, `service/web/labels/"app.kubernetes.io/name"`, []byte(`"api"`)},
		{specDoc, "service/web/startup", []byte("\\\\#!/bin/sh\n\\\\echo ok")},
		{specDoc, "service/web/new_route/GET/\"/health\"", []byte(`{ status = 200 }`)},
		{[]byte("@lkal \"0.1\"\nserver {}\n"), "server/mem", []byte("1.5Gi")},
		{[]byte("@lkal \"0.1\"\nport \"http\" {}\n"), "port/http/number", []byte("80")},
		{[]byte("@lkal \"0.1\"\n\"\" { x = 1 }\n"), `""/x`, []byte("2")},
		{[]byte("@lkal \"0.1\"\nrec { a = 1, b = 2 }\n"), "rec/a", []byte("[10, 20]")},
		{[]byte("@lkal \"0.1\"\na \"b\" \"c\" = 1\na \"b\" \"d\" = 2\n"), "a/b", []byte(`{ e = 3 }`)},
		{[]byte("@lkal \"0.1\"\na = 1\n"), "", []byte("2")},
		{[]byte("@lkal \"0.1\"\na = 1\n"), "a//b", []byte("2")},
		{[]byte("@lkal \"0.1\"\na = 1\n"), "a/b", []byte("2")},
		{[]byte("@lkal \"0.1\"\na = 1\n"), "$bad", []byte("2")},
		{[]byte("@lkal \"0.1\"\na {}\n"), "a/$bad", []byte("2")},
	}
	for _, s := range seeds {
		f.Add(s.doc, s.path, s.valSrc)
	}

	f.Fuzz(func(t *testing.T, doc []byte, path string, valSrc []byte) {
		f0, err := Parse("fuzz.lkal", doc)
		if err != nil {
			return
		}
		origData := f0.Data()
		origFmt := f0.Format()

		var reachable []fuzzReachablePath
		fuzzCollectPaths(origData, nil, &reachable, 8)
		pathSegs, pathValid := parseRefBody(path)

		// 1. Test arbitrary Get(path)
		gotVal, getErr := f0.Get(path)
		if getErr != nil {
			if gotVal != nil {
				t.Errorf("Get(%q) returned non-nil Value on error: %#v", path, gotVal)
			}
			fuzzCheckSinglePathError(t, getErr, "fuzz.lkal", "L401", "L402")
			if !bytes.Equal(f0.Format(), origFmt) {
				t.Errorf("Get(%q) error mutated File", path)
			}
		} else {
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value with nil error", path)
			}
			fv := FormatValue(gotVal)
			pv, pvErr := ParseValue(fv)
			if pvErr != nil || !Equal(pv, gotVal) || !fuzzSameValue(pv, gotVal) {
				t.Errorf("FormatValue/ParseValue roundtrip failed on Get(%q): val=%#v fv=%q err=%v", path, gotVal, fv, pvErr)
			}
			// No-aliasing check on Get result
			fuzzMutateValueInPlace(gotVal)
			gotVal2, getErr2 := f0.Get(path)
			if getErr2 != nil || !fuzzSameValue(gotVal2, pv) {
				t.Errorf("caller mutation of Get(%q) result mutated File data", path)
			}
			if !bytes.Equal(f0.Format(), origFmt) {
				t.Errorf("caller mutation of Get(%q) result mutated File format", path)
			}
		}

		// 2. Test Lens Laws (PutGet, PutPut, Frame Condition) on arbitrary Set(path, val)
		if val, valErr := ParseValue(valSrc); valErr == nil {
			valCopy, _ := ParseValue(FormatValue(val))
			fSet, _ := Parse("fuzz.lkal", doc)
			setErr := fSet.Set(path, val)
			if setErr != nil {
				fuzzCheckSinglePathError(t, setErr, "fuzz.lkal", "L401", "L402", "L403")
				if !bytes.Equal(fSet.Format(), origFmt) {
					t.Errorf("failed Set(%q) mutated File format", path)
				}
			} else {
				// Mutate caller's val in-place to verify Set copied it
				fuzzMutateValueInPlace(val)

				// Lens Law 1: PutGet (setP v p x = some v' -> getP v' p = some x)
				gotAfterSet, err := fSet.Get(path)
				if err != nil {
					t.Fatalf("PutGet failed: Get(%q) failed right after successful Set: %v", path, err)
				}
				if !Equal(gotAfterSet, valCopy) || !fuzzSameValue(gotAfterSet, valCopy) {
					t.Errorf("PutGet failed: Get(%q) after Set mismatch:\ngot:  %#v\nwant: %#v", path, gotAfterSet, valCopy)
				}

				// Lens Law 4: Frame Condition (DisjointPath p1 p2 -> getP v' p2 = getP v p2)
				if pathValid {
					for _, rp := range reachable {
						if fuzzDisjointPaths(pathSegs, rp.segs) {
							p2 := refBody(Ref(rp.segs))
							got2, err2 := fSet.Get(p2)
							if err2 != nil || !Equal(got2, rp.val) || !fuzzSameValue(got2, rp.val) {
								t.Errorf("Frame Condition failed on Set(%q): disjoint path %q changed:\ngot:  %#v (%v)\nwant: %#v", path, p2, got2, err2, rp.val)
							}
						}
					}
				}

				// Lens Law 3: PutPut (setP v p x = some v1 -> setP v1 p y = some v2 -> setP v p y = some v2)
				val2 := Value(Int(777))
				if len(reachable) > 0 && fuzzValueRecordDepth(reachable[0].val) == 0 {
					val2 = reachable[0].val
				}
				fDirect, _ := Parse("fuzz.lkal", doc)
				errPutPut := fSet.Set(path, val2)
				errDirect := fDirect.Set(path, val2)
				if (errPutPut == nil) != (errDirect == nil) {
					t.Errorf("PutPut error mismatch on %q: errPutPut=%v, errDirect=%v", path, errPutPut, errDirect)
				} else if errPutPut == nil {
					if !Equal(fSet.Data(), fDirect.Data()) || !fuzzSameValue(fSet.Data(), fDirect.Data()) {
						t.Errorf("PutPut failed on %q:\ngot:  %#v\nwant: %#v", path, fSet.Data(), fDirect.Data())
					}
				}

				setFmt := fSet.Format()
				if lintMsg := fuzzCanonicalLint(setFmt); lintMsg != "" {
					t.Errorf("canonical lint failed after Set(%q): %s\nout:\n%s", path, lintMsg, setFmt)
				}
				fSetParsed, err := Parse("fuzz.lkal", setFmt)
				if err != nil {
					t.Fatalf("Parse(fSet.Format()) failed after Set(%q): %v\nout:\n%s", path, err, setFmt)
				}
				if !Equal(fSetParsed.Data(), fSet.Data()) || !fuzzSameValue(fSetParsed.Data(), fSet.Data()) {
					t.Errorf("Parse(fSet.Format()).Data() != fSet.Data() after Set(%q)", path)
				}
			}
		}

		// 3. Test arbitrary Delete(path) & Frame Condition
		fDel, _ := Parse("fuzz.lkal", doc)
		delErr := fDel.Delete(path)
		if delErr != nil {
			fuzzCheckSinglePathError(t, delErr, "fuzz.lkal", "L401", "L402")
			if !bytes.Equal(fDel.Format(), origFmt) {
				t.Errorf("failed Delete(%q) mutated File format", path)
			}
		} else {
			_, getAfterDelErr := fDel.Get(path)
			fuzzCheckSinglePathError(t, getAfterDelErr, "fuzz.lkal", "L402")
			if pathValid {
				for _, rp := range reachable {
					if fuzzDisjointPaths(pathSegs, rp.segs) {
						p2 := refBody(Ref(rp.segs))
						got2, err2 := fDel.Get(p2)
						if err2 != nil || !Equal(got2, rp.val) || !fuzzSameValue(got2, rp.val) {
							t.Errorf("Frame Condition failed on Delete(%q): disjoint path %q changed:\ngot:  %#v (%v)\nwant: %#v", path, p2, got2, err2, rp.val)
						}
					}
				}
			}
			delFmt := fDel.Format()
			if lintMsg := fuzzCanonicalLint(delFmt); lintMsg != "" {
				t.Errorf("canonical lint failed after Delete(%q): %s\nout:\n%s", path, lintMsg, delFmt)
			}
			fDelParsed, err := Parse("fuzz.lkal", delFmt)
			if err != nil {
				t.Fatalf("Parse(fDel.Format()) failed after Delete(%q): %v\nout:\n%s", path, err, delFmt)
			}
			if !Equal(fDelParsed.Data(), fDel.Data()) || !fuzzSameValue(fDelParsed.Data(), fDel.Data()) {
				t.Errorf("Parse(fDel.Format()).Data() != fDel.Data() after Delete(%q)", path)
			}
		}

		// 4. Reachable-path closure checks (GetPut, Frame Condition, Delete+Set restoration)
		for idx, rp := range reachable {
			p := refBody(Ref(rp.segs))
			if idx%2 == 1 {
				p = fuzzQuotedPath(rp.segs)
			}
			v, err := f0.Get(p)
			if err != nil {
				t.Fatalf("Get reachable path %q failed: %v", p, err)
			}
			if !Equal(v, rp.val) || !fuzzSameValue(v, rp.val) {
				t.Fatalf("Get reachable path %q mismatch", p)
			}

			// Lens Law 2: GetPut (WF v -> getP v p = some w -> setP v p w = some v)
			f1, _ := Parse("fuzz.lkal", doc)
			if err := f1.Set(p, v); err != nil {
				t.Fatalf("GetPut failed: Set(%q, Get(%q)) failed: %v", p, p, err)
			}
			if !Equal(f1.Data(), origData) || !fuzzSameValue(f1.Data(), origData) {
				t.Errorf("GetPut failed: Data() changed after Set(%q, Get(%q))", p, p)
			}
			if lintMsg := fuzzCanonicalLint(f1.Format()); lintMsg != "" {
				t.Errorf("canonical lint failed after Set(%q, Get(%q)): %s", p, p, lintMsg)
			}

			// Delete(p) then Set(p, v) restoration closure (ADR 0010)
			f2, _ := Parse("fuzz.lkal", doc)
			if err := f2.Delete(p); err != nil {
				t.Fatalf("Delete reachable path %q failed: %v", p, err)
			}
			_, errAfterDel := f2.Get(p)
			fuzzCheckSinglePathError(t, errAfterDel, "fuzz.lkal", "L402")
			if lintMsg := fuzzCanonicalLint(f2.Format()); lintMsg != "" {
				t.Errorf("canonical lint failed after Delete(%q): %s", p, lintMsg)
			}
			if err := f2.Set(p, v); err != nil {
				t.Fatalf("Set(%q, v) after Delete(%q) failed: %v", p, p, err)
			}
			if !Equal(f2.Data(), origData) {
				t.Errorf("Delete(%q) followed by Set(%q, v) did not restore Equal Data:\ngot:  %#v\nwant: %#v", p, p, f2.Data(), origData)
			}
			if lintMsg := fuzzCanonicalLint(f2.Format()); lintMsg != "" {
				t.Errorf("canonical lint failed after Delete+Set(%q): %s", p, lintMsg)
			}
		}
	})
}

// ============================================================================
// Structure-Aware Byte-Consumer Fuzzing (FuzzStructured)
// ============================================================================

type fuzzConsumer struct {
	data []byte
	off  int
	unit string
}

func newFuzzConsumer(data []byte) *fuzzConsumer {
	return &fuzzConsumer{data: data, unit: "  "}
}

func (c *fuzzConsumer) nextByte() byte {
	idx := c.off
	c.off++
	if len(c.data) == 0 {
		return byte(idx * 31)
	}
	if idx < len(c.data) {
		return c.data[idx]
	}
	return c.data[idx%len(c.data)] ^ byte((idx/len(c.data))*31+17)
}

func (c *fuzzConsumer) n(k int) int {
	if k <= 1 {
		return 0
	}
	return int(c.nextByte()) % k
}

func (c *fuzzConsumer) coin(k int) bool {
	return c.n(k) == 0
}

func (c *fuzzConsumer) pick(ss ...string) string {
	return ss[c.n(len(ss))]
}

var fuzzSimpleRunes = []rune("abcxyzAQZ019_-")

var fuzzWildRunes = []rune{
	'a', 'z', 'Q', '0', '9', ' ', '_', '-', '.', '/', '"', '\\', '#', '{', '}', '[', ']',
	'=', ',', '&', '$', '<', '>', '\'', '\t', '\n', '\r', 0x00, 0x01, 0x08, 0x0c, 0x1b, 0x1f, 0x7f,
	'é', 'ß', '日', '本', 0x1F600, 0x2028, 0x2029, 0xFEFF, 0xFFFD, 0x10FFFF,
}

var fuzzLineRunes = []rune{
	'a', 'z', 'Q', '0', '9', ' ', '_', '-', '.', '/', '"', '\\', '#', '{', '}', '[', ']',
	'=', ',', '&', '$', '<', '>', '\'', '\t',
	'é', 'ß', '日', '本', 0x1F600, 0x2028, 0x2029, 0xFEFF, 0xFFFD, 0x10FFFF,
}

func (c *fuzzConsumer) randRune() rune {
	if c.coin(2) {
		return fuzzSimpleRunes[c.n(len(fuzzSimpleRunes))]
	}
	return fuzzWildRunes[c.n(len(fuzzWildRunes))]
}

func (c *fuzzConsumer) randString(max int) string {
	n := c.n(max + 1)
	rs := make([]rune, n)
	for i := range rs {
		rs[i] = c.randRune()
	}
	return string(rs)
}

func (c *fuzzConsumer) lineString(max int) string {
	n := c.n(max + 1)
	rs := make([]rune, n)
	for i := range rs {
		if c.coin(2) {
			rs[i] = fuzzSimpleRunes[c.n(len(fuzzSimpleRunes))]
		} else {
			rs[i] = fuzzLineRunes[c.n(len(fuzzLineRunes))]
		}
	}
	return string(rs)
}

func (c *fuzzConsumer) rawFriendly() string {
	n := 2 + c.n(2)
	lines := make([]string, n)
	for i := range lines {
		lines[i] = c.lineString(5)
	}
	return strings.Join(lines, "\n")
}

func (c *fuzzConsumer) genName() string {
	const first = "abcxyzAZ_"
	const rest = "abcxyz019_-"
	b := []byte{first[c.n(len(first))]}
	for i, n := 0, c.n(4); i < n; i++ {
		b = append(b, rest[c.n(len(rest))])
	}
	return string(b)
}

func (c *fuzzConsumer) genKey(used map[string]bool) string {
	for attempt := 0; attempt < 16; attempt++ {
		var k string
		switch c.n(6) {
		case 0, 1, 2:
			k = c.genName()
		case 3:
			k = c.pick("true", "false", "", "a b", "1x", "x.y", "-k", "日本", "a/b")
		default:
			k = c.randString(4)
		}
		if strings.HasPrefix(k, "$") || used[k] {
			continue
		}
		used[k] = true
		return k
	}
	k := fmt.Sprintf("k%d", len(used))
	used[k] = true
	return k
}

func (c *fuzzConsumer) digits(n int, nonzeroFirst bool) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('0' + c.n(10))
	}
	if nonzeroFirst && n > 0 {
		b[0] = byte('1' + c.n(9))
	}
	return string(b)
}

func (c *fuzzConsumer) genDecimal() string {
	ip := "0"
	switch c.n(3) {
	case 1:
		ip = strconv.Itoa(1 + c.n(999))
	case 2:
		ip = c.digits(1+c.n(12), true)
	}
	frac := "0"
	if !c.coin(4) {
		frac = c.digits(c.n(3), false) + string(rune('1'+c.n(9)))
	}
	if c.coin(3) && !(ip == "0" && frac == "0") {
		return "-" + ip + "." + frac
	}
	return ip + "." + frac
}

func (c *fuzzConsumer) genQtyNumber() string {
	if c.coin(2) {
		return c.genDecimal()
	}
	if c.coin(4) {
		return "0"
	}
	s := c.digits(1+c.n(6), true)
	if c.coin(3) {
		s = "-" + s
	}
	return s
}

func (c *fuzzConsumer) genScalar(kind int) Value {
	switch kind {
	case 0:
		if c.coin(3) {
			return String(c.rawFriendly())
		}
		return String(c.randString(6))
	case 1:
		switch c.n(4) {
		case 0:
			return Int(c.n(201) - 100)
		case 1:
			return Int([]int64{math.MinInt64, math.MaxInt64, 0, -1}[c.n(4)])
		default:
			return Int(int64(c.nextByte()) - 128)
		}
	case 2:
		return Decimal(c.genDecimal())
	case 3:
		return Bool(c.coin(2))
	case 4:
		return Enum(c.genName())
	case 5:
		num := c.genQtyNumber()
		unit := c.pick("Gi", "Mi", "Ki", "s", "ms", "h", "m", "abc", "Q")
		if num == "0" {
			unit = c.pick("Gi", "Mi", "Ki", "s", "ms", "h", "m")
		}
		return Quantity{Number: num, Unit: unit}
	default:
		n := 1 + c.n(3)
		segs := make(Ref, n)
		for i := range segs {
			if c.coin(2) {
				const bare = "abcAZ09_.-"
				b := make([]byte, 1+c.n(3))
				for j := range b {
					b[j] = bare[c.n(len(bare))]
				}
				segs[i] = string(b)
			} else {
				segs[i] = c.randString(4)
			}
		}
		return segs
	}
}

func (c *fuzzConsumer) genList() List {
	kind := c.n(7)
	n := c.n(4)
	l := make(List, 0, n)
	for i := 0; i < n; i++ {
		l = append(l, c.genScalar(kind))
	}
	return l
}

func (c *fuzzConsumer) genValue(depth int) Value {
	k := c.n(16)
	switch {
	case depth > 0 && k < 4:
		return c.genRecord(depth-1, 3)
	case k < 7:
		return c.genList()
	default:
		return c.genScalar(c.n(7))
	}
}

func (c *fuzzConsumer) genRecord(depth, maxFields int) Record {
	n := c.n(maxFields + 1)
	used := map[string]bool{}
	var r Record
	for i := 0; i < n; i++ {
		k := c.genKey(used)
		r.Fields = append(r.Fields, Field{Key: k, Value: c.genValue(depth)})
	}
	return r
}

func (c *fuzzConsumer) numSpell(n string) string {
	if strings.Contains(n, ".") {
		n += strings.Repeat("0", c.n(3))
		if strings.Trim(n, "0.") == "" && c.coin(2) {
			n = "-" + n
		}
		return n
	}
	if n == "0" && c.coin(3) {
		return "-0"
	}
	return n
}

func (c *fuzzConsumer) uEsc(r rune) string {
	h := strconv.FormatInt(int64(r), 16)
	pad := 0
	if len(h) < 6 && c.coin(3) {
		pad = c.n(6 - len(h) + 1)
	}
	return `\u{` + strings.Repeat("0", pad) + h + `}`
}

func (c *fuzzConsumer) lkalQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			if c.coin(4) {
				b.WriteString(c.uEsc(r))
			} else {
				b.WriteString(`\"`)
			}
		case r == '\\':
			if c.coin(4) {
				b.WriteString(c.uEsc(r))
			} else {
				b.WriteString(`\\`)
			}
		case r == '\n':
			if c.coin(4) {
				b.WriteString(c.uEsc(r))
			} else {
				b.WriteString(`\n`)
			}
		case r == '\r':
			if c.coin(4) {
				b.WriteString(c.uEsc(r))
			} else {
				b.WriteString(`\r`)
			}
		case r == '\t':
			switch c.n(3) {
			case 0:
				b.WriteRune(r)
			case 1:
				b.WriteString(`\t`)
			default:
				b.WriteString(c.uEsc(r))
			}
		case r < 0x20 || r == 0x7f:
			b.WriteString(c.uEsc(r))
		default:
			if c.coin(10) {
				b.WriteString(c.uEsc(r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (c *fuzzConsumer) refText(r Ref) string {
	parts := make([]string, len(r))
	for i, s := range r {
		if isBareRefSegment(s) && !c.coin(4) {
			parts[i] = s
		} else {
			parts[i] = c.lkalQuote(s)
		}
	}
	return strings.Join(parts, "/")
}

func (c *fuzzConsumer) keyText(k string) string {
	if isBareKey(k) && !c.coin(5) {
		return k
	}
	return c.lkalQuote(k)
}

func (c *fuzzConsumer) scalarText(v Value) string {
	switch x := v.(type) {
	case String:
		return c.lkalQuote(string(x))
	case Int:
		if x == 0 && c.coin(3) {
			return "-0"
		}
		return strconv.FormatInt(int64(x), 10)
	case Decimal:
		return c.numSpell(string(x))
	case Bool:
		return strconv.FormatBool(bool(x))
	case Enum:
		return "." + string(x)
	case Quantity:
		if x.Number == "0" && isRadixPrefix(x.Unit[0]) {
			return "0" + x.Unit
		}
		return c.numSpell(x.Number) + x.Unit
	case Ref:
		return "&" + c.refText(x)
	}
	return `""`
}

func (c *fuzzConsumer) listOneLine(l List) string {
	if len(l) == 0 {
		return c.pick("[]", "[ ]")
	}
	parts := make([]string, len(l))
	for i, e := range l {
		parts[i] = c.scalarText(e)
	}
	return c.pick("[", "[ ") + strings.Join(parts, c.pick(", ", ",", " , ")) + c.pick("]", " ]", ",]", ", ]")
}

func (c *fuzzConsumer) listMulti(l List, lvl int) string {
	var b strings.Builder
	b.WriteString("[\n")
	for i, e := range l {
		b.WriteString(strings.Repeat(c.unit, lvl+1) + c.scalarText(e))
		if i < len(l)-1 || c.coin(2) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat(c.unit, lvl) + "]")
	return b.String()
}

type fuzzSynthEntry struct {
	key    string
	labels []string
	val    Value
}

func (c *fuzzConsumer) expandRecordEntries(r Record) []fuzzSynthEntry {
	var out []fuzzSynthEntry
	for _, f := range r.Fields {
		if sub, ok := f.Value.(Record); ok && len(sub.Fields) > 0 && c.coin(2) {
			for _, sf := range sub.Fields {
				if sub2, ok2 := sf.Value.(Record); ok2 && len(sub2.Fields) > 0 && c.coin(2) {
					for _, sf2 := range sub2.Fields {
						out = append(out, fuzzSynthEntry{
							key:    f.Key,
							labels: []string{sf.Key, sf2.Key},
							val:    sf2.Value,
						})
					}
					continue
				}
				out = append(out, fuzzSynthEntry{
					key:    f.Key,
					labels: []string{sf.Key},
					val:    sf.Value,
				})
			}
			continue
		}
		out = append(out, fuzzSynthEntry{key: f.Key, val: f.Value})
	}
	return out
}

func (c *fuzzConsumer) headerText(e fuzzSynthEntry) string {
	var b strings.Builder
	b.WriteString(c.keyText(e.key))
	for _, lbl := range e.labels {
		b.WriteString(c.pick(" ", "  "))
		b.WriteString(c.lkalQuote(lbl))
	}
	return b.String()
}

func (c *fuzzConsumer) inlineRecord(rec Record) string {
	if len(rec.Fields) == 0 {
		return c.pick("{}", "{ }")
	}
	parts := make([]string, 0, len(rec.Fields))
	for _, f := range rec.Fields {
		kt := c.keyText(f.Key)
		switch sv := f.Value.(type) {
		case Record:
			parts = append(parts, kt+" "+c.inlineRecord(sv))
		case List:
			parts = append(parts, kt+c.pick(" = ", "=")+c.listOneLine(sv))
		default:
			parts = append(parts, kt+c.pick(" = ", "=")+c.scalarText(sv))
		}
	}
	return c.pick("{ ", "{") + strings.Join(parts, c.pick(", ", ",")) + c.pick("", ",") + c.pick(" }", "}")
}

func (c *fuzzConsumer) entryText(e fuzzSynthEntry, lvl int) (string, bool, bool) {
	hdr := c.headerText(e)
	if r, ok := e.val.(Record); ok {
		if c.coin(3) {
			return hdr + " " + c.inlineRecord(r), true, false
		}
		var b strings.Builder
		b.WriteString(hdr + c.pick(" {", "{") + "\n")
		c.bodyText(&b, r, lvl+1)
		b.WriteString(strings.Repeat(c.unit, lvl) + "}")
		return b.String(), false, false
	}
	if s, ok := e.val.(String); ok && isRawCandidate(string(s)) && c.coin(2) {
		var b strings.Builder
		b.WriteString(hdr + c.pick(" =", "=") + "\n")
		for i, ln := range strings.Split(string(s), "\n") {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(strings.Repeat(c.unit, lvl+1) + `\\` + ln)
		}
		return b.String(), false, true
	}
	if l, ok := e.val.(List); ok {
		if c.coin(3) {
			return hdr + c.pick(" = ", "=") + c.listMulti(l, lvl), false, false
		}
		return hdr + c.pick(" = ", "=") + c.listOneLine(l), true, false
	}
	return hdr + c.pick(" = ", "=", "  =  ") + c.scalarText(e.val), true, false
}

func (c *fuzzConsumer) commentLine() string {
	text := c.lineString(6)
	switch c.n(4) {
	case 0:
		return "###"
	case 1:
		return "###" + text
	case 2:
		return "#### " + text
	default:
		return "### " + text
	}
}

func (c *fuzzConsumer) bodyText(b *strings.Builder, r Record, lvl int) {
	ents := c.expandRecordEntries(r)
	same := false
	for i, e := range ents {
		if !same {
			if c.coin(4) {
				b.WriteString("\n")
			}
			if c.coin(4) {
				b.WriteString(strings.Repeat(c.unit, lvl) + c.commentLine() + "\n")
			}
			b.WriteString(strings.Repeat(c.unit, lvl))
		}
		text, single, endsRaw := c.entryText(e, lvl)
		b.WriteString(text)
		if i < len(ents)-1 && single && c.coin(4) {
			b.WriteString(c.pick(", ", ","))
			same = true
			continue
		}
		same = false
		if !endsRaw && c.coin(5) {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if c.coin(5) {
		b.WriteString(strings.Repeat(c.unit, lvl) + c.commentLine() + "\n")
	}
}

func (c *fuzzConsumer) genLKALDoc(model Record) []byte {
	c.unit = c.pick("  ", "    ", "\t")
	var b strings.Builder
	if c.coin(3) {
		b.WriteString(c.commentLine() + "\n")
		if c.coin(2) {
			b.WriteString("\n")
		}
	}
	b.WriteString(`@lkal "0.1"` + "\n")
	c.bodyText(&b, model, 0)
	src := b.String()
	if c.coin(4) {
		src = strings.ReplaceAll(src, "\n", "\r\n")
	}
	return []byte(src)
}

func (c *fuzzConsumer) jsonHex4(r rune) string {
	if c.coin(2) {
		return fmt.Sprintf(`\u%04x`, r)
	}
	return fmt.Sprintf(`\u%04X`, r)
}

func (c *fuzzConsumer) jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			if c.coin(4) {
				b.WriteString(c.jsonHex4(r))
			} else {
				b.WriteString(`\"`)
			}
		case r == '\\':
			if c.coin(4) {
				b.WriteString(c.jsonHex4(r))
			} else {
				b.WriteString(`\\`)
			}
		case r == '/' && c.coin(3):
			b.WriteString(`\/`)
		case r < 0x20:
			short := map[rune]string{'\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}
			if esc, ok := short[r]; ok && c.coin(2) {
				b.WriteString(esc)
			} else {
				b.WriteString(c.jsonHex4(r))
			}
		case r >= 0x10000 && c.coin(2):
			hi, lo := utf16.EncodeRune(r)
			b.WriteString(c.jsonHex4(hi) + c.jsonHex4(lo))
		case r < 0x10000 && c.coin(8):
			b.WriteString(c.jsonHex4(r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (c *fuzzConsumer) jsonWS() string {
	switch c.n(6) {
	case 1:
		return " "
	case 2:
		return "\n"
	case 3:
		return "\r\n "
	case 4:
		return "\t"
	default:
		return ""
	}
}

func (c *fuzzConsumer) writeJSONRecord(b *strings.Builder, fields []Field, head string) {
	b.WriteByte('{')
	b.WriteString(c.jsonWS())
	first := true
	if head != "" {
		b.WriteString(head)
		b.WriteString(c.jsonWS())
		first = false
	}
	for _, f := range fields {
		if !first {
			b.WriteByte(',')
			b.WriteString(c.jsonWS())
		}
		first = false
		b.WriteString(c.jsonQuote(f.Key))
		b.WriteString(c.jsonWS())
		b.WriteByte(':')
		b.WriteString(c.jsonWS())
		c.writeJSONVal(b, f.Value)
		b.WriteString(c.jsonWS())
	}
	b.WriteByte('}')
}

func (c *fuzzConsumer) writeJSONTag(b *strings.Builder, tag, payload string) {
	b.WriteByte('{')
	b.WriteString(c.jsonWS())
	b.WriteString(c.jsonQuote(tag))
	b.WriteString(c.jsonWS())
	b.WriteByte(':')
	b.WriteString(c.jsonWS())
	b.WriteString(payload)
	b.WriteString(c.jsonWS())
	b.WriteByte('}')
}

func (c *fuzzConsumer) writeJSONVal(b *strings.Builder, v Value) {
	switch x := v.(type) {
	case String:
		b.WriteString(c.jsonQuote(string(x)))
	case Int:
		if x == 0 && c.coin(3) {
			b.WriteString("-0")
		} else {
			b.WriteString(strconv.FormatInt(int64(x), 10))
		}
	case Decimal:
		b.WriteString(c.numSpell(string(x)))
	case Bool:
		b.WriteString(strconv.FormatBool(bool(x)))
	case Enum:
		c.writeJSONTag(b, "$enum", c.jsonQuote(string(x)))
	case Quantity:
		num := c.numSpell(x.Number)
		if x.Number == "0" && isRadixPrefix(x.Unit[0]) {
			num = "0"
		}
		c.writeJSONTag(b, "$qty", c.jsonQuote(num+x.Unit))
	case Ref:
		c.writeJSONTag(b, "$ref", c.jsonQuote(c.refText(x)))
	case List:
		b.WriteByte('[')
		b.WriteString(c.jsonWS())
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
				b.WriteString(c.jsonWS())
			}
			c.writeJSONVal(b, e)
			b.WriteString(c.jsonWS())
		}
		b.WriteByte(']')
	case Record:
		c.writeJSONRecord(b, x.Fields, "")
	}
}

func (c *fuzzConsumer) genJSONDoc(model Record) []byte {
	var b strings.Builder
	b.WriteString(c.jsonWS())
	head := c.jsonQuote("$lkal") + c.jsonWS() + ":" + c.jsonWS() + c.jsonQuote("0.1")
	c.writeJSONRecord(&b, model.Fields, head)
	b.WriteString(c.jsonWS())
	return []byte(b.String())
}

func FuzzStructured(f *testing.F) {
	seeds := [][]byte{
		{},
		{0, 1, 2, 3, 4, 5, 6, 7},
		{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		{255, 128, 64, 32, 16, 8, 4, 2, 1, 0},
		[]byte("structure-aware-seed-alpha-0123456789"),
		[]byte("structure-aware-seed-beta-9876543210"),
		[]byte("\x01\x03\x07\x0f\x1f\x3f\x7f\xff\xfe\xfc\xf8\xf0"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		c := newFuzzConsumer(data)
		model := c.genRecord(2, 4)
		lkalSrc := c.genLKALDoc(model)
		jsonSrc := c.genJSONDoc(model)

		// 1. Parse synthesized LKAL document
		fLKAL, err := Parse("struct.lkal", lkalSrc)
		if err != nil {
			t.Fatalf("Parse failed on synthesized valid LKAL:\n%s\nerr: %v", lkalSrc, err)
		}
		if !Equal(fLKAL.Data(), model) || !fuzzSameValue(fLKAL.Data(), model) {
			t.Fatalf("fLKAL.Data() mismatch with model:\ngot:  %#v\nwant: %#v\nsrc:\n%s", fLKAL.Data(), model, lkalSrc)
		}

		// 2. FromJSON on synthesized LKAL-JSON document
		fJSON, err := FromJSON(jsonSrc)
		if err != nil {
			t.Fatalf("FromJSON failed on synthesized valid JSON:\n%s\nerr: %v", jsonSrc, err)
		}
		if !Equal(fJSON.Data(), model) || !fuzzSameValue(fJSON.Data(), model) {
			t.Fatalf("fJSON.Data() mismatch with model:\ngot:  %#v\nwant: %#v\nsrc:\n%s", fJSON.Data(), model, jsonSrc)
		}
		fuzzCheckEqualLaws(t, model, fLKAL.Data(), fJSON.Data())

		// 3. Format & canonical lint on both fLKAL and fJSON
		fmtLKAL, err := Format(lkalSrc)
		if err != nil {
			t.Fatalf("Format(lkalSrc) failed: %v", err)
		}
		if !bytes.Equal(fmtLKAL, fLKAL.Format()) {
			t.Errorf("Format(lkalSrc) != fLKAL.Format()")
		}
		if lintMsg := fuzzCanonicalLint(fmtLKAL); lintMsg != "" {
			t.Errorf("canonical lint failed on Format(lkalSrc): %s\nout:\n%s", lintMsg, fmtLKAL)
		}
		fmtLKAL2, err := Format(fmtLKAL)
		if err != nil || !bytes.Equal(fmtLKAL, fmtLKAL2) {
			t.Errorf("Format(fmtLKAL) not idempotent")
		}

		fmtJSON := fJSON.Format()
		if lintMsg := fuzzCanonicalLint(fmtJSON); lintMsg != "" {
			t.Errorf("canonical lint failed on fJSON.Format(): %s\nout:\n%s", lintMsg, fmtJSON)
		}
		fJSONReparsed, err := Parse("struct.lkal", fmtJSON)
		if err != nil || !fuzzSameValue(fJSONReparsed.Data(), model) {
			t.Errorf("Parse(fJSON.Format()) mismatch: err=%v", err)
		}

		// 4. Commutative ToJSON comparison (false and true) against independent oracle
		for _, plain := range []bool{false, true} {
			j1, err1 := ToJSON(fLKAL, plain)
			j2, err2 := ToJSON(fJSON, plain)
			want, errW := fuzzExpectedJSON("0.1", model, plain)
			if err1 != nil || err2 != nil || errW != nil {
				t.Fatalf("ToJSON(plain=%v) errors: %v / %v / %v", plain, err1, err2, errW)
			}
			if !bytes.Equal(j1, j2) {
				t.Errorf("ToJSON(fLKAL, %v) != ToJSON(fJSON, %v)", plain, plain)
			}
			if !bytes.Equal(j1, want) {
				t.Errorf("ToJSON(plain=%v) != oracle want:\ngot:\n%s\nwant:\n%s", plain, j1, want)
			}
		}

		// 5. Reachable path Lens laws & Delete+Set roundtrip on synthesized document
		var paths []fuzzReachablePath
		fuzzCollectPaths(model, nil, &paths, 8)
		if len(paths) > 0 {
			idx := c.n(len(paths))
			rp := paths[idx]
			p := refBody(Ref(rp.segs))
			if c.coin(2) {
				p = fuzzQuotedPath(rp.segs)
			}
			v, err := fLKAL.Get(p)
			if err != nil || !fuzzSameValue(v, rp.val) {
				t.Fatalf("fLKAL.Get(%q) = %#v, %v; want %#v", p, v, err, rp.val)
			}
			// Delete then Set restoration + Frame condition on disjoint paths
			if err := fLKAL.Delete(p); err != nil {
				t.Fatalf("fLKAL.Delete(%q) failed: %v", p, err)
			}
			for _, other := range paths {
				if fuzzDisjointPaths(rp.segs, other.segs) {
					pOther := refBody(Ref(other.segs))
					gotOther, errOther := fLKAL.Get(pOther)
					if errOther != nil || !fuzzSameValue(gotOther, other.val) {
						t.Errorf("Frame Condition failed on synthesized Delete(%q): disjoint %q changed", p, pOther)
					}
				}
			}
			if err := fLKAL.Set(p, v); err != nil {
				t.Fatalf("fLKAL.Set(%q, v) after Delete failed: %v", p, err)
			}
			if !Equal(fLKAL.Data(), model) {
				t.Errorf("Data() after Delete+Set(%q) not Equal to model", p)
			}
			if lintMsg := fuzzCanonicalLint(fLKAL.Format()); lintMsg != "" {
				t.Errorf("canonical lint after Delete+Set(%q) failed: %s", p, lintMsg)
			}
		}
	})
}
