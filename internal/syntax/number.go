package syntax

import (
	"strconv"
	"strings"
)

// NumberKind classifies a validated number literal.
type NumberKind int

const (
	NumberInvalid NumberKind = iota
	NumberInt
	NumberDecimal
	NumberQuantity
)

// parseDigits parses the numeric prefix (optional '-', integer digits, optional '.fraction')
// and returns (totalDigits, hasFraction, remainingSuffix, ok).
func parseDigits(s string) (int, bool, string, bool) {
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	if len(s) == 0 || s[0] < '0' || s[0] > '9' {
		return 0, false, "", false
	}

	var intDigits int
	if s[0] == '0' {
		if len(s) > 1 && s[1] >= '0' && s[1] <= '9' {
			return 0, false, "", false
		}
		intDigits = 1
		s = s[1:]
	} else {
		k := 0
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		intDigits = k
		s = s[k:]
	}

	hasFraction := false
	var fracDigits int
	if strings.HasPrefix(s, ".") {
		hasFraction = true
		s = s[1:]
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			return 0, false, "", false
		}
		k := 0
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		fracDigits = k
		s = s[k:]
	}

	return intDigits + fracDigits, hasFraction, s, true
}

// ScanPlainNumber validates a plain number string (without unit) and returns
// (totalDigits, hasFraction, ok).
func ScanPlainNumber(s string) (int, bool, bool) {
	digits, hasFrac, rest, ok := parseDigits(s)
	if !ok || rest != "" {
		return 0, false, false
	}
	return digits, hasFrac, true
}

// ValidateNumber validates a number or quantity token run and returns its kind
// or an error code ("L004", "L005", "L009").
func ValidateNumber(run string) (NumberKind, string) {
	totalDigits, hasFraction, s, ok := parseDigits(run)
	if !ok {
		return NumberInvalid, "L009"
	}

	hasUnit := false
	if len(s) > 0 && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= 'a' && s[0] <= 'z')) {
		hasUnit = true
		k := 0
		for k < len(s) && ((s[k] >= 'A' && s[k] <= 'Z') || (s[k] >= 'a' && s[k] <= 'z')) {
			k++
		}
		s = s[k:]
	}

	if len(s) > 0 {
		if hasUnit && s[0] >= '0' && s[0] <= '9' {
			return NumberInvalid, "L004"
		}
		return NumberInvalid, "L009"
	}

	if totalDigits > 100 {
		return NumberInvalid, "L005"
	}

	if !hasFraction && !hasUnit {
		if _, err := strconv.ParseInt(run, 10, 64); err != nil {
			return NumberInvalid, "L005"
		}
		return NumberInt, ""
	}

	if hasUnit {
		return NumberQuantity, ""
	}
	return NumberDecimal, ""
}

// NormalizeNumber returns the canonical string representation of an integer or decimal.
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

// SplitQuantity splits a quantity literal into its numeric prefix and unit suffix.
func SplitQuantity(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return s[:i], s[i:]
		}
	}
	return s, ""
}
