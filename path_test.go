package lkal

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func pathTParse(t *testing.T, name, src string) *File {
	t.Helper()
	f, err := Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", name, err)
	}
	if f == nil {
		t.Fatalf("Parse(%q) returned nil File", name)
	}
	return f
}

func pathTCheckEqualAndOrder(t *testing.T, got, want Value) {
	t.Helper()
	if !Equal(got, want) {
		t.Fatalf("values not equal:\ngot:  %#v\nwant: %#v", got, want)
	}
	pathTCheckOrder(t, got, want)
}

func pathTCheckOrder(t *testing.T, got, want Value) {
	t.Helper()
	switch g := got.(type) {
	case Record:
		w, ok := want.(Record)
		if !ok {
			t.Fatalf("type mismatch: got Record, want %T", want)
		}
		if len(g.Fields) != len(w.Fields) {
			t.Fatalf("record fields length mismatch: got %d, want %d", len(g.Fields), len(w.Fields))
		}
		for i := range g.Fields {
			if g.Fields[i].Key != w.Fields[i].Key {
				t.Fatalf("field %d key mismatch: got %q, want %q", i, g.Fields[i].Key, w.Fields[i].Key)
			}
			pathTCheckOrder(t, g.Fields[i].Value, w.Fields[i].Value)
		}
	case List:
		w, ok := want.(List)
		if !ok {
			t.Fatalf("type mismatch: got List, want %T", want)
		}
		if len(g) != len(w) {
			t.Fatalf("list length mismatch: got %d, want %d", len(g), len(w))
		}
		for i := range g {
			pathTCheckOrder(t, g[i], w[i])
		}
	}
}

func TestGet(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	wantPortRec := Record{Fields: []Field{
		{
			Key: "http",
			Value: Record{Fields: []Field{
				{Key: "number", Value: Int(8080)},
				{Key: "protocol", Value: Enum("tcp")},
			}},
		},
		{
			Key: "metrics",
			Value: Record{Fields: []Field{
				{Key: "number", Value: Int(9090)},
				{Key: "protocol", Value: Enum("tcp")},
			}},
		},
	}}

	wantAppRecord := Record{Fields: []Field{
		{Key: "image", Value: String("ghcr.io/acme/web:1.4.2")},
		{Key: "args", Value: List{String("--port"), String("8080")}},
		{Key: "memory", Value: Quantity{Number: "512", Unit: "Mi"}},
		{Key: "cpu", Value: Quantity{Number: "250", Unit: "m"}},
		{Key: "timeout", Value: Quantity{Number: "30", Unit: "s"}},
		{Key: "port", Value: wantPortRec},
		{Key: "env", Value: Record{Fields: []Field{{Key: "LOG_LEVEL", Value: String("info")}}}},
		{Key: "secret", Value: Ref{"secret", "db-creds"}},
	}}

	wantServiceRec := Record{Fields: []Field{
		{
			Key: "web",
			Value: Record{Fields: []Field{
				{Key: "replicas", Value: Int(3)},
				{Key: "traffic-share", Value: Decimal("0.25")},
				{Key: "labels", Value: Record{Fields: []Field{
					{Key: "app.kubernetes.io/name", Value: String("web")},
					{Key: "tier", Value: String("frontend")},
				}}},
				{Key: "container", Value: Record{Fields: []Field{
					{Key: "app", Value: wantAppRecord},
				}}},
				{Key: "startup", Value: String("#!/bin/sh\nexec /app/web --port 8080")},
			}},
		},
	}}

	tests := []struct {
		path string
		want Value
	}{
		{
			path: "service/web/replicas",
			want: Int(3),
		},
		{
			path: "service/web/traffic-share",
			want: Decimal("0.25"),
		},
		{
			path: "service/web/labels/\"app.kubernetes.io/name\"",
			want: String("web"),
		},
		{
			path: "service/web/container/app/port/http/number",
			want: Int(8080),
		},
		{
			path: "service/web/container/app/port",
			want: wantPortRec,
		},
		{
			path: "service/web/container/app",
			want: wantAppRecord,
		},
		{
			path: "service",
			want: wantServiceRec,
		},
		{
			path: "service/web/startup",
			want: String("#!/bin/sh\nexec /app/web --port 8080"),
		},
		{
			path: "service/web/container/app/secret",
			want: Ref{"secret", "db-creds"},
		},
		{
			path: "service/web/container/app/args",
			want: List{String("--port"), String("8080")},
		},
		{
			path: "\"service\"/web/replicas",
			want: Int(3),
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			val, err := f.Get(tc.path)
			if err != nil {
				t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
			}
			if val == nil {
				t.Fatalf("Get(%q) returned nil Value", tc.path)
			}
			pathTCheckEqualAndOrder(t, val, tc.want)
		})
	}
}

func TestGetErrors(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	const malformedHint = "separate segments with / and quote segments that contain other characters"

	tests := []struct {
		name         string
		file         *File
		fileName     string
		path         string
		wantCode     string
		wantMsg      string
		wantHint     string
		wantErrorStr string
	}{
		{
			name:     "empty_path",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "leading_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "/a",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "trailing_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "double_slash",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a//b",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "space_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a b",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "unclosed_quote",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "\"open",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "invalid_escape_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/\"\\q\"",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "invalid_character_in_segment",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "a/%",
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: malformedHint,
		},
		{
			name:     "missing_key_in_root",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "missing_key_in_record",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "missing_label_in_collection",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/missing",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"missing\"",
		},
		{
			name:     "segment_after_scalar",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/replicas/x",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"x\"",
		},
		{
			name:     "segment_after_list",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/web/container/app/args/x",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"x\"",
		},
		{
			name:     "l402_escaped_segment_in_hint",
			file:     f,
			fileName: "spec_example.lkal",
			path:     "service/\"x\\u{41}\"",
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"xA\"",
		},
		{
			name:         "exact_error_string_api",
			file:         pathTParse(t, "web.lkal", "@lkal \"0.1\"\n"),
			fileName:     "web.lkal",
			path:         "api",
			wantCode:     "L402",
			wantMsg:      "path not found",
			wantHint:     "check the path at segment \"api\"",
			wantErrorStr: "web.lkal: L402 path not found\n  hint: check the path at segment \"api\"",
		},
		{
			name:         "l402_empty_file_name_from_json_style",
			file:         pathTParse(t, "", "@lkal \"0.1\"\n"),
			fileName:     "",
			path:         "api",
			wantCode:     "L402",
			wantMsg:      "path not found",
			wantHint:     "check the path at segment \"api\"",
			wantErrorStr: "L402 path not found\n  hint: check the path at segment \"api\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := tc.file.Get(tc.path)
			if err == nil {
				t.Fatalf("Get(%q) succeeded unexpectedly, got %v", tc.path, val)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("Get(%q) error type = %T, want *ErrorList", tc.path, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("Get(%q) error count = %d, want 1", tc.path, len(errList.Errors))
			}
			e := errList.Errors[0]
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if e.File != tc.fileName {
				t.Errorf("File = %q, want %q", e.File, tc.fileName)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos = %v, want zero Pos", e.Pos)
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			if tc.wantErrorStr != "" && e.Error() != tc.wantErrorStr {
				t.Errorf("Error() =\n%q\nwant:\n%q", e.Error(), tc.wantErrorStr)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		path       string
		value      Value
		wantFormat string
		wantGet    Value
	}{
		{
			name: "replace_scalar_keeping_comments_and_blank_line",
			src: `@lkal "0.1"

first = 1

### count comment
count = 2
`,
			path:  "count",
			value: Int(10),
			wantFormat: `@lkal "0.1"

first = 1

### count comment
count = 10
`,
		},
		{
			name: "add_new_key_appended_after_last_entry_before_trailing_comments",
			src: `@lkal "0.1"

a = 1

### trailing note
`,
			path:  "b",
			value: Int(2),
			wantFormat: `@lkal "0.1"

a = 1
b = 2

### trailing note
`,
		},
		{
			name: "replace_assignment_by_record",
			src: `@lkal "0.1"

a = 1
`,
			path: "a",
			value: Record{Fields: []Field{
				{Key: "x", Value: Int(10)},
			}},
			wantFormat: `@lkal "0.1"

a {
  x = 10
}
`,
		},
		{
			name: "replace_record_by_scalar",
			src: `@lkal "0.1"

a {
  x = 10
}
`,
			path:  "a",
			value: Int(42),
			wantFormat: `@lkal "0.1"

a = 42
`,
		},
		{
			name: "multiline_list_replaced_by_list_stays_multiline",
			src: `@lkal "0.1"

items = [
  1,
  2,
]
`,
			path:  "items",
			value: List{Int(3), Int(4)},
			wantFormat: `@lkal "0.1"

items = [
  3,
  4,
]
`,
		},
		{
			name: "oneline_record_replaced_by_record_stays_oneline_when_qualifies",
			src: `@lkal "0.1"

config { x = 1, y = 2 }
`,
			path: "config",
			value: Record{Fields: []Field{
				{Key: "a", Value: Int(10)},
				{Key: "b", Value: String("hi")},
			}},
			wantFormat: `@lkal "0.1"

config { a = 10, b = "hi" }
`,
		},
		{
			name: "replace_labeled_entries_by_scalar",
			src: `@lkal "0.1"

item "one" {
  x = 1
}
item "two" {
  x = 2
}
trailing = true
`,
			path:  "item",
			value: String("single"),
			wantFormat: `@lkal "0.1"

item = "single"
trailing = true
`,
		},
		{
			name: "set_nested_record_value",
			src: `@lkal "0.1"

item = "single"
trailing = true
`,
			path: "item",
			value: Record{Fields: []Field{
				{Key: "first", Value: Record{Fields: []Field{{Key: "x", Value: Int(1)}}}},
				{Key: "second", Value: Record{Fields: []Field{{Key: "y", Value: Int(2)}}}},
			}},
			wantFormat: `@lkal "0.1"

item {
  first {
    x = 1
  }
  second {
    y = 2
  }
}
trailing = true
`,
		},
		{
			name: "add_labeled_entry_when_zero_elements_exist",
			src: `@lkal "0.1"

service "web" {
  replicas = 3
}
`,
			path:  "service/web/port/http",
			value: Record{Fields: []Field{{Key: "number", Value: Int(8080)}}},
			wantFormat: `@lkal "0.1"

service "web" {
  replicas = 3
  port "http" {
    number = 8080
  }
}
`,
		},
		{
			name: "add_labeled_assignment_at_root",
			src: `@lkal "0.1"

a = 1
`,
			path:  "env/LOG_LEVEL",
			value: String("info"),
			wantFormat: `@lkal "0.1"

a = 1
env "LOG_LEVEL" = "info"
`,
		},
		{
			name: "add_element_to_collection_followed_by_another_entry",
			src: `@lkal "0.1"

item "first" {
  x = 1
}
trailing = true
`,
			path:  "item/second",
			value: Record{Fields: []Field{{Key: "y", Value: Int(2)}}},
			wantFormat: `@lkal "0.1"

item "first" {
  x = 1
}
item "second" {
  y = 2
}
trailing = true
`,
		},
		{
			name: "replace_element_body_keeping_comments",
			src: `@lkal "0.1"

### element comment
item "first" {
  x = 1
}
`,
			path:  "item/first",
			value: Record{Fields: []Field{{Key: "x", Value: Int(99)}}},
			wantFormat: `@lkal "0.1"

### element comment
item "first" {
  x = 99
}
`,
		},
		{
			name: "replace_element_body_keeps_oneline",
			src: `@lkal "0.1"

port "http" { number = 8080 }
`,
			path: "port/http",
			value: Record{Fields: []Field{
				{Key: "number", Value: Int(9000)},
			}},
			wantFormat: `@lkal "0.1"

port "http" { number = 9000 }
`,
		},
		{
			name: "replace_record_entry_keeps_entry_comments_and_drops_body_comments",
			src: `@lkal "0.1"

first = 1

### record entry comment
server {
  ### old body comment
  old_field = 1
}
`,
			path: "server",
			value: Record{Fields: []Field{
				{Key: "new_field", Value: Int(2)},
			}},
			wantFormat: `@lkal "0.1"

first = 1

### record entry comment
server {
  new_field = 2
}
`,
		},
		{
			name: "new_list_new_key_is_oneline",
			src: `@lkal "0.1"

a = 1
`,
			path:  "items",
			value: List{Int(1), Int(2)},
			wantFormat: `@lkal "0.1"

a = 1
items = [1, 2]
`,
		},
		{
			name: "new_list_replacing_scalar_is_oneline",
			src: `@lkal "0.1"

items = 42
`,
			path:  "items",
			value: List{Int(1), Int(2)},
			wantFormat: `@lkal "0.1"

items = [1, 2]
`,
		},
		{
			name: "oneline_list_replaced_by_list_stays_oneline",
			src: `@lkal "0.1"

items = [1, 2]
`,
			path:  "items",
			value: List{Int(3), Int(4)},
			wantFormat: `@lkal "0.1"

items = [3, 4]
`,
		},
		{
			name: "set_inside_element_body",
			src: `@lkal "0.1"

service "web" {
  replicas = 3
}
`,
			path:  "service/web/replicas",
			value: Int(5),
			wantFormat: `@lkal "0.1"

service "web" {
  replicas = 5
}
`,
		},
		{
			name: "decimal_stored_and_printed_as_normalized",
			src: `@lkal "0.1"

rate = 1.0
`,
			path:    "rate",
			value:   Decimal("1.50"),
			wantGet: Decimal("1.5"),
			wantFormat: `@lkal "0.1"

rate = 1.5
`,
		},
		{
			name: "quantity_decimal_normalized",
			src: `@lkal "0.1"

mem = 1Gi
`,
			path:    "mem",
			value:   Quantity{Number: "1.50", Unit: "Gi"},
			wantGet: Quantity{Number: "1.5", Unit: "Gi"},
			wantFormat: `@lkal "0.1"

mem = 1.5Gi
`,
		},
		{
			name: "quantity_normalized_inside_record",
			src: `@lkal "0.1"

server {}
`,
			path: "server",
			value: Record{Fields: []Field{
				{Key: "mem", Value: Quantity{Number: "1.50", Unit: "Gi"}},
			}},
			wantGet: Quantity{Number: "1.5", Unit: "Gi"},
			wantFormat: `@lkal "0.1"

server {
  mem = 1.5Gi
}
`,
		},
		{
			name: "quantity_normalized_inside_list",
			src: `@lkal "0.1"

limits = []
`,
			path: "limits",
			value: List{
				Quantity{Number: "1.50", Unit: "Gi"},
				Quantity{Number: "2.00", Unit: "Gi"},
			},
			wantGet: List{
				Quantity{Number: "1.5", Unit: "Gi"},
				Quantity{Number: "2.0", Unit: "Gi"},
			},
			wantFormat: `@lkal "0.1"

limits = [1.5Gi, 2.0Gi]
`,
		},
		{
			name: "string_containing_lf_printed_as_raw_lines",
			src: `@lkal "0.1"

msg = "hello"
`,
			path:  "msg",
			value: String("line 1\nline 2"),
			wantFormat: `@lkal "0.1"

msg =
  \\line 1
  \\line 2
`,
		},
		{
			name: "string_with_lf_inside_list_is_quoted",
			src: `@lkal "0.1"

items = []
`,
			path:  "items",
			value: List{String("a\nb")},
			wantFormat: `@lkal "0.1"

items = ["a\nb"]
`,
		},
		{
			name: "string_with_lf_and_control_char_is_quoted",
			src: `@lkal "0.1"

msg = "old"
`,
			path:  "msg",
			value: String("a\nb\x01"),
			wantFormat: `@lkal "0.1"

msg = "a\nb\u{1}"
`,
		},
		{
			name: "key_with_space_needs_quotes",
			src: `@lkal "0.1"
`,
			path:  "\"db creds\"",
			value: Int(1),
			wantFormat: `@lkal "0.1"

"db creds" = 1
`,
		},
		{
			name: "boolean_word_key_needs_quotes",
			src: `@lkal "0.1"
`,
			path:  "\"true\"",
			value: Int(1),
			wantFormat: `@lkal "0.1"

"true" = 1
`,
		},
		{
			name: "anchor_example",
			src: `@lkal "0.1"

### web
service "web" {
  replicas = 3
  port "http" { number = 8080 }
}
`,
			path: "service/web/port/https",
			value: Record{Fields: []Field{
				{Key: "number", Value: Int(8443)},
			}},
			wantFormat: `@lkal "0.1"

### web
service "web" {
  replicas = 3
  port "http" { number = 8080 }
  port "https" {
    number = 8443
  }
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			err := f.Set(tc.path, tc.value)
			if err != nil {
				t.Fatalf("Set(%q, ...) unexpected error: %v", tc.path, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

			if tc.wantGet != nil {
				getPath := tc.path
				if tc.name == "quantity_normalized_inside_record" {
					getPath = tc.path + "/mem"
				}
				val, getErr := f.Get(getPath)
				if getErr != nil {
					t.Fatalf("Get(%q) unexpected error: %v", getPath, getErr)
				}
				if !Equal(val, tc.wantGet) {
					t.Errorf("Get(%q) = %#v, want %#v", getPath, val, tc.wantGet)
				}
			}
		})
	}
}

func TestSetErrors(t *testing.T) {
	baseSrc := `@lkal "0.1"

a = 1
service "web" {
  replicas = 3
}
`
	tests := []struct {
		name     string
		src      string
		path     string
		value    Value
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "l401_empty_path",
			src:      baseSrc,
			path:     "",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_leading_slash",
			src:      baseSrc,
			path:     "/a",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_space_in_segment",
			src:      baseSrc,
			path:     "a b",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l401_unquoted_dollar_in_path",
			src:      baseSrc,
			path:     "$bad",
			value:    Int(1),
			wantCode: "L401",
			wantMsg:  "malformed path",
			wantHint: "separate segments with / and quote segments that contain other characters",
		},
		{
			name:     "l402_root_parent_is_assignment",
			src:      baseSrc,
			path:     "a/key",
			value:    Int(1),
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"key\"",
		},
		{
			name:     "l402_parent_is_assignment",
			src:      baseSrc,
			path:     "service/web/replicas/key",
			value:    Int(1),
			wantCode: "L402",
			wantMsg:  "path not found",
			wantHint: "check the path at segment \"key\"",
		},
		{
			name:     "l403_nil_top_level",
			src:      baseSrc,
			path:     "a",
			value:    nil,
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_nil_list_element",
			src:      baseSrc,
			path:     "list",
			value:    List{nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_nil_field_value",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "k", Value: nil}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
		{
			name:     "l403_bad_decimal_trailing_dot",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal("1."),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "l403_bad_decimal_no_integer",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal(".5"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "l403_bad_quantity_empty_unit",
			src:      baseSrc,
			path:     "qty",
			value:    Quantity{Number: "1", Unit: ""},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a quantity such as 512Mi",
		},
		{
			name:     "l403_bad_quantity_invalid_number",
			src:      baseSrc,
			path:     "qty",
			value:    Quantity{Number: "x", Unit: "Mi"},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a quantity such as 512Mi",
		},
		{
			name:     "l403_101_digit_decimal",
			src:      baseSrc,
			path:     "dec",
			value:    Decimal("1." + strings.Repeat("0", 100)),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "keep numbers within 100 digits",
		},
		{
			name:     "l403_bad_enum_leading_digit",
			src:      baseSrc,
			path:     "e",
			value:    Enum("1x"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write an enum name that starts with a letter or _",
		},
		{
			name:     "l403_ref_with_no_segments",
			src:      baseSrc,
			path:     "r",
			value:    Ref{},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a reference with at least one segment",
		},
		{
			name:     "l403_invalid_utf8_string",
			src:      baseSrc,
			path:     "s",
			value:    String("\xff"),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_invalid_utf8_key",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "k\xff", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_invalid_utf8_ref_segment",
			src:      baseSrc,
			path:     "r",
			value:    Ref{"s\xff"},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use valid UTF-8 text",
		},
		{
			name:     "l403_list_inside_list",
			src:      baseSrc,
			path:     "list",
			value:    List{List{Int(1)}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "l403_record_inside_list",
			src:      baseSrc,
			path:     "list",
			value:    List{Record{}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use only scalars as list elements",
		},
		{
			name:     "l403_mixed_list",
			src:      baseSrc,
			path:     "list",
			value:    List{Int(1), String("two")},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use one type for all list elements",
		},
		{
			name:     "l403_dollar_key_in_value",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "$bad", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "l403_dollar_key_as_last_path_segment",
			src:      baseSrc,
			path:     "\"$bad\"",
			value:    Int(1),
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "l403_dollar_label_in_path",
			src:      baseSrc,
			path:     "service/\"$admin\"",
			value:    Record{Fields: []Field{{Key: "enabled", Value: Bool(true)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "l403_repeated_key",
			src:      baseSrc,
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "dup", Value: Int(1)}, {Key: "dup", Value: Int(2)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			origFormat := f.Format()
			err := f.Set(tc.path, tc.value)
			if err == nil {
				t.Fatalf("Set(%q, ...) succeeded unexpectedly, want error", tc.path)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("expected exactly 1 error, got %d: %v", len(errList.Errors), errList)
			}
			e := errList.Errors[0]
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos = %v, want zero Pos", e.Pos)
			}
			if e.File != "test.lkal" {
				t.Errorf("File = %q, want %q", e.File, "test.lkal")
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg = %q, want %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			if !bytes.Equal(f.Format(), origFormat) {
				t.Errorf("File format changed after failed Set")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		fileParsedName string
		path           string
		wantFormat     string
		wantErrCode    string
		wantFile       string
		wantMsg        string
		wantHint       string
		wantErrorStr   string
	}{
		{
			name: "remove_assignment_with_comments",
			src: `@lkal "0.1"

### comment on a
a = 1
b = 2
`,
			path: "a",
			wantFormat: `@lkal "0.1"

b = 2
`,
		},
		{
			name: "blank_line_transfer_to_next_entry",
			src: `@lkal "0.1"

a = 1

### comment on b
b = 2
c = 3
`,
			path: "b",
			wantFormat: `@lkal "0.1"

a = 1

c = 3
`,
		},
		{
			name: "blank_line_transfer_to_trailing_comment",
			src: `@lkal "0.1"

a = 1

b = 2
### trailing comment
`,
			path: "b",
			wantFormat: `@lkal "0.1"

a = 1

### trailing comment
`,
		},
		{
			name: "remove_record_entry",
			src: `@lkal "0.1"

a = 1
rec {
  x = 10
}
b = 2
`,
			path: "rec",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "remove_whole_collection_key",
			src: `@lkal "0.1"

a = 1
item "one" {
  x = 1
}
item "two" {
  x = 2
}
b = 2
`,
			path: "item",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "remove_one_element",
			src: `@lkal "0.1"

item "one" {
  x = 1
}
item "two" {
  x = 2
}
`,
			path: "item/one",
			wantFormat: `@lkal "0.1"

item "two" {
  x = 2
}
`,
		},
		{
			name: "remove_last_element_collection_disappears",
			src: `@lkal "0.1"

a = 1
item "only" {
  x = 1
}
b = 2
`,
			path: "item/only",
			wantFormat: `@lkal "0.1"

a = 1
b = 2
`,
		},
		{
			name: "nested_path",
			src: `@lkal "0.1"

service "web" {
  container "app" {
    image = "alpine"
    memory = 512Mi
  }
}
`,
			path: "service/web/container/app/memory",
			wantFormat: `@lkal "0.1"

service "web" {
  container "app" {
    image = "alpine"
  }
}
`,
		},
		{
			name: "l401_error_leaves_file_unchanged",
			src: `@lkal "0.1"

a = 1
`,
			path:        "/a",
			wantErrCode: "L401",
			wantFile:    "test.lkal",
			wantMsg:     "malformed path",
			wantHint:    "separate segments with / and quote segments that contain other characters",
		},
		{
			name: "l402_error_leaves_file_unchanged",
			src: `@lkal "0.1"

a = 1
`,
			path:        "missing",
			wantErrCode: "L402",
			wantFile:    "test.lkal",
			wantMsg:     "path not found",
			wantHint:    "check the path at segment \"missing\"",
		},
		{
			name: "l402_empty_file_name_from_json_style",
			src: `@lkal "0.1"

a = 1
`,
			fileParsedName: "",
			path:           "missing",
			wantErrCode:    "L402",
			wantFile:       "",
			wantMsg:        "path not found",
			wantHint:       "check the path at segment \"missing\"",
			wantErrorStr:   "L402 path not found\n  hint: check the path at segment \"missing\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fileName := "test.lkal"
			if tc.wantErrCode != "" && tc.name == "l402_empty_file_name_from_json_style" {
				fileName = ""
			}
			f := pathTParse(t, fileName, tc.src)
			origFormat := f.Format()
			err := f.Delete(tc.path)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("Delete(%q) succeeded unexpectedly, want error", tc.path)
				}
				errList, ok := err.(*ErrorList)
				if !ok || len(errList.Errors) != 1 {
					t.Fatalf("expected *ErrorList with 1 error, got %v", err)
				}
				e := errList.Errors[0]
				if e.Code != tc.wantErrCode {
					t.Errorf("Code = %q, want %q", e.Code, tc.wantErrCode)
				}
				if e.File != tc.wantFile {
					t.Errorf("File = %q, want %q", e.File, tc.wantFile)
				}
				if e.Pos != (Pos{}) {
					t.Errorf("Pos = %v, want zero Pos", e.Pos)
				}
				if e.Msg != tc.wantMsg {
					t.Errorf("Msg = %q, want %q", e.Msg, tc.wantMsg)
				}
				if e.Hint != tc.wantHint {
					t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
				}
				if tc.wantErrorStr != "" && e.Error() != tc.wantErrorStr {
					t.Errorf("Error() =\n%q\nwant:\n%q", e.Error(), tc.wantErrorStr)
				}
				if !bytes.Equal(f.Format(), origFormat) {
					t.Errorf("File changed on Delete error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Delete(%q) unexpected error: %v", tc.path, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	}
}

func TestParseValue(t *testing.T) {
	validTests := []struct {
		name string
		src  string
		want Value
	}{
		{
			name: "scalar_string",
			src:  `"hello"`,
			want: String("hello"),
		},
		{
			name: "scalar_int_positive",
			src:  "42",
			want: Int(42),
		},
		{
			name: "scalar_int_negative",
			src:  "-17",
			want: Int(-17),
		},
		{
			name: "scalar_decimal",
			src:  "0.25",
			want: Decimal("0.25"),
		},
		{
			name: "scalar_decimal_normalized",
			src:  "1.50",
			want: Decimal("1.5"),
		},
		{
			name: "scalar_bool_true",
			src:  "true",
			want: Bool(true),
		},
		{
			name: "scalar_bool_false",
			src:  "false",
			want: Bool(false),
		},
		{
			name: "scalar_enum",
			src:  ".tcp",
			want: Enum("tcp"),
		},
		{
			name: "scalar_quantity",
			src:  "512Mi",
			want: Quantity{Number: "512", Unit: "Mi"},
		},
		{
			name: "scalar_quantity_decimal_normalized",
			src:  "1.50Gi",
			want: Quantity{Number: "1.5", Unit: "Gi"},
		},
		{
			name: "scalar_ref",
			src:  "&secret/db-creds",
			want: Ref{"secret", "db-creds"},
		},
		{
			name: "list_scalars",
			src:  "[1, 2, 3]",
			want: List{Int(1), Int(2), Int(3)},
		},
		{
			name: "list_empty",
			src:  "[]",
			want: List{},
		},
		{
			name: "list_multiline",
			src: `[
  "a",
  "b"
]`,
			want: List{String("a"), String("b")},
		},
		{
			name: "raw_string_lines",
			src:  "\\\\one\n\\\\two",
			want: String("one\ntwo"),
		},
		{
			name: "raw_string_surrounded_by_comments",
			src: `### comment before
\\line 1
\\line 2
### comment after
`,
			want: String("line 1\nline 2"),
		},
		{
			name: "record_oneline",
			src:  "{ a = 1, b { c = 2 } }",
			want: Record{Fields: []Field{
				{Key: "a", Value: Int(1)},
				{Key: "b", Value: Record{Fields: []Field{{Key: "c", Value: Int(2)}}}},
			}},
		},
		{
			name: "record_multiline_with_comments",
			src: `{
  ### note
  a = 1
  ### another note
  b = 2
}`,
			want: Record{Fields: []Field{
				{Key: "a", Value: Int(1)},
				{Key: "b", Value: Int(2)},
			}},
		},
		{
			name: "record_with_labeled_entries",
			src:  "{ a \"x\" { y = 1 }, env \"LOG_LEVEL\" = \"info\" }",
			want: Record{Fields: []Field{
				{Key: "a", Value: Record{Fields: []Field{{Key: "x", Value: Record{Fields: []Field{{Key: "y", Value: Int(1)}}}}}}},
				{Key: "env", Value: Record{Fields: []Field{{Key: "LOG_LEVEL", Value: String("info")}}}},
			}},
		},
		{
			name: "leading_and_trailing_comments_and_blank_lines",
			src: `
### header

42

### footer
`,
			want: Int(42),
		},
	}

	for _, tc := range validTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err != nil {
				t.Fatalf("ParseValue(%q) unexpected error: %v", tc.src, err)
			}
			if val == nil {
				t.Fatalf("ParseValue(%q) returned nil Value", tc.src)
			}
			pathTCheckEqualAndOrder(t, val, tc.want)
		})
	}

	errorTests := []struct {
		name     string
		src      string
		wantCode string
		wantTag  string
		wantHint string
	}{
		{
			name:     "bare_word",
			src:      "tcp",
			wantCode: "L102",
			wantTag:  "L102 1:1",
			wantHint: `write "tcp" for a string or .tcp for an enum`,
		},
		{
			name:     "empty_input",
			src:      "",
			wantCode: "L106",
			wantTag:  "L106 1:1",
			wantHint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
		},
		{
			name:     "extra_token",
			src:      "1 2",
			wantCode: "L106",
			wantTag:  "L106 1:3",
			wantHint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
		},
		{
			name:     "duplicate_key_in_record",
			src:      "{ a = 1, a = 2 }",
			wantCode: "L201",
			wantTag:  "L201 1:10",
			wantHint: "remove one of them",
		},
		{
			name:     "list_mixes_types",
			src:      `[1, "x"]`,
			wantCode: "L204",
			wantTag:  "L204 1:5",
			wantHint: "use one type for all elements",
		},
		{
			name:     "duplicate_label_in_record",
			src:      "{ a \"x\" {}\n  a \"x\" {} }",
			wantCode: "L201",
			wantTag:  "L201 2:5",
			wantHint: "remove one of them",
		},
		{
			name:     "unclosed_bracket_brace",
			src:      "{",
			wantCode: "L103",
			wantTag:  "L103 1:1",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "unclosed_bracket_list",
			src:      "[",
			wantCode: "L103",
			wantTag:  "L103 1:1",
			wantHint: "add the matching closing bracket",
		},
		{
			name:     "dollar_key_inside_braces",
			src:      "{ \"$x\" = 1 }",
			wantCode: "L205",
			wantTag:  "L205 1:3",
			wantHint: "rename the key",
		},
		{
			name:     "record_missing_separator_two_blocks",
			src:      `{ a {} b {} }`,
			wantCode: "L106",
			wantTag:  "L106 1:8",
			wantHint: "separate entries with a newline or a comma",
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err == nil {
				t.Fatalf("ParseValue(%q) succeeded unexpectedly, got %#v", tc.src, val)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("expected *ErrorList, got %T: %v", err, err)
			}
			if len(errList.Errors) == 0 {
				t.Fatalf("expected at least 1 error in list, got 0")
			}
			e := errList.Errors[0]
			if e.File != "" {
				t.Errorf("File = %q, want empty", e.File)
			}
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if tc.wantHint != "" && e.Hint != tc.wantHint {
				t.Errorf("Hint = %q, want %q", e.Hint, tc.wantHint)
			}
			gotTag := fmt.Sprintf("%s %d:%d", e.Code, e.Pos.Line, e.Pos.Col)
			if gotTag != tc.wantTag {
				t.Errorf("got %q, want %q", gotTag, tc.wantTag)
			}
		})
	}
}

func TestPathGetSetRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	paths := []string{
		"service/web/replicas",
		"service/web/traffic-share",
		"service/web/labels/\"app.kubernetes.io/name\"",
		"service/web/labels/tier",
		"service/web/labels",
		"service/web/container/app/image",
		"service/web/container/app/args",
		"service/web/container/app/memory",
		"service/web/container/app/cpu",
		"service/web/container/app/timeout",
		"service/web/container/app/port/http/number",
		"service/web/container/app/port/http/protocol",
		"service/web/container/app/port/http",
		"service/web/container/app/port/metrics/number",
		"service/web/container/app/port/metrics/protocol",
		"service/web/container/app/port/metrics",
		"service/web/container/app/port",
		"service/web/container/app/env/LOG_LEVEL",
		"service/web/container/app/env",
		"service/web/container/app/secret",
		"service/web/container/app",
		"service/web/startup",
		"service/web",
		"service",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			val, err := f.Get(p)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", p, err)
			}
			if val == nil {
				t.Fatalf("Get(%q) returned nil Value", p)
			}
			formatted := FormatValue(val)
			if formatted == nil {
				t.Fatalf("FormatValue(Get(%q)) returned nil", p)
			}
			parsedVal, err := ParseValue(formatted)
			if err != nil {
				t.Fatalf("ParseValue(FormatValue(Get(%q))) error: %v", p, err)
			}
			if !Equal(parsedVal, val) {
				t.Errorf("ParseValue(FormatValue(Get(%q))) not Equal:\ngot:  %#v\nwant: %#v", p, parsedVal, val)
			}

			fCopy := pathTParse(t, "spec_example.lkal", string(data))
			if err := fCopy.Set(p, val); err != nil {
				t.Fatalf("Set(%q, Get(%q)) error: %v", p, p, err)
			}
			if !Equal(fCopy.Data(), f.Data()) {
				t.Errorf("Set(%q, Get(%q)) changed Data():\ngot:  %#v\nwant: %#v", p, p, fCopy.Data(), f.Data())
			}
		})
	}
}

func TestPathNoAliasing(t *testing.T) {
	t.Run("set_caller_mutation", func(t *testing.T) {
		tests := []struct {
			name       string
			path       string
			setup      func() (Value, func())
			wantGet    Value
			wantFormat string
		}{
			{
				name: "set_ref_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					return r, func() {
						r[0] = "b"
					}
				},
				wantGet:    Ref{"a"},
				wantFormat: "@lkal \"0.1\"\n\nx = &a\n",
			},
			{
				name: "set_ref_inside_list_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					l := List{r}
					return l, func() {
						r[0] = "b"
					}
				},
				wantGet:    List{Ref{"a"}},
				wantFormat: "@lkal \"0.1\"\n\nx = [&a]\n",
			},
			{
				name: "set_ref_inside_record_mutated",
				path: "x",
				setup: func() (Value, func()) {
					r := Ref{"a"}
					rec := Record{Fields: []Field{{Key: "k", Value: r}}}
					return rec, func() {
						r[0] = "b"
					}
				},
				wantGet:    Record{Fields: []Field{{Key: "k", Value: Ref{"a"}}}},
				wantFormat: "@lkal \"0.1\"\n\nx {\n  k = &a\n}\n",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
				val, mutate := tc.setup()
				if err := f.Set(tc.path, val); err != nil {
					t.Fatalf("Set(%q) unexpected error: %v", tc.path, err)
				}
				mutate()

				got, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got, tc.wantGet) {
					t.Errorf("Get(%q) after caller mutation =\ngot:  %#v\nwant: %#v", tc.path, got, tc.wantGet)
				}
				pathTCheckOrder(t, got, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != tc.wantFormat {
					t.Errorf("Format() after caller mutation =\ngot:\n%s\nwant:\n%s", gotFmt, tc.wantFormat)
				}
			})
		}
	})

	t.Run("get_ref_caller_mutation", func(t *testing.T) {
		specData, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
		if err != nil {
			t.Fatalf("failed to read spec_example.lkal: %v", err)
		}

		tests := []struct {
			name    string
			src     string
			path    string
			mutate  func(t *testing.T, v Value)
			wantGet Value
		}{
			{
				name: "spec_example_secret",
				src:  string(specData),
				path: "service/web/container/app/secret",
				mutate: func(t *testing.T, v Value) {
					r, ok := v.(Ref)
					if !ok {
						t.Fatalf("expected Ref, got %T", v)
					}
					if len(r) == 0 {
						t.Fatalf("expected non-empty Ref")
					}
					r[0] = "tampered"
				},
				wantGet: Ref{"secret", "db-creds"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "spec_example.lkal", tc.src)
				origFormat := string(f.Format())

				got1, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got1 == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}

				tc.mutate(t, got1)

				got2, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("second Get(%q) unexpected error: %v", tc.path, err)
				}
				if got2 == nil {
					t.Fatalf("second Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got2, tc.wantGet) {
					t.Errorf("second Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, got2, tc.wantGet)
				}
				pathTCheckOrder(t, got2, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != origFormat {
					t.Errorf("Format() after caller modified Get result =\ngot:\n%s\nwant:\n%s", gotFmt, origFormat)
				}
			})
		}
	})

	t.Run("get_list_caller_mutation", func(t *testing.T) {
		specData, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
		if err != nil {
			t.Fatalf("failed to read spec_example.lkal: %v", err)
		}

		tests := []struct {
			name    string
			src     string
			path    string
			mutate  func(t *testing.T, v Value)
			wantGet Value
		}{
			{
				name: "spec_example_args",
				src:  string(specData),
				path: "service/web/container/app/args",
				mutate: func(t *testing.T, v Value) {
					l, ok := v.(List)
					if !ok {
						t.Fatalf("expected List, got %T", v)
					}
					if len(l) == 0 {
						t.Fatalf("expected non-empty List")
					}
					l[0] = String("--tampered")
				},
				wantGet: List{String("--port"), String("8080")},
			},
			{
				name: "list_containing_ref_element",
				src:  "@lkal \"0.1\"\n\nitems = [&a, &b]\n",
				path: "items",
				mutate: func(t *testing.T, v Value) {
					l, ok := v.(List)
					if !ok {
						t.Fatalf("expected List, got %T", v)
					}
					if len(l) == 0 {
						t.Fatalf("expected non-empty List")
					}
					r, ok := l[0].(Ref)
					if !ok {
						t.Fatalf("expected Ref element in List, got %T", l[0])
					}
					r[0] = "tampered"
				},
				wantGet: List{Ref{"a"}, Ref{"b"}},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", tc.src)
				origFormat := string(f.Format())
				origParsed := pathTParse(t, "", origFormat)

				got1, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, err)
				}
				if got1 == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}

				tc.mutate(t, got1)

				got2, err := f.Get(tc.path)
				if err != nil {
					t.Fatalf("second Get(%q) unexpected error: %v", tc.path, err)
				}
				if got2 == nil {
					t.Fatalf("second Get(%q) returned nil Value", tc.path)
				}
				if !Equal(got2, tc.wantGet) {
					t.Errorf("second Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, got2, tc.wantGet)
				}
				pathTCheckOrder(t, got2, tc.wantGet)

				gotFmt := string(f.Format())
				if gotFmt != origFormat {
					t.Errorf("Format() after caller modified Get List =\ngot:\n%s\nwant:\n%s", gotFmt, origFormat)
				}

				pathTCheckEqualAndOrder(t, f.Data(), origParsed.Data())
			})
		}
	})
}

func TestSetErrorOrder(t *testing.T) {
	specData, err := os.ReadFile("testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}

	tests := []struct {
		name     string
		src      string
		path     string
		value    Value
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "list_decimal_invalid_before_nil",
			src:      string(specData),
			path:     "new_list",
			value:    List{Decimal("1."), nil},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write a decimal such as 0.25",
		},
		{
			name:     "list_enum_invalid_before_non_scalar",
			src:      string(specData),
			path:     "new_list",
			value:    List{Enum("1x"), List{}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "write an enum name that starts with a letter or _",
		},
		{
			name:     "list_nil_before_decimal_invalid",
			src:      string(specData),
			path:     "new_list",
			value:    List{nil, Decimal("1.")},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "use a non-nil value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "spec_example.lkal", tc.src)
			origFormat := f.Format()
			err := f.Set(tc.path, tc.value)
			if err == nil {
				t.Fatalf("Set(%q, %#v) succeeded unexpectedly, want error", tc.path, tc.value)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("Set(%q) error type =\ngot:  %T (%v)\nwant: *ErrorList", tc.path, err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("Set(%q) error count =\ngot:  %d (%v)\nwant: 1", tc.path, len(errList.Errors), errList)
			}
			e := errList.Errors[0]
			if e == nil {
				t.Fatalf("Set(%q) error is nil", tc.path)
			}
			if e.Code != tc.wantCode {
				t.Errorf("Code =\ngot:  %q\nwant: %q", e.Code, tc.wantCode)
			}
			if e.Pos != (Pos{}) {
				t.Errorf("Pos =\ngot:  %v\nwant: zero Pos", e.Pos)
			}
			if e.File != "spec_example.lkal" {
				t.Errorf("File =\ngot:  %q\nwant: %q", e.File, "spec_example.lkal")
			}
			if e.Msg != tc.wantMsg {
				t.Errorf("Msg =\ngot:  %q\nwant: %q", e.Msg, tc.wantMsg)
			}
			if e.Hint != tc.wantHint {
				t.Errorf("Hint =\ngot:  %q\nwant: %q", e.Hint, tc.wantHint)
			}
			if !bytes.Equal(f.Format(), origFormat) {
				t.Errorf("File format changed after failed Set:\ngot:\n%s\nwant:\n%s", string(f.Format()), string(origFormat))
			}
		})
	}
}

func TestSetAddToEmptyBlock(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		path       string
		value      Value
		wantFormat string
		wantGet    Value
	}{
		{
			name:       "add_to_empty_record_block",
			src:        "@lkal \"0.1\"\n\nserver {}\n",
			path:       "server/mem",
			value:      Quantity{Number: "1.5", Unit: "Gi"},
			wantFormat: "@lkal \"0.1\"\n\nserver {\n  mem = 1.5Gi\n}\n",
			wantGet:    Quantity{Number: "1.5", Unit: "Gi"},
		},
		{
			name:       "add_to_empty_element_body",
			src:        "@lkal \"0.1\"\n\nport \"http\" {}\n",
			path:       "port/http/number",
			value:      Int(80),
			wantFormat: "@lkal \"0.1\"\n\nport \"http\" {\n  number = 80\n}\n",
			wantGet:    Int(80),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			err := f.Set(tc.path, tc.value)
			if err != nil {
				t.Fatalf("Set(%q, %#v) unexpected error: %v", tc.path, tc.value, err)
			}
			gotFormat := string(f.Format())
			if gotFormat != tc.wantFormat {
				t.Errorf("Format() =\ngot:\n%s\nwant:\n%s", gotFormat, tc.wantFormat)
			}
			parsed := pathTParse(t, "", tc.wantFormat)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

			gotVal, getErr := f.Get(tc.path)
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", tc.path, getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", tc.path)
			}
			if !Equal(gotVal, tc.wantGet) {
				t.Errorf("Get(%q) =\ngot:  %#v\nwant: %#v", tc.path, gotVal, tc.wantGet)
			}
		})
	}
}

func TestPathEmptySegmentMissing(t *testing.T) {
	const src = "@lkal \"0.1\"\na = 1\n"
	const missingPath = "\"\"/x"
	const wantHint = `check the path at segment ""`

	negativeTests := []struct {
		name string
		op   func(t *testing.T, f *File) error
	}{
		{
			name: "Get",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				val, err := f.Get(missingPath)
				if val != nil {
					t.Errorf("Get(%q) value: got %v, want nil", missingPath, val)
				}
				return err
			},
		},
		{
			name: "Set",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				return f.Set("a/\"\"", Int(1))
			},
		},
		{
			name: "Delete",
			op: func(t *testing.T, f *File) error {
				t.Helper()
				return f.Delete(missingPath)
			},
		},
	}

	for _, tc := range negativeTests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", src)
			origFormatBytes := f.Format()
			if origFormatBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFormat := string(origFormatBytes)

			var (
				err      error
				panicked bool
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
						t.Errorf("%s panicked: got %v, want no panic", tc.name, r)
					}
				}()
				err = tc.op(t, f)
			}()

			if panicked {
				return
			}
			if err == nil {
				t.Fatalf("%s error: got nil, want *ErrorList", tc.name)
			}
			errList, ok := err.(*ErrorList)
			if !ok {
				t.Fatalf("%s error: got %T (%v), want *ErrorList", tc.name, err, err)
			}
			if len(errList.Errors) != 1 {
				t.Fatalf("%s error count: got %d, want 1", tc.name, len(errList.Errors))
			}
			e := errList.Errors[0]
			if e == nil {
				t.Fatalf("%s Error[0]: got nil, want non-nil", tc.name)
			}
			if e.Code != "L402" {
				t.Errorf("%s Code: got %q, want %q", tc.name, e.Code, "L402")
			}
			if e.File != "test.lkal" {
				t.Errorf("%s File: got %q, want %q", tc.name, e.File, "test.lkal")
			}
			if e.Pos != (Pos{}) {
				t.Errorf("%s Pos: got %v, want %v", tc.name, e.Pos, Pos{})
			}
			if e.Hint != wantHint {
				t.Errorf("%s Hint: got %q, want %q", tc.name, e.Hint, wantHint)
			}

			fmtBytes := f.Format()
			if fmtBytes == nil {
				t.Fatalf("%s Format() returned nil, want non-nil", tc.name)
			}
			gotFormat := string(fmtBytes)
			if gotFormat != origFormat {
				t.Errorf("%s Format() mismatch:\ngot:\n%s\nwant:\n%s", tc.name, gotFormat, origFormat)
			}
		})
	}

	t.Run("positive", func(t *testing.T) {
		const posSrc = "@lkal \"0.1\"\n\"\" { x = 1 }\n"
		f := pathTParse(t, "test_pos.lkal", posSrc)
		var (
			val      Value
			err      error
			panicked bool
		)
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					t.Errorf("Get(%q) panicked: got %v, want no panic", missingPath, r)
				}
			}()
			val, err = f.Get(missingPath)
		}()
		if panicked {
			return
		}
		if err != nil {
			t.Fatalf("Get(%q) error: got %v, want nil", missingPath, err)
		}
		if val == nil {
			t.Fatalf("Get(%q) value: got nil, want %v", missingPath, Int(1))
		}
		if !Equal(val, Int(1)) {
			t.Errorf("Get(%q) value: got %#v, want %#v", missingPath, val, Int(1))
		}
	})
}

type pathTExpectedError struct {
	code string
	line int
	col  int
	msg  string
	hint string
}

func pathTCheckSingleError(t *testing.T, err error, wantFile, wantCode, wantMsg, wantHint string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error is nil, want %s error", wantCode)
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("error type:\ngot:  %T (%v)\nwant: *ErrorList", err, err)
	}
	if len(errList.Errors) != 1 {
		t.Fatalf("error count:\ngot:  %d (%v)\nwant: 1", len(errList.Errors), errList)
	}
	e := errList.Errors[0]
	if e == nil {
		t.Fatalf("errList.Errors[0] is nil")
	}
	if e.Code != wantCode {
		t.Errorf("Code:\ngot:  %q\nwant: %q", e.Code, wantCode)
	}
	if e.Pos != (Pos{}) {
		t.Errorf("Pos:\ngot:  %v\nwant: zero Pos", e.Pos)
	}
	if e.File != wantFile {
		t.Errorf("File:\ngot:  %q\nwant: %q", e.File, wantFile)
	}
	if wantMsg != "" && e.Msg != wantMsg {
		t.Errorf("Msg:\ngot:  %q\nwant: %q", e.Msg, wantMsg)
	}
	if wantHint != "" && e.Hint != wantHint {
		t.Errorf("Hint:\ngot:  %q\nwant: %q", e.Hint, wantHint)
	}
}

func pathTCheckErrors(t *testing.T, err error, wantErrors []pathTExpectedError) {
	t.Helper()
	if err == nil {
		t.Fatalf("error is nil, want error")
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("error type:\ngot:  %T (%v)\nwant: *ErrorList", err, err)
	}
	if len(errList.Errors) != len(wantErrors) {
		t.Fatalf("error count:\ngot:  %d (%v)\nwant: %d", len(errList.Errors), errList, len(wantErrors))
	}
	for i, want := range wantErrors {
		e := errList.Errors[i]
		if e == nil {
			t.Fatalf("errList.Errors[%d] is nil", i)
		}
		if e.File != "" {
			t.Errorf("errList.Errors[%d].File:\ngot:  %q\nwant: %q", i, e.File, "")
		}
		if e.Code != want.code {
			t.Errorf("errList.Errors[%d].Code:\ngot:  %q\nwant: %q", i, e.Code, want.code)
		}
		if e.Pos.Line != want.line || e.Pos.Col != want.col {
			t.Errorf("errList.Errors[%d].Pos:\ngot:  %v\nwant: %d:%d", i, e.Pos, want.line, want.col)
		}
		if want.msg != "" && e.Msg != want.msg {
			t.Errorf("errList.Errors[%d].Msg:\ngot:  %q\nwant: %q", i, e.Msg, want.msg)
		}
		if want.hint != "" && e.Hint != want.hint {
			t.Errorf("errList.Errors[%d].Hint:\ngot:  %q\nwant: %q", i, e.Hint, want.hint)
		}
	}
}

func TestSetNumberChecks(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		tests := []struct {
			name       string
			src        string
			path       string
			value      Value
			wantFormat string
			wantGet    Value
		}{
			{
				name:       "quantity_large_integer_no_int64_limit",
				src:        "@lkal \"0.1\"\n",
				path:       "q",
				value:      Quantity{Number: "99999999999999999999", Unit: "Mi"},
				wantFormat: "@lkal \"0.1\"\n\nq = 99999999999999999999Mi\n",
				wantGet:    Quantity{Number: "99999999999999999999", Unit: "Mi"},
			},
			{
				name:       "quantity_negative_overflow_int64",
				src:        "@lkal \"0.1\"\n",
				path:       "q",
				value:      Quantity{Number: "-9223372036854775809", Unit: "Gi"},
				wantFormat: "@lkal \"0.1\"\n\nq = -9223372036854775809Gi\n",
				wantGet:    Quantity{Number: "-9223372036854775809", Unit: "Gi"},
			},
			{
				name: "list_of_quantities_overflow_int64",
				src:  "@lkal \"0.1\"\n",
				path: "q",
				value: List{
					Quantity{Number: "-9223372036854775809", Unit: "Gi"},
					Quantity{Number: "-9223372036854775810", Unit: "Gi"},
				},
				wantFormat: "@lkal \"0.1\"\n\nq = [-9223372036854775809Gi, -9223372036854775810Gi]\n",
				wantGet: List{
					Quantity{Number: "-9223372036854775809", Unit: "Gi"},
					Quantity{Number: "-9223372036854775810", Unit: "Gi"},
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", tc.src)
				err := f.Set(tc.path, tc.value)
				if err != nil {
					t.Fatalf("Set(%q, %#v) unexpected error: %v", tc.path, tc.value, err)
				}
				fmtBytes := f.Format()
				if fmtBytes == nil {
					t.Fatalf("Format() returned nil, want non-nil")
				}
				gotFmt := string(fmtBytes)
				if gotFmt != tc.wantFormat {
					t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", gotFmt, tc.wantFormat)
				}
				parsed := pathTParse(t, "", gotFmt)
				pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())

				gotVal, getErr := f.Get(tc.path)
				if getErr != nil {
					t.Fatalf("Get(%q) unexpected error: %v", tc.path, getErr)
				}
				if gotVal == nil {
					t.Fatalf("Get(%q) returned nil Value", tc.path)
				}
				pathTCheckEqualAndOrder(t, gotVal, tc.wantGet)
			})
		}

		t.Run("quantity_parsed_from_text_then_set_get_roundtrip", func(t *testing.T) {
			const parsedSrc = "@lkal \"0.1\"\n\nq = 99999999999999999999Mi\n"
			f := pathTParse(t, "test.lkal", parsedSrc)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			gotVal, getErr := f.Get("q")
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", "q", getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", "q")
			}
			wantVal := Quantity{Number: "99999999999999999999", Unit: "Mi"}
			pathTCheckEqualAndOrder(t, gotVal, wantVal)

			err := f.Set("q", gotVal)
			if err != nil {
				t.Fatalf("Set(%q, Get(%q)) unexpected error: %v", "q", "q", err)
			}
			newFmtBytes := f.Format()
			if newFmtBytes == nil {
				t.Fatalf("Format() after Set returned nil, want non-nil")
			}
			newFmt := string(newFmtBytes)
			if newFmt != origFmt {
				t.Errorf("Format() mismatch:\ngot:\n%s\nwant:\n%s", newFmt, origFmt)
			}
			parsed := pathTParse(t, "", newFmt)
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	})

	t.Run("negative_hints", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			value    Value
			wantCode string
			wantMsg  string
			wantHint string
		}{
			{
				name:     "decimal_101_digits_without_point",
				path:     "dec",
				value:    Decimal(strings.Repeat("1", 101)),
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a decimal such as 0.25",
			},
			{
				name:     "quantity_101_digits_with_invalid_char",
				path:     "qty",
				value:    Quantity{Number: strings.Repeat("1", 101) + "x", Unit: "Mi"},
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a quantity such as 512Mi",
			},
			{
				name:     "decimal_101_digits_with_point_range_exceeded",
				path:     "dec",
				value:    Decimal("1." + strings.Repeat("1", 100)),
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "keep numbers within 100 digits",
			},
			{
				name:     "quantity_unit_with_digit",
				path:     "qty",
				value:    Quantity{Number: "1.5", Unit: "M1"},
				wantCode: "L403",
				wantMsg:  "invalid value",
				wantHint: "write a quantity such as 512Mi",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
				origFmtBytes := f.Format()
				if origFmtBytes == nil {
					t.Fatalf("Format() returned nil, want non-nil")
				}
				origFmt := string(origFmtBytes)

				err := f.Set(tc.path, tc.value)
				pathTCheckSingleError(t, err, "test.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

				postFmtBytes := f.Format()
				if postFmtBytes == nil {
					t.Fatalf("Format() returned nil after failed Set, want non-nil")
				}
				postFmt := string(postFmtBytes)
				if postFmt != origFmt {
					t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
				}
			})
		}
	})
}

func TestSetL403Order(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		path     string
		value    Value
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "record_dollar_key_precedes_field_value_validation",
			src:      "@lkal \"0.1\"\n",
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "a", Value: Decimal("1.")}, {Key: "$b", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "rename keys that start with $",
		},
		{
			name:     "record_duplicate_key_precedes_field_value_validation",
			src:      "@lkal \"0.1\"\n",
			path:     "rec",
			value:    Record{Fields: []Field{{Key: "a", Value: Decimal("1.")}, {Key: "a", Value: Int(1)}}},
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "remove the duplicate key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", tc.src)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set(tc.path, tc.value)
			pathTCheckSingleError(t, err, "test.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after failed Set, want non-nil")
			}
			postFmt := string(postFmtBytes)
			if postFmt != origFmt {
				t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
			}
		})
	}
}

func TestSetDepth(t *testing.T) {
	docSrc := "@lkal \"0.1\"\n" + strings.Repeat("r {\n", 999) + "x = 1\n" + strings.Repeat("}\n", 999)
	setPath := strings.Repeat("r/", 999) + "sub"

	tests := []struct {
		name     string
		value    Value
		wantErr  bool
		wantCode string
		wantMsg  string
		wantHint string
	}{
		{
			name:     "innermost_record_add_two_levels_exceeds_1000",
			value:    Record{Fields: []Field{{Key: "a", Value: Record{Fields: []Field{{Key: "b", Value: Int(1)}}}}}},
			wantErr:  true,
			wantCode: "L403",
			wantMsg:  "invalid value",
			wantHint: "nest blocks at most 1000 levels deep",
		},
		{
			name:    "innermost_record_add_one_level_succeeds",
			value:   Record{Fields: []Field{{Key: "a", Value: Int(1)}}},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "depth.lkal", docSrc)
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set(setPath, tc.value)
			if tc.wantErr {
				pathTCheckSingleError(t, err, "depth.lkal", tc.wantCode, tc.wantMsg, tc.wantHint)

				postFmtBytes := f.Format()
				if postFmtBytes == nil {
					t.Fatalf("Format() returned nil after failed Set, want non-nil")
				}
				postFmt := string(postFmtBytes)
				if postFmt != origFmt {
					t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
				}
				return
			}

			if err != nil {
				t.Fatalf("Set(%q, %#v) unexpected error: %v", setPath, tc.value, err)
			}
			gotVal, getErr := f.Get(setPath)
			if getErr != nil {
				t.Fatalf("Get(%q) unexpected error: %v", setPath, getErr)
			}
			if gotVal == nil {
				t.Fatalf("Get(%q) returned nil Value", setPath)
			}
			pathTCheckEqualAndOrder(t, gotVal, tc.value)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after successful Set, want non-nil")
			}
			parsed := pathTParse(t, "", string(postFmtBytes))
			pathTCheckEqualAndOrder(t, f.Data(), parsed.Data())
		})
	}

	t.Run("scalar_at_depth_1000_succeeds_and_record_fails", func(t *testing.T) {
		doc1000 := "@lkal \"0.1\"\n" + strings.Repeat("r {\n", 1000) + "x = 1\n" + strings.Repeat("}\n", 1000)
		path1000 := strings.Repeat("r/", 1000) + "x"
		f := pathTParse(t, "depth.lkal", doc1000)
		if err := f.Set(path1000, Int(2)); err != nil {
			t.Fatalf("Set(depth 1000 scalar) unexpected error: %v", err)
		}
		got, err := f.Get(path1000)
		if err != nil || !Equal(got, Int(2)) {
			t.Fatalf("Get(depth 1000 scalar) = %v, %v; want 2", got, err)
		}
		if err := f.Set(path1000, Record{}); err == nil {
			t.Fatalf("Set(depth 1000 record) succeeded unexpectedly, want L403")
		} else {
			pathTCheckSingleError(t, err, "depth.lkal", "L403", "invalid value", "nest blocks at most 1000 levels deep")
		}
		if err := f.Set(path1000+"/y", Int(3)); err == nil {
			t.Fatalf("Set(depth 1001 scalar) succeeded unexpectedly, want L403")
		}
	})
}

func TestSetDepthOrder(t *testing.T) {
	deep := Record{}
	for i := 0; i < 1000; i++ {
		deep = Record{Fields: []Field{{"x", deep}}}
	}

	tests := []struct {
		name     string
		value    Value
		wantHint string
	}{
		{
			name:     "deep_record_first_exceeds_depth_limit",
			value:    Record{Fields: []Field{{"a", deep}, {"b", Decimal("1.")}}},
			wantHint: "nest blocks at most 1000 levels deep",
		},
		{
			name:     "invalid_decimal_first_fails_decimal_validation",
			value:    Record{Fields: []Field{{"b", Decimal("1.")}, {"a", deep}}},
			wantHint: "write a decimal such as 0.25",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := pathTParse(t, "test.lkal", "@lkal \"0.1\"\n")
			if f == nil {
				t.Fatalf("pathTParse returned nil File: got nil, want non-nil")
			}
			origFmtBytes := f.Format()
			if origFmtBytes == nil {
				t.Fatalf("Format() returned nil: got nil, want non-nil")
			}
			origFmt := string(origFmtBytes)

			err := f.Set("r", tc.value)
			if err == nil {
				t.Fatalf("Set(%q) error: got nil, want error", "r")
			}
			pathTCheckSingleError(t, err, "test.lkal", "L403", "invalid value", tc.wantHint)

			postFmtBytes := f.Format()
			if postFmtBytes == nil {
				t.Fatalf("Format() returned nil after failed Set: got nil, want non-nil")
			}
			postFmt := string(postFmtBytes)
			if postFmt != origFmt {
				t.Errorf("File changed after failed Set:\ngot:\n%s\nwant:\n%s", postFmt, origFmt)
			}
		})
	}
}

func TestParseValueImplementationReview(t *testing.T) {
	deepSrc1001 := "{ " + strings.Repeat("a { ", 1000) + "x = 1" + strings.Repeat(" }", 1001)
	col1001 := strings.LastIndex(deepSrc1001, "{") + 1
	if col1001 != 4001 {
		t.Fatalf("computed col1001 mismatch: got %d, want %d", col1001, 4001)
	}

	errorTests := []struct {
		name       string
		src        string
		wantErrors []pathTExpectedError
	}{
		{
			name: "list_mixed_types_with_extra_token",
			src:  `[1, "a"] x`,
			wantErrors: []pathTExpectedError{
				{
					code: "L204",
					line: 1,
					col:  5,
					msg:  "list mixes types",
					hint: "use one type for all elements",
				},
				{
					code: "L106",
					line: 1,
					col:  10,
					msg:  "unexpected token",
					hint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
				},
			},
		},
		{
			name: "record_dollar_key_with_extra_token",
			src:  `{ "$k" = 1 } junk`,
			wantErrors: []pathTExpectedError{
				{
					code: "L205",
					line: 1,
					col:  3,
					msg:  "key starts with $",
					hint: "rename the key",
				},
				{
					code: "L106",
					line: 1,
					col:  14,
					msg:  "unexpected token",
					hint: "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines",
				},
			},
		},
		{
			name: "list_syntax_error_no_extra_token_l106",
			src:  "[\n### c\n1] x",
			wantErrors: []pathTExpectedError{
				{
					code: "L002",
					line: 2,
					col:  1,
					msg:  "comment at the end of a line or inside a list",
					hint: "move the comment to its own line outside the list",
				},
			},
		},
		{
			name: "record_value_nested_1001_levels_l107",
			src:  deepSrc1001,
			wantErrors: []pathTExpectedError{
				{
					code: "L107",
					line: 1,
					col:  col1001,
					msg:  "blocks nested too deeply",
					hint: "nest blocks at most 1000 levels deep",
				},
			},
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ParseValue([]byte(tc.src))
			if err == nil {
				t.Fatalf("ParseValue(%q) succeeded unexpectedly, got %#v", tc.src, val)
			}
			if val != nil {
				t.Errorf("ParseValue(%q) value:\ngot:  %#v\nwant: nil", tc.src, val)
			}
			pathTCheckErrors(t, err, tc.wantErrors)
		})
	}

	t.Run("record_value_nested_1000_levels_succeeds", func(t *testing.T) {
		deepSrc1000 := "{ " + strings.Repeat("a { ", 999) + "x = 1" + strings.Repeat(" }", 1000)
		val, err := ParseValue([]byte(deepSrc1000))
		if err != nil {
			t.Fatalf("ParseValue(1000 levels) unexpected error: %v", err)
		}
		if val == nil {
			t.Fatalf("ParseValue(1000 levels) returned nil Value, want Record")
		}
		if _, ok := val.(Record); !ok {
			t.Errorf("ParseValue(1000 levels) type:\ngot:  %T\nwant: Record", val)
		}
	})
}
