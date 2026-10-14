package lkaljson

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
)

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

// FromJSON parses LKAL-JSON bytes into an *ast.File or returns *diag.ErrorList.
func FromJSON(src []byte) (*ast.File, error) {
	r := &jsonReader{
		src: src,
		dec: jsontext.NewDecoder(bytes.NewReader(src), jsontext.AllowDuplicateNames(true)),
	}
	data := r.top()
	if !r.broken {
		rest := src[r.dec.InputOffset():]
		if n := len(rest) - len(bytes.TrimLeft(rest, " \t\r\n")); n < len(rest) {
			r.fail(int(r.dec.InputOffset())+n, "L301", "")
		}
	}
	if len(r.errs) > 0 {
		return nil, buildErrorList(src, r.errs)
	}
	return &ast.File{Version: "0.1", Body: ast.BlockFromRecord(data)}, nil
}

func (r *jsonReader) fail(off int, code, hint string) {
	r.errs = append(r.errs, jsonError{off: off, code: code, hint: hint})
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

// buildErrorList is a pure function that converts byte-offset errors into a sorted *diag.ErrorList.
func buildErrorList(src []byte, rawErrs []jsonError) *diag.ErrorList {
	sorted := slices.Clone(rawErrs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].off < sorted[j].off })
	errs := make([]*diag.Error, len(sorted))
	pos, off := diag.Pos{Line: 1, Col: 1}, 0
	for i, e := range sorted {
		for off < e.off && off < len(src) {
			if src[off] == '\n' {
				pos.Line, pos.Col = pos.Line+1, 0
			}
			_, n := utf8.DecodeRune(src[off:])
			off += n
			pos.Col++
		}
		errs[i] = diag.NewError("", pos, e.code, e.hint)
	}
	return &diag.ErrorList{Errors: diag.SortErrors(errs)}
}
