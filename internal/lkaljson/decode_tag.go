package lkaljson

import (
	"encoding/json/jsontext"
	"strconv"
	"strings"

	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

func (r *jsonReader) convert(tok jsontext.Token, off int, inList bool) (model.Value, bool) {
	switch tok.Kind() {
	case 'n':
		r.fail(off, "L302", "")
	case 't', 'f':
		return model.Bool(tok.Bool()), true
	case '"':
		return model.String(tok.String()), true
	case '0':
		raw := tok.String()
		if strings.ContainsAny(raw, "eE") {
			r.fail(off, "L303", "")
			return nil, false
		}
		kind, code := syntax.ValidateNumber(raw)
		if code != "" {
			r.fail(off, code, "")
			return nil, false
		}
		if kind == syntax.NumberDecimal {
			return model.Decimal(syntax.NormalizeNumber(raw)), true
		}
		n, _ := strconv.ParseInt(raw, 10, 64)
		return model.Int(n), true
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

func (r *jsonReader) list() (model.Value, bool) {
	var list model.List
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
		if model.ScalarType(v) != model.ScalarType(list[0]) {
			r.fail(offs[i], "L204", "")
			return nil, false
		}
	}
	return list, true
}

func (r *jsonReader) tag(name string, depth int) (model.Value, bool) {
	stackDepth := r.dec.StackDepth()
	tok, off, ok := r.next()
	if !ok {
		return nil, false
	}
	prev := r.tagL305
	r.tagL305 = false
	defer func() { r.tagL305 = prev }()

	var (
		v     model.Value
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

func (r *jsonReader) scalarTag(name, s string, off int) (model.Value, bool) {
	switch name {
	case "$enum":
		if syntax.IsValidEnumName(s) {
			return model.Enum(s), true
		}
	case "$qty":
		if s != "" && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) {
			switch kind, code := syntax.ValidateNumber(s); {
			case kind == syntax.NumberQuantity:
				num, unit := syntax.SplitQuantity(s)
				return model.Quantity{Number: syntax.NormalizeNumber(num), Unit: unit}, true
			case code == "L005":
				r.fail(off, "L005", "")
				return nil, false
			}
		}
	case "$ref":
		if segs, ok := syntax.ParseRefBody(s); ok {
			return model.Ref(segs), true
		}
	}
	r.failTag(off, tagHints[name])
	return nil, false
}

func (r *jsonReader) coll(tok jsontext.Token, off, depth, stackDepth int) (model.Value, bool) {
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
	var c model.Collection
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
					c.Elems = append(c.Elems, model.Elem{Label: label, Body: body})
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
