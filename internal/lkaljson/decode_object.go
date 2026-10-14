package lkaljson

import (
	"strings"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
)

func (r *jsonReader) top() model.Record {
	tok, off, ok := r.next()
	if !ok {
		return model.Record{}
	}
	if tok.Kind() != '{' {
		r.fail(off, "L105", hintVersion)
		r.skip(tok)
		return model.Record{}
	}
	if tok, off, ok = r.next(); !ok {
		return model.Record{}
	}
	if tok.Kind() == '}' {
		r.fail(off, "L105", hintVersion)
		return model.Record{}
	}
	if key := tok.String(); key != "$lkal" {
		r.fail(off, "L105", hintVersion)
		rec, _ := r.members(key, off, 0, true)
		return rec
	}
	if tok, off, ok = r.next(); !ok {
		return model.Record{}
	}
	if tok.Kind() != '"' {
		r.fail(off, "L105", hintVersion)
		r.skip(tok)
	} else if tok.String() != "0.1" {
		r.fail(off, "L207", "")
	}
	if tok, off, ok = r.next(); !ok || tok.Kind() == '}' {
		return model.Record{}
	}
	rec, _ := r.members(tok.String(), off, 0, true)
	return rec
}

func (r *jsonReader) members(key string, off, depth int, top bool) (model.Record, bool) {
	var rec model.Record
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
			rec.Fields = append(rec.Fields, model.Field{Key: key, Value: v})
		} else {
			ok = false
		}
		tok, tokOff, rok := r.next()
		if !rok {
			return model.Record{}, false
		}
		if tok.Kind() == '}' {
			return rec, ok
		}
		key, off = tok.String(), tokOff
	}
}

func (r *jsonReader) value(depth int) (model.Value, bool) {
	tok, off, ok := r.next()
	if !ok {
		return nil, false
	}
	if tok.Kind() == '{' {
		return r.object(off, depth, false)
	}
	return r.convert(tok, off, false)
}

func (r *jsonReader) object(off, depth int, elem bool) (model.Value, bool) {
	tok, keyOff, ok := r.next()
	if !ok {
		return nil, false
	}
	if tok.Kind() == '}' {
		if depth > diag.MaxBlockDepth {
			r.fail(off, "L107", "")
			return nil, false
		}
		return model.Record{}, true
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
	if depth > diag.MaxBlockDepth {
		r.fail(off, "L107", "")
		r.skipTo(r.dec.StackDepth() - 1)
		return nil, false
	}
	return r.members(key, keyOff, depth, false)
}

func (r *jsonReader) elemRecord(off, depth int) (model.Record, bool) {
	v, ok := r.object(off, depth, true)
	if rec, isRec := v.(model.Record); isRec {
		return rec, ok
	}
	return model.Record{}, false
}
