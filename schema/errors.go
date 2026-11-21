package schema

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL"
)

const maxErrors = 20

var errorMessages = map[string]string{
	"L403": "invalid value",
	"L501": "type mismatch",
	"L502": "missing required field",
	"L503": "unknown field",
	"L504": "value out of range",
	"L505": "value does not match pattern",
	"L506": "value not allowed",
	"L507": "unit not allowed",
	"L508": "reference target not allowed",
	"L509": "duplicate value",
	"L510": "invalid map key",
	"L601": "invalid schema",
	"L602": "unresolved definition",
	"L603": "definition cycle",
	"L604": "invalid pattern",
	"L605": "default does not match its type",
	"L606": "conflicting constraints",
	"L607": "unsupported schema version",
}

func newError(file string, pos lkal.Pos, path, code, hint string) *lkal.Error {
	return &lkal.Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  errorMessages[code],
		Hint: hint,
		Path: path,
	}
}

func sortErrors(errs []*lkal.Error) []*lkal.Error {
	sort.SliceStable(errs, func(i, j int) bool {
		li, lj := errs[i].Pos.Line > 0, errs[j].Pos.Line > 0
		if li != lj {
			return li
		}
		if li {
			if errs[i].Pos.Line != errs[j].Pos.Line {
				return errs[i].Pos.Line < errs[j].Pos.Line
			}
			if errs[i].Pos.Col != errs[j].Pos.Col {
				return errs[i].Pos.Col < errs[j].Pos.Col
			}
			if errs[i].Code != errs[j].Code {
				return errs[i].Code < errs[j].Code
			}
			return errs[i].Path < errs[j].Path
		}
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		return errs[i].Code < errs[j].Code
	})
	if len(errs) > maxErrors {
		return errs[:maxErrors]
	}
	return errs
}

func toErrorList(errs []*lkal.Error) error {
	if len(errs) == 0 {
		return nil
	}
	return &lkal.ErrorList{Errors: sortErrors(errs)}
}

func formatPath(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	return string(lkal.FormatValue(lkal.Ref(segs))[1:])
}

func isNameStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isNameChar(b byte) bool {
	return isNameStart(b) || (b >= '0' && b <= '9') || b == '-'
}

func isBareKey(k string) bool {
	if k == "" || k == "true" || k == "false" || !isNameStart(k[0]) {
		return false
	}
	for i := 1; i < len(k); i++ {
		if !isNameChar(k[i]) {
			return false
		}
	}
	return true
}

func formatKey(k string) string {
	if isBareKey(k) {
		return k
	}
	return quoteString(k)
}

func isControlChar(r rune) bool {
	return (r < 0x20 && r != '\t') || r == 0x7f
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

func formatCanonicalValue(v lkal.Value) string {
	switch val := v.(type) {
	case lkal.String:
		return quoteString(string(val))
	case lkal.Record:
		if len(val.Fields) == 0 {
			return "{}"
		}
		var b strings.Builder
		b.WriteString("{ ")
		for i, f := range val.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(formatKey(f.Key))
			if _, isRec := f.Value.(lkal.Record); isRec {
				b.WriteByte(' ')
			} else {
				b.WriteString(" = ")
			}
			b.WriteString(formatCanonicalValue(f.Value))
		}
		b.WriteString(" }")
		return b.String()
	default:
		return string(lkal.FormatValue(v))
	}
}

func formatStringList(vals []string) string {
	parts := make([]string, len(vals))
	for i, s := range vals {
		parts[i] = quoteString(s)
	}
	return strings.Join(parts, ", ")
}

func formatIntList(vals []int64) string {
	parts := make([]string, len(vals))
	for i, n := range vals {
		parts[i] = strconv.FormatInt(n, 10)
	}
	return strings.Join(parts, ", ")
}

func formatEnumList(vals []string) string {
	parts := make([]string, len(vals))
	for i, e := range vals {
		parts[i] = "." + e
	}
	return strings.Join(parts, ", ")
}

func formatRefList(vals []lkal.Ref) string {
	parts := make([]string, len(vals))
	for i, r := range vals {
		parts[i] = string(lkal.FormatValue(r))
	}
	return strings.Join(parts, ", ")
}

func joinOr(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " or " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " or " + parts[len(parts)-1]
	}
}

func runeLevenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	na, nb := len(ra), len(rb)
	diff := na - nb
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		return 3
	}
	if na == 0 {
		return nb
	}
	if nb == 0 {
		return na
	}
	prev := make([]int, nb+1)
	curr := make([]int, nb+1)
	for j := 0; j <= nb; j++ {
		prev[j] = j
	}
	for i := 1; i <= na; i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= nb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
			if m < rowMin {
				rowMin = m
			}
		}
		if rowMin > 2 {
			return 3
		}
		prev, curr = curr, prev
	}
	return prev[nb]
}

func unknownFieldHint(key string, declared []string) string {
	if len(declared) == 0 {
		return "no fields are allowed"
	}
	keyLen := utf8.RuneCountInString(key)
	bestDist := 3
	best := ""
	for _, cand := range declared {
		diff := keyLen - utf8.RuneCountInString(cand)
		if diff < -2 || diff > 2 {
			continue
		}
		d := runeLevenshtein(key, cand)
		if d < bestDist || (d == bestDist && cand < best) {
			bestDist = d
			best = cand
		}
	}
	if bestDist <= 2 {
		return "did you mean " + quoteString(best) + "?"
	}
	allowed := make([]string, len(declared))
	for i, name := range declared {
		allowed[i] = formatKey(name)
	}
	return "allowed fields: " + strings.Join(allowed, ", ")
}
