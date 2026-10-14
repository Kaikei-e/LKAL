package syntax

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// QuoteString formats s as a canonical double-quoted LKAL string literal.
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

// FormatKey formats k as a bare key if allowed, or as a quoted string otherwise.
func FormatKey(k string) string {
	if IsBareKey(k) {
		return k
	}
	return QuoteString(k)
}

// FormatRefBody formats reference segments separated by '/', quoting segments as needed.
func FormatRefBody(segs []string) string {
	var b strings.Builder
	for i, seg := range segs {
		if i > 0 {
			b.WriteByte('/')
		}
		if IsBareRefSegment(seg) {
			b.WriteString(seg)
		} else {
			b.WriteString(QuoteString(seg))
		}
	}
	return b.String()
}

// StringScan holds the result of scanning a double-quoted string starting at off.
type StringScan struct {
	Length  int
	Text    string
	ErrCode string
	ErrOff  int
}

func (s *StringScan) setErr(code string, off int) {
	if s.ErrCode == "" {
		s.ErrCode = code
		s.ErrOff = off
	}
}

// ScanString scans a double-quoted string literal starting at src[off] == '"'.
func ScanString(src []byte, off int) StringScan {
	var res StringScan
	var decoded strings.Builder

	i := off + 1
	closed := false

	for i < len(src) {
		if src[i] == '"' {
			closed = true
			i++
			break
		}
		if src[i] == '\\' {
			backslashOff := i
			i++
			if i >= len(src) {
				break
			}
			switch src[i] {
			case '"':
				decoded.WriteByte('"')
				i++
			case '\\':
				decoded.WriteByte('\\')
				i++
			case 'n':
				decoded.WriteByte('\n')
				i++
			case 'r':
				decoded.WriteByte('\r')
				i++
			case 't':
				decoded.WriteByte('\t')
				i++
			case 'u':
				i++
				if i < len(src) && src[i] == '{' {
					i++
					hexStart := i
					for i < len(src) && IsHexDigit(src[i]) {
						i++
					}
					if i < len(src) && src[i] == '}' {
						hexStr := string(src[hexStart:i])
						i++
						val, err := strconv.ParseUint(hexStr, 16, 32)
						if err != nil || len(hexStr) > 6 || val > 0x10FFFF || (val >= 0xD800 && val <= 0xDFFF) {
							res.setErr("L003", backslashOff)
						} else {
							decoded.WriteRune(rune(val))
						}
					} else {
						res.setErr("L003", backslashOff)
					}
				} else {
					res.setErr("L003", backslashOff)
				}
			default:
				_, sz := utf8.DecodeRune(src[i:])
				i += sz
				res.setErr("L003", backslashOff)
			}
			continue
		}

		r, sz := utf8.DecodeRune(src[i:])
		if (r == utf8.RuneError && sz == 1) || IsControlChar(r) {
			res.setErr("L008", i)
			i += sz
			continue
		}

		decoded.WriteRune(r)
		i += sz
	}

	if !closed {
		res.ErrCode = "L007"
		res.ErrOff = off
	}

	res.Length = i - off
	if res.ErrCode == "" {
		res.Text = decoded.String()
	}
	return res
}

// RefScan holds the result of scanning a reference starting at off ('&').
type RefScan struct {
	Length  int
	Text    string
	Segs    []string
	ErrCode string
	ErrOff  int
}

func (r *RefScan) setErr(code string, off int) {
	if r.ErrCode == "" {
		r.ErrCode = code
		r.ErrOff = off
	}
}

// ScanRef scans a reference literal starting at src[off] == '&'.
func ScanRef(src []byte, off int) RefScan {
	var res RefScan
	i := off + 1
	var segs []string

	for {
		if i < len(src) && src[i] == '"' {
			strRes := ScanString(src, i)
			segs = append(segs, strRes.Text)
			if strRes.ErrCode != "" {
				res.setErr(strRes.ErrCode, strRes.ErrOff)
			}
			i += strRes.Length
		} else if i < len(src) && IsRefSegmentChar(src[i]) {
			start := i
			for i < len(src) && IsRefSegmentChar(src[i]) {
				i++
			}
			segs = append(segs, string(src[start:i]))
		} else {
			res.setErr("L009", off)
		}

		if i < len(src) && src[i] == '/' {
			i++
			continue
		}
		break
	}

	res.Length = i - off
	if res.ErrCode != "" {
		return res
	}
	res.Text = string(src[off+1 : i])
	res.Segs = segs
	return res
}

// ParseRefBody parses a reference or path string (without leading '&') into segments.
func ParseRefBody(s string) ([]string, bool) {
	res := ScanRef([]byte("&"+s), 0)
	if res.ErrCode == "" && res.Length == len(s)+1 {
		return res.Segs, true
	}
	return nil, false
}
