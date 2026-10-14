package parser

import (
	"reflect"
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func specFullExampleRecord() model.Record {
	return model.Record{
		Fields: []model.Field{
			{
				Key: "service",
				Value: model.Collection{
					Elems: []model.Elem{
						{
							Label: "web",
							Body: model.Record{
								Fields: []model.Field{
									{Key: "replicas", Value: model.Int(3)},
									{Key: "traffic-share", Value: model.Decimal("0.25")},
									{
										Key: "labels",
										Value: model.Record{
											Fields: []model.Field{
												{Key: "app.kubernetes.io/name", Value: model.String("web")},
												{Key: "tier", Value: model.String("frontend")},
											},
										},
									},
									{
										Key: "container",
										Value: model.Collection{
											Elems: []model.Elem{
												{
													Label: "app",
													Body: model.Record{
														Fields: []model.Field{
															{Key: "image", Value: model.String("ghcr.io/acme/web:1.4.2")},
															{Key: "args", Value: model.List{model.String("--port"), model.String("8080")}},
															{Key: "memory", Value: model.Quantity{Number: "512", Unit: "Mi"}},
															{Key: "cpu", Value: model.Quantity{Number: "250", Unit: "m"}},
															{Key: "timeout", Value: model.Quantity{Number: "30", Unit: "s"}},
															{
																Key: "port",
																Value: model.Collection{
																	Elems: []model.Elem{
																		{
																			Label: "http",
																			Body: model.Record{
																				Fields: []model.Field{
																					{Key: "number", Value: model.Int(8080)},
																					{Key: "protocol", Value: model.Enum("tcp")},
																				},
																			},
																		},
																		{
																			Label: "metrics",
																			Body: model.Record{
																				Fields: []model.Field{
																					{Key: "number", Value: model.Int(9090)},
																					{Key: "protocol", Value: model.Enum("tcp")},
																				},
																			},
																		},
																	},
																},
															},
															{
																Key: "env",
																Value: model.Record{
																	Fields: []model.Field{
																		{Key: "LOG_LEVEL", Value: model.String("info")},
																	},
																},
															},
															{Key: "secret", Value: model.Ref{"secret", "db-creds"}},
														},
													},
												},
											},
										},
									},
									{Key: "startup", Value: model.String("#!/bin/sh\nexec /app/web --port 8080")},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestParse_MultilineExamples(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantData  model.Record
		wantLines []string
		wantPos   diag.Pos
	}{
		{
			name: "example_a",
			src: `@lkal "0.1"

### 値は "one\ntwo"
a =
  \\one
  \\two
`,
			wantData: model.Record{
				Fields: []model.Field{{Key: "a", Value: model.String("one\ntwo")}},
			},
			wantLines: []string{"one", "two"},
			wantPos:   diag.Pos{Line: 5, Col: 3},
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
			wantData: model.Record{
				Fields: []model.Field{{Key: "b", Value: model.String("one\n\ntwo")}},
			},
			wantLines: []string{"one", "", "two"},
			wantPos:   diag.Pos{Line: 5, Col: 3},
		},
		{
			name: "example_c",
			src: `@lkal "0.1"

### 値は "  indented"（\\ より前の空白は無視し、後ろの空白は中身に含める）
c =
  \\  indented
`,
			wantData: model.Record{
				Fields: []model.Field{{Key: "c", Value: model.String("  indented")}},
			},
			wantLines: []string{"  indented"},
			wantPos:   diag.Pos{Line: 5, Col: 3},
		},
		{
			name: "example_d",
			src: `@lkal "0.1"

### 値は "line\n"（末尾に改行が要るときは、最後に \\ だけの行を置く）
d =
  \\line
  \\
`,
			wantData: model.Record{
				Fields: []model.Field{{Key: "d", Value: model.String("line\n")}},
			},
			wantLines: []string{"line", ""},
			wantPos:   diag.Pos{Line: 5, Col: 3},
		},
		{
			name: "example_e",
			src: `@lkal "0.1"

### 値は "### memo"（\\ の後ろの ### は中身で、コメントではない）
e =
  \\### memo
`,
			wantData: model.Record{
				Fields: []model.Field{{Key: "e", Value: model.String("### memo")}},
			},
			wantLines: []string{"### memo"},
			wantPos:   diag.Pos{Line: 5, Col: 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file := parserMustParse(t, tc.name+".lkal", tc.src)
			gotData := ast.BuildRecord(file.Body)
			if !reflect.DeepEqual(gotData, tc.wantData) {
				t.Errorf("Data() mismatch:\ngot:  %+v\nwant: %+v", gotData, tc.wantData)
			}
			if file.Body == nil || len(file.Body.Entries) != 1 {
				t.Fatalf("expected 1 entry in body, got %+v", file.Body)
			}
			entry := file.Body.Entries[0]
			raw, ok := entry.Value.(*ast.RawNode)
			if !ok {
				t.Fatalf("entry.Value is %T, want *ast.RawNode", entry.Value)
			}
			if !reflect.DeepEqual(raw.Lines, tc.wantLines) {
				t.Errorf("raw.Lines mismatch:\ngot:  %#v\nwant: %#v", raw.Lines, tc.wantLines)
			}
			if raw.Pos != tc.wantPos {
				t.Errorf("raw.Pos mismatch: got %v want %v", raw.Pos, tc.wantPos)
			}
		})
	}
}
