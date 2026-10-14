package lkaljson_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func jsonTDecodeTokens(data []byte) ([]json.Token, error) {
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

func jsonTDiffJSON(gotBytes, wantBytes []byte) []string {
	gotTokens, err := jsonTDecodeTokens(gotBytes)
	if err != nil {
		return []string{fmt.Sprintf("decoding got: %v", err)}
	}
	wantTokens, err := jsonTDecodeTokens(wantBytes)
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
			diffs = append(diffs, fmt.Sprintf("token %d: got %v, want %v", i, gt, wt))
		}
	}
	if len(gotTokens) != len(wantTokens) {
		diffs = append(diffs, fmt.Sprintf("token count mismatch: got %d, want %d", len(gotTokens), len(wantTokens)))
	}
	return diffs
}

func jsonTCheckOrder(a, b Value) error {
	switch va := a.(type) {
	case Record:
		vb, ok := b.(Record)
		if !ok {
			return fmt.Errorf("type mismatch: %T vs %T", a, b)
		}
		if len(va.Fields) != len(vb.Fields) {
			return fmt.Errorf("record field count: got %d, want %d", len(va.Fields), len(vb.Fields))
		}
		for i := range va.Fields {
			if va.Fields[i].Key != vb.Fields[i].Key {
				return fmt.Errorf("field %d key: got %q, want %q", i, va.Fields[i].Key, vb.Fields[i].Key)
			}
			if err := jsonTCheckOrder(va.Fields[i].Value, vb.Fields[i].Value); err != nil {
				return fmt.Errorf("field %q -> %w", va.Fields[i].Key, err)
			}
		}
	case Collection:
		vb, ok := b.(Collection)
		if !ok {
			return fmt.Errorf("type mismatch: %T vs %T", a, b)
		}
		if len(va.Elems) != len(vb.Elems) {
			return fmt.Errorf("collection elem count: got %d, want %d", len(va.Elems), len(vb.Elems))
		}
		for i := range va.Elems {
			if va.Elems[i].Label != vb.Elems[i].Label {
				return fmt.Errorf("elem %d label: got %q, want %q", i, va.Elems[i].Label, vb.Elems[i].Label)
			}
			if err := jsonTCheckOrder(va.Elems[i].Body, vb.Elems[i].Body); err != nil {
				return fmt.Errorf("elem %q -> %w", va.Elems[i].Label, err)
			}
		}
	case List:
		vb, ok := b.(List)
		if !ok {
			return fmt.Errorf("type mismatch: %T vs %T", a, b)
		}
		if len(va) != len(vb) {
			return fmt.Errorf("list len: got %d, want %d", len(va), len(vb))
		}
		for i := range va {
			if err := jsonTCheckOrder(va[i], vb[i]); err != nil {
				return fmt.Errorf("list[%d] -> %w", i, err)
			}
		}
	}
	return nil
}

func jsonTExtractErrors(el *ErrorList) []string {
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

func jsonTIsKnownJSONErrorCode(code string) bool {
	switch code {
	case "L005", "L105", "L107", "L201", "L204", "L205", "L207", "L301", "L302", "L303", "L304", "L305":
		return true
	default:
		return false
	}
}

func jsonTComputeBlockBraceCol(prefix, unit string, count int) int {
	return len(prefix) + (count-1)*len(unit) + strings.LastIndex(unit, "{") + 1
}

func TestToJSONLayout(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "contract_f_example",
			src:  "@lkal \"0.1\"\na = 1\nb = [1, 2]\nc \"x\" {}\nd {}\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"a\": 1,\n  \"b\": [\n    1,\n    2\n  ],\n  \"c\": {\n    \"$coll\": {\n      \"x\": {}\n    }\n  },\n  \"d\": {}\n}\n",
		},
		{
			name: "empty_body",
			src:  "@lkal \"0.1\"\n",
			want: "{\n  \"$lkal\": \"0.1\"\n}\n",
		},
		{
			name: "unescaped_html_characters",
			src:  "@lkal \"0.1\"\ntext = \"<tag> & 'other'\"\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"text\": \"<tag> & 'other'\"\n}\n",
		},
		{
			name: "string_escaping_set_escape_html_false",
			src:  "@lkal \"0.1\"\ns = \"quote: \\\" backslash: \\\\ newline: \\n tab: \\t u1: \\u{1}\"\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"s\": \"quote: \\\" backslash: \\\\ newline: \\n tab: \\t u1: \\u0001\"\n}\n",
		},
		{
			name: "control_chars_and_unicode_escapes",
			src:  "@lkal \"0.1\"\ns = \"\\u{8}\\u{c}\\u{d}\\u{2028}\\u{2029}\\u{7f}\"\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"s\": \"\\b\\f\\r\u2028\u2029\x7f\"\n}\n",
		},
		{
			name: "escaping_in_keys_and_labels",
			src:  "@lkal \"0.1\"\n\"k\\\"ey\\n\" = 1\ncoll \"l\\\"abel\\n\" {\n  x = 2\n}\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"k\\\"ey\\n\": 1,\n  \"coll\": {\n    \"$coll\": {\n      \"l\\\"abel\\n\": {\n        \"x\": 2\n      }\n    }\n  }\n}\n",
		},
		{
			name: "tag_object_spans_three_lines",
			src:  "@lkal \"0.1\"\nproto = .tcp\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"proto\": {\n    \"$enum\": \"tcp\"\n  }\n}\n",
		},
		{
			name: "list_of_tags_expanded",
			src:  "@lkal \"0.1\"\nitems = [.tcp, .udp]\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"items\": [\n    {\n      \"$enum\": \"tcp\"\n    },\n    {\n      \"$enum\": \"udp\"\n    }\n  ]\n}\n",
		},
		{
			name: "empty_record_and_empty_list",
			src:  "@lkal \"0.1\"\nrec {}\nlst = []\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"rec\": {},\n  \"lst\": []\n}\n",
		},
		{
			name: "nested_records",
			src:  "@lkal \"0.1\"\nrec {\n  inner {\n    x = 1\n  }\n}\n",
			want: "{\n  \"$lkal\": \"0.1\",\n  \"rec\": {\n    \"inner\": {\n      \"x\": 1\n    }\n  }\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.name+".lkal", []byte(tt.src))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if f == nil {
				t.Fatalf("Parse returned nil File")
			}

			got, err := ToJSON(f, false)
			if err != nil {
				t.Fatalf("ToJSON failed: %v", err)
			}

			if string(got) != tt.want {
				t.Errorf("ToJSON layout mismatch:\ngot:\n%s\nwant:\n%s", string(got), tt.want)
			}
		})
	}
}

func TestToJSONPlain(t *testing.T) {
	src := `@lkal "0.1"

str = "hello"
num = 42
dec = 0.25
flag = true
proto = .tcp
mem = 512Mi
target = &labels/"app.kubernetes.io/name"
target_ctrl = &a/"\u{1}"
items = [1, 2]
rec {
  x = 1
}
coll "item" {
  sub "child" {
    val = "ok"
  }
}
`

	want := "{\n  \"str\": \"hello\",\n  \"num\": 42,\n  \"dec\": 0.25,\n  \"flag\": true,\n  \"proto\": \"tcp\",\n  \"mem\": \"512Mi\",\n  \"target\": \"labels/\\\"app.kubernetes.io/name\\\"\",\n  \"target_ctrl\": \"a/\\\"\\\\u{1}\\\"\",\n  \"items\": [\n    1,\n    2\n  ],\n  \"rec\": {\n    \"x\": 1\n  },\n  \"coll\": {\n    \"item\": {\n      \"sub\": {\n        \"child\": {\n          \"val\": \"ok\"\n        }\n      }\n    }\n  }\n}\n"

	f, err := Parse("plain.lkal", []byte(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if f == nil {
		t.Fatalf("Parse returned nil File")
	}

	got, err := ToJSON(f, true)
	if err != nil {
		t.Fatalf("ToJSON(plain=true) failed: %v", err)
	}

	if string(got) != want {
		t.Errorf("plain JSON exact bytes mismatch:\ngot:\n%s\nwant:\n%s", string(got), want)
	}
}

func TestToJSONNil(t *testing.T) {
	for _, plain := range []bool{false, true} {
		t.Run(fmt.Sprintf("plain_%v", plain), func(t *testing.T) {
			got, err := ToJSON(nil, plain)
			if err == nil {
				t.Fatalf("ToJSON(nil, %v) succeeded unexpectedly, want error", plain)
			}
			if got != nil {
				t.Errorf("ToJSON(nil, %v) returned non-nil bytes: %v", plain, got)
			}
		})
	}
}
