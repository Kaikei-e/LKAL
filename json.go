package lkal

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func ToJSON(f *File, plain bool) ([]byte, error) {
	if f == nil {
		return nil, errors.New("lkal: nil File")
	}
	top := f.Data()
	if !plain {
		top.Fields = append([]Field{{Key: "$lkal", Value: String(f.Version())}}, top.Fields...)
	}
	var buf bytes.Buffer
	w := &jsonWriter{enc: jsontext.NewEncoder(&buf, jsontext.WithIndent("  ")), plain: plain}
	w.value(top)
	if w.err != nil {
		return nil, w.err
	}
	return buf.Bytes(), nil
}

type jsonWriter struct {
	enc   *jsontext.Encoder
	plain bool
	err   error
}

func (w *jsonWriter) token(t jsontext.Token) {
	if w.err == nil {
		w.err = w.enc.WriteToken(t)
	}
}

func (w *jsonWriter) value(v Value) {
	switch v := v.(type) {
	case String:
		w.token(jsontext.String(string(v)))
	case Int:
		w.token(jsontext.Int(int64(v)))
	case Decimal:
		if w.err == nil {
			w.err = w.enc.WriteValue(jsontext.Value(v))
		}
	case Bool:
		w.token(jsontext.Bool(bool(v)))
	case Enum:
		w.tag("$enum", string(v))
	case Quantity:
		w.tag("$qty", v.Number+v.Unit)
	case Ref:
		w.tag("$ref", refBody(v))
	case List:
		w.token(jsontext.BeginArray)
		for _, el := range v {
			w.value(el)
		}
		w.token(jsontext.EndArray)
	case Record:
		w.token(jsontext.BeginObject)
		for _, f := range v.Fields {
			w.token(jsontext.String(f.Key))
			w.value(f.Value)
		}
		w.token(jsontext.EndObject)
	case Collection:
		if !w.plain {
			w.token(jsontext.BeginObject)
			w.token(jsontext.String("$coll"))
		}
		w.token(jsontext.BeginObject)
		for _, el := range v.Elems {
			w.token(jsontext.String(el.Label))
			w.value(el.Body)
		}
		w.token(jsontext.EndObject)
		if !w.plain {
			w.token(jsontext.EndObject)
		}
	}
}

func (w *jsonWriter) tag(name, payload string) {
	if w.plain {
		w.token(jsontext.String(payload))
		return
	}
	w.token(jsontext.BeginObject)
	w.token(jsontext.String(name))
	w.token(jsontext.String(payload))
	w.token(jsontext.EndObject)
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
	b := &block{}
	for _, f := range r.Fields {
		switch val := f.Value.(type) {
		case Record:
			b.entries = append(b.entries, &entry{kind: entryRecord, key: f.Key, body: blockFromRecord(val)})
		case Collection:
			for _, el := range val.Elems {
				b.entries = append(b.entries, &entry{kind: entryElem, key: f.Key, label: el.Label, body: blockFromRecord(el.Body)})
			}
		default:
			b.entries = append(b.entries, &entry{kind: entryAssign, key: f.Key, value: nodeFromValue(val)})
		}
	}
	return b
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
	if len(s) == 0 || !lexIsNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !lexIsNameChar(s[i]) {
			return false
		}
	}
	return true
}

func parseRefBody(s string) (Ref, bool) {
	res := lexScanRef([]byte("&"+s), 0)
	if res.kind == tokRef && res.length == len(s)+1 {
		return Ref(res.segs), true
	}
	return nil, false
}

const (
	hintVersion = `start the top-level object with "$lkal": "0.1"`
	hintOneKey  = "write a tag object with only one key"
)

var tagHints = map[string]string{
	"$enum": `write the enum name as a string, for example {"$enum": "tcp"}`,
	"$qty":  `write the quantity as a string, for example {"$qty": "512Mi"}`,
	"$ref":  `write the reference as a string, for example {"$ref": "secret/db-creds"}`,
	"$coll": `write {"$coll": {"label": {...}}} with at least one element`,
}

type jsonError struct {
	off        int
	code, hint string
}

type jsonReader struct {
	src     []byte
	dec     *jsontext.Decoder
	errs    []jsonError
	broken  bool
	tagL305 bool
}

func FromJSON(src []byte) (*File, error) {
	r := &jsonReader{src: src, dec: jsontext.NewDecoder(bytes.NewReader(src), jsontext.AllowDuplicateNames(true))}
	data := r.top()
	if !r.broken {
		rest := src[r.dec.InputOffset():]
		if n := len(rest) - len(bytes.TrimLeft(rest, " \t\r\n")); n < len(rest) {
			r.fail(int(r.dec.InputOffset())+n, "L301", "")
		}
	}
	if len(r.errs) > 0 {
		return nil, r.errorList()
	}
	return &File{version: "0.1", body: blockFromRecord(data)}, nil
}

func (r *jsonReader) fail(off int, code, hint string) {
	r.errs = append(r.errs, jsonError{off, code, hint})
}

func (r *jsonReader) failTag(off int, hint string) {
	if !r.tagL305 {
		r.fail(off, "L305", hint)
		r.tagL305 = true
	}
}

func (r *jsonReader) next() (jsontext.Token, int, bool) {
	if r.broken {
		return jsontext.Token{}, 0, false
	}
	off := int(r.dec.InputOffset())
	for off < len(r.src) && strings.IndexByte(" \t\r\n,:", r.src[off]) >= 0 {
		off++
	}
	tok, err := r.dec.ReadToken()
	if err != nil {
		r.broken = true
		off, hint := len(r.src), ""
		var se *jsontext.SyntacticError
		if errors.As(err, &se) {
			off = int(se.ByteOffset)
			if se.Err != nil && se.Err.Error() == "exceeded max depth" {
				hint = "nest objects and arrays at most 10000 levels deep"
			}
		}
		r.fail(off, "L301", hint)
		return tok, off, false
	}
	return tok, off, true
}

func (r *jsonReader) skip(tok jsontext.Token) {
	if k := tok.Kind(); k == '[' || k == '{' {
		r.skipTo(r.dec.StackDepth() - 1)
	}
}

func (r *jsonReader) skipTo(depth int) {
	for r.dec.StackDepth() > depth {
		if _, _, ok := r.next(); !ok {
			return
		}
	}
}

func (r *jsonReader) skipValue() {
	if tok, _, ok := r.next(); ok {
		r.skip(tok)
	}
}

func (r *jsonReader) top() Record {
	tok, off, ok := r.next()
	if !ok {
		return Record{}
	}
	if tok.Kind() != '{' {
		r.fail(off, "L105", hintVersion)
		r.skip(tok)
		return Record{}
	}
	if tok, off, ok = r.next(); !ok {
		return Record{}
	}
	if tok.Kind() == '}' {
		r.fail(off, "L105", hintVersion)
		return Record{}
	}
	if key := tok.String(); key != "$lkal" {
		r.fail(off, "L105", hintVersion)
		rec, _ := r.members(key, off, 0, true)
		return rec
	}
	if tok, off, ok = r.next(); !ok {
		return Record{}
	}
	if tok.Kind() != '"' {
		r.fail(off, "L105", hintVersion)
		r.skip(tok)
	} else if tok.String() != "0.1" {
		r.fail(off, "L207", "")
	}
	if tok, off, ok = r.next(); !ok || tok.Kind() == '}' {
		return Record{}
	}
	rec, _ := r.members(tok.String(), off, 0, true)
	return rec
}

func (r *jsonReader) members(key string, off, depth int, top bool) (Record, bool) {
	var rec Record
	seen := map[string]bool{}
	ok := true
	for {
		var code, hint string
		switch {
		case top && key == "$lkal":
			code, hint = "L105", hintVersion
		case strings.HasPrefix(key, "$"):
			code = "L205"
		case seen[key]:
			code = "L201"
		}
		seen[key] = true
		if code != "" {
			r.fail(off, code, hint)
			r.skipValue()
			ok = false
		} else if v, vok := r.value(depth + 1); vok {
			rec.Fields = append(rec.Fields, Field{Key: key, Value: v})
		} else {
			ok = false
		}
		tok, tokOff, rok := r.next()
		if !rok {
			return Record{}, false
		}
		if tok.Kind() == '}' {
			return rec, ok
		}
		key, off = tok.String(), tokOff
	}
}

func (r *jsonReader) value(depth int) (Value, bool) {
	tok, off, ok := r.next()
	if !ok {
		return nil, false
	}
	if tok.Kind() == '{' {
		return r.object(off, depth, false)
	}
	return r.convert(tok, off, false)
}

func (r *jsonReader) convert(tok jsontext.Token, off int, inList bool) (Value, bool) {
	switch tok.Kind() {
	case 'n':
		r.fail(off, "L302", "")
	case 't', 'f':
		return Bool(tok.Bool()), true
	case '"':
		return String(tok.String()), true
	case '0':
		raw := tok.String()
		if strings.ContainsAny(raw, "eE") {
			r.fail(off, "L303", "")
			return nil, false
		}
		kind, code := lexValidateNumber(raw)
		if code != "" {
			r.fail(off, code, "")
			return nil, false
		}
		if kind == tokDecimal {
			return Decimal(normalizeNumber(raw)), true
		}
		n, _ := strconv.ParseInt(raw, 10, 64)
		return Int(n), true
	case '[':
		if !inList {
			return r.list()
		}
		r.fail(off, "L304", "")
		r.skip(tok)
	case '{':
		tok, _, ok := r.next()
		if !ok {
			return nil, false
		}
		if tok.Kind() == '}' {
			r.fail(off, "L304", "")
			return nil, false
		}
		key := tok.String()
		if tagHints[key] == "" || key == "$coll" {
			r.fail(off, "L304", "")
			r.skipTo(r.dec.StackDepth() - 1)
			return nil, false
		}
		return r.tag(key, 0)
	}
	return nil, false
}

func (r *jsonReader) object(off, depth int, elem bool) (Value, bool) {
	tok, keyOff, ok := r.next()
	if !ok {
		return nil, false
	}
	if tok.Kind() == '}' {
		if depth > maxBlockDepth {
			r.fail(off, "L107", "")
			return nil, false
		}
		return Record{}, true
	}
	key := tok.String()
	if tagHints[key] != "" {
		if elem {
			r.failTag(off, tagHints["$coll"])
			r.skipTo(r.dec.StackDepth() - 1)
			return nil, false
		}
		return r.tag(key, depth-1)
	}
	if depth > maxBlockDepth {
		r.fail(off, "L107", "")
		r.skipTo(r.dec.StackDepth() - 1)
		return nil, false
	}
	return r.members(key, keyOff, depth, false)
}

func (r *jsonReader) list() (Value, bool) {
	var list List
	var offs []int
	ok := true
	for {
		tok, off, rok := r.next()
		if !rok {
			return nil, false
		}
		if tok.Kind() == ']' {
			break
		}
		if v, vok := r.convert(tok, off, true); vok {
			list = append(list, v)
			offs = append(offs, off)
		} else {
			ok = false
		}
	}
	if !ok {
		return nil, false
	}
	for i, v := range list {
		if scalarType(v) != scalarType(list[0]) {
			r.fail(offs[i], "L204", "")
			return nil, false
		}
	}
	return list, true
}

func (r *jsonReader) tag(name string, depth int) (Value, bool) {
	stackDepth := r.dec.StackDepth()
	tok, off, ok := r.next()
	if !ok {
		return nil, false
	}
	prev := r.tagL305
	r.tagL305 = false
	defer func() { r.tagL305 = prev }()

	var (
		v     Value
		valid bool
	)
	switch {
	case tok.Kind() == 'n':
		r.fail(off, "L302", "")
		r.skipTo(stackDepth - 1)
		return nil, false
	case name == "$coll":
		v, valid = r.coll(tok, off, depth, stackDepth)
		if r.dec.StackDepth() < stackDepth {
			return nil, false
		}
	case tok.Kind() != '"':
		r.failTag(off, tagHints[name])
		r.skipTo(stackDepth - 1)
		return nil, false
	default:
		v, valid = r.scalarTag(name, tok.String(), off)
		if !valid {
			r.skipTo(stackDepth - 1)
			return nil, false
		}
	}
	if tok, off, ok = r.next(); !ok {
		return nil, false
	}
	if tok.Kind() != '}' {
		r.failTag(off, hintOneKey)
		r.skipTo(stackDepth - 1)
		return nil, false
	}
	if !valid {
		return nil, false
	}
	return v, true
}

func (r *jsonReader) scalarTag(name, s string, off int) (Value, bool) {
	switch name {
	case "$enum":
		if isValidEnumName(s) {
			return Enum(s), true
		}
	case "$qty":
		if s != "" && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) {
			switch kind, code := lexValidateNumber(s); {
			case kind == tokQuantity:
				num, unit := splitQuantity(s)
				return Quantity{Number: normalizeNumber(num), Unit: unit}, true
			case code == "L005":
				r.fail(off, "L005", "")
				return nil, false
			}
		}
	case "$ref":
		if ref, ok := parseRefBody(s); ok {
			return ref, true
		}
	}
	r.failTag(off, tagHints[name])
	return nil, false
}

func (r *jsonReader) coll(tok jsontext.Token, off, depth, stackDepth int) (Value, bool) {
	if tok.Kind() != '{' {
		r.failTag(off, tagHints["$coll"])
		r.skipTo(stackDepth - 1)
		return nil, false
	}
	tok, labelOff, rok := r.next()
	if !rok {
		return nil, false
	}
	if tok.Kind() == '}' {
		r.failTag(off, tagHints["$coll"])
		r.skipTo(stackDepth - 1)
		return nil, false
	}
	var c Collection
	seen := map[string]bool{}
	ok := true
	for {
		label := tok.String()
		if seen[label] {
			r.fail(labelOff, "L201", "")
			ok = false
			r.skipValue()
		} else {
			seen[label] = true
			valTok, valOff, vrok := r.next()
			if !vrok {
				return nil, false
			}
			switch valTok.Kind() {
			case 'n':
				r.fail(valOff, "L302", "")
				ok = false
			case '{':
				body, bok := r.elemRecord(valOff, depth+1)
				if bok {
					c.Elems = append(c.Elems, Elem{Label: label, Body: body})
				} else {
					ok = false
				}
			default:
				r.failTag(valOff, tagHints["$coll"])
				r.skip(valTok)
				ok = false
			}
		}
		tok, labelOff, rok = r.next()
		if !rok {
			return nil, false
		}
		if tok.Kind() == '}' {
			break
		}
	}
	if !ok || len(c.Elems) == 0 {
		return nil, false
	}
	return c, true
}

func (r *jsonReader) elemRecord(off, depth int) (Record, bool) {
	v, ok := r.object(off, depth, true)
	if rec, isRec := v.(Record); isRec {
		return rec, ok
	}
	return Record{}, false
}

func (r *jsonReader) errorList() *ErrorList {
	sort.SliceStable(r.errs, func(i, j int) bool { return r.errs[i].off < r.errs[j].off })
	errs := make([]*Error, len(r.errs))
	pos, off := Pos{Line: 1, Col: 1}, 0
	for i, e := range r.errs {
		for off < e.off && off < len(r.src) {
			if r.src[off] == '\n' {
				pos.Line, pos.Col = pos.Line+1, 0
			}
			_, n := utf8.DecodeRune(r.src[off:])
			off += n
			pos.Col++
		}
		errs[i] = newError("", pos, e.code, e.hint)
	}
	return &ErrorList{Errors: sortErrors(errs)}
}
