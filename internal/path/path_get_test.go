package path_test

import . "github.com/Kaikei-e/LKAL"

import (
	"os"
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
	case Collection:
		w, ok := want.(Collection)
		if !ok {
			t.Fatalf("type mismatch: got Collection, want %T", want)
		}
		if len(g.Elems) != len(w.Elems) {
			t.Fatalf("collection elems length mismatch: got %d, want %d", len(g.Elems), len(w.Elems))
		}
		for i := range g.Elems {
			if g.Elems[i].Label != w.Elems[i].Label {
				t.Fatalf("elem %d label mismatch: got %q, want %q", i, g.Elems[i].Label, w.Elems[i].Label)
			}
			pathTCheckOrder(t, g.Elems[i].Body, w.Elems[i].Body)
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

func TestGet(t *testing.T) {
	data, err := os.ReadFile("../../testdata/conformance/valid/spec_example.lkal")
	if err != nil {
		t.Fatalf("failed to read spec_example.lkal: %v", err)
	}
	f := pathTParse(t, "spec_example.lkal", string(data))

	wantPortColl := Collection{Elems: []Elem{
		{
			Label: "http",
			Body: Record{Fields: []Field{
				{Key: "number", Value: Int(8080)},
				{Key: "protocol", Value: Enum("tcp")},
			}},
		},
		{
			Label: "metrics",
			Body: Record{Fields: []Field{
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
		{Key: "port", Value: wantPortColl},
		{Key: "env", Value: Record{Fields: []Field{{Key: "LOG_LEVEL", Value: String("info")}}}},
		{Key: "secret", Value: Ref{"secret", "db-creds"}},
	}}

	wantServiceColl := Collection{Elems: []Elem{
		{
			Label: "web",
			Body: Record{Fields: []Field{
				{Key: "replicas", Value: Int(3)},
				{Key: "traffic-share", Value: Decimal("0.25")},
				{Key: "labels", Value: Record{Fields: []Field{
					{Key: "app.kubernetes.io/name", Value: String("web")},
					{Key: "tier", Value: String("frontend")},
				}}},
				{Key: "container", Value: Collection{Elems: []Elem{
					{Label: "app", Body: wantAppRecord},
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
			want: wantPortColl,
		},
		{
			path: "service/web/container/app",
			want: wantAppRecord,
		},
		{
			path: "service",
			want: wantServiceColl,
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
