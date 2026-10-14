package lkal

import (
	"bytes"
	"strconv"
	"strings"
)

// Format parses src and returns its canonical form. On a parse error it
// returns (nil, *ErrorList) exactly as Parse("", src) does.
func Format(src []byte) ([]byte, error) {
	f, err := Parse("", src)
	if err != nil {
		return nil, err
	}
	return f.Format(), nil
}

// Format returns the canonical text of f (Files from Parse, FromJSON, or after Set/Delete).
func (f *File) Format() []byte {
	if f == nil {
		return nil
	}
	var b bytes.Buffer

	for i, c := range f.head {
		if i > 0 && c.blankBefore {
			b.WriteByte('\n')
		}
		writeComment(&b, "", c.text)
	}
	if len(f.head) > 0 && f.versionPos.Line > f.head[len(f.head)-1].pos.Line+1 {
		b.WriteByte('\n')
	}

	b.WriteString("@lkal ")
	b.WriteString(quoteString(f.version))
	b.WriteByte('\n')

	if len(f.body.entries) > 0 || len(f.body.trailing) > 0 {
		b.WriteByte('\n')
		formatBlockItems(&b, f.body, 0)
	}
	return b.Bytes()
}

// FormatValue returns the canonical text of one value, without a trailing newline.
func FormatValue(v Value) []byte {
	if v == nil {
		return nil
	}
	var b bytes.Buffer
	switch val := v.(type) {
	case Int, Decimal, Bool, Enum, Quantity, Ref:
		b.WriteString(formatScalarValue(val))
	case String:
		s := string(val)
		if isRawCandidate(s) {
			writeRawLines(&b, s, "")
		} else {
			b.WriteString(quoteString(s))
		}
	case List:
		writeList(&b, len(val), func(i int) Value { return val[i] })
	case Record:
		writeBlock(&b, blockFromRecord(val), 0)
	}
	return b.Bytes()
}

func formatBlockItems(b *bytes.Buffer, blk *block, d int) {
	indent := strings.Repeat("  ", d)
	childIndent := indent + "  "
	isFirstItem := true
	for _, ent := range blk.entries {
		if !isFirstItem {
			blank := ent.blankBefore
			if len(ent.comments) > 0 {
				blank = ent.comments[0].blankBefore
			}
			if blank {
				b.WriteByte('\n')
			}
		}
		isFirstItem = false

		for i, c := range ent.comments {
			if i > 0 && c.blankBefore {
				b.WriteByte('\n')
			}
			writeComment(b, indent, c.text)
		}
		if len(ent.comments) > 0 && ent.blankBefore {
			b.WriteByte('\n')
		}

		formatEntry(b, ent, d, indent, childIndent)
	}

	for _, c := range blk.trailing {
		if !isFirstItem && c.blankBefore {
			b.WriteByte('\n')
		}
		isFirstItem = false
		writeComment(b, indent, c.text)
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

func formatHeader(ent *entry) string {
	var b strings.Builder
	b.WriteString(formatKey(ent.key))
	for _, lbl := range ent.labels {
		b.WriteByte(' ')
		b.WriteString(quoteString(lbl.text))
	}
	return b.String()
}

func formatEntry(b *bytes.Buffer, ent *entry, d int, indent, childIndent string) {
	switch ent.kind {
	case entryAssign:
		formatAssign(b, ent, indent, childIndent)
	case entryRecord:
		formatBody(b, indent+formatHeader(ent), ent.body, d)
	}
}

func formatAssign(b *bytes.Buffer, ent *entry, indent, childIndent string) {
	b.WriteString(indent)
	b.WriteString(formatHeader(ent))
	if s, ok := nodeString(ent.value); ok && isRawCandidate(s) {
		b.WriteString(" =\n")
		writeRawLines(b, s, childIndent)
		b.WriteByte('\n')
		return
	}
	if l, ok := ent.value.(*listNode); ok && !l.oneLine && len(l.elems) > 0 {
		b.WriteString(" = [\n")
		for _, elem := range l.elems {
			b.WriteString(childIndent)
			b.WriteString(formatScalarValue(elem.val))
			b.WriteString(",\n")
		}
		b.WriteString(indent)
		b.WriteString("]\n")
		return
	}
	b.WriteString(" = ")
	writeInlineValue(b, ent.value)
	b.WriteByte('\n')
}

func formatBody(b *bytes.Buffer, headerPrefix string, body *block, d int) {
	if isOneLineBlock(body) {
		b.WriteString(headerPrefix)
		b.WriteString(" { ")
		for i, ent := range body.entries {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(formatKey(ent.key))
			b.WriteString(" = ")
			writeInlineValue(b, ent.value)
		}
		b.WriteString(" }\n")
		return
	}
	b.WriteString(headerPrefix)
	b.WriteByte(' ')
	writeBlock(b, body, d)
	b.WriteByte('\n')
}

func writeBlock(b *bytes.Buffer, blk *block, d int) {
	if len(blk.entries) == 0 && len(blk.trailing) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	formatBlockItems(b, blk, d+1)
	b.WriteString(strings.Repeat("  ", d))
	b.WriteString("}")
}

func writeInlineValue(b *bytes.Buffer, n node) {
	switch vn := n.(type) {
	case *scalarNode:
		b.WriteString(formatScalarValue(vn.val))
	case *rawNode:
		b.WriteString(quoteString(strings.Join(vn.lines, "\n")))
	case *listNode:
		writeList(b, len(vn.elems), func(i int) Value { return vn.elems[i].val })
	}
}

func writeList(b *bytes.Buffer, n int, at func(int) Value) {
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatScalarValue(at(i)))
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

func isOneLineBlock(b *block) bool {
	if !b.oneLine || len(b.trailing) > 0 || len(b.entries) == 0 {
		return false
	}
	for _, e := range b.entries {
		if e.kind != entryAssign || len(e.labels) > 0 || len(e.comments) > 0 || !isOneLineValue(e.value) {
			return false
		}
	}
	return true
}

func isOneLineValue(n node) bool {
	switch vn := n.(type) {
	case *scalarNode:
		if s, ok := vn.val.(String); ok {
			return !isRawCandidate(string(s))
		}
		return true
	case *listNode:
		return vn.oneLine
	default:
		return false
	}
}

func nodeString(n node) (string, bool) {
	switch vn := n.(type) {
	case *scalarNode:
		if s, ok := vn.val.(String); ok {
			return string(s), true
		}
	case *rawNode:
		return strings.Join(vn.lines, "\n"), true
	}
	return "", false
}

func isRawCandidate(s string) bool {
	hasLF := false
	for _, r := range s {
		if r == '\n' {
			hasLF = true
		} else if isControlChar(r) {
			return false
		}
	}
	return hasLF
}

func formatScalarValue(v Value) string {
	switch val := v.(type) {
	case Int:
		return strconv.FormatInt(int64(val), 10)
	case Decimal:
		return string(val)
	case Bool:
		if val {
			return "true"
		}
		return "false"
	case Enum:
		return "." + string(val)
	case Quantity:
		return val.Number + val.Unit
	case Ref:
		return "&" + refBody(val)
	case String:
		return quoteString(string(val))
	default:
		return ""
	}
}

func formatKey(k string) string {
	if isBareKey(k) {
		return k
	}
	return quoteString(k)
}

func isBareKey(k string) bool {
	if k == "" || k == "true" || k == "false" || !lexIsNameStart(k[0]) {
		return false
	}
	for i := 1; i < len(k); i++ {
		if !lexIsNameChar(k[i]) {
			return false
		}
	}
	return true
}

func isBareRefSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !lexIsRefSegmentChar(s[i]) {
			return false
		}
	}
	return true
}

func refBody(r Ref) string {
	var b strings.Builder
	for i, seg := range r {
		if i > 0 {
			b.WriteByte('/')
		}
		if isBareRefSegment(seg) {
			b.WriteString(seg)
		} else {
			b.WriteString(quoteString(seg))
		}
	}
	return b.String()
}

func quoteString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if isControlChar(r) {
				b.WriteString(`\u{`)
				b.WriteString(strconv.FormatInt(int64(r), 16))
				b.WriteByte('}')
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
