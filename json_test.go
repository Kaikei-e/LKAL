package lkal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
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
	case "L005", "L105", "L201", "L204", "L205", "L207", "L301", "L302", "L303", "L304", "L305":
		return true
	default:
		return false
	}
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

func TestToJSONValues(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantJSON string
	}{
		{
			name:     "string",
			src:      "@lkal \"0.1\"\nstr = \"hello\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"str\": \"hello\"\n}\n",
		},
		{
			name:     "int_positive",
			src:      "@lkal \"0.1\"\nnum = 42\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"num\": 42\n}\n",
		},
		{
			name:     "int_negative",
			src:      "@lkal \"0.1\"\nnum = -42\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"num\": -42\n}\n",
		},
		{
			name:     "decimal_2_0",
			src:      "@lkal \"0.1\"\ndec = 2.0\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": 2.0\n}\n",
		},
		{
			name:     "decimal_neg_1_5",
			src:      "@lkal \"0.1\"\ndec = -1.5\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": -1.5\n}\n",
		},
		{
			name:     "decimal_0_25",
			src:      "@lkal \"0.1\"\ndec = 0.25\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"dec\": 0.25\n}\n",
		},
		{
			name:     "bool",
			src:      "@lkal \"0.1\"\nb = true\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"b\": true\n}\n",
		},
		{
			name:     "enum",
			src:      "@lkal \"0.1\"\nproto = .tcp\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"proto\": {\n    \"$enum\": \"tcp\"\n  }\n}\n",
		},
		{
			name:     "quantity_512mi",
			src:      "@lkal \"0.1\"\nmem = 512Mi\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"mem\": {\n    \"$qty\": \"512Mi\"\n  }\n}\n",
		},
		{
			name:     "quantity_1_5gi",
			src:      "@lkal \"0.1\"\nmem = 1.5Gi\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"mem\": {\n    \"$qty\": \"1.5Gi\"\n  }\n}\n",
		},
		{
			name:     "ref_bare_segments",
			src:      "@lkal \"0.1\"\ntarget = &secret/db-creds\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"secret/db-creds\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_with_space",
			src:      "@lkal \"0.1\"\ntarget = &secret/\"db creds\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"secret/\\\"db creds\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_simple_segment_canonical_bare",
			src:      "@lkal \"0.1\"\ntarget = &a/\"b\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/b\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_empty_segment",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_slash_segment",
			src:      "@lkal \"0.1\"\ntarget = &labels/\"app.kubernetes.io/name\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"labels/\\\"app.kubernetes.io/name\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_holding_u0001",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\\u{1}\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\\\u{1}\\\"\"\n  }\n}\n",
		},
		{
			name:     "ref_quoted_segment_holding_raw_tab",
			src:      "@lkal \"0.1\"\ntarget = &a/\"\t\"\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"target\": {\n    \"$ref\": \"a/\\\"\\t\\\"\"\n  }\n}\n",
		},
		{
			name:     "list",
			src:      "@lkal \"0.1\"\nitems = [1, 2]\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"items\": [\n    1,\n    2\n  ]\n}\n",
		},
		{
			name:     "record",
			src:      "@lkal \"0.1\"\nsub {\n  leaf = \"ok\"\n}\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"sub\": {\n    \"leaf\": \"ok\"\n  }\n}\n",
		},
		{
			name:     "collection",
			src:      "@lkal \"0.1\"\nport \"http\" {\n  number = 80\n}\n",
			wantJSON: "{\n  \"$lkal\": \"0.1\",\n  \"port\": {\n    \"$coll\": {\n      \"http\": {\n        \"number\": 80\n      }\n    }\n  }\n}\n",
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

			diffs := jsonTDiffJSON(got, []byte(tt.wantJSON))
			if len(diffs) > 0 {
				t.Errorf("token differences found:\n%s\ngot:\n%s\nwant:\n%s",
					strings.Join(diffs, "\n"), string(got), tt.wantJSON)
			}
			if string(got) != tt.wantJSON {
				t.Errorf("exact bytes mismatch:\ngot:\n%s\nwant:\n%s", string(got), tt.wantJSON)
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

func TestFromJSONValid(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVersion string
		wantRecord  Record
	}{
		{
			name:        "scalars_all_types",
			input:       `{"$lkal": "0.1", "s": "hello", "i": 42, "d": 0.25, "b": true, "proto": {"$enum": "tcp"}, "mem": {"$qty": "512Mi"}, "ref": {"$ref": "secret/db-creds"}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "s", Value: String("hello")},
					{Key: "i", Value: Int(42)},
					{Key: "d", Value: Decimal("0.25")},
					{Key: "b", Value: Bool(true)},
					{Key: "proto", Value: Enum("tcp")},
					{Key: "mem", Value: Quantity{Number: "512", Unit: "Mi"}},
					{Key: "ref", Value: Ref{"secret", "db-creds"}},
				},
			},
		},
		{
			name:        "number_normalizations",
			input:       `{"$lkal": "0.1", "d1": 1.50, "d2": 2.00, "i1": -0, "d3": -0.0, "max_int": 9223372036854775807}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "d1", Value: Decimal("1.5")},
					{Key: "d2", Value: Decimal("2.0")},
					{Key: "i1", Value: Int(0)},
					{Key: "d3", Value: Decimal("0.0")},
					{Key: "max_int", Value: Int(9223372036854775807)},
				},
			},
		},
		{
			name:        "quantity_normalizations",
			input:       `{"$lkal": "0.1", "q1": {"$qty": "1.50Gi"}, "q2": {"$qty": "-0Mi"}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "q1", Value: Quantity{Number: "1.5", Unit: "Gi"}},
					{Key: "q2", Value: Quantity{Number: "0", Unit: "Mi"}},
				},
			},
		},
		{
			name:        "ref_with_quoted_segments",
			input:       `{"$lkal": "0.1", "r": {"$ref": "a/\"b c\""}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "r", Value: Ref{"a", "b c"}},
				},
			},
		},
		{
			name:        "ref_decoded_escape",
			input:       `{"$lkal": "0.1", "r": {"$ref": "\"\\u{41}\""}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "r", Value: Ref{"A"}},
				},
			},
		},
		{
			name:        "decimal_100_digits_accepted",
			input:       `{"$lkal": "0.1", "d": ` + strings.Repeat("1", 50) + "." + strings.Repeat("2", 50) + `}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "d", Value: Decimal(strings.Repeat("1", 50) + "." + strings.Repeat("2", 50))},
				},
			},
		},
		{
			name:        "collection_with_dollar_label",
			input:       `{"$lkal": "0.1", "c": {"$coll": {"$special": {"v": 1}}}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "c",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "$special",
									Body: Record{
										Fields: []Field{
											{Key: "v", Value: Int(1)},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "list_of_enums",
			input:       `{"$lkal": "0.1", "enums": [{"$enum": "tcp"}, {"$enum": "udp"}]}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "enums", Value: List{Enum("tcp"), Enum("udp")}},
				},
			},
		},
		{
			name:        "list_of_quantities_different_units",
			input:       `{"$lkal": "0.1", "qtys": [{"$qty": "10m"}, {"$qty": "20s"}]}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "qtys", Value: List{Quantity{Number: "10", Unit: "m"}, Quantity{Number: "20", Unit: "s"}}},
				},
			},
		},
		{
			name:        "empty_list",
			input:       `{"$lkal": "0.1", "items": []}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "items", Value: List{}},
				},
			},
		},
		{
			name:        "empty_record",
			input:       `{"$lkal": "0.1", "rec": {}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "rec", Value: Record{}},
				},
			},
		},
		{
			name:        "nested_collections",
			input:       `{"$lkal": "0.1", "outer": {"$coll": {"first": {"inner": {"$coll": {"nested": {"v": 10}}}}}}}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "outer",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "first",
									Body: Record{
										Fields: []Field{
											{
												Key: "inner",
												Value: Collection{
													Elems: []Elem{
														{
															Label: "nested",
															Body: Record{
																Fields: []Field{
																	{Key: "v", Value: Int(10)},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:        "crlf_and_tab_whitespace",
			input:       "{\r\n\t\"$lkal\":\t\"0.1\",\r\n\t\"a\":\t1\r\n}\r\n",
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "a", Value: Int(1)},
				},
			},
		},
		{
			name:        "emoji_surrogate_escape",
			input:       `{"$lkal": "0.1", "e": "\uD83D\uDE00"}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{Key: "e", Value: String("😀")},
				},
			},
		},
		{
			name:        "members_after_record_key_order_preserved",
			input:       `{"$lkal": "0.1", "rec": {"b": 2, "a": 1}, "after": 3}`,
			wantVersion: "0.1",
			wantRecord: Record{
				Fields: []Field{
					{
						Key: "rec",
						Value: Record{
							Fields: []Field{
								{Key: "b", Value: Int(2)},
								{Key: "a", Value: Int(1)},
							},
						},
					},
					{Key: "after", Value: Int(3)},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err != nil {
				t.Fatalf("FromJSON returned unexpected error: %v", err)
			}
			if f == nil {
				t.Fatalf("FromJSON returned nil File")
			}

			if gotVer := f.Version(); gotVer != tt.wantVersion {
				t.Errorf("Version mismatch: got %q, want %q", gotVer, tt.wantVersion)
			}

			gotData := f.Data()
			if !Equal(gotData, tt.wantRecord) {
				t.Errorf("Equal(Data(), wantRecord) = false:\ngot:  %#v\nwant: %#v", gotData, tt.wantRecord)
			}

			if err := jsonTCheckOrder(gotData, tt.wantRecord); err != nil {
				t.Errorf("order check failed: %v", err)
			}
		})
	}
}

func TestFromJSONErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErrors  []string
		wantCount20 bool
	}{
		{
			name:       "L301_contract_f_ex1_empty_value",
			input:      `{"$lkal": "0.1", "a": }`,
			wantErrors: []string{"L301 1:23"},
		},
		{
			name:       "L301_contract_f_ex2_unclosed_object",
			input:      `{"$lkal": "0.1", "a": 1`,
			wantErrors: []string{"L301 1:24"},
		},
		{
			name:       "L301_contract_f_ex3_trailing_char",
			input:      `{"$lkal": "0.1"} x`,
			wantErrors: []string{"L301 1:18"},
		},
		{
			name:       "L301_contract_f_ex4_leading_zero",
			input:      `{"$lkal": "0.1", "a": 01}`,
			wantErrors: []string{"L301 1:24"},
		},
		{
			name:       "L301_contract_f_ex5_empty_input",
			input:      ``,
			wantErrors: []string{"L301 1:1"},
		},
		{
			name:       "L301_trailing_comma",
			input:      `{"$lkal": "0.1", "a": 1,}`,
			wantErrors: []string{"L301 1:24"},
		},
		{
			name:       "L301_invalid_utf8_byte",
			input:      "{\"$lkal\": \"0.1\", \"a\": \"\xff\"}",
			wantErrors: []string{"L301 1:24"},
		},
		{
			name:       "L301_utf8_bom",
			input:      "\xEF\xBB\xBF{\"$lkal\": \"0.1\"}",
			wantErrors: []string{"L301 1:1"},
		},
		{
			name:       "L301_unpaired_surrogate_escape",
			input:      `{"$lkal": "0.1", "a": "\uD800"}`,
			wantErrors: []string{"L301 1:24"},
		},
		{
			name:       "L301_raw_control_char_in_string",
			input:      "{\"$lkal\": \"0.1\", \"a\": \"x\x01y\"}",
			wantErrors: []string{"L301 1:25"},
		},
		{
			name:       "L301_truncated_string",
			input:      "{\"$lkal\": \"0.1\", \"a\": \"unclosed",
			wantErrors: []string{"L301 1:32"},
		},
		{
			name:       "L301_end_of_input_with_lf",
			input:      "{\"$lkal\": \"0.1\",\n",
			wantErrors: []string{"L301 2:1"},
		},
		{
			name:       "L301_whitespace_only",
			input:      "  \n ",
			wantErrors: []string{"L301 2:2"},
		},
		{
			name:       "L301_multiline_line3_syntax_error",
			input:      "{\n  \"$lkal\": \"0.1\",\n  \"a\": }\n",
			wantErrors: []string{"L301 3:8"},
		},
		{
			name:       "L302_member_value_null",
			input:      `{"$lkal": "0.1", "a": null}`,
			wantErrors: []string{"L302 1:23"},
		},
		{
			name:       "L302_array_element_null",
			input:      `{"$lkal": "0.1", "a": [null]}`,
			wantErrors: []string{"L302 1:24"},
		},
		{
			name:       "L302_tag_payload_null",
			input:      `{"$lkal": "0.1", "a": {"$enum": null}}`,
			wantErrors: []string{"L302 1:33"},
		},
		{
			name:       "L302_coll_member_null",
			input:      `{"$lkal": "0.1", "c": {"$coll": {"x": null}}}`,
			wantErrors: []string{"L302 1:39"},
		},
		{
			name:       "L303_exp_lowercase",
			input:      `{"$lkal": "0.1", "a": 1e5}`,
			wantErrors: []string{"L303 1:23"},
		},
		{
			name:       "L303_exp_uppercase_negative",
			input:      `{"$lkal": "0.1", "a": 1E-2}`,
			wantErrors: []string{"L303 1:23"},
		},
		{
			name:       "L303_exp_decimal",
			input:      `{"$lkal": "0.1", "a": 1.5e3}`,
			wantErrors: []string{"L303 1:23"},
		},
		{
			name:       "L303_exp_inside_array",
			input:      `{"$lkal": "0.1", "a": [1e5]}`,
			wantErrors: []string{"L303 1:24"},
		},
		{
			name:       "L304_nested_array_in_list",
			input:      `{"$lkal": "0.1", "a": [[]]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L304_plain_object_in_list",
			input:      `{"$lkal": "0.1", "a": [{"x": 1}]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L304_empty_object_in_list",
			input:      `{"$lkal": "0.1", "a": [{}]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L304_coll_object_in_list",
			input:      `{"$lkal": "0.1", "a": [{"$coll": {"x": {}}}]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L304_unknown_tag_in_list",
			input:      `{"$lkal": "0.1", "a": [{"$foo": 1}]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L304_dropped_nested_null_not_checked",
			input:      `{"$lkal": "0.1", "a": [[null]]}`,
			wantErrors: []string{"L304 1:24"},
		},
		{
			name:       "L305_enum_payload_not_string",
			input:      `{"$lkal": "0.1", "a": {"$enum": 123}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_enum_empty_string",
			input:      `{"$lkal": "0.1", "a": {"$enum": ""}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_bad_enum_name",
			input:      `{"$lkal": "0.1", "a": {"$enum": "1bad"}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_qty_payload_not_string",
			input:      `{"$lkal": "0.1", "a": {"$qty": 123}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_qty_no_unit",
			input:      `{"$lkal": "0.1", "a": {"$qty": "512"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_qty_leading_zero",
			input:      `{"$lkal": "0.1", "a": {"$qty": "01Mi"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_qty_trailing_dot",
			input:      `{"$lkal": "0.1", "a": {"$qty": "1.Mi"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_qty_leading_space",
			input:      `{"$lkal": "0.1", "a": {"$qty": " 1Mi"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_qty_digits_after_unit",
			input:      `{"$lkal": "0.1", "a": {"$qty": "1h30m"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_ref_payload_not_string",
			input:      `{"$lkal": "0.1", "a": {"$ref": 123}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_ref_empty_string",
			input:      `{"$lkal": "0.1", "a": {"$ref": ""}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_ref_trailing_slash",
			input:      `{"$lkal": "0.1", "a": {"$ref": "a/"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_ref_bad_escape",
			input:      `{"$lkal": "0.1", "a": {"$ref": "\"\\q\""}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_malformed_ref",
			input:      `{"$lkal": "0.1", "a": {"$ref": "a//b"}}`,
			wantErrors: []string{"L305 1:32"},
		},
		{
			name:       "L305_second_member_in_tag",
			input:      `{"$lkal": "0.1", "a": {"$enum": "tcp", "extra": 1}}`,
			wantErrors: []string{"L305 1:40"},
		},
		{
			name:       "L305_second_tag_key_in_tag_object",
			input:      `{"$lkal": "0.1", "a": {"$enum": "a", "$qty": "1m"}}`,
			wantErrors: []string{"L305 1:38"},
		},
		{
			name:       "L305_dropped_tag_extra_not_checked",
			input:      `{"$lkal": "0.1", "a": {"$enum": 1, "x": null}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_coll_payload_not_object",
			input:      `{"$lkal": "0.1", "c": {"$coll": []}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_coll_empty",
			input:      `{"$lkal": "0.1", "c": {"$coll": {}}}`,
			wantErrors: []string{"L305 1:33"},
		},
		{
			name:       "L305_coll_member_not_object",
			input:      `{"$lkal": "0.1", "c": {"$coll": {"x": 1}}}`,
			wantErrors: []string{"L305 1:39"},
		},
		{
			name:       "L305_coll_member_tag_object",
			input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {"$enum": "tcp"}}}}`,
			wantErrors: []string{"L305 1:39"},
		},
		{
			name:       "L005_int64_overflow",
			input:      `{"$lkal": "0.1", "a": 9223372036854775808}`,
			wantErrors: []string{"L005 1:23"},
		},
		{
			name:       "L005_101_digit_integer",
			input:      `{"$lkal": "0.1", "a": ` + strings.Repeat("9", 101) + `}`,
			wantErrors: []string{"L005 1:23"},
		},
		{
			name:       "L005_101_digit_decimal",
			input:      `{"$lkal": "0.1", "a": ` + strings.Repeat("1", 50) + "." + strings.Repeat("2", 51) + `}`,
			wantErrors: []string{"L005 1:23"},
		},
		{
			name:       "L005_101_digit_qty",
			input:      `{"$lkal": "0.1", "a": {"$qty": "` + strings.Repeat("9", 101) + `Mi"}}`,
			wantErrors: []string{"L005 1:32"},
		},
		{
			name:       "L105_top_level_array",
			input:      `[]`,
			wantErrors: []string{"L105 1:1"},
		},
		{
			name:       "L105_top_level_string",
			input:      `"hello"`,
			wantErrors: []string{"L105 1:1"},
		},
		{
			name:       "L105_top_level_number",
			input:      `123`,
			wantErrors: []string{"L105 1:1"},
		},
		{
			name:       "L105_first_key_not_lkal",
			input:      `{"a": 1}`,
			wantErrors: []string{"L105 1:2"},
		},
		{
			name:       "L105_empty_object",
			input:      `{}`,
			wantErrors: []string{"L105 1:2"},
		},
		{
			name:       "L105_lkal_not_string",
			input:      `{"$lkal": 1}`,
			wantErrors: []string{"L105 1:11"},
		},
		{
			name:       "L105_repeated_lkal",
			input:      `{"$lkal": "0.1", "$lkal": "0.1"}`,
			wantErrors: []string{"L105 1:18"},
		},
		{
			name:       "L105_first_key_not_lkal_then_null",
			input:      `{"a": null}`,
			wantErrors: []string{"L105 1:2", "L302 1:7"},
		},
		{
			name:       "L207_unsupported_version_02",
			input:      `{"$lkal": "0.2"}`,
			wantErrors: []string{"L207 1:11"},
		},
		{
			name:       "L207_unsupported_version_010",
			input:      `{"$lkal": "0.10"}`,
			wantErrors: []string{"L207 1:11"},
		},
		{
			name:       "L201_duplicate_key",
			input:      `{"$lkal": "0.1", "a": 1, "a": 2}`,
			wantErrors: []string{"L201 1:26"},
		},
		{
			name:       "L201_duplicate_label_in_coll",
			input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {}, "x": {}}}}`,
			wantErrors: []string{"L201 1:43"},
		},
		{
			name:       "L204_mixed_list",
			input:      `{"$lkal": "0.1", "a": [1, "two"]}`,
			wantErrors: []string{"L204 1:27"},
		},
		{
			name:       "L204_int_and_decimal",
			input:      `{"$lkal": "0.1", "a": [1, 1.5]}`,
			wantErrors: []string{"L204 1:27"},
		},
		{
			name:       "L204_suppressed_after_null",
			input:      `{"$lkal": "0.1", "a": [1, null]}`,
			wantErrors: []string{"L302 1:27"},
		},
		{
			name:       "L204_suppressed_after_l303",
			input:      `{"$lkal": "0.1", "a": [1, 1e5]}`,
			wantErrors: []string{"L303 1:27"},
		},
		{
			name:       "L204_suppressed_after_l304",
			input:      `{"$lkal": "0.1", "a": [1, []]}`,
			wantErrors: []string{"L304 1:27"},
		},
		{
			name:       "L204_suppressed_after_l305",
			input:      `{"$lkal": "0.1", "a": [1, {"$enum": 123}]}`,
			wantErrors: []string{"L305 1:37"},
		},
		{
			name:       "L205_dollar_key_non_first",
			input:      `{"$lkal": "0.1", "a": 1, "$extra": 2}`,
			wantErrors: []string{"L205 1:26"},
		},
		{
			name:       "L205_nested_record_first_dollar_key",
			input:      `{"$lkal": "0.1", "rec": {"$schema": 1}}`,
			wantErrors: []string{"L205 1:26"},
		},
		{
			name:       "L205_tag_name_in_non_first_position",
			input:      `{"$lkal": "0.1", "a": 1, "$enum": "tcp"}`,
			wantErrors: []string{"L205 1:26"},
		},
		{
			name:       "L205_dollar_key_precedes_l201_duplicate",
			input:      `{"$lkal": "0.1", "$a": 1, "$a": 2}`,
			wantErrors: []string{"L205 1:18", "L205 1:27"},
		},
		{
			name:       "L205_coll_member_nested_schema_key",
			input:      `{"$lkal": "0.1", "c": {"$coll": {"x": {"$schema": 1}}}}`,
			wantErrors: []string{"L205 1:40"},
		},
		{
			name:       "several_errors_in_order",
			input:      `{"$lkal": "0.1", "a": 1e5, "b": null}`,
			wantErrors: []string{"L303 1:23", "L302 1:33"},
		},
		{
			name:       "errors_before_syntax_error_kept",
			input:      `{"$lkal": "0.1", "a": null, "b": }`,
			wantErrors: []string{"L302 1:23", "L301 1:34"},
		},
		{
			name:       "multiline_and_unicode_column_count",
			input:      "{\n  \"$lkal\": \"0.1\",\n  \"名前\": null\n}",
			wantErrors: []string{"L302 3:9"},
		},
		{
			name:        "error_cap_25_nulls",
			input:       `{"$lkal": "0.1", "a": [null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null]}`,
			wantCount20: true,
		},
		{
			name:       "L301_escape_unknown_char",
			input:      `{"a":"\q"}`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_escape_hex_cutoff",
			input:      `{"a":"\u1x`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_escape_unpaired_high_surrogate",
			input:      `{"a":"\uD800"}`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_escape_surrogate_cutoff",
			input:      `{"a":"\uD800`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_escape_surrogate_bad_low_hex",
			input:      `{"a":"\uD800\uDC0G"}`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_escape_unpaired_low_surrogate",
			input:      `{"a":"\uDC00"}`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_invalid_utf8_in_string",
			input:      "{\"a\":\"x\xffy\"}",
			wantErrors: []string{"L105 1:2", "L301 1:8"},
		},
		{
			name:       "L301_utf8_sequence_cutoff_at_end",
			input:      "{\"a\":\"\xe6\x97",
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_trailing_comma_in_array",
			input:      `[1,]`,
			wantErrors: []string{"L105 1:1", "L301 1:3"},
		},
		{
			name:       "L301_leading_comma_in_array",
			input:      `[,1]`,
			wantErrors: []string{"L105 1:1", "L301 1:2"},
		},
		{
			name:       "L301_consecutive_commas_in_object",
			input:      `{"a":1,,}`,
			wantErrors: []string{"L105 1:2", "L301 1:8"},
		},
		{
			name:       "L301_partial_literal_tru_at_end",
			input:      `{"a":tru`,
			wantErrors: []string{"L105 1:2", "L301 1:9"},
		},
		{
			name:       "L301_partial_literal_tRue",
			input:      `{"a":tRue}`,
			wantErrors: []string{"L105 1:2", "L301 1:7"},
		},
		{
			name:       "L301_partial_literal_nul_before_brace",
			input:      `{"a":nul}`,
			wantErrors: []string{"L105 1:2", "L301 1:9"},
		},
		{
			name:       "L301_unterminated_string_at_end",
			input:      `{"a":"abc`,
			wantErrors: []string{"L105 1:2", "L301 1:10"},
		},
		{
			name:       "L301_top_level_invalid_char_x",
			input:      `x`,
			wantErrors: []string{"L301 1:1"},
		},
		{
			name:       "L301_top_level_invalid_char_close_brace",
			input:      `}`,
			wantErrors: []string{"L301 1:1"},
		},
		{
			name:       "L105_L205_first_key_dollar_x",
			input:      `{"$x": null}`,
			wantErrors: []string{"L105 1:2", "L205 1:2"},
		},
		{
			name:       "L105_misplaced_lkal_after_data",
			input:      `{"a": 1, "$lkal": "0.1"}`,
			wantErrors: []string{"L105 1:2", "L105 1:10"},
		},
		{
			name:       "L105_top_level_null",
			input:      `null`,
			wantErrors: []string{"L105 1:1"},
		},
		{
			name:       "L105_lkal_value_null",
			input:      `{"$lkal": null}`,
			wantErrors: []string{"L105 1:11"},
		},
		{
			name:       "L302_coll_nested_null_members",
			input:      `{"$lkal":"0.1","c":{"$coll":{"e1":{"x":null,"y":null},"e2":{"z":null}}}}`,
			wantErrors: []string{"L302 1:40", "L302 1:49", "L302 1:65"},
		},
		{
			name:       "L305_L301_enum_array_with_syntax_error",
			input:      `{"$lkal":"0.1","a":{"$enum":[1,}}`,
			wantErrors: []string{"L305 1:29", "L301 1:32"},
		},
		{
			name:       "L105_L301_lkal_array_with_syntax_error",
			input:      `{"$lkal": [1,}`,
			wantErrors: []string{"L105 1:11", "L301 1:14"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err == nil {
				t.Fatalf("expected error from FromJSON, got nil error, file: %v", f)
			}
			if f != nil {
				t.Errorf("expected nil File on error, got: %v", f)
			}

			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}

			got := jsonTExtractErrors(errList)
			if tt.wantCount20 {
				if len(got) != 20 {
					t.Fatalf("expected exactly 20 errors, got %d: %v", len(got), got)
				}
				for i, e := range errList.Errors {
					if e == nil {
						t.Fatalf("expected non-nil error at index %d", i)
					}
					if e.Code != "L302" {
						t.Errorf("expected L302 at error[%d], got %s", i, e.Code)
					}
				}
			} else {
				if !reflect.DeepEqual(got, tt.wantErrors) {
					t.Errorf("error mismatch:\ngot:  %v\nwant: %v", got, tt.wantErrors)
				}
			}
		})
	}
}

func TestFromJSONErrorDetails(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "L301_details",
			input:    `{"$lkal": "0.1", "a": }`,
			wantCode: "L301",
			wantMsg:  "invalid JSON",
			wantHint: "fix the JSON syntax",
		},
		{
			name:     "L302_details",
			input:    `{"$lkal": "0.1", "a": null}`,
			wantCode: "L302",
			wantMsg:  "null value",
			wantHint: "omit the key instead of writing null",
		},
		{
			name:     "L303_details",
			input:    `{"$lkal": "0.1", "a": 1e5}`,
			wantCode: "L303",
			wantMsg:  "number in exponent notation",
			wantHint: "write the number without an exponent",
		},
		{
			name:     "L304_details",
			input:    `{"$lkal": "0.1", "a": [[]]}`,
			wantCode: "L304",
			wantMsg:  "list element is not a scalar",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "L305_second_member_hint",
			input:    `{"$lkal": "0.1", "a": {"$enum": "tcp", "extra": 1}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: "write a tag object with only one key",
		},
		{
			name:     "L305_enum_hint",
			input:    `{"$lkal": "0.1", "a": {"$enum": 123}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the enum name as a string, for example {"$enum": "tcp"}`,
		},
		{
			name:     "L305_qty_hint",
			input:    `{"$lkal": "0.1", "a": {"$qty": "512"}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the quantity as a string, for example {"$qty": "512Mi"}`,
		},
		{
			name:     "L305_ref_hint",
			input:    `{"$lkal": "0.1", "a": {"$ref": "a//b"}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write the reference as a string, for example {"$ref": "secret/db-creds"}`,
		},
		{
			name:     "L305_coll_hint",
			input:    `{"$lkal": "0.1", "c": {"$coll": {}}}`,
			wantCode: "L305",
			wantMsg:  "malformed tag",
			wantHint: `write {"$coll": {"label": {...}}} with at least one element`,
		},
		{
			name:     "L105_json_hint",
			input:    `{"a": 1}`,
			wantCode: "L105",
			wantMsg:  "missing, misplaced or repeated version declaration",
			wantHint: `start the top-level object with "$lkal": "0.1"`,
		},
		{
			name:     "L005_details",
			input:    `{"$lkal": "0.1", "a": 9223372036854775808}`,
			wantCode: "L005",
			wantMsg:  "number out of range",
			wantHint: "keep integers within int64 and numbers within 100 digits",
		},
		{
			name:     "L201_details",
			input:    `{"$lkal": "0.1", "a": 1, "a": 2}`,
			wantCode: "L201",
			wantMsg:  "duplicate key or label",
			wantHint: "remove one of them",
		},
		{
			name:     "L204_details",
			input:    `{"$lkal": "0.1", "a": [1, "two"]}`,
			wantCode: "L204",
			wantMsg:  "list mixes types",
			wantHint: "use one type for all elements",
		},
		{
			name:     "L205_details",
			input:    `{"$lkal": "0.1", "$foo": 1}`,
			wantCode: "L205",
			wantMsg:  "key starts with $",
			wantHint: "rename the key",
		},
		{
			name:     "L207_details",
			input:    `{"$lkal": "0.2"}`,
			wantCode: "L207",
			wantMsg:  "unsupported version",
			wantHint: `this implementation accepts only "0.1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := FromJSON([]byte(tt.input))
			if err == nil {
				t.Fatalf("expected error from FromJSON, got nil error, file: %v", f)
			}
			if f != nil {
				t.Errorf("expected nil File on error, got: %v", f)
			}

			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if errList == nil || len(errList.Errors) == 0 {
				t.Fatalf("expected non-empty ErrorList")
			}

			first := errList.Errors[0]
			if first == nil {
				t.Fatalf("expected non-nil first error")
			}
			if first.File != "" {
				t.Errorf("File mismatch: got %q, want %q", first.File, "")
			}
			if first.Code != tt.wantCode {
				t.Errorf("Code mismatch: got %q, want %q", first.Code, tt.wantCode)
			}
			if first.Msg != tt.wantMsg {
				t.Errorf("Msg mismatch: got %q, want %q", first.Msg, tt.wantMsg)
			}
			if first.Hint != tt.wantHint {
				t.Errorf("Hint mismatch: got %q, want %q", first.Hint, tt.wantHint)
			}
		})
	}

	t.Run("render_exact_example", func(t *testing.T) {
		f, err := FromJSON([]byte(`{"$lkal": "0.1", "a": }`))
		if err == nil {
			t.Fatalf("expected error, got nil file: %v", f)
		}
		want := "1:23: L301 invalid JSON\n  hint: fix the JSON syntax"
		if got := err.Error(); got != want {
			t.Errorf("Error() rendering mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})
}

func TestJSONTypedNilErrorList(t *testing.T) {
	var el *ErrorList
	if res := jsonTExtractErrors(el); res != nil {
		t.Errorf("expected nil result for typed-nil ErrorList, got %v", res)
	}
}

func TestFromJSONDepthLimit(t *testing.T) {
	tests := []struct {
		name       string
		bracketN   int
		wantL304   bool
		wantL301   bool
		wantErrors []string
		wantHint   string
		wantCol    int
	}{
		{
			name:       "depth_10000_ok",
			bracketN:   9999,
			wantL304:   true,
			wantErrors: []string{"L304 1:21"},
		},
		{
			name:     "depth_10001_exceeds_limit",
			bracketN: 10000,
			wantL301: true,
			wantCol:  10019,
			wantHint: "nest objects and arrays at most 10000 levels deep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{"$lkal":"0.1","a":` + strings.Repeat("[", tt.bracketN) + strings.Repeat("]", tt.bracketN) + `}`
			f, err := FromJSON([]byte(input))
			if f != nil {
				t.Errorf("expected nil File, got: %v", f)
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}

			if tt.wantL304 {
				got := jsonTExtractErrors(errList)
				if !reflect.DeepEqual(got, tt.wantErrors) {
					t.Errorf("error mismatch:\ngot:  %v\nwant: %v", got, tt.wantErrors)
				}
			}

			if tt.wantL301 {
				var foundL301 *Error
				for _, e := range errList.Errors {
					if e != nil && e.Code == "L301" {
						foundL301 = e
						break
					}
				}
				if foundL301 == nil {
					t.Fatalf("expected L301 in errors, got: %v", jsonTExtractErrors(errList))
				}
				wantPos := Pos{Line: 1, Col: tt.wantCol}
				if foundL301.Pos != wantPos {
					t.Errorf("L301 Pos mismatch: got %v, want %v", foundL301.Pos, wantPos)
				}
				if foundL301.Hint != tt.wantHint {
					t.Errorf("L301 Hint mismatch: got %q, want %q", foundL301.Hint, tt.wantHint)
				}
			}
		})
	}
}

func TestFromJSONManyErrorsLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	buildInput := func(k int) []byte {
		return []byte(`{"$lkal":"0.1","a":[` + strings.Repeat("null,", k) + "null]}")
	}

	checkErrors := func(name string, src []byte) {
		f, err := FromJSON(src)
		if f != nil {
			t.Errorf("%s: FromJSON returned non-nil File on error: %v", name, f)
		}
		if err == nil {
			t.Fatalf("%s: expected error, got nil", name)
		}
		errList, ok := err.(*ErrorList)
		if !ok {
			t.Fatalf("%s: expected *ErrorList, got %T: %v", name, err, err)
		}
		if len(errList.Errors) != 20 {
			t.Fatalf("%s: error count mismatch: got %d, want 20", name, len(errList.Errors))
		}
		for i, e := range errList.Errors {
			if e == nil {
				t.Fatalf("%s: error[%d] is nil", name, i)
			}
			if e.Code != "L302" {
				t.Errorf("%s: error[%d] code mismatch: got %q, want %q", name, i, e.Code, "L302")
			}
		}
	}

	measureMin := func(src []byte) time.Duration {
		var minDur time.Duration
		for i := 0; i < 3; i++ {
			runtime.GC()
			start := time.Now()
			_, _ = FromJSON(src)
			d := time.Since(start)
			if i == 0 || d < minDur {
				minDur = d
			}
		}
		return minDur
	}

	baseN := 20000
	inputN := buildInput(baseN)
	input4N := buildInput(baseN * 4)

	checkErrors("N_20000", inputN)
	checkErrors("4N_80000", input4N)

	durN := measureMin(inputN)
	dur4N := measureMin(input4N)

	if dur4N > 8*durN {
		ratio := float64(dur4N)
		if durN > 0 {
			ratio /= float64(durN)
			t.Errorf("4n duration %v exceeds 8x n duration %v (ratio: %.2fx)", dur4N, durN, ratio)
		} else {
			t.Errorf("4n duration %v exceeds 8x n duration %v", dur4N, durN)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	fixtureFiles, err := filepath.Glob(filepath.Join("testdata", "conformance", "valid", "*.lkal"))
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

	validJSONs, _ := filepath.Glob(filepath.Join("testdata", "conformance", "valid", "*.json"))
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
