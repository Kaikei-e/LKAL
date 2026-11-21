package schema

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL"
)

// Default returns a deep-cloned copy of the declarative default value declared
// at path, or (nil, false) if path is malformed or does not resolve to a record
// field with a default.
func (s *Schema) Default(path string) (lkal.Value, bool) {
	if s == nil || s.root == nil {
		return nil, false
	}
	segs, ok := parsePath(path)
	if !ok || len(segs) == 0 {
		return nil, false
	}

	cur := s.root
	for i, seg := range segs {
		cur = cur.deref()
		if cur == nil || cur.isUnion {
			return nil, false
		}
		isLast := i == len(segs)-1
		switch cur.kind {
		case kindRecord:
			fs := cur.fields[seg]
			if fs == nil {
				return nil, false
			}
			if isLast {
				if !fs.hasDefault {
					return nil, false
				}
				return normalizeAndClone(fs.defVal), true
			}
			cur = fs.spec
		case kindMap:
			if isLast {
				return nil, false
			}
			if cur.mapKey != nil && !cur.mapKey.matches(seg) {
				return nil, false
			}
			cur = cur.mapValue
		default:
			return nil, false
		}
	}
	return nil, false
}

// Complete returns a deep-cloned, normalized copy of v with missing optional
// fields that declare defaults populated for every existing record matching a
// record spec. It never mutates v or schema defaults, never implicitly creates
// missing parent records, and is idempotent.
func (s *Schema) Complete(v lkal.Value) (lkal.Value, error) {
	if s == nil || s.root == nil {
		return nil, toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L601", "use a non-nil schema"),
		})
	}
	if hint := validateStructuralValue(v, 0); hint != "" {
		return nil, toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L403", hint),
		})
	}
	if _, isRec := v.(lkal.Record); !isRec {
		return nil, toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L403", "expected a record"),
		})
	}
	out, hint := completeNode(s.root, v, 0)
	if hint != "" {
		return nil, toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L403", hint),
		})
	}
	return out, nil
}

func completeNode(spec *typeSpec, v lkal.Value, depth int) (lkal.Value, string) {
	if spec != nil {
		spec = spec.deref()
	}
	rec, isRec := v.(lkal.Record)
	if !isRec {
		return normalizeAndClone(v), ""
	}
	if depth > maxBlockDepth {
		return nil, "nest blocks at most 1000 levels deep"
	}
	if spec == nil || spec.isUnion {
		return normalizeAndClone(rec), ""
	}

	switch spec.kind {
	case kindRecord:
		present := make(map[string]bool, len(rec.Fields))
		var outFields []lkal.Field
		if len(rec.Fields) > 0 {
			outFields = make([]lkal.Field, 0, len(rec.Fields))
		}
		for _, f := range rec.Fields {
			present[f.Key] = true
			var childSpec *typeSpec
			if fs, ok := spec.fields[f.Key]; ok {
				childSpec = fs.spec
			}
			cv, hint := completeNode(childSpec, f.Value, depth+1)
			if hint != "" {
				return nil, hint
			}
			outFields = append(outFields, lkal.Field{Key: f.Key, Value: cv})
		}
		for _, name := range spec.fieldOrder {
			fs := spec.fields[name]
			if !present[name] && fs.hasDefault {
				cv, hint := completeNode(fs.spec, fs.defVal, depth+1)
				if hint != "" {
					return nil, hint
				}
				outFields = append(outFields, lkal.Field{Key: name, Value: cv})
			}
		}
		return lkal.Record{Fields: outFields}, ""

	case kindMap:
		var outFields []lkal.Field
		if len(rec.Fields) > 0 {
			outFields = make([]lkal.Field, 0, len(rec.Fields))
		}
		for _, entry := range rec.Fields {
			cv, hint := completeNode(spec.mapValue, entry.Value, depth+1)
			if hint != "" {
				return nil, hint
			}
			outFields = append(outFields, lkal.Field{Key: entry.Key, Value: cv})
		}
		return lkal.Record{Fields: outFields}, ""

	default:
		return normalizeAndClone(rec), ""
	}
}

func normalizeAndClone(v lkal.Value) lkal.Value {
	switch val := v.(type) {
	case lkal.Decimal:
		return lkal.Decimal(normalizeNumber(string(val)))
	case lkal.Quantity:
		return lkal.Quantity{Number: normalizeNumber(val.Number), Unit: val.Unit}
	case lkal.Ref:
		return slices.Clone(val)
	case lkal.List:
		res := make(lkal.List, len(val))
		for i, el := range val {
			res[i] = normalizeAndClone(el)
		}
		return res
	case lkal.Record:
		if len(val.Fields) == 0 {
			return lkal.Record{}
		}
		fields := make([]lkal.Field, len(val.Fields))
		for i, f := range val.Fields {
			fields[i] = lkal.Field{Key: f.Key, Value: normalizeAndClone(f.Value)}
		}
		return lkal.Record{Fields: fields}
	default:
		return val
	}
}

func normalizeNumber(s string) string {
	if s == "-0" {
		return "0"
	}
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if !hasDot {
		return s
	}
	isNeg := false
	if strings.HasPrefix(intPart, "-") {
		isNeg = true
		intPart = intPart[1:]
	}
	fracPart = strings.TrimRight(fracPart, "0")
	if fracPart == "" {
		fracPart = "0"
	}
	if intPart == "0" && fracPart == "0" {
		return "0.0"
	}
	if isNeg {
		return "-" + intPart + "." + fracPart
	}
	return intPart + "." + fracPart
}

func isRefSegmentChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') ||
		b == '_' || b == '.' || b == '-'
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// parsePath parses a strict LKAL `/` path (matching root parseRefBody syntax).
func parsePath(s string) ([]string, bool) {
	if s == "" {
		return nil, false
	}
	var segs []string
	i := 0
	for {
		if i >= len(s) {
			return nil, false
		}
		if s[i] == '"' {
			seg, next, ok := scanQuotedSegment(s, i)
			if !ok {
				return nil, false
			}
			segs = append(segs, seg)
			i = next
		} else if isRefSegmentChar(s[i]) {
			start := i
			for i < len(s) && isRefSegmentChar(s[i]) {
				i++
			}
			segs = append(segs, s[start:i])
		} else {
			return nil, false
		}

		if i == len(s) {
			return segs, true
		}
		if s[i] == '/' {
			i++
			continue
		}
		return nil, false
	}
}

func scanQuotedSegment(s string, off int) (string, int, bool) {
	var b strings.Builder
	i := off + 1
	for i < len(s) {
		if s[i] == '"' {
			return b.String(), i + 1, true
		}
		if s[i] == '\\' {
			i++
			if i >= len(s) {
				return "", 0, false
			}
			switch s[i] {
			case '"':
				b.WriteByte('"')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case 'u':
				i++
				if i >= len(s) || s[i] != '{' {
					return "", 0, false
				}
				i++
				hexStart := i
				for i < len(s) && isHexDigit(s[i]) {
					i++
				}
				if i >= len(s) || s[i] != '}' {
					return "", 0, false
				}
				hexStr := s[hexStart:i]
				i++
				val, err := strconv.ParseUint(hexStr, 16, 32)
				if err != nil || len(hexStr) == 0 || len(hexStr) > 6 || val > 0x10FFFF || (val >= 0xD800 && val <= 0xDFFF) {
					return "", 0, false
				}
				b.WriteRune(rune(val))
			default:
				return "", 0, false
			}
			continue
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && sz == 1) || isControlChar(r) {
			return "", 0, false
		}
		b.WriteRune(r)
		i += sz
	}
	return "", 0, false
}
