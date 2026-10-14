package syntax

// IsSymbol reports whether b is a single-byte punctuation token in LKAL.
func IsSymbol(b byte) bool {
	switch b {
	case '=', '{', '}', '[', ']', ',':
		return true
	default:
		return false
	}
}

// IsDelimiter reports whether b terminates a number, enum, or directive run.
func IsDelimiter(b byte) bool {
	return b == ' ' || b == '\t' || b == '"' || b == '#' || IsSymbol(b)
}

// DelimiterRun returns the byte length of the run starting at off up to the next delimiter.
func DelimiterRun(src []byte, off int) int {
	k := off
	for k < len(src) && !IsDelimiter(src[k]) {
		k++
	}
	return k - off
}

// IsNameStart reports whether b can start an unquoted name or enum.
func IsNameStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

// IsNameChar reports whether b can appear after the first byte of an unquoted name.
func IsNameChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

// IsRefSegmentChar reports whether b can appear in an unquoted reference segment.
func IsRefSegmentChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.' || b == '-'
}

// IsHexDigit reports whether b is an ASCII hexadecimal digit.
func IsHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

// IsControlChar reports whether r is a disallowed control character (all C0 except tab, plus DEL).
func IsControlChar(r rune) bool {
	return (r < 0x20 && r != '\t') || r == 0x7f
}

// IsValidEnumName reports whether s is a syntactically valid enum name (without leading dot).
func IsValidEnumName(s string) bool {
	if len(s) == 0 || !IsNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !IsNameChar(s[i]) {
			return false
		}
	}
	return true
}

// IsBareKey reports whether k can be written as an unquoted key in canonical LKAL.
func IsBareKey(k string) bool {
	if k == "" || k == "true" || k == "false" {
		return false
	}
	return IsValidEnumName(k)
}

// IsBareRefSegment reports whether s can be written as an unquoted reference segment.
func IsBareRefSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !IsRefSegmentChar(s[i]) {
			return false
		}
	}
	return true
}

// IsRawCandidate reports whether s should be formatted as multi-line raw (\\) lines.
func IsRawCandidate(s string) bool {
	hasLF := false
	for _, r := range s {
		if r == '\n' {
			hasLF = true
		} else if IsControlChar(r) {
			return false
		}
	}
	return hasLF
}
