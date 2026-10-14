package parser

import (
	"reflect"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func parserMustParse(t *testing.T, name, src string) *ast.File {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("Parse(%q) failed unexpectedly: %v", name, err)
	}
	if file == nil {
		t.Fatalf("Parse(%q) returned nil File with nil error", name)
	}
	return file
}

func parserMustFail(t *testing.T, name, src string) *diag.ErrorList {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err == nil {
		t.Fatalf("Parse(%q) succeeded unexpectedly, expected error", name)
	}
	if file != nil {
		t.Errorf("Parse(%q) returned non-nil File on error: %v", name, file)
	}
	errList, ok := err.(*diag.ErrorList)
	if !ok {
		t.Fatalf("Parse(%q) returned error of type %T, want *diag.ErrorList", name, err)
	}
	return errList
}

func TestParse_Data(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want model.Record
	}{
		{
			name: "scalars_string",
			src: `@lkal "0.1"

plain = "hello"
escaped = "quote: \" backslash: \\ newline: \n return: \r tab: \t unicode: \u{1f600}"
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "plain", Value: model.String("hello")},
					{Key: "escaped", Value: model.String("quote: \" backslash: \\ newline: \n return: \r tab: \t unicode: \U0001F600")},
				},
			},
		},
		{
			name: "scalars_int",
			src: `@lkal "0.1"

zero = 0
neg_zero = -0
positive = 42
negative = -17
max_int = 9223372036854775807
min_int = -9223372036854775808
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "zero", Value: model.Int(0)},
					{Key: "neg_zero", Value: model.Int(0)},
					{Key: "positive", Value: model.Int(42)},
					{Key: "negative", Value: model.Int(-17)},
					{Key: "max_int", Value: model.Int(9223372036854775807)},
					{Key: "min_int", Value: model.Int(-9223372036854775808)},
				},
			},
		},
		{
			name: "scalars_decimal_normalization",
			src: `@lkal "0.1"

d1 = 0.25
d2 = 1.50
d3 = 2.0
d4 = 2.00
d5 = 2.000
d6 = -0.0
d7 = -0.00
d8 = -0.50
d9 = 100.10
d10 = 0.0
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "d1", Value: model.Decimal("0.25")},
					{Key: "d2", Value: model.Decimal("1.5")},
					{Key: "d3", Value: model.Decimal("2.0")},
					{Key: "d4", Value: model.Decimal("2.0")},
					{Key: "d5", Value: model.Decimal("2.0")},
					{Key: "d6", Value: model.Decimal("0.0")},
					{Key: "d7", Value: model.Decimal("0.0")},
					{Key: "d8", Value: model.Decimal("-0.5")},
					{Key: "d9", Value: model.Decimal("100.1")},
					{Key: "d10", Value: model.Decimal("0.0")},
				},
			},
		},
		{
			name: "scalars_quantity_normalization",
			src: `@lkal "0.1"

q1 = 512Mi
q2 = 1.50Gi
q3 = -0Mi
q4 = 2.00s
q5 = 250m
q6 = 0m
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "q1", Value: model.Quantity{Number: "512", Unit: "Mi"}},
					{Key: "q2", Value: model.Quantity{Number: "1.5", Unit: "Gi"}},
					{Key: "q3", Value: model.Quantity{Number: "0", Unit: "Mi"}},
					{Key: "q4", Value: model.Quantity{Number: "2.0", Unit: "s"}},
					{Key: "q5", Value: model.Quantity{Number: "250", Unit: "m"}},
					{Key: "q6", Value: model.Quantity{Number: "0", Unit: "m"}},
				},
			},
		},
		{
			name: "scalars_bool",
			src: `@lkal "0.1"

t = true
f = false
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "t", Value: model.Bool(true)},
					{Key: "f", Value: model.Bool(false)},
				},
			},
		},
		{
			name: "scalars_enum",
			src: `@lkal "0.1"

e1 = .tcp
e2 = .ready
e3 = .http_2
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "e1", Value: model.Enum("tcp")},
					{Key: "e2", Value: model.Enum("ready")},
					{Key: "e3", Value: model.Enum("http_2")},
				},
			},
		},
		{
			name: "scalars_ref",
			src: `@lkal "0.1"

r1 = &secret/db-creds
r2 = &a/"b c"/d
r3 = &item
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "r1", Value: model.Ref{"secret", "db-creds"}},
					{Key: "r2", Value: model.Ref{"a", "b c", "d"}},
					{Key: "r3", Value: model.Ref{"item"}},
				},
			},
		},
		{
			name: "lists",
			src: `@lkal "0.1"

empty = []
ints = [1, 2, 3]
strs = ["a", "b",]
qtys = [10ms, 20ms]
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "empty", Value: model.List{}},
					{Key: "ints", Value: model.List{model.Int(1), model.Int(2), model.Int(3)}},
					{Key: "strs", Value: model.List{model.String("a"), model.String("b")}},
					{Key: "qtys", Value: model.List{model.Quantity{Number: "10", Unit: "ms"}, model.Quantity{Number: "20", Unit: "ms"}}},
				},
			},
		},
		{
			name: "records_and_source_order",
			src: `@lkal "0.1"

empty_rec {}
z_first = 1
middle {
  nested_b = "b"
  nested_a = "a"
}
a_last = 2
"quoted.key/name" = "val"
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "empty_rec", Value: model.Record{}},
					{Key: "z_first", Value: model.Int(1)},
					{
						Key: "middle",
						Value: model.Record{
							Fields: []model.Field{
								{Key: "nested_b", Value: model.String("b")},
								{Key: "nested_a", Value: model.String("a")},
							},
						},
					},
					{Key: "a_last", Value: model.Int(2)},
					{Key: "quoted.key/name", Value: model.String("val")},
				},
			},
		},
		{
			name: "collections_source_order",
			src: `@lkal "0.1"

head = 0
port "http" { number = 8080 }
port "metrics" { number = 9090 }
tail = 1
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "head", Value: model.Int(0)},
					{
						Key: "port",
						Value: model.Collection{
							Elems: []model.Elem{
								{Label: "http", Body: model.Record{Fields: []model.Field{{Key: "number", Value: model.Int(8080)}}}},
								{Label: "metrics", Body: model.Record{Fields: []model.Field{{Key: "number", Value: model.Int(9090)}}}},
							},
						},
					},
					{Key: "tail", Value: model.Int(1)},
				},
			},
		},
		{
			name: "spec_full_example",
			src: `@lkal "0.1"

### フロントエンドのWebサーバー
service "web" {
  replicas = 3
  traffic-share = 0.25
  labels { "app.kubernetes.io/name" = "web", tier = "frontend" }

  container "app" {
    image = "ghcr.io/acme/web:1.4.2"
    args = ["--port", "8080"]
    memory = 512Mi
    cpu = 250m
    timeout = 30s
    port "http" { number = 8080, protocol = .tcp }
    port "metrics" { number = 9090, protocol = .tcp }
    env { LOG_LEVEL = "info" }
    secret = &secret/db-creds
  }

  ### 起動スクリプト。各行を \\ で始める
  startup =
    \\#!/bin/sh
    \\exec /app/web --port 8080
}
`,
			want: specFullExampleRecord(),
		},
		{
			name: "extra_and_leading_separators",
			src: `@lkal "0.1"
, a = 1,, b = 2
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "a", Value: model.Int(1)},
					{Key: "b", Value: model.Int(2)},
				},
			},
		},
		{
			name: "blank_lines_between_assign_and_scalar",
			src: `@lkal "0.1"
a =

  1
`,
			want: model.Record{
				Fields: []model.Field{
					{Key: "a", Value: model.Int(1)},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file := parserMustParse(t, tc.name+".lkal", tc.src)
			got := ast.BuildRecord(file.Body)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Data() mismatch:\ngot:  %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}
