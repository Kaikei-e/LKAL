package model

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// ValidateValue validates v at the given block nesting depth.
// It returns an empty string if v is valid, or an L403 error hint otherwise.
func ValidateValue(v Value, depth int) string {
	if v == nil {
		return "use a non-nil value"
	}
	switch val := v.(type) {
	case String:
		if !utf8.ValidString(string(val)) {
			return "use valid UTF-8 text"
		}
	case Decimal:
		digits, hasFrac, ok := syntax.ScanPlainNumber(string(val))
		if !ok || !hasFrac {
			return "write a decimal such as 0.25"
		}
		if digits > 100 {
			return "keep numbers within 100 digits"
		}
	case Quantity:
		digits, _, ok := syntax.ScanPlainNumber(val.Number)
		if !ok || len(val.Unit) == 0 {
			return "write a quantity such as 512Mi"
		}
		for _, r := range val.Unit {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return "write a quantity such as 512Mi"
			}
		}
		if digits > 100 {
			return "keep numbers within 100 digits"
		}
	case Enum:
		s := string(val)
		if !utf8.ValidString(s) {
			return "use valid UTF-8 text"
		}
		if !syntax.IsValidEnumName(s) {
			return "write an enum name that starts with a letter or _"
		}
	case Ref:
		if len(val) == 0 {
			return "write a reference with at least one segment"
		}
		for _, seg := range val {
			if !utf8.ValidString(seg) {
				return "use valid UTF-8 text"
			}
		}
	case List:
		return validateList(val, depth)
	case Record:
		return validateRecord(val, depth)
	case Collection:
		return validateCollection(val, depth)
	}
	return ""
}

func validateList(val List, depth int) string {
	for _, el := range val {
		switch el.(type) {
		case List, Record, Collection:
			return "use only scalars as list elements"
		}
		if hint := ValidateValue(el, depth); hint != "" {
			return hint
		}
	}
	if len(val) > 1 {
		t0 := ScalarType(val[0])
		for _, el := range val[1:] {
			if ScalarType(el) != t0 {
				return "use one type for all list elements"
			}
		}
	}
	return ""
}

func validateRecord(val Record, depth int) string {
	if depth > diag.MaxBlockDepth {
		return diag.DefaultHint("L107")
	}
	seenKeys := make(map[string]bool, len(val.Fields))
	for _, f := range val.Fields {
		if !utf8.ValidString(f.Key) {
			return "use valid UTF-8 text"
		}
		if strings.HasPrefix(f.Key, "$") {
			return "rename keys that start with $"
		}
		if seenKeys[f.Key] {
			return "remove the duplicate key"
		}
		seenKeys[f.Key] = true
	}
	for _, f := range val.Fields {
		if hint := ValidateValue(f.Value, depth+1); hint != "" {
			return hint
		}
	}
	return ""
}

func validateCollection(val Collection, depth int) string {
	if len(val.Elems) == 0 {
		return "write at least one collection element"
	}
	seenLabels := make(map[string]bool, len(val.Elems))
	for _, el := range val.Elems {
		if !utf8.ValidString(el.Label) {
			return "use valid UTF-8 text"
		}
		if seenLabels[el.Label] {
			return "remove the duplicate label"
		}
		seenLabels[el.Label] = true
	}
	for _, el := range val.Elems {
		if hint := ValidateValue(el.Body, depth); hint != "" {
			return hint
		}
	}
	return ""
}

// NormalizeValue returns a deep-copied Value with normalized Decimal and Quantity numbers.
func NormalizeValue(v Value) Value {
	switch val := v.(type) {
	case Decimal:
		return Decimal(syntax.NormalizeNumber(string(val)))
	case Quantity:
		return Quantity{Number: syntax.NormalizeNumber(val.Number), Unit: val.Unit}
	case Ref:
		return slices.Clone(val)
	case List:
		res := make(List, len(val))
		for i, el := range val {
			res[i] = NormalizeValue(el)
		}
		return res
	case Record:
		fields := make([]Field, len(val.Fields))
		for i, f := range val.Fields {
			fields[i] = Field{Key: f.Key, Value: NormalizeValue(f.Value)}
		}
		return Record{Fields: fields}
	case Collection:
		elems := make([]Elem, len(val.Elems))
		for i, el := range val.Elems {
			elems[i] = Elem{Label: el.Label, Body: NormalizeValue(el.Body).(Record)}
		}
		return Collection{Elems: elems}
	default:
		return val
	}
}
