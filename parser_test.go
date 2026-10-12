package lkal

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func parserMustParse(t *testing.T, name, src string) *File {
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

func parserMustFail(t *testing.T, name, src string) *ErrorList {
	t.Helper()
	file, err := Parse(name, []byte(src))
	if err == nil {
		t.Fatalf("Parse(%q) succeeded unexpectedly, expected error", name)
	}
	if file != nil {
		t.Errorf("Parse(%q) returned non-nil File on error: %v", name, file)
	}
	errList, ok := err.(*ErrorList)
	if !ok {
		t.Fatalf("Parse(%q) returned error of type %T, want *ErrorList", name, err)
	}
	return errList
}

func TestParse_Data(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want Record
	}{
		{
			name: "scalars_string",
			src: `@lkal "0.1"

plain = "hello"
escaped = "quote: \" backslash: \\ newline: \n return: \r tab: \t unicode: \u{1f600}"
`,
			want: Record{
				Fields: []Field{
					{Key: "plain", Value: String("hello")},
					{Key: "escaped", Value: String("quote: \" backslash: \\ newline: \n return: \r tab: \t unicode: \U0001F600")},
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
			want: Record{
				Fields: []Field{
					{Key: "zero", Value: Int(0)},
					{Key: "neg_zero", Value: Int(0)},
					{Key: "positive", Value: Int(42)},
					{Key: "negative", Value: Int(-17)},
					{Key: "max_int", Value: Int(9223372036854775807)},
					{Key: "min_int", Value: Int(-9223372036854775808)},
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
			want: Record{
				Fields: []Field{
					{Key: "d1", Value: Decimal("0.25")},
					{Key: "d2", Value: Decimal("1.5")},
					{Key: "d3", Value: Decimal("2.0")},
					{Key: "d4", Value: Decimal("2.0")},
					{Key: "d5", Value: Decimal("2.0")},
					{Key: "d6", Value: Decimal("0.0")},
					{Key: "d7", Value: Decimal("0.0")},
					{Key: "d8", Value: Decimal("-0.5")},
					{Key: "d9", Value: Decimal("100.1")},
					{Key: "d10", Value: Decimal("0.0")},
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
			want: Record{
				Fields: []Field{
					{Key: "q1", Value: Quantity{Number: "512", Unit: "Mi"}},
					{Key: "q2", Value: Quantity{Number: "1.5", Unit: "Gi"}},
					{Key: "q3", Value: Quantity{Number: "0", Unit: "Mi"}},
					{Key: "q4", Value: Quantity{Number: "2.0", Unit: "s"}},
					{Key: "q5", Value: Quantity{Number: "250", Unit: "m"}},
					{Key: "q6", Value: Quantity{Number: "0", Unit: "m"}},
				},
			},
		},
		{
			name: "scalars_bool",
			src: `@lkal "0.1"

t = true
f = false
`,
			want: Record{
				Fields: []Field{
					{Key: "t", Value: Bool(true)},
					{Key: "f", Value: Bool(false)},
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
			want: Record{
				Fields: []Field{
					{Key: "e1", Value: Enum("tcp")},
					{Key: "e2", Value: Enum("ready")},
					{Key: "e3", Value: Enum("http_2")},
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
			want: Record{
				Fields: []Field{
					{Key: "r1", Value: Ref{"secret", "db-creds"}},
					{Key: "r2", Value: Ref{"a", "b c", "d"}},
					{Key: "r3", Value: Ref{"item"}},
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
			want: Record{
				Fields: []Field{
					{Key: "empty", Value: List{}},
					{Key: "ints", Value: List{Int(1), Int(2), Int(3)}},
					{Key: "strs", Value: List{String("a"), String("b")}},
					{Key: "qtys", Value: List{Quantity{Number: "10", Unit: "ms"}, Quantity{Number: "20", Unit: "ms"}}},
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
			want: Record{
				Fields: []Field{
					{Key: "empty_rec", Value: Record{}},
					{Key: "z_first", Value: Int(1)},
					{
						Key: "middle",
						Value: Record{
							Fields: []Field{
								{Key: "nested_b", Value: String("b")},
								{Key: "nested_a", Value: String("a")},
							},
						},
					},
					{Key: "a_last", Value: Int(2)},
					{Key: "quoted.key/name", Value: String("val")},
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
			want: Record{
				Fields: []Field{
					{Key: "head", Value: Int(0)},
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
								{
									Label: "metrics",
									Body: Record{
										Fields: []Field{
											{Key: "number", Value: Int(9090)},
										},
									},
								},
							},
						},
					},
					{Key: "tail", Value: Int(1)},
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
			want: Record{
				Fields: []Field{
					{
						Key: "service",
						Value: Collection{
							Elems: []Elem{
								{
									Label: "web",
									Body: Record{
										Fields: []Field{
											{Key: "replicas", Value: Int(3)},
											{Key: "traffic-share", Value: Decimal("0.25")},
											{
												Key: "labels",
												Value: Record{
													Fields: []Field{
														{Key: "app.kubernetes.io/name", Value: String("web")},
														{Key: "tier", Value: String("frontend")},
													},
												},
											},
											{
												Key: "container",
												Value: Collection{
													Elems: []Elem{
														{
															Label: "app",
															Body: Record{
																Fields: []Field{
																	{Key: "image", Value: String("ghcr.io/acme/web:1.4.2")},
																	{
																		Key: "args",
																		Value: List{
																			String("--port"),
																			String("8080"),
																		},
																	},
																	{Key: "memory", Value: Quantity{Number: "512", Unit: "Mi"}},
																	{Key: "cpu", Value: Quantity{Number: "250", Unit: "m"}},
																	{Key: "timeout", Value: Quantity{Number: "30", Unit: "s"}},
																	{
																		Key: "port",
																		Value: Collection{
																			Elems: []Elem{
																				{
																					Label: "http",
																					Body: Record{
																						Fields: []Field{
																							{Key: "number", Value: Int(8080)},
																							{Key: "protocol", Value: Enum("tcp")},
																						},
																					},
																				},
																				{
																					Label: "metrics",
																					Body: Record{
																						Fields: []Field{
																							{Key: "number", Value: Int(9090)},
																							{Key: "protocol", Value: Enum("tcp")},
																						},
																					},
																				},
																			},
																		},
																	},
																	{
																		Key: "env",
																		Value: Record{
																			Fields: []Field{
																				{Key: "LOG_LEVEL", Value: String("info")},
																			},
																		},
																	},
																	{
																		Key:   "secret",
																		Value: Ref{"secret", "db-creds"},
																	},
																},
															},
														},
													},
												},
											},
											{
												Key:   "startup",
												Value: String("#!/bin/sh\nexec /app/web --port 8080"),
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
			name: "extra_and_leading_separators",
			src: `@lkal "0.1"
, a = 1,, b = 2
`,
			want: Record{
				Fields: []Field{
					{Key: "a", Value: Int(1)},
					{Key: "b", Value: Int(2)},
				},
			},
		},
		{
			name: "blank_lines_between_assign_and_scalar",
			src: `@lkal "0.1"
a =

  1
`,
			want: Record{
				Fields: []Field{
					{Key: "a", Value: Int(1)},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := Parse(tc.name+".lkal", []byte(tc.src))
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.name, err)
			}
			if file == nil {
				t.Fatalf("Parse(%q) returned nil File", tc.name)
			}
			got := file.Data()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Data() mismatch:\ngot:  %+v\nwant: %+v", got, tc.want)
			}
		})
	}
}

func TestParse_MultilineExamples(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantData  Record
		wantLines []string
		wantPos   Pos
	}{
		{
			name: "example_a",
			src: `@lkal "0.1"

### 値は "one\ntwo"
a =
  \\one
  \\two
`,
			wantData: Record{
				Fields: []Field{
					{Key: "a", Value: String("one\ntwo")},
				},
			},
			wantLines: []string{"one", "two"},
			wantPos:   Pos{Line: 5, Col: 3},
		},
		{
			name: "example_b",
			src: `@lkal "0.1"

### 値は "one\n\ntwo"（中身が空の行は \\ だけで書く）
b =
  \\one
  \\
  \\two
`,
			wantData: Record{
				Fields: []Field{
					{Key: "b", Value: String("one\n\ntwo")},
				},
			},
			wantLines: []string{"one", "", "two"},
			wantPos:   Pos{Line: 5, Col: 3},
		},
		{
			name: "example_c",
			src: `@lkal "0.1"

### 値は "  indented"（\\ より前の空白は無視し、後ろの空白は中身に含める）
c =
  \\  indented
`,
			wantData: Record{
				Fields: []Field{
					{Key: "c", Value: String("  indented")},
				},
			},
			wantLines: []string{"  indented"},
			wantPos:   Pos{Line: 5, Col: 3},
		},
		{
			name: "example_d",
			src: `@lkal "0.1"

### 値は "line\n"（末尾に改行が要るときは、最後に \\ だけの行を置く）
d =
  \\line
  \\
`,
			wantData: Record{
				Fields: []Field{
					{Key: "d", Value: String("line\n")},
				},
			},
			wantLines: []string{"line", ""},
			wantPos:   Pos{Line: 5, Col: 3},
		},
		{
			name: "example_e",
			src: `@lkal "0.1"

### 値は "### memo"（\\ の後ろの ### は中身で、コメントではない）
e =
  \\### memo
`,
			wantData: Record{
				Fields: []Field{
					{Key: "e", Value: String("### memo")},
				},
			},
			wantLines: []string{"### memo"},
			wantPos:   Pos{Line: 5, Col: 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := Parse(tc.name+".lkal", []byte(tc.src))
			if err != nil {
				t.Fatalf("Parse(%q) failed: %v", tc.name, err)
			}
			if file == nil {
				t.Fatalf("Parse(%q) returned nil File", tc.name)
			}
			gotData := file.Data()
			if !reflect.DeepEqual(gotData, tc.wantData) {
				t.Errorf("Data() mismatch:\ngot:  %+v\nwant: %+v", gotData, tc.wantData)
			}
			if file.body == nil || len(file.body.entries) != 1 {
				t.Fatalf("expected 1 entry in body, got %+v", file.body)
			}
			entry := file.body.entries[0]
			raw, ok := entry.value.(*rawNode)
			if !ok {
				t.Fatalf("entry.value is %T, want *rawNode", entry.value)
			}
			if !reflect.DeepEqual(raw.lines, tc.wantLines) {
				t.Errorf("raw.lines mismatch:\ngot:  %#v\nwant: %#v", raw.lines, tc.wantLines)
			}
			if raw.pos != tc.wantPos {
				t.Errorf("raw.pos mismatch: got %v want %v", raw.pos, tc.wantPos)
			}
		})
	}
}

func TestParse_VersionAndAST(t *testing.T) {
	t.Run("version_and_head", func(t *testing.T) {
		src := `### Header line 1
### Header line 2

@lkal "0.1"
`
		f := parserMustParse(t, "version_head.lkal", src)
		if got := f.Version(); got != "0.1" {
			t.Errorf("Version() = %q, want %q", got, "0.1")
		}
		if f.versionPos != (Pos{Line: 4, Col: 1}) {
			t.Errorf("versionPos = %v, want 4:1", f.versionPos)
		}
		wantHead := []comment{
			{pos: Pos{Line: 1, Col: 1}, text: "Header line 1", blankBefore: false},
			{pos: Pos{Line: 2, Col: 1}, text: "Header line 2", blankBefore: false},
		}
		if !reflect.DeepEqual(f.head, wantHead) {
			t.Errorf("head mismatch:\ngot:  %+v\nwant: %+v", f.head, wantHead)
		}
	})

	t.Run("entry_comments_and_blank_before", func(t *testing.T) {
		src := `@lkal "0.1"

### Comment for a
a = 1

b = 2
c = 3
`
		f := parserMustParse(t, "blank_before.lkal", src)
		if f.body == nil || len(f.body.entries) != 3 {
			t.Fatalf("expected 3 entries in body, got %+v", f.body)
		}

		ea := f.body.entries[0]
		if ea.key != "a" || ea.keyPos != (Pos{Line: 4, Col: 1}) {
			t.Errorf("ea mismatch: key=%q pos=%v", ea.key, ea.keyPos)
		}
		if ea.blankBefore != false {
			t.Errorf("ea.blankBefore = %v, want false", ea.blankBefore)
		}
		wantComments := []comment{
			{pos: Pos{Line: 3, Col: 1}, text: "Comment for a", blankBefore: true},
		}
		if !reflect.DeepEqual(ea.comments, wantComments) {
			t.Errorf("ea.comments mismatch:\ngot:  %+v\nwant: %+v", ea.comments, wantComments)
		}

		eb := f.body.entries[1]
		if eb.key != "b" || eb.keyPos != (Pos{Line: 6, Col: 1}) {
			t.Errorf("eb mismatch: key=%q pos=%v", eb.key, eb.keyPos)
		}
		if eb.blankBefore != true {
			t.Errorf("eb.blankBefore = %v, want true", eb.blankBefore)
		}
		if len(eb.comments) != 0 {
			t.Errorf("eb.comments should be empty, got %+v", eb.comments)
		}

		ec := f.body.entries[2]
		if ec.key != "c" || ec.keyPos != (Pos{Line: 7, Col: 1}) {
			t.Errorf("ec mismatch: key=%q pos=%v", ec.key, ec.keyPos)
		}
		if ec.blankBefore != false {
			t.Errorf("ec.blankBefore = %v, want false", ec.blankBefore)
		}
	})

	t.Run("trailing_comments", func(t *testing.T) {
		src := `@lkal "0.1"

rec {
  x = 1
  ### Trailing inside record
}

### Trailing at end of document
`
		f := parserMustParse(t, "trailing.lkal", src)
		if f.body == nil || len(f.body.entries) != 1 {
			t.Fatalf("expected 1 entry in body, got %+v", f.body)
		}
		recEntry := f.body.entries[0]
		if recEntry.body == nil {
			t.Fatalf("recEntry.body is nil")
		}
		wantRecTrailing := []comment{
			{pos: Pos{Line: 5, Col: 3}, text: "Trailing inside record", blankBefore: false},
		}
		if !reflect.DeepEqual(recEntry.body.trailing, wantRecTrailing) {
			t.Errorf("recEntry.body.trailing mismatch:\ngot:  %+v\nwant: %+v", recEntry.body.trailing, wantRecTrailing)
		}

		wantDocTrailing := []comment{
			{pos: Pos{Line: 8, Col: 1}, text: "Trailing at end of document", blankBefore: true},
		}
		if !reflect.DeepEqual(f.body.trailing, wantDocTrailing) {
			t.Errorf("f.body.trailing mismatch:\ngot:  %+v\nwant: %+v", f.body.trailing, wantDocTrailing)
		}
	})

	t.Run("one_line_flags_and_braces", func(t *testing.T) {
		src := `@lkal "0.1"

rec_one { a = 1 }
rec_multi {
  a = 1
}
list_one = [1, 2]
list_multi = [
  1,
  2,
]
`
		f := parserMustParse(t, "oneline.lkal", src)
		if f.body == nil || len(f.body.entries) != 4 {
			t.Fatalf("expected 4 entries in body, got %+v", f.body)
		}
		if f.body.oneLine != false || f.body.lbrace != (Pos{}) || f.body.rbrace != (Pos{}) {
			t.Errorf("top-level body mismatch: oneLine=%v lbrace=%v rbrace=%v", f.body.oneLine, f.body.lbrace, f.body.rbrace)
		}

		e0 := f.body.entries[0]
		if e0.body == nil {
			t.Fatalf("rec_one body is nil")
		}
		if !e0.body.oneLine {
			t.Errorf("rec_one oneLine = false, want true")
		}
		if e0.body.lbrace != (Pos{Line: 3, Col: 9}) || e0.body.rbrace != (Pos{Line: 3, Col: 17}) {
			t.Errorf("rec_one braces mismatch: lbrace=%v rbrace=%v, want 3:9, 3:17", e0.body.lbrace, e0.body.rbrace)
		}

		e1 := f.body.entries[1]
		if e1.body == nil {
			t.Fatalf("rec_multi body is nil")
		}
		if e1.body.oneLine {
			t.Errorf("rec_multi oneLine = true, want false")
		}
		if e1.body.lbrace != (Pos{Line: 4, Col: 11}) || e1.body.rbrace != (Pos{Line: 6, Col: 1}) {
			t.Errorf("rec_multi braces mismatch: lbrace=%v rbrace=%v, want 4:11, 6:1", e1.body.lbrace, e1.body.rbrace)
		}

		e2 := f.body.entries[2]
		l1, ok1 := e2.value.(*listNode)
		if !ok1 {
			t.Fatalf("list_one value is %T, want *listNode", e2.value)
		}
		if !l1.oneLine || l1.lbrack != (Pos{Line: 7, Col: 12}) || l1.rbrack != (Pos{Line: 7, Col: 17}) {
			t.Errorf("list_one mismatch: oneLine=%v lbrack=%v rbrack=%v, want true, 7:12, 7:17", l1.oneLine, l1.lbrack, l1.rbrack)
		}
		if len(l1.elems) != 2 {
			t.Errorf("list_one len(elems) = %d, want 2", len(l1.elems))
		}

		e3 := f.body.entries[3]
		l2, ok2 := e3.value.(*listNode)
		if !ok2 {
			t.Fatalf("list_multi value is %T, want *listNode", e3.value)
		}
		if l2.oneLine || l2.lbrack != (Pos{Line: 8, Col: 14}) || l2.rbrack != (Pos{Line: 11, Col: 1}) {
			t.Errorf("list_multi mismatch: oneLine=%v lbrack=%v rbrack=%v, want false, 8:14, 11:1", l2.oneLine, l2.lbrack, l2.rbrack)
		}
		if len(l2.elems) != 2 {
			t.Errorf("list_multi len(elems) = %d, want 2", len(l2.elems))
		}
	})

	t.Run("entry_kinds_and_positions", func(t *testing.T) {
		src := `@lkal "0.1"

assign = "val"
record { x = 1 }
elem "label1" { y = 2 }
`
		f := parserMustParse(t, "kinds.lkal", src)
		if f.body == nil || len(f.body.entries) != 3 {
			t.Fatalf("expected 3 entries in body, got %+v", f.body)
		}

		e0 := f.body.entries[0]
		if e0.kind != entryAssign || e0.key != "assign" || e0.keyPos != (Pos{Line: 3, Col: 1}) {
			t.Errorf("e0 assign mismatch: kind=%v key=%q pos=%v", e0.kind, e0.key, e0.keyPos)
		}
		if e0.body != nil {
			t.Errorf("entryAssign body must be nil, got %v", e0.body)
		}
		sn, ok := e0.value.(*scalarNode)
		if !ok {
			t.Fatalf("e0 value is %T, want *scalarNode", e0.value)
		}
		if !Equal(sn.val, String("val")) {
			t.Errorf("e0 value mismatch: %+v", sn.val)
		}
		if sn.nodePos() != (Pos{Line: 3, Col: 10}) {
			t.Errorf("e0 nodePos mismatch: got %v want 3:10", sn.nodePos())
		}

		e1 := f.body.entries[1]
		if e1.kind != entryRecord || e1.key != "record" || e1.keyPos != (Pos{Line: 4, Col: 1}) {
			t.Errorf("e1 record mismatch: kind=%v key=%q pos=%v", e1.kind, e1.key, e1.keyPos)
		}
		if e1.label != "" || e1.labelPos != (Pos{}) {
			t.Errorf("entryRecord label should be empty, got label=%q pos=%v", e1.label, e1.labelPos)
		}
		if e1.value != nil || e1.body == nil {
			t.Errorf("entryRecord value must be nil and body non-nil")
		}

		e2 := f.body.entries[2]
		if e2.kind != entryElem || e2.key != "elem" || e2.keyPos != (Pos{Line: 5, Col: 1}) {
			t.Errorf("e2 elem mismatch: kind=%v key=%q pos=%v", e2.kind, e2.key, e2.keyPos)
		}
		if e2.label != "label1" || e2.labelPos != (Pos{Line: 5, Col: 6}) {
			t.Errorf("e2 elem label mismatch: label=%q pos=%v, want label1, 5:6", e2.label, e2.labelPos)
		}
		if e2.value != nil || e2.body == nil {
			t.Errorf("entryElem value must be nil and body non-nil")
		}
	})

	t.Run("spec_full_example_ast", func(t *testing.T) {
		src := `@lkal "0.1"

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
`
		f := parserMustParse(t, "spec_ast.lkal", src)
		if f.body == nil || len(f.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", f.body)
		}
		svc := f.body.entries[0]
		if svc.kind != entryElem || svc.key != "service" || svc.label != "web" {
			t.Errorf("service entry mismatch: kind=%v key=%q label=%q", svc.kind, svc.key, svc.label)
		}
		if len(svc.comments) != 1 || svc.comments[0].text != "フロントエンドのWebサーバー" {
			t.Errorf("service comments mismatch: %+v", svc.comments)
		}
		if svc.body == nil || len(svc.body.entries) != 5 {
			t.Fatalf("expected 5 entries in service body, got %+v", svc.body)
		}

		labels := svc.body.entries[2]
		if labels.key != "labels" || labels.body == nil {
			t.Fatalf("labels entry invalid or body is nil: %+v", labels)
		}
		if !labels.body.oneLine {
			t.Errorf("labels oneLine = false, want true")
		}

		container := svc.body.entries[3]
		if container.key != "container" || container.label != "app" || !container.blankBefore {
			t.Errorf("container blankBefore = %v, want true", container.blankBefore)
		}
		if container.body == nil || len(container.body.entries) != 9 {
			t.Fatalf("expected 9 entries in container, got %+v", container.body)
		}
		args := container.body.entries[1]
		argsList, ok := args.value.(*listNode)
		if !ok {
			t.Fatalf("args value is %T, want *listNode", args.value)
		}
		if !argsList.oneLine || len(argsList.elems) != 2 {
			t.Errorf("args list mismatch: oneLine=%v len=%d", argsList.oneLine, len(argsList.elems))
		}

		startup := svc.body.entries[4]
		if startup.key != "startup" || startup.blankBefore != false {
			t.Errorf("startup blankBefore = %v, want false", startup.blankBefore)
		}
		if len(startup.comments) != 1 || startup.comments[0].text != `起動スクリプト。各行を \\ で始める` || startup.comments[0].blankBefore != true {
			t.Errorf("startup comments mismatch: %+v", startup.comments)
		}
		startupRaw, ok := startup.value.(*rawNode)
		if !ok {
			t.Fatalf("startup value is %T, want *rawNode", startup.value)
		}
		wantStartupLines := []string{"#!/bin/sh", "exec /app/web --port 8080"}
		if !reflect.DeepEqual(startupRaw.lines, wantStartupLines) {
			t.Errorf("startup lines mismatch: got %v want %v", startupRaw.lines, wantStartupLines)
		}
		if startupRaw.pos != (Pos{Line: 23, Col: 5}) {
			t.Errorf("startup raw pos mismatch: got %v want 23:5", startupRaw.pos)
		}
	})
}

func TestNestedBlockFirstItemBlankBefore(t *testing.T) {
	t.Run("entry_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  child = 1
}
`
		file := parserMustParse(t, "nested_entry_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if child.blankBefore != false {
			t.Errorf("child.blankBefore = %v, want false", child.blankBefore)
		}
	})

	t.Run("entry_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  child = 1
}
`
		file := parserMustParse(t, "nested_entry_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if child.blankBefore != true {
			t.Errorf("child.blankBefore = %v, want true", child.blankBefore)
		}
	})

	t.Run("comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### nested comment
  child = 1
}
`
		file := parserMustParse(t, "nested_comment_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if len(child.comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.comments))
		}
		if child.comments[0].blankBefore != false {
			t.Errorf("child comment blankBefore = %v, want false", child.comments[0].blankBefore)
		}
	})

	t.Run("comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### nested comment
  child = 1
}
`
		file := parserMustParse(t, "nested_comment_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil || len(parent.body.entries) != 1 {
			t.Fatalf("expected 1 child entry in block, got %+v", parent.body)
		}
		child := parent.body.entries[0]
		if len(child.comments) != 1 {
			t.Fatalf("expected 1 comment attached to child, got %d", len(child.comments))
		}
		if child.comments[0].blankBefore != true {
			t.Errorf("child comment blankBefore = %v, want true", child.comments[0].blankBefore)
		}
	})

	t.Run("trailing_comment_without_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {
  ### trailing comment
}
`
		file := parserMustParse(t, "nested_trailing_no_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.body.trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.body.trailing))
		}
		if parent.body.trailing[0].blankBefore != false {
			t.Errorf("trailing comment blankBefore = %v, want false", parent.body.trailing[0].blankBefore)
		}
	})

	t.Run("trailing_comment_with_blank_line", func(t *testing.T) {
		src := `@lkal "0.1"

parent {

  ### trailing comment
}
`
		file := parserMustParse(t, "nested_trailing_with_blank.lkal", src)
		if file.body == nil || len(file.body.entries) != 1 {
			t.Fatalf("expected 1 top-level entry, got %+v", file.body)
		}
		parent := file.body.entries[0]
		if parent.body == nil {
			t.Fatalf("parent body is nil")
		}
		if len(parent.body.trailing) != 1 {
			t.Fatalf("expected 1 trailing comment in block, got %d", len(parent.body.trailing))
		}
		if parent.body.trailing[0].blankBefore != true {
			t.Errorf("trailing comment blankBefore = %v, want true", parent.body.trailing[0].blankBefore)
		}
	})
}

func TestParse_Errors(t *testing.T) {
	t.Run("spec_error_rendering", func(t *testing.T) {
		src := `@lkal "0.1"










port =    tcp
`
		file, err := Parse("web.lkal", []byte(src))
		if err == nil {
			t.Fatalf("expected Parse to fail, but succeeded")
		}
		if file != nil {
			t.Errorf("expected nil File on error, got %v", file)
		}
		want := "web.lkal:12:11: L102 値の位置に裸の単語は書けない\n  hint: 文字列なら \"tcp\"、列挙子なら .tcp と書く"
		if got := err.Error(); got != want {
			t.Errorf("err.Error() mismatch:\ngot:  %q\nwant: %q", got, want)
		}
	})

	t.Run("syntax_and_semantic_codes", func(t *testing.T) {
		tests := []struct {
			name     string
			src      string
			wantCode string
			wantLine int
			wantCol  int
		}{
			{
				name:     "L101_heading_newline",
				src:      "@lkal \"0.1\"\nservice \"web\"\n{ replicas = 1 }",
				wantCode: "L101",
				wantLine: 2,
				wantCol:  14,
			},
			{
				name:     "L102_bare_word",
				src:      "@lkal \"0.1\"\nval = bare",
				wantCode: "L102",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L103_unclosed_brace",
				src:      "@lkal \"0.1\"\nrec {",
				wantCode: "L103",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L103_unclosed_bracket",
				src:      "@lkal \"0.1\"\nlst = [1, 2",
				wantCode: "L103",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L104_raw_gap",
				src:      "@lkal \"0.1\"\na =\n  \\\\line1\n\n  \\\\line2",
				wantCode: "L104",
				wantLine: 5,
				wantCol:  3,
			},
			{
				name:     "L105_missing_version",
				src:      "a = 1",
				wantCode: "L105",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L105_bad_version_token",
				src:      "@lkal 123\n",
				wantCode: "L105",
				wantLine: 1,
				wantCol:  7,
			},
			{
				name:     "L105_duplicate_version",
				src:      "@lkal \"0.1\"\n@lkal \"0.1\"\n",
				wantCode: "L105",
				wantLine: 2,
				wantCol:  1,
			},
			{
				name:     "L106_unexpected_token",
				src:      "@lkal \"0.1\"\na = 1 b = 2",
				wantCode: "L106",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L106_brace_after_equal",
				src:      "@lkal \"0.1\"\na = { b = 1 }",
				wantCode: "L106",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L201_duplicate_key",
				src:      "@lkal \"0.1\"\na = 1\na = 2",
				wantCode: "L201",
				wantLine: 3,
				wantCol:  1,
			},
			{
				name:     "L201_duplicate_label",
				src:      "@lkal \"0.1\"\ns \"item\" {}\ns \"item\" {}",
				wantCode: "L201",
				wantLine: 3,
				wantCol:  3,
			},
			{
				name:     "L202_conflicting_kind",
				src:      "@lkal \"0.1\"\na = 1\na {}",
				wantCode: "L202",
				wantLine: 3,
				wantCol:  1,
			},
			{
				name:     "L203_discontinuous_collection",
				src:      "@lkal \"0.1\"\ns \"a\" {}\nx = 1\ns \"b\" {}",
				wantCode: "L203",
				wantLine: 4,
				wantCol:  1,
			},
			{
				name:     "L204_mixed_list",
				src:      "@lkal \"0.1\"\nlst = [1, \"two\"]",
				wantCode: "L204",
				wantLine: 2,
				wantCol:  11,
			},
			{
				name:     "L205_dollar_key",
				src:      "@lkal \"0.1\"\n\"$key\" = 1",
				wantCode: "L205",
				wantLine: 2,
				wantCol:  1,
			},
			{
				name:     "L206_assign_to_collection_elem",
				src:      "@lkal \"0.1\"\ns \"a\" = 1",
				wantCode: "L206",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L207_unsupported_version",
				src:      "@lkal \"0.2\"\n",
				wantCode: "L207",
				wantLine: 1,
				wantCol:  7,
			},
			{
				name:     "L001_bad_comment",
				src:      "# comment\n@lkal \"0.1\"\n",
				wantCode: "L001",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L002_inline_comment",
				src:      "@lkal \"0.1\"\na = 1 # bad\n",
				wantCode: "L002",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L002_list_comment",
				src:      "@lkal \"0.1\"\na = [1, # bad\n 2]\n",
				wantCode: "L002",
				wantLine: 2,
				wantCol:  9,
			},
			{
				name:     "L002_list_comment_line",
				src:      "@lkal \"0.1\"\na = [\n  ### c\n  1,\n]\n",
				wantCode: "L002",
				wantLine: 3,
				wantCol:  3,
			},
			{
				name:     "L003_bad_escape",
				src:      "@lkal \"0.1\"\na = \"\\q\"\n",
				wantCode: "L003",
				wantLine: 2,
				wantCol:  6,
			},
			{
				name:     "L004_digit_after_unit",
				src:      "@lkal \"0.1\"\na = 1h30m\n",
				wantCode: "L004",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L005_integer_overflow",
				src:      "@lkal \"0.1\"\na = 99999999999999999999999999999999999999999\n",
				wantCode: "L005",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L006_unsupported_directive",
				src:      "@custom \"0.1\"\n",
				wantCode: "L006",
				wantLine: 1,
				wantCol:  1,
			},
			{
				name:     "L007_unclosed_string",
				src:      "@lkal \"0.1\"\na = \"unclosed\n",
				wantCode: "L007",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L008_illegal_character",
				src:      "@lkal \"0.1\"\na = 1 ;\n",
				wantCode: "L008",
				wantLine: 2,
				wantCol:  7,
			},
			{
				name:     "L009_malformed_number",
				src:      "@lkal \"0.1\"\na = 01\n",
				wantCode: "L009",
				wantLine: 2,
				wantCol:  5,
			},
			{
				name:     "L009_malformed_ref",
				src:      "@lkal \"0.1\"\na = &a/\n",
				wantCode: "L009",
				wantLine: 2,
				wantCol:  5,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				errList := parserMustFail(t, tc.name+".lkal", tc.src)
				if len(errList.Errors) == 0 {
					t.Fatalf("errList.Errors is empty")
				}
				first := errList.Errors[0]
				if first.Code != tc.wantCode {
					t.Errorf("error code mismatch: got %q, want %q", first.Code, tc.wantCode)
				}
				if first.Pos.Line != tc.wantLine || first.Pos.Col != tc.wantCol {
					t.Errorf("error pos mismatch: got %v, want %d:%d", first.Pos, tc.wantLine, tc.wantCol)
				}
				if first.Msg == "" {
					t.Errorf("error Msg should not be empty")
				}
				if first.Hint == "" {
					t.Errorf("error Hint should not be empty")
				}
			})
		}
	})

	t.Run("error_sorting_and_truncation", func(t *testing.T) {
		src := "@lkal \"0.1\"\n" + strings.Repeat("?\n", 25)
		errList := parserMustFail(t, "truncation.lkal", src)
		if len(errList.Errors) != 20 {
			t.Errorf("len(errList.Errors) = %d, want 20 (truncated)", len(errList.Errors))
		}
		for i := 1; i < len(errList.Errors); i++ {
			prev := errList.Errors[i-1]
			curr := errList.Errors[i]
			if prev.Pos.Line > curr.Pos.Line ||
				(prev.Pos.Line == curr.Pos.Line && prev.Pos.Col > curr.Pos.Col) ||
				(prev.Pos.Line == curr.Pos.Line && prev.Pos.Col == curr.Pos.Col && prev.Code > curr.Code) {
				t.Errorf("errors not sorted: [%d]=%v:%s vs [%d]=%v:%s", i-1, prev.Pos, prev.Code, i, curr.Pos, curr.Code)
			}
		}
	})
}

func TestParseScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaling test in short mode")
	}

	buildInput := func(prefix, repeat, suffix string, k int) []byte {
		return []byte(prefix + strings.Repeat(repeat, k) + suffix)
	}

	measureMin := func(src []byte) time.Duration {
		var minDur time.Duration
		for i := 0; i < 5; i++ {
			start := time.Now()
			_, _ = Parse("scale.lkal", src)
			d := time.Since(start)
			if i == 0 || d < minDur {
				minDur = d
			}
		}
		return minDur
	}

	families := []struct {
		name   string
		prefix string
		repeat string
		suffix string
		baseK  int
	}{
		{
			name:   "long_valid_line",
			prefix: "@lkal \"0.1\"\na = [",
			repeat: "1, ",
			suffix: "1]\n",
			baseK:  20000,
		},
		{
			name:   "long_line_illegal_characters",
			prefix: "@lkal \"0.1\"\na = ",
			repeat: "%",
			suffix: "\n",
			baseK:  60000,
		},
		{
			name:   "failed_entry_skip_long_block",
			prefix: "@lkal \"0.1\"\na {\n  = 1\n",
			repeat: "  b = 1\n",
			suffix: "}\nc = 1\n",
			baseK:  7500,
		},
	}

	for _, fam := range families {
		t.Run(fam.name, func(t *testing.T) {
			inputN := buildInput(fam.prefix, fam.repeat, fam.suffix, fam.baseK)
			input4N := buildInput(fam.prefix, fam.repeat, fam.suffix, fam.baseK*4)

			durN := measureMin(inputN)
			dur4N := measureMin(input4N)

			if dur4N > 8*durN {
				ratio := float64(dur4N)
				if durN > 0 {
					ratio /= float64(durN)
					t.Errorf("%s: 4n duration %v exceeds 8x n duration %v (ratio: %.2fx)", fam.name, dur4N, durN, ratio)
				} else {
					t.Errorf("%s: 4n duration %v exceeds 8x n duration %v", fam.name, dur4N, durN)
				}
			}
		})
	}
}
