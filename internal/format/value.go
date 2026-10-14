package format

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// FormatScalarValue returns the inline canonical representation of a scalar Value.
func FormatScalarValue(v model.Value) string {
	switch val := v.(type) {
	case model.Int:
		return strconv.FormatInt(int64(val), 10)
	case model.Decimal:
		return string(val)
	case model.Bool:
		if val {
			return "true"
		}
		return "false"
	case model.Enum:
		return "." + string(val)
	case model.Quantity:
		return val.Number + val.Unit
	case model.Ref:
		return "&" + syntax.FormatRefBody(val)
	case model.String:
		return syntax.QuoteString(string(val))
	default:
		return ""
	}
}

func writeInlineValue(b *bytes.Buffer, n ast.Node) {
	switch vn := n.(type) {
	case *ast.ScalarNode:
		b.WriteString(FormatScalarValue(vn.Val))
	case *ast.RawNode:
		b.WriteString(syntax.QuoteString(strings.Join(vn.Lines, "\n")))
	case *ast.ListNode:
		writeList(b, len(vn.Elems), func(i int) model.Value { return vn.Elems[i].Val })
	}
}

func writeList(b *bytes.Buffer, n int, at func(int) model.Value) {
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(FormatScalarValue(at(i)))
	}
	b.WriteByte(']')
}

func writeRawLines(b *bytes.Buffer, s, indent string) {
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(indent)
		b.WriteString(`\\`)
		b.WriteString(line)
	}
}

func isOneLineBlock(b *ast.Block) bool {
	if !b.OneLine || len(b.Trailing) > 0 || len(b.Entries) == 0 {
		return false
	}
	for _, e := range b.Entries {
		if e.Kind != ast.EntryAssign || len(e.Comments) > 0 || !isOneLineValue(e.Value) {
			return false
		}
	}
	return true
}

func isOneLineValue(n ast.Node) bool {
	switch vn := n.(type) {
	case *ast.ScalarNode:
		if s, ok := vn.Val.(model.String); ok {
			return !syntax.IsRawCandidate(string(s))
		}
		return true
	case *ast.ListNode:
		return vn.OneLine
	default:
		return false
	}
}

func nodeString(n ast.Node) (string, bool) {
	switch vn := n.(type) {
	case *ast.ScalarNode:
		if s, ok := vn.Val.(model.String); ok {
			return string(s), true
		}
	case *ast.RawNode:
		return strings.Join(vn.Lines, "\n"), true
	}
	return "", false
}
