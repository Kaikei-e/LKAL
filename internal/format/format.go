package format

import (
	"bytes"
	"strings"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/parser"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// Format parses src and returns its canonical LKAL representation.
func Format(src []byte) ([]byte, error) {
	f, err := parser.Parse("", src)
	if err != nil {
		return nil, err
	}
	return FormatFile(f), nil
}

// FormatFile returns the canonical LKAL text of f without mutating f.
func FormatFile(f *ast.File) []byte {
	if f == nil {
		return nil
	}
	var b bytes.Buffer

	for i, c := range f.Head {
		if i > 0 && c.BlankBefore {
			b.WriteByte('\n')
		}
		writeComment(&b, "", c.Text)
	}
	if len(f.Head) > 0 && f.VersionPos.Line > f.Head[len(f.Head)-1].Pos.Line+1 {
		b.WriteByte('\n')
	}

	b.WriteString("@lkal ")
	b.WriteString(syntax.QuoteString(f.Version))
	b.WriteByte('\n')

	if f.Body != nil && (len(f.Body.Entries) > 0 || len(f.Body.Trailing) > 0) {
		b.WriteByte('\n')
		formatBlockItems(&b, f.Body, 0)
	}
	return b.Bytes()
}

func formatBlockItems(b *bytes.Buffer, blk *ast.Block, d int) {
	indent := strings.Repeat("  ", d)
	childIndent := indent + "  "
	isFirstItem := true
	for _, ent := range blk.Entries {
		if !isFirstItem {
			blank := ent.BlankBefore
			if len(ent.Comments) > 0 {
				blank = ent.Comments[0].BlankBefore
			}
			if blank {
				b.WriteByte('\n')
			}
		}
		isFirstItem = false

		for i, c := range ent.Comments {
			if i > 0 && c.BlankBefore {
				b.WriteByte('\n')
			}
			writeComment(b, indent, c.Text)
		}
		if len(ent.Comments) > 0 && ent.BlankBefore {
			b.WriteByte('\n')
		}

		formatEntry(b, ent, d, indent, childIndent)
	}

	for _, c := range blk.Trailing {
		if !isFirstItem && c.BlankBefore {
			b.WriteByte('\n')
		}
		isFirstItem = false
		writeComment(b, indent, c.Text)
	}
}

func writeComment(b *bytes.Buffer, indent, text string) {
	b.WriteString(indent)
	b.WriteString("###")
	if text != "" {
		b.WriteByte(' ')
		b.WriteString(text)
	}
	b.WriteByte('\n')
}

func formatEntry(b *bytes.Buffer, ent *ast.Entry, d int, indent, childIndent string) {
	switch ent.Kind {
	case ast.EntryAssign:
		formatAssign(b, ent, indent, childIndent)
	case ast.EntryRecord:
		formatBody(b, indent+syntax.FormatKey(ent.Key), ent.Body, d)
	case ast.EntryElem:
		formatBody(b, indent+syntax.FormatKey(ent.Key)+" "+syntax.QuoteString(ent.Label), ent.Body, d)
	}
}

func formatAssign(b *bytes.Buffer, ent *ast.Entry, indent, childIndent string) {
	b.WriteString(indent)
	b.WriteString(syntax.FormatKey(ent.Key))
	if s, ok := nodeString(ent.Value); ok && syntax.IsRawCandidate(s) {
		b.WriteString(" =\n")
		writeRawLines(b, s, childIndent)
		b.WriteByte('\n')
		return
	}
	if l, ok := ent.Value.(*ast.ListNode); ok && !l.OneLine && len(l.Elems) > 0 {
		b.WriteString(" = [\n")
		for _, elem := range l.Elems {
			b.WriteString(childIndent)
			b.WriteString(FormatScalarValue(elem.Val))
			b.WriteString(",\n")
		}
		b.WriteString(indent)
		b.WriteString("]\n")
		return
	}
	b.WriteString(" = ")
	writeInlineValue(b, ent.Value)
	b.WriteByte('\n')
}

func formatBody(b *bytes.Buffer, headerPrefix string, body *ast.Block, d int) {
	if isOneLineBlock(body) {
		b.WriteString(headerPrefix)
		b.WriteString(" { ")
		for i, ent := range body.Entries {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(syntax.FormatKey(ent.Key))
			b.WriteString(" = ")
			writeInlineValue(b, ent.Value)
		}
		b.WriteString(" }\n")
		return
	}
	b.WriteString(headerPrefix)
	b.WriteByte(' ')
	writeBlock(b, body, d)
	b.WriteByte('\n')
}

func writeBlock(b *bytes.Buffer, blk *ast.Block, d int) {
	if len(blk.Entries) == 0 && len(blk.Trailing) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	formatBlockItems(b, blk, d+1)
	b.WriteString(strings.Repeat("  ", d))
	b.WriteString("}")
}

// FormatValue returns the canonical text of one Value without a trailing newline.
func FormatValue(v model.Value) []byte {
	if v == nil {
		return nil
	}
	var b bytes.Buffer
	switch val := v.(type) {
	case model.Int, model.Decimal, model.Bool, model.Enum, model.Quantity, model.Ref:
		b.WriteString(FormatScalarValue(val))
	case model.String:
		s := string(val)
		if syntax.IsRawCandidate(s) {
			writeRawLines(&b, s, "")
		} else {
			b.WriteString(syntax.QuoteString(s))
		}
	case model.List:
		writeList(&b, len(val), func(i int) model.Value { return val[i] })
	case model.Record:
		writeBlock(&b, ast.BlockFromRecord(val), 0)
	case model.Collection:
		for i, elem := range val.Elems {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(syntax.QuoteString(elem.Label))
			b.WriteByte(' ')
			writeBlock(&b, ast.BlockFromRecord(elem.Body), 0)
		}
	}
	return b.Bytes()
}
