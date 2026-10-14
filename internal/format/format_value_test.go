package format_test

import . "github.com/Kaikei-e/LKAL"

import (
	"bytes"
	"testing"
)

func TestFormatValue(t *testing.T) {
	cases := []struct {
		name string
		val  Value
		want []byte
	}{
		{
			name: "Int",
			val:  Int(42),
			want: []byte("42"),
		},
		{
			name: "Decimal",
			val:  Decimal("1.5"),
			want: []byte("1.5"),
		},
		{
			name: "BoolTrue",
			val:  Bool(true),
			want: []byte("true"),
		},
		{
			name: "BoolFalse",
			val:  Bool(false),
			want: []byte("false"),
		},
		{
			name: "Enum",
			val:  Enum("tcp"),
			want: []byte(".tcp"),
		},
		{
			name: "Quantity",
			val:  Quantity{Number: "512", Unit: "Mi"},
			want: []byte("512Mi"),
		},
		{
			name: "RefSimple",
			val:  Ref{"secret", "db-creds"},
			want: []byte("&secret/db-creds"),
		},
		{
			name: "RefQuotedSegment",
			val:  Ref{"a", "b c"},
			want: []byte("&a/\"b c\""),
		},
		{
			name: "StringSimple",
			val:  String("hello"),
			want: []byte("\"hello\""),
		},
		{
			name: "StringRawCandidate",
			val:  String("one\ntwo"),
			want: []byte("\\\\one\n\\\\two"),
		},
		{
			name: "StringQuotedWithCR",
			val:  String("one\rtwo"),
			want: []byte("\"one\\rtwo\""),
		},
		{
			name: "ListEmpty",
			val:  List{},
			want: []byte("[]"),
		},
		{
			name: "ListScalars",
			val:  List{Int(1), Int(2)},
			want: []byte("[1, 2]"),
		},
		{
			name: "RecordEmpty",
			val:  Record{},
			want: []byte("{}"),
		},
		{
			name: "RecordWithNestedAndCollection",
			val: Record{
				Fields: []Field{
					{Key: "scalar", Value: Int(1)},
					{
						Key: "nested",
						Value: Record{
							Fields: []Field{
								{Key: "x", Value: String("ok")},
							},
						},
					},
					{
						Key: "service",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "web",
									Body: Record{
										Fields: []Field{
											{Key: "port", Value: Int(8080)},
										},
									},
								},
							},
						},
					},
				},
			},
			want: []byte("{\n  scalar = 1\n  nested {\n    x = \"ok\"\n  }\n  service \"web\" {\n    port = 8080\n  }\n}"),
		},
		{
			name: "CollectionWithTwoElementsOneEmpty",
			val: Collection{
				Elems: []Elem{
					{
						Label: "first",
						Body: Record{
							Fields: []Field{
								{Key: "x", Value: Int(1)},
							},
						},
					},
					{
						Label: "second",
						Body:  Record{},
					},
				},
			},
			want: []byte("\"first\" {\n  x = 1\n}\n\"second\" {}"),
		},
		{
			name: "RecordWithRawCandidateString",
			val: Record{
				Fields: []Field{
					{Key: "script", Value: String("line1\nline2")},
				},
			},
			want: []byte("{\n  script =\n    \\\\line1\n    \\\\line2\n}"),
		},
		{
			name: "RecordWithList",
			val: Record{
				Fields: []Field{
					{Key: "items", Value: List{Int(1), Int(2)}},
				},
			},
			want: []byte("{\n  items = [1, 2]\n}"),
		},
		{
			name: "NestedCollectionInsideElementBody",
			val: Collection{
				Elems: []Elem{
					{
						Label: "service",
						Body: Record{
							Fields: []Field{
								{
									Key: "port",
									Value: Collection{
										Elems: []Elem{
											{
												Label: "http",
												Body: Record{
													Fields: []Field{
														{Key: "number", Value: Int(8080)},
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
			want: []byte("\"service\" {\n  port \"http\" {\n    number = 8080\n  }\n}"),
		},
		{
			name: "DecimalStoredText",
			val:  Decimal("2.0"),
			want: []byte("2.0"),
		},
		{
			name: "NilValue",
			val:  nil,
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatValue(tc.val)
			if tc.want == nil {
				if got != nil {
					t.Errorf("FormatValue(%s) got %q, want nil", tc.name, string(got))
				}
				return
			}
			if got == nil {
				t.Fatalf("FormatValue(%s) got nil, want %q", tc.name, string(tc.want))
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("FormatValue(%s) mismatch:\ngot:\n%s\nwant:\n%s\nquoted got:  %q\nquoted want: %q",
					tc.name, string(got), string(tc.want), string(got), string(tc.want))
			}
		})
	}
}
