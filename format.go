package lkal

import (
	"strconv"
	"strings"
	"unicode/utf8"
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
	var b strings.Builder

	for i, c := range f.head {
		if i > 0 && c.blankBefore {
			b.WriteByte('\n')
		}
		writeComment(&b, 0, c.text)
	}
	if len(f.head) > 0 {
		lastCommentLine := f.head[len(f.head)-1].pos.Line
		if f.versionPos.Line > lastCommentLine+1 {
			b.WriteByte('\n')
		}
	}

	b.WriteString("@lkal ")
	b.WriteString(quoteString(f.version))
	b.WriteByte('\n')

	hasBodyItems := f.body != nil && (len(f.body.entries) > 0 || len(f.body.trailing) > 0)
	if !hasBodyItems {
		return []byte(b.String())
	}

	b.WriteByte('\n')
	formatBlockItems(&b, f.body, 0)
	return []byte(b.String())
}

// FormatValue returns the canonical text of one value, without a trailing newline.
func FormatValue(v Value) []byte {
	if v == nil {
		return nil
	}
	var b strings.Builder
	switch val := v.(type) {
	case Int, Decimal, Bool, Enum, Quantity, Ref:
		b.WriteString(formatScalarValue(val))
	case String:
		s := string(val)
		if isRawCandidate(s) {
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(`\\`)
				b.WriteString(line)
			}
		} else {
			b.WriteString(quoteString(s))
		}
	case List:
		b.WriteString(formatValueList(val))
	case Record:
		if len(val.Fields) == 0 {
			b.WriteString("{}")
		} else {
			b.WriteString("{\n")
			formatValueRecordFields(&b, val, 1)
			b.WriteString("}")
		}
	case Collection:
		for i, elem := range val.Elems {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(quoteString(elem.Label))
			if len(elem.Body.Fields) == 0 {
				b.WriteString(" {}")
			} else {
				b.WriteString(" {\n")
				formatValueRecordFields(&b, elem.Body, 1)
				b.WriteString("}")
			}
		}
	}
	return []byte(b.String())
}

func formatValueList(l List) string {
	if len(l) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, elem := range l {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatScalarValue(elem))
	}
	b.WriteByte(']')
	return b.String()
}

func formatValueRecordFields(b *strings.Builder, r Record, level int) {
	indent := strings.Repeat("  ", level)
	childIndent := strings.Repeat("  ", level+1)
	for _, f := range r.Fields {
		keyText := formatKey(f.Key)
		switch fv := f.Value.(type) {
		case Int, Decimal, Bool, Enum, Quantity, Ref:
			b.WriteString(indent)
			b.WriteString(keyText)
			b.WriteString(" = ")
			b.WriteString(formatScalarValue(fv))
			b.WriteByte('\n')
		case String:
			s := string(fv)
			if isRawCandidate(s) {
				b.WriteString(indent)
				b.WriteString(keyText)
				b.WriteString(" =\n")
				lines := strings.Split(s, "\n")
				for _, line := range lines {
					b.WriteString(childIndent)
					b.WriteString(`\\`)
					b.WriteString(line)
					b.WriteByte('\n')
				}
			} else {
				b.WriteString(indent)
				b.WriteString(keyText)
				b.WriteString(" = ")
				b.WriteString(quoteString(s))
				b.WriteByte('\n')
			}
		case List:
			b.WriteString(indent)
			b.WriteString(keyText)
			b.WriteString(" = ")
			b.WriteString(formatValueList(fv))
			b.WriteByte('\n')
		case Record:
			if len(fv.Fields) == 0 {
				b.WriteString(indent)
				b.WriteString(keyText)
				b.WriteString(" {}\n")
			} else {
				b.WriteString(indent)
				b.WriteString(keyText)
				b.WriteString(" {\n")
				formatValueRecordFields(b, fv, level+1)
				b.WriteString(indent)
				b.WriteString("}\n")
			}
		case Collection:
			for _, elem := range fv.Elems {
				b.WriteString(indent)
				b.WriteString(keyText)
				b.WriteByte(' ')
				b.WriteString(quoteString(elem.Label))
				if len(elem.Body.Fields) == 0 {
					b.WriteString(" {}\n")
				} else {
					b.WriteString(" {\n")
					formatValueRecordFields(b, elem.Body, level+1)
					b.WriteString(indent)
					b.WriteString("}\n")
				}
			}
		}
	}
}

func formatBlockItems(b *strings.Builder, blk *block, d int) {
	if blk == nil {
		return
	}
	isFirstItem := true
	for _, ent := range blk.entries {
		if !isFirstItem {
			itemBlankBefore := ent.blankBefore
			if len(ent.comments) > 0 {
				itemBlankBefore = ent.comments[0].blankBefore
			}
			if itemBlankBefore {
				b.WriteByte('\n')
			}
		}
		isFirstItem = false

		for i, c := range ent.comments {
			if i > 0 && c.blankBefore {
				b.WriteByte('\n')
			}
			writeComment(b, d, c.text)
		}
		if len(ent.comments) > 0 && ent.blankBefore {
			b.WriteByte('\n')
		}

		formatEntry(b, ent, d)
	}

	for _, c := range blk.trailing {
		if !isFirstItem {
			if c.blankBefore {
				b.WriteByte('\n')
			}
		}
		isFirstItem = false
		writeComment(b, d, c.text)
	}
}

func writeComment(b *strings.Builder, d int, text string) {
	b.WriteString(strings.Repeat("  ", d))
	b.WriteString("###")
	if text != "" {
		b.WriteByte(' ')
		b.WriteString(text)
	}
	b.WriteByte('\n')
}

func formatEntry(b *strings.Builder, ent *entry, d int) {
	indent := strings.Repeat("  ", d)
	switch ent.kind {
	case entryAssign:
		formatAssign(b, ent, d)
	case entryRecord:
		headerPrefix := indent + formatKey(ent.key)
		formatBody(b, headerPrefix, ent.body, d)
	case entryElem:
		headerPrefix := indent + formatKey(ent.key) + " " + quoteString(ent.label)
		formatBody(b, headerPrefix, ent.body, d)
	}
}

func formatAssign(b *strings.Builder, ent *entry, d int) {
	indent := strings.Repeat("  ", d)
	childIndent := strings.Repeat("  ", d+1)
	keyText := formatKey(ent.key)

	str, isRaw := getRawCandidateString(ent.value)
	if isRaw {
		b.WriteString(indent)
		b.WriteString(keyText)
		b.WriteString(" =\n")
		lines := strings.Split(str, "\n")
		for _, line := range lines {
			b.WriteString(childIndent)
			b.WriteString(`\\`)
			b.WriteString(line)
			b.WriteByte('\n')
		}
		return
	}

	switch valNode := ent.value.(type) {
	case *rawNode:
		s := strings.Join(valNode.lines, "\n")
		b.WriteString(indent)
		b.WriteString(keyText)
		b.WriteString(" = ")
		b.WriteString(quoteString(s))
		b.WriteByte('\n')
	case *scalarNode:
		b.WriteString(indent)
		b.WriteString(keyText)
		b.WriteString(" = ")
		b.WriteString(formatScalarValue(valNode.val))
		b.WriteByte('\n')
	case *listNode:
		if len(valNode.elems) == 0 {
			b.WriteString(indent)
			b.WriteString(keyText)
			b.WriteString(" = []\n")
		} else if valNode.oneLine {
			b.WriteString(indent)
			b.WriteString(keyText)
			b.WriteString(" = [")
			for i, elem := range valNode.elems {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(formatScalarValue(elem.val))
			}
			b.WriteString("]\n")
		} else {
			b.WriteString(indent)
			b.WriteString(keyText)
			b.WriteString(" = [\n")
			for _, elem := range valNode.elems {
				b.WriteString(childIndent)
				b.WriteString(formatScalarValue(elem.val))
				b.WriteString(",\n")
			}
			b.WriteString(indent)
			b.WriteString("]\n")
		}
	}
}

func formatBody(b *strings.Builder, headerPrefix string, body *block, d int) {
	if body == nil || (len(body.entries) == 0 && len(body.trailing) == 0) {
		b.WriteString(headerPrefix)
		b.WriteString(" {}\n")
		return
	}
	if isOneLineBlock(body) {
		b.WriteString(headerPrefix)
		b.WriteString(" { ")
		for i, ent := range body.entries {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(formatKey(ent.key))
			b.WriteString(" = ")
			b.WriteString(formatInlineValue(ent.value))
		}
		b.WriteString(" }\n")
		return
	}
	b.WriteString(headerPrefix)
	b.WriteString(" {\n")
	formatBlockItems(b, body, d+1)
	b.WriteString(strings.Repeat("  ", d))
	b.WriteString("}\n")
}

func formatInlineValue(n node) string {
	switch vn := n.(type) {
	case *scalarNode:
		return formatScalarValue(vn.val)
	case *rawNode:
		return quoteString(strings.Join(vn.lines, "\n"))
	case *listNode:
		if len(vn.elems) == 0 {
			return "[]"
		}
		var b strings.Builder
		b.WriteByte('[')
		for i, elem := range vn.elems {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(formatScalarValue(elem.val))
		}
		b.WriteByte(']')
		return b.String()
	default:
		return ""
	}
}

func isOneLineBlock(b *block) bool {
	if b == nil || !b.oneLine || len(b.trailing) > 0 || len(b.entries) == 0 {
		return false
	}
	for _, e := range b.entries {
		if e.kind != entryAssign || len(e.comments) > 0 {
			return false
		}
		if !isOneLineValue(e.value) {
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
	case *rawNode:
		return !isRawCandidate(strings.Join(vn.lines, "\n"))
	case *listNode:
		return vn.oneLine
	default:
		return false
	}
}

func getRawCandidateString(n node) (string, bool) {
	switch vn := n.(type) {
	case *scalarNode:
		if s, ok := vn.val.(String); ok {
			str := string(s)
			if isRawCandidate(str) {
				return str, true
			}
		}
	case *rawNode:
		str := strings.Join(vn.lines, "\n")
		if isRawCandidate(str) {
			return str, true
		}
	}
	return "", false
}

func isRawCandidate(s string) bool {
	hasLF := false
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if r == '\n' {
			hasLF = true
		} else if (r < 0x20 && r != '\t') || r == 0x7f {
			return false
		}
		i += sz
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
	if k == "" || k == "true" || k == "false" {
		return false
	}
	if !lexIsNameStart(k[0]) {
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
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && sz == 1 {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteByte('\t')
		default:
			if (r < 0x20) || r == 0x7f {
				b.WriteString(`\u{`)
				b.WriteString(strconv.FormatInt(int64(r), 16))
				b.WriteByte('}')
			} else {
				b.WriteRune(r)
			}
		}
		i += sz
	}
	b.WriteByte('"')
	return b.String()
}
