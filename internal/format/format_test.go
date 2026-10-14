package format

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func TestFormatAndFormatFile(t *testing.T) {
	if got := FormatFile(nil); got != nil {
		t.Errorf("FormatFile(nil) = %v, want nil", got)
	}

	if _, err := Format([]byte("invalid")); err == nil {
		t.Errorf("Format(invalid) succeeded, expected error")
	}

	src := []byte(`### header 1

### header 2

@lkal "0.1"

### entry comment
host = "localhost"
ports = [
  80,
  443,
]
inline { a = 1, b = "two" }
empty {}
svc "api" {
  script = "line1\nline2"
  ### trailing comment
}
`)
	out, err := Format(src)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	want := `### header 1

### header 2

@lkal "0.1"

### entry comment
host = "localhost"
ports = [
  80,
  443,
]
inline { a = 1, b = "two" }
empty {}
svc "api" {
  script =
    \\line1
    \\line2
  ### trailing comment
}
`
	if string(out) != want {
		t.Errorf("Format mismatch:\ngot:\n%s\nwant:\n%s", out, want)
	}
}

func TestFormatFilePure(t *testing.T) {
	f := &ast.File{
		Version:    "0.1",
		VersionPos: diag.Pos{Line: 3, Col: 1},
		Head: []ast.Comment{
			{Pos: diag.Pos{Line: 1, Col: 1}, Text: "top"},
		},
		Body: &ast.Block{
			Entries: []*ast.Entry{
				{
					Kind:        ast.EntryAssign,
					Key:         "msg",
					BlankBefore: true,
					Comments:    []ast.Comment{{Pos: diag.Pos{Line: 5, Col: 1}, Text: "doc"}},
					Value:       &ast.RawNode{Lines: []string{"a", "b"}},
				},
			},
			Trailing: []ast.Comment{{Pos: diag.Pos{Line: 9, Col: 1}, Text: "end", BlankBefore: true}},
		},
	}

	out1 := FormatFile(f)
	out2 := FormatFile(f)
	if string(out1) != string(out2) {
		t.Fatalf("FormatFile is not deterministic")
	}
}

func TestFormatValueAndFormatScalarValue(t *testing.T) {
	if got := FormatValue(nil); got != nil {
		t.Errorf("FormatValue(nil) = %v, want nil", got)
	}
	if got := FormatScalarValue(model.List{}); got != "" {
		t.Errorf("FormatScalarValue(non-scalar) = %q, want empty", got)
	}

	tests := []struct {
		v    model.Value
		want string
	}{
		{model.Int(-10), "-10"},
		{model.Decimal("0.25"), "0.25"},
		{model.Bool(true), "true"},
		{model.Bool(false), "false"},
		{model.Enum("http"), ".http"},
		{model.Quantity{Number: "10", Unit: "s"}, "10s"},
		{model.Ref{"a", "b/c"}, `&a/"b/c"`},
		{model.String("hello"), `"hello"`},
		{model.String("a\nb"), "\\\\a\n\\\\b"},
		{model.List{model.Int(1), model.Int(2)}, "[1, 2]"},
		{model.Record{Fields: []model.Field{{Key: "k", Value: model.Int(1)}}}, "{\n  k = 1\n}"},
		{model.Collection{Elems: []model.Elem{
			{Label: "x", Body: model.Record{}},
			{Label: "y", Body: model.Record{Fields: []model.Field{{Key: "a", Value: model.Bool(true)}}}},
		}}, "\"x\" {}\n\"y\" {\n  a = true\n}"},
	}

	for _, tt := range tests {
		if got := string(FormatValue(tt.v)); got != tt.want {
			t.Errorf("FormatValue(%#v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}
