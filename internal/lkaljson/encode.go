package lkaljson

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// ToJSON encodes f into LKAL-JSON (plain == false) or plain JSON (plain == true).
func ToJSON(f *ast.File, plain bool) ([]byte, error) {
	if f == nil {
		return nil, errors.New("lkal: nil File")
	}
	top := ast.BuildRecord(f.Body)
	if !plain {
		top.Fields = append([]model.Field{{Key: "$lkal", Value: model.String(f.Version)}}, top.Fields...)
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

func (w *jsonWriter) value(v model.Value) {
	switch v := v.(type) {
	case model.String:
		w.token(jsontext.String(string(v)))
	case model.Int:
		w.token(jsontext.Int(int64(v)))
	case model.Decimal:
		if w.err == nil {
			w.err = w.enc.WriteValue(jsontext.Value(v))
		}
	case model.Bool:
		w.token(jsontext.Bool(bool(v)))
	case model.Enum:
		w.tag("$enum", string(v))
	case model.Quantity:
		w.tag("$qty", v.Number+v.Unit)
	case model.Ref:
		w.tag("$ref", syntax.FormatRefBody(v))
	case model.List:
		w.token(jsontext.BeginArray)
		for _, el := range v {
			w.value(el)
		}
		w.token(jsontext.EndArray)
	case model.Record:
		w.token(jsontext.BeginObject)
		for _, f := range v.Fields {
			w.token(jsontext.String(f.Key))
			w.value(f.Value)
		}
		w.token(jsontext.EndObject)
	case model.Collection:
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
