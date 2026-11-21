// Package syntax holds LKAL text helpers shared by the root package and schema.
// schema imports this package; the root package does too. Neither imports the other
// through these helpers, so the schema dependency stays one-way.
package syntax

import (
	"strconv"
	"strings"
)

// NormalizeNumber returns the canonical spelling of a base-10 integer or decimal.
// "-0" becomes "0". A decimal keeps one fractional digit when the fraction is zero,
// so "1.0" and "1.00" both become "1.0".
func NormalizeNumber(s string) string {
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

// IsControlChar reports a byte-or-rune that LKAL rejects in strings, comments, and raw lines.
// Tab is allowed.
func IsControlChar(r rune) bool {
	return (r < 0x20 && r != '\t') || r == 0x7f
}

// IsNameStart reports an ASCII letter or underscore.
func IsNameStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

// IsNameChar reports a character that may continue a bare name.
func IsNameChar(b byte) bool {
	return IsNameStart(b) || (b >= '0' && b <= '9') || b == '-'
}

// IsBareKey reports whether k can be written as an unquoted key.
// "true" and "false" are quoted, matching the formatter.
func IsBareKey(k string) bool {
	if k == "" || k == "true" || k == "false" || !IsNameStart(k[0]) {
		return false
	}
	for i := 1; i < len(k); i++ {
		if !IsNameChar(k[i]) {
			return false
		}
	}
	return true
}

// QuoteString returns s as an LKAL quoted string.
func QuoteString(s string) string {
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
			if IsControlChar(r) {
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

// IsRadixPrefix reports the first byte of a radix prefix LKAL rejects after 0.
func IsRadixPrefix(b byte) bool {
	return b == 'x' || b == 'X' || b == 'b' || b == 'B' || b == 'o' || b == 'O'
}

// ScanNumber checks a base-10 integer or decimal, ignoring one leading '-'.
// It returns the count of significant digits, whether a fraction is present, and ok.
func ScanNumber(s string) (int, bool, bool) {
	s = strings.TrimPrefix(s, "-")
	if len(s) == 0 {
		return 0, false, false
	}
	intDigits := 0
	if s[0] == '0' {
		intDigits = 1
		s = s[1:]
		if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			return 0, false, false
		}
	} else if s[0] >= '1' && s[0] <= '9' {
		for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			intDigits++
			s = s[1:]
		}
	} else {
		return 0, false, false
	}
	hasFrac := false
	fracDigits := 0
	if strings.HasPrefix(s, ".") {
		hasFrac = true
		s = s[1:]
		for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			fracDigits++
			s = s[1:]
		}
		if fracDigits == 0 {
			return 0, false, false
		}
	}
	if len(s) > 0 {
		return 0, false, false
	}
	return intDigits + fracDigits, hasFrac, true
}
