package lkal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

func ToJSON(f *File, plain bool) ([]byte, error) {
	if f == nil {
		return nil, errors.New("lkal: nil File")
	}
	var buf bytes.Buffer
	fields := f.Data().Fields
	if plain {
		if len(fields) == 0 {
			return []byte("{}\n"), nil
		}
		buf.WriteString("{\n")
		for i, field := range fields {
			buf.WriteString("  ")
			buf.WriteString(escapeJSONString(field.Key))
			buf.WriteString(": ")
			formatValue(&buf, field.Value, 1, true)
			if i < len(fields)-1 {
				buf.WriteString(",")
			}
			buf.WriteByte('\n')
		}
		buf.WriteString("}\n")
		return buf.Bytes(), nil
	}

	buf.WriteString("{\n  \"$lkal\": ")
	buf.WriteString(escapeJSONString(f.Version()))
	for _, field := range fields {
		buf.WriteString(",\n  ")
		buf.WriteString(escapeJSONString(field.Key))
		buf.WriteString(": ")
		formatValue(&buf, field.Value, 1, false)
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}

func escapeJSONString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return string(b)
}

func writeIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString("  ")
	}
}

func formatValue(buf *bytes.Buffer, v Value, depth int, plain bool) {
	switch val := v.(type) {
	case String:
		buf.WriteString(escapeJSONString(string(val)))
	case Int:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case Decimal:
		buf.WriteString(string(val))
	case Bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case Enum:
		if plain {
			buf.WriteString(escapeJSONString(string(val)))
		} else {
			buf.WriteString("{\n")
			writeIndent(buf, depth+1)
			buf.WriteString("\"$enum\": ")
			buf.WriteString(escapeJSONString(string(val)))
			buf.WriteByte('\n')
			writeIndent(buf, depth)
			buf.WriteString("}")
		}
	case Quantity:
		payload := val.Number + val.Unit
		if plain {
			buf.WriteString(escapeJSONString(payload))
		} else {
			buf.WriteString("{\n")
			writeIndent(buf, depth+1)
			buf.WriteString("\"$qty\": ")
			buf.WriteString(escapeJSONString(payload))
			buf.WriteByte('\n')
			writeIndent(buf, depth)
			buf.WriteString("}")
		}
	case Ref:
		payload := refBody(val)
		if plain {
			buf.WriteString(escapeJSONString(payload))
		} else {
			buf.WriteString("{\n")
			writeIndent(buf, depth+1)
			buf.WriteString("\"$ref\": ")
			buf.WriteString(escapeJSONString(payload))
			buf.WriteByte('\n')
			writeIndent(buf, depth)
			buf.WriteString("}")
		}
	case List:
		if len(val) == 0 {
			buf.WriteString("[]")
			return
		}
		buf.WriteString("[\n")
		for i, el := range val {
			writeIndent(buf, depth+1)
			formatValue(buf, el, depth+1, plain)
			if i < len(val)-1 {
				buf.WriteString(",")
			}
			buf.WriteByte('\n')
		}
		writeIndent(buf, depth)
		buf.WriteString("]")
	case Record:
		if len(val.Fields) == 0 {
			buf.WriteString("{}")
			return
		}
		buf.WriteString("{\n")
		for i, f := range val.Fields {
			writeIndent(buf, depth+1)
			buf.WriteString(escapeJSONString(f.Key))
			buf.WriteString(": ")
			formatValue(buf, f.Value, depth+1, plain)
			if i < len(val.Fields)-1 {
				buf.WriteString(",")
			}
			buf.WriteByte('\n')
		}
		writeIndent(buf, depth)
		buf.WriteString("}")
	case Collection:
		if plain {
			if len(val.Elems) == 0 {
				buf.WriteString("{}")
				return
			}
			buf.WriteString("{\n")
			for i, el := range val.Elems {
				writeIndent(buf, depth+1)
				buf.WriteString(escapeJSONString(el.Label))
				buf.WriteString(": ")
				formatValue(buf, el.Body, depth+1, plain)
				if i < len(val.Elems)-1 {
					buf.WriteString(",")
				}
				buf.WriteByte('\n')
			}
			writeIndent(buf, depth)
			buf.WriteString("}")
		} else {
			buf.WriteString("{\n")
			writeIndent(buf, depth+1)
			buf.WriteString("\"$coll\": ")
			if len(val.Elems) == 0 {
				buf.WriteString("{}")
			} else {
				buf.WriteString("{\n")
				for i, el := range val.Elems {
					writeIndent(buf, depth+2)
					buf.WriteString(escapeJSONString(el.Label))
					buf.WriteString(": ")
					formatValue(buf, el.Body, depth+2, plain)
					if i < len(val.Elems)-1 {
						buf.WriteString(",")
					}
					buf.WriteByte('\n')
				}
				writeIndent(buf, depth+1)
				buf.WriteString("}")
			}
			buf.WriteByte('\n')
			writeIndent(buf, depth)
			buf.WriteString("}")
		}
	}
}

func nodeFromValue(v Value) node {
	if lst, ok := v.(List); ok {
		elems := make([]*scalarNode, len(lst))
		for i, el := range lst {
			elems[i] = &scalarNode{val: el}
		}
		return &listNode{elems: elems, oneLine: true}
	}
	return &scalarNode{val: v}
}

func blockFromRecord(r Record) *block {
	var entries []*entry
	for _, f := range r.Fields {
		switch val := f.Value.(type) {
		case Record:
			entries = append(entries, &entry{
				kind: entryRecord,
				key:  f.Key,
				body: blockFromRecord(val),
			})
		case Collection:
			for _, el := range val.Elems {
				entries = append(entries, &entry{
					kind:  entryElem,
					key:   f.Key,
					label: el.Label,
					body:  blockFromRecord(el.Body),
				})
			}
		default:
			entries = append(entries, &entry{
				kind:  entryAssign,
				key:   f.Key,
				value: nodeFromValue(val),
			})
		}
	}
	return &block{
		entries: entries,
		oneLine: false,
	}
}

func FromJSON(src []byte) (*File, error) {
	if bytes.HasPrefix(src, []byte("\xef\xbb\xbf")) {
		return nil, &ErrorList{Errors: []*Error{newError("", Pos{Line: 1, Col: 1}, "L301", "")}}
	}

	p := &jsonParser{src: src}
	if !p.skipWhitespace() {
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}

	if p.src[p.cursor] != '{' {
		p.recordError(p.cursor, "L105", `start the top-level object with "$lkal": "0.1"`)
		p.skipValue()
		p.skipWhitespace()
		if p.cursor < len(p.src) {
			p.syntaxError(p.cursor)
		}
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}

	p.cursor++
	if !p.skipWhitespace() {
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	if p.src[p.cursor] == '}' {
		p.recordError(p.cursor, "L105", `start the top-level object with "$lkal": "0.1"`)
		p.cursor++
		p.skipWhitespace()
		if p.cursor < len(p.src) {
			p.syntaxError(p.cursor)
		}
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}

	if p.src[p.cursor] != '"' {
		p.syntaxError(p.cursor)
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}

	firstKey, firstKeyOff, ok := p.parseString()
	if !ok {
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	if !p.skipWhitespace() {
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
		p.syntaxError(p.cursor)
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}
	p.cursor++

	seenKeys := make(map[string]bool)
	seenTopLkal := false
	var topFields []Field

	if firstKey == "$lkal" {
		seenTopLkal = true
		if !p.skipWhitespace() {
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.src[p.cursor] != '"' {
			valOff := p.cursor
			p.skipValue()
			p.recordError(valOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		} else {
			verStr, verOff, ok := p.parseString()
			if !ok {
				return nil, &ErrorList{Errors: sortErrors(p.errors)}
			}
			if verStr != "0.1" {
				p.recordError(verOff, "L207", "")
			}
		}
	} else {
		p.recordError(firstKeyOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		hasKeyError := false
		if strings.HasPrefix(firstKey, "$") {
			hasKeyError = true
		}
		seenKeys[firstKey] = true
		if hasKeyError {
			p.skipValue()
		} else {
			val, ok := p.parseValue(false)
			if ok {
				topFields = append(topFields, Field{Key: firstKey, Value: val})
			}
		}
	}

	for {
		if !p.skipWhitespace() {
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.src[p.cursor] == '}' {
			p.cursor++
			break
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		p.cursor++
		if !p.skipWhitespace() {
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.src[p.cursor] == '}' {
			p.syntaxError(p.cursor)
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.src[p.cursor] != '"' {
			p.syntaxError(p.cursor)
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		k, kOff, ok := p.parseString()
		if !ok {
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if !p.skipWhitespace() {
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
			p.syntaxError(p.cursor)
			return nil, &ErrorList{Errors: sortErrors(p.errors)}
		}
		p.cursor++

		hasKeyError := false
		if k == "$lkal" && seenTopLkal {
			hasKeyError = true
			p.recordError(kOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		} else if strings.HasPrefix(k, "$") {
			hasKeyError = true
			p.recordError(kOff, "L205", "")
		} else if seenKeys[k] {
			hasKeyError = true
			p.recordError(kOff, "L201", "")
		}
		seenKeys[k] = true

		if hasKeyError {
			p.skipValue()
		} else {
			val, ok := p.parseValue(false)
			if ok {
				topFields = append(topFields, Field{Key: k, Value: val})
			}
		}
	}

	p.skipWhitespace()
	if p.cursor < len(p.src) {
		p.syntaxError(p.cursor)
	}

	if len(p.errors) > 0 {
		return nil, &ErrorList{Errors: sortErrors(p.errors)}
	}

	return &File{version: "0.1", body: blockFromRecord(Record{Fields: topFields})}, nil
}

type jsonParser struct {
	src    []byte
	cursor int
	errors []*Error
	failed bool
}

func (p *jsonParser) syntaxError(off int) {
	if !p.failed {
		p.failed = true
		pos := posAt(p.src, off)
		p.errors = append(p.errors, newError("", pos, "L301", ""))
	}
}

func (p *jsonParser) recordError(off int, code, hint string) {
	if !p.failed {
		pos := posAt(p.src, off)
		p.errors = append(p.errors, newError("", pos, code, hint))
	}
}

func posAt(src []byte, off int) Pos {
	if off > len(src) {
		off = len(src)
	}
	line := 1
	col := 1
	for i := 0; i < off; {
		if src[i] == '\n' {
			line++
			col = 1
			i++
			continue
		}
		if src[i] == '\r' && i+1 < len(src) && src[i+1] == '\n' {
			i++
			continue
		}
		_, sz := utf8.DecodeRune(src[i:])
		col++
		i += sz
	}
	return Pos{Line: line, Col: col}
}

func (p *jsonParser) skipWhitespace() bool {
	for p.cursor < len(p.src) {
		b := p.src[p.cursor]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			p.cursor++
			continue
		}
		r, sz := utf8.DecodeRune(p.src[p.cursor:])
		if r == utf8.RuneError && sz == 1 {
			p.syntaxError(p.cursor)
			return false
		}
		return true
	}
	return true
}

func parseHex4(b []byte) (rune, bool) {
	var val rune
	for _, c := range b {
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case c >= 'a' && c <= 'f':
			d = rune(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			d = rune(c - 'A' + 10)
		default:
			return 0, false
		}
		val = (val << 4) | d
	}
	return val, true
}

func (p *jsonParser) parseString() (string, int, bool) {
	if !p.skipWhitespace() {
		return "", 0, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return "", 0, false
	}
	startOff := p.cursor
	if p.src[p.cursor] != '"' {
		p.syntaxError(p.cursor)
		return "", 0, false
	}
	p.cursor++

	var decoded strings.Builder
	for {
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return "", 0, false
		}
		b := p.src[p.cursor]
		if b == '"' {
			p.cursor++
			return decoded.String(), startOff, true
		}
		if b == '\\' {
			bsOff := p.cursor
			p.cursor++
			if p.cursor >= len(p.src) {
				p.syntaxError(len(p.src))
				return "", 0, false
			}
			esc := p.src[p.cursor]
			p.cursor++
			switch esc {
			case '"':
				decoded.WriteByte('"')
			case '\\':
				decoded.WriteByte('\\')
			case '/':
				decoded.WriteByte('/')
			case 'b':
				decoded.WriteByte('\b')
			case 'f':
				decoded.WriteByte('\f')
			case 'n':
				decoded.WriteByte('\n')
			case 'r':
				decoded.WriteByte('\r')
			case 't':
				decoded.WriteByte('\t')
			case 'u':
				if p.cursor+4 > len(p.src) {
					p.syntaxError(len(p.src))
					return "", 0, false
				}
				hex4 := p.src[p.cursor : p.cursor+4]
				val, ok := parseHex4(hex4)
				if !ok {
					for k := 0; k < 4; k++ {
						if !isHexDigit(hex4[k]) {
							p.syntaxError(p.cursor + k)
							return "", 0, false
						}
					}
					p.syntaxError(p.cursor)
					return "", 0, false
				}
				p.cursor += 4
				if val >= 0xD800 && val <= 0xDBFF {
					if p.cursor+6 <= len(p.src) && p.src[p.cursor] == '\\' && p.src[p.cursor+1] == 'u' {
						lowHex4 := p.src[p.cursor+2 : p.cursor+6]
						lowVal, lowOk := parseHex4(lowHex4)
						if lowOk && lowVal >= 0xDC00 && lowVal <= 0xDFFF {
							p.cursor += 6
							combined := 0x10000 + ((val - 0xD800) << 10) | (lowVal - 0xDC00)
							decoded.WriteRune(rune(combined))
							continue
						}
					}
					p.syntaxError(bsOff)
					return "", 0, false
				} else if val >= 0xDC00 && val <= 0xDFFF {
					p.syntaxError(bsOff)
					return "", 0, false
				} else {
					decoded.WriteRune(rune(val))
				}
			default:
				p.syntaxError(p.cursor - 1)
				return "", 0, false
			}
			continue
		}
		if b < 0x20 {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
		r, sz := utf8.DecodeRune(p.src[p.cursor:])
		if r == utf8.RuneError && sz == 1 {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
		decoded.WriteRune(r)
		p.cursor += sz
	}
}

func (p *jsonParser) parseNumber() (string, int, bool) {
	if !p.skipWhitespace() {
		return "", 0, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return "", 0, false
	}
	startOff := p.cursor

	if p.src[p.cursor] == '-' {
		p.cursor++
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return "", 0, false
		}
		if p.src[p.cursor] < '0' || p.src[p.cursor] > '9' {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
	}

	if p.src[p.cursor] == '0' {
		p.cursor++
		if p.cursor < len(p.src) && p.src[p.cursor] >= '0' && p.src[p.cursor] <= '9' {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
	} else if p.src[p.cursor] >= '1' && p.src[p.cursor] <= '9' {
		p.cursor++
		for p.cursor < len(p.src) && p.src[p.cursor] >= '0' && p.src[p.cursor] <= '9' {
			p.cursor++
		}
	} else {
		p.syntaxError(p.cursor)
		return "", 0, false
	}

	if p.cursor < len(p.src) && p.src[p.cursor] == '.' {
		p.cursor++
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return "", 0, false
		}
		if p.src[p.cursor] < '0' || p.src[p.cursor] > '9' {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
		for p.cursor < len(p.src) && p.src[p.cursor] >= '0' && p.src[p.cursor] <= '9' {
			p.cursor++
		}
	}

	if p.cursor < len(p.src) && (p.src[p.cursor] == 'e' || p.src[p.cursor] == 'E') {
		p.cursor++
		if p.cursor < len(p.src) && (p.src[p.cursor] == '+' || p.src[p.cursor] == '-') {
			p.cursor++
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return "", 0, false
		}
		if p.src[p.cursor] < '0' || p.src[p.cursor] > '9' {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
		for p.cursor < len(p.src) && p.src[p.cursor] >= '0' && p.src[p.cursor] <= '9' {
			p.cursor++
		}
	}

	if p.cursor < len(p.src) {
		b := p.src[p.cursor]
		if !(b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ',' || b == ']' || b == '}') {
			p.syntaxError(p.cursor)
			return "", 0, false
		}
	}

	return string(p.src[startOff:p.cursor]), startOff, true
}

func (p *jsonParser) parseLiteral() (string, int, bool) {
	if !p.skipWhitespace() {
		return "", 0, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return "", 0, false
	}
	startOff := p.cursor

	var lit string
	switch {
	case bytes.HasPrefix(p.src[p.cursor:], []byte("true")):
		lit = "true"
	case bytes.HasPrefix(p.src[p.cursor:], []byte("false")):
		lit = "false"
	case bytes.HasPrefix(p.src[p.cursor:], []byte("null")):
		lit = "null"
	default:
		p.syntaxError(p.cursor)
		return "", 0, false
	}

	nextOff := p.cursor + len(lit)
	if nextOff < len(p.src) {
		b := p.src[nextOff]
		if !(b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ',' || b == ']' || b == '}') {
			p.syntaxError(nextOff)
			return "", 0, false
		}
	}
	p.cursor = nextOff
	return lit, startOff, true
}

func (p *jsonParser) skipValue() bool {
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return false
	}
	b := p.src[p.cursor]
	switch b {
	case '"':
		_, _, ok := p.parseString()
		return ok
	case '{':
		return p.skipObject()
	case '[':
		return p.skipArray()
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		_, _, ok := p.parseNumber()
		return ok
	case 't', 'f', 'n':
		_, _, ok := p.parseLiteral()
		return ok
	default:
		p.syntaxError(p.cursor)
		return false
	}
}

func (p *jsonParser) skipMembersAfterColon() bool {
	if !p.skipValue() {
		return false
	}
	for {
		if !p.skipWhitespace() {
			return false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return false
		}
		if p.src[p.cursor] == '}' {
			p.cursor++
			return true
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return false
		}
		p.cursor++
		if !p.skipWhitespace() {
			return false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return false
		}
		if p.src[p.cursor] == '}' {
			p.syntaxError(p.cursor)
			return false
		}
		_, _, ok := p.parseString()
		if !ok {
			return false
		}
		if !p.skipWhitespace() {
			return false
		}
		if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
			p.syntaxError(p.cursor)
			return false
		}
		p.cursor++
		if !p.skipValue() {
			return false
		}
	}
}

func (p *jsonParser) skipObject() bool {
	p.cursor++ // skip '{'
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return false
	}
	if p.src[p.cursor] == '}' {
		p.cursor++
		return true
	}
	_, _, ok := p.parseString()
	if !ok {
		return false
	}
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
		p.syntaxError(p.cursor)
		return false
	}
	p.cursor++
	return p.skipMembersAfterColon()
}

func (p *jsonParser) skipArray() bool {
	p.cursor++ // skip '['
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return false
	}
	if p.src[p.cursor] == ']' {
		p.cursor++
		return true
	}
	for {
		if !p.skipValue() {
			return false
		}
		if !p.skipWhitespace() {
			return false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return false
		}
		if p.src[p.cursor] == ']' {
			p.cursor++
			return true
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return false
		}
		p.cursor++
		if !p.skipWhitespace() {
			return false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return false
		}
		if p.src[p.cursor] == ']' {
			p.syntaxError(p.cursor)
			return false
		}
	}
}

func (p *jsonParser) hasSecondMemberAndSkip(tagHadError bool) bool {
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return false
	}
	if p.src[p.cursor] == '}' {
		p.cursor++
		return false
	}
	if p.src[p.cursor] != ',' {
		p.syntaxError(p.cursor)
		return false
	}
	p.cursor++
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return false
	}
	if p.src[p.cursor] == '}' {
		p.syntaxError(p.cursor)
		return false
	}
	_, kOff, ok := p.parseString()
	if !ok {
		return false
	}
	if !p.skipWhitespace() {
		return false
	}
	if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
		p.syntaxError(p.cursor)
		return false
	}
	p.cursor++
	if !tagHadError {
		p.recordError(kOff, "L305", "write a tag object with only one key")
	}
	p.skipMembersAfterColon()
	return true
}

func scalarType(v Value) string {
	switch v.(type) {
	case String:
		return "String"
	case Int:
		return "Int"
	case Decimal:
		return "Decimal"
	case Bool:
		return "Bool"
	case Enum:
		return "Enum"
	case Quantity:
		return "Quantity"
	case Ref:
		return "Ref"
	default:
		return ""
	}
}

func isValidEnumName(s string) bool {
	if len(s) == 0 {
		return false
	}
	if !lexIsNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !lexIsNameChar(s[i]) {
			return false
		}
	}
	return true
}

func (p *jsonParser) parseValue(skipErrors bool) (Value, bool) {
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	b := p.src[p.cursor]
	switch b {
	case '"':
		s, _, ok := p.parseString()
		if !ok {
			return nil, false
		}
		return String(s), true
	case 't', 'f':
		lit, _, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		return Bool(lit == "true"), true
	case 'n':
		lit, off, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		if lit == "null" && !skipErrors {
			p.recordError(off, "L302", "")
		}
		return nil, false
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		numStr, off, ok := p.parseNumber()
		if !ok {
			return nil, false
		}
		if skipErrors {
			return nil, true
		}
		if strings.ContainsAny(numStr, "eE") {
			p.recordError(off, "L303", "")
			return nil, false
		}
		digitCount := 0
		for i := 0; i < len(numStr); i++ {
			if numStr[i] >= '0' && numStr[i] <= '9' {
				digitCount++
			}
		}
		if digitCount > 100 {
			p.recordError(off, "L005", "")
			return nil, false
		}
		if strings.Contains(numStr, ".") {
			return Decimal(normalizeNumber(numStr)), true
		}
		if numStr == "-0" {
			return Int(0), true
		}
		n, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			p.recordError(off, "L005", "")
			return nil, false
		}
		return Int(n), true
	case '[':
		return p.parseArray(skipErrors)
	case '{':
		return p.parseObject(skipErrors)
	default:
		p.syntaxError(p.cursor)
		return nil, false
	}
}

func (p *jsonParser) parseArray(skipErrors bool) (Value, bool) {
	p.cursor++ // skip '['
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	if p.src[p.cursor] == ']' {
		p.cursor++
		return List{}, true
	}

	var elements []Value
	var elemOffsets []int
	hadElementError := false

	for {
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		elemStartOff := p.cursor
		elemB := p.src[p.cursor]

		if elemB == '[' {
			if !skipErrors {
				p.recordError(elemStartOff, "L304", "")
			}
			hadElementError = true
			if !p.skipArray() {
				return nil, false
			}
		} else if elemB == '{' {
			braceOff := p.cursor
			p.cursor++ // skip '{'
			if !p.skipWhitespace() {
				return nil, false
			}
			if p.cursor >= len(p.src) {
				p.syntaxError(len(p.src))
				return nil, false
			}
			if p.src[p.cursor] == '}' {
				p.cursor++
				if !skipErrors {
					p.recordError(braceOff, "L304", "")
				}
				hadElementError = true
			} else if p.src[p.cursor] != '"' {
				p.syntaxError(p.cursor)
				return nil, false
			} else {
				firstKey, _, ok := p.parseString()
				if !ok {
					return nil, false
				}
				if !p.skipWhitespace() {
					return nil, false
				}
				if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
					p.syntaxError(p.cursor)
					return nil, false
				}
				p.cursor++
				if firstKey == "$enum" || firstKey == "$qty" || firstKey == "$ref" {
					tagVal, ok := p.parseScalarTag(firstKey, skipErrors)
					if !ok {
						hadElementError = true
					} else {
						elements = append(elements, tagVal)
						elemOffsets = append(elemOffsets, braceOff)
					}
				} else {
					if !skipErrors {
						p.recordError(braceOff, "L304", "")
					}
					hadElementError = true
					if !p.skipMembersAfterColon() {
						return nil, false
					}
				}
			}
		} else {
			val, ok := p.parseValue(skipErrors)
			if !ok {
				hadElementError = true
			} else {
				elements = append(elements, val)
				elemOffsets = append(elemOffsets, elemStartOff)
			}
		}

		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == ']' {
			p.cursor++
			break
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		p.cursor++
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == ']' {
			p.syntaxError(p.cursor)
			return nil, false
		}
	}

	if hadElementError {
		return nil, false
	}
	if !skipErrors && len(elements) > 1 {
		firstType := scalarType(elements[0])
		for i := 1; i < len(elements); i++ {
			if scalarType(elements[i]) != firstType {
				p.recordError(elemOffsets[i], "L204", "")
				return nil, false
			}
		}
	}
	return List(elements), true
}

func (p *jsonParser) parseScalarTag(tag string, skipErrors bool) (Value, bool) {
	var val Value
	var ok bool
	switch tag {
	case "$enum":
		val, ok = p.parseEnumPayload(skipErrors)
	case "$qty":
		val, ok = p.parseQtyPayload(skipErrors)
	case "$ref":
		val, ok = p.parseRefPayload(skipErrors)
	}
	hasSecond := p.hasSecondMemberAndSkip(!ok || skipErrors)
	if hasSecond || !ok {
		return nil, false
	}
	return val, true
}

func (p *jsonParser) parseObject(skipErrors bool) (Value, bool) {
	p.cursor++ // skip '{'
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	if p.src[p.cursor] == '}' {
		p.cursor++
		return Record{}, true
	}
	if p.src[p.cursor] != '"' {
		p.syntaxError(p.cursor)
		return nil, false
	}
	firstKey, firstKeyOff, ok := p.parseString()
	if !ok {
		return nil, false
	}
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
		p.syntaxError(p.cursor)
		return nil, false
	}
	p.cursor++

	switch firstKey {
	case "$enum", "$qty", "$ref":
		return p.parseScalarTag(firstKey, skipErrors)
	case "$coll":
		return p.parseCollTag(skipErrors)
	default:
		return p.parseRecordMembers(firstKey, firstKeyOff, skipErrors)
	}
}

func (p *jsonParser) parseCollTag(skipErrors bool) (Value, bool) {
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	b := p.src[p.cursor]
	if b == 'n' {
		lit, nullOff, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		if lit == "null" && !skipErrors {
			p.recordError(nullOff, "L302", "")
		}
		p.hasSecondMemberAndSkip(true)
		return nil, false
	}
	if b != '{' {
		valOff := p.cursor
		p.skipValue()
		if !skipErrors {
			p.recordError(valOff, "L305", `write {"$coll": {"label": {...}}} with at least one element`)
		}
		p.hasSecondMemberAndSkip(true)
		return nil, false
	}

	collObjOff := p.cursor
	p.cursor++ // skip '{'
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	if p.src[p.cursor] == '}' {
		p.cursor++
		if !skipErrors {
			p.recordError(collObjOff, "L305", `write {"$coll": {"label": {...}}} with at least one element`)
		}
		p.hasSecondMemberAndSkip(true)
		return nil, false
	}

	seenLabels := make(map[string]bool)
	var elems []Elem
	collHadError := false

	for {
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] != '"' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		label, labelOff, ok := p.parseString()
		if !ok {
			return nil, false
		}
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		p.cursor++

		isDup := seenLabels[label]
		if isDup && !collHadError && !skipErrors {
			p.recordError(labelOff, "L201", "")
			collHadError = true
		}
		seenLabels[label] = true

		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		valB := p.src[p.cursor]
		if valB == 'n' {
			lit, nullOff, ok := p.parseLiteral()
			if !ok {
				return nil, false
			}
			if lit == "null" && !collHadError && !skipErrors {
				p.recordError(nullOff, "L302", "")
				collHadError = true
			}
		} else if valB != '{' {
			valOff := p.cursor
			p.skipValue()
			if !collHadError && !skipErrors {
				p.recordError(valOff, "L305", `write {"$coll": {"label": {...}}} with at least one element`)
				collHadError = true
			}
		} else {
			recBraceOff := p.cursor
			elemRec, ok := p.parseCollElemRecord(recBraceOff, collHadError || skipErrors)
			if !ok {
				collHadError = true
			} else if !isDup && !collHadError {
				elems = append(elems, Elem{Label: label, Body: elemRec})
			}
		}

		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == '}' {
			p.cursor++
			break
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		p.cursor++
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == '}' {
			p.syntaxError(p.cursor)
			return nil, false
		}
	}

	hasSecond := p.hasSecondMemberAndSkip(collHadError || skipErrors)
	if hasSecond || collHadError {
		return nil, false
	}
	return Collection{Elems: elems}, true
}

func (p *jsonParser) parseCollElemRecord(braceOff int, skipErrors bool) (Record, bool) {
	p.cursor++ // skip '{'
	if !p.skipWhitespace() {
		return Record{}, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return Record{}, false
	}
	if p.src[p.cursor] == '}' {
		p.cursor++
		return Record{}, true
	}
	if p.src[p.cursor] != '"' {
		p.syntaxError(p.cursor)
		return Record{}, false
	}
	firstKey, firstKeyOff, ok := p.parseString()
	if !ok {
		return Record{}, false
	}
	if !p.skipWhitespace() {
		return Record{}, false
	}
	if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
		p.syntaxError(p.cursor)
		return Record{}, false
	}
	p.cursor++

	if firstKey == "$enum" || firstKey == "$qty" || firstKey == "$ref" || firstKey == "$coll" {
		if !skipErrors {
			p.recordError(braceOff, "L305", `write {"$coll": {"label": {...}}} with at least one element`)
		}
		p.skipMembersAfterColon()
		return Record{}, false
	}

	val, ok := p.parseRecordMembers(firstKey, firstKeyOff, skipErrors)
	if !ok {
		return Record{}, false
	}
	return val.(Record), true
}

func (p *jsonParser) parseRecordMembers(firstKey string, firstKeyOff int, skipErrors bool) (Value, bool) {
	seenKeys := make(map[string]bool)
	var fields []Field
	recordHadError := false

	curKey := firstKey
	curKeyOff := firstKeyOff

	for {
		hasKeyError := false
		if strings.HasPrefix(curKey, "$") {
			hasKeyError = true
			if !skipErrors {
				p.recordError(curKeyOff, "L205", "")
			}
		} else if seenKeys[curKey] {
			hasKeyError = true
			if !skipErrors {
				p.recordError(curKeyOff, "L201", "")
			}
		}
		seenKeys[curKey] = true

		if hasKeyError {
			p.skipValue()
			recordHadError = true
		} else {
			val, ok := p.parseValue(skipErrors)
			if !ok {
				recordHadError = true
			} else {
				fields = append(fields, Field{Key: curKey, Value: val})
			}
		}

		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == '}' {
			p.cursor++
			break
		}
		if p.src[p.cursor] != ',' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		p.cursor++
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) {
			p.syntaxError(len(p.src))
			return nil, false
		}
		if p.src[p.cursor] == '}' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		if p.src[p.cursor] != '"' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		k, kOff, ok := p.parseString()
		if !ok {
			return nil, false
		}
		if !p.skipWhitespace() {
			return nil, false
		}
		if p.cursor >= len(p.src) || p.src[p.cursor] != ':' {
			p.syntaxError(p.cursor)
			return nil, false
		}
		p.cursor++
		curKey = k
		curKeyOff = kOff
	}

	if recordHadError {
		return nil, false
	}
	return Record{Fields: fields}, true
}

func (p *jsonParser) parseEnumPayload(skipErrors bool) (Value, bool) {
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	b := p.src[p.cursor]
	if b == 'n' {
		lit, nullOff, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		if lit == "null" && !skipErrors {
			p.recordError(nullOff, "L302", "")
		}
		return nil, false
	}
	if b == '"' {
		str, strOff, ok := p.parseString()
		if !ok {
			return nil, false
		}
		if isValidEnumName(str) {
			return Enum(str), true
		}
		if !skipErrors {
			p.recordError(strOff, "L305", `write the enum name as a string, for example {"$enum": "tcp"}`)
		}
		return nil, false
	}
	valOff := p.cursor
	p.skipValue()
	if !skipErrors {
		p.recordError(valOff, "L305", `write the enum name as a string, for example {"$enum": "tcp"}`)
	}
	return nil, false
}

func (p *jsonParser) parseQtyPayload(skipErrors bool) (Value, bool) {
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	b := p.src[p.cursor]
	if b == 'n' {
		lit, nullOff, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		if lit == "null" && !skipErrors {
			p.recordError(nullOff, "L302", "")
		}
		return nil, false
	}
	if b == '"' {
		str, strOff, ok := p.parseString()
		if !ok {
			return nil, false
		}
		if len(str) == 0 || (str[0] != '-' && (str[0] < '0' || str[0] > '9')) {
			if !skipErrors {
				p.recordError(strOff, "L305", `write the quantity as a string, for example {"$qty": "512Mi"}`)
			}
			return nil, false
		}
		kind, errCode := lexValidateNumber(str)
		if kind == tokQuantity && errCode == "" {
			num, unit := splitQuantity(str)
			return Quantity{Number: normalizeNumber(num), Unit: unit}, true
		}
		if errCode == "L005" {
			if !skipErrors {
				p.recordError(strOff, "L005", "")
			}
			return nil, false
		}
		if !skipErrors {
			p.recordError(strOff, "L305", `write the quantity as a string, for example {"$qty": "512Mi"}`)
		}
		return nil, false
	}
	valOff := p.cursor
	p.skipValue()
	if !skipErrors {
		p.recordError(valOff, "L305", `write the quantity as a string, for example {"$qty": "512Mi"}`)
	}
	return nil, false
}

func (p *jsonParser) parseRefPayload(skipErrors bool) (Value, bool) {
	if !p.skipWhitespace() {
		return nil, false
	}
	if p.cursor >= len(p.src) {
		p.syntaxError(len(p.src))
		return nil, false
	}
	b := p.src[p.cursor]
	if b == 'n' {
		lit, nullOff, ok := p.parseLiteral()
		if !ok {
			return nil, false
		}
		if lit == "null" && !skipErrors {
			p.recordError(nullOff, "L302", "")
		}
		return nil, false
	}
	if b == '"' {
		str, strOff, ok := p.parseString()
		if !ok {
			return nil, false
		}
		res := lexScanRef([]byte("&"+str), 0)
		if res.kind == tokRef && res.length == len(str)+1 && len(res.segs) > 0 {
			return Ref(res.segs), true
		}
		if !skipErrors {
			p.recordError(strOff, "L305", `write the reference as a string, for example {"$ref": "secret/db-creds"}`)
		}
		return nil, false
	}
	valOff := p.cursor
	p.skipValue()
	if !skipErrors {
		p.recordError(valOff, "L305", `write the reference as a string, for example {"$ref": "secret/db-creds"}`)
	}
	return nil, false
}
