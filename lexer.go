package lkal

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

var symbolTokens = [256]tokKind{
	'=': tokAssign,
	'{': tokLBrace,
	'}': tokRBrace,
	'[': tokLBrack,
	']': tokRBrack,
	',': tokComma,
}

type scanResult struct {
	kind    tokKind
	length  int
	text    string
	segs    []string
	errCode string
	errOff  int
}

func (r *scanResult) setErr(code string, off int) {
	if r.errCode == "" {
		r.errCode = code
		r.errOff = off
	}
}

func lex(name string, src []byte) ([]token, []*Error) {
	var tokens []token
	var errs []*Error

	startCol := 1
	if bytes.HasPrefix(src, []byte("\xef\xbb\xbf")) {
		errs = append(errs, newError(name, Pos{Line: 1, Col: 1}, "L008", ""))
		src = src[3:]
		startCol = 2
	}

	lineNum := 1
	eofPos := Pos{Line: 1, Col: startCol}
	for len(src) > 0 {
		var line []byte
		term := ""
		idx := bytes.IndexByte(src, '\n')
		if idx >= 0 {
			term = "\n"
			if idx > 0 && src[idx-1] == '\r' {
				line = src[:idx-1]
				term = "\r\n"
			} else {
				line = src[:idx]
			}
			src = src[idx+1:]
		} else {
			line = src
			src = nil
		}

		cols := lineCols(line, startCol)
		lineTokens, lineErrs := lexLine(name, lineNum, cols, line)
		tokens = append(tokens, lineTokens...)
		errs = append(errs, lineErrs...)

		if term != "" {
			tokens = append(tokens, token{
				kind: tokNewline,
				pos:  Pos{Line: lineNum, Col: cols[len(line)]},
				lit:  term,
			})
			eofPos = Pos{Line: lineNum + 1, Col: 1}
		} else {
			eofPos = Pos{Line: lineNum, Col: cols[len(line)]}
		}

		lineNum++
		startCol = 1
	}

	tokens = append(tokens, token{
		kind: tokEOF,
		pos:  eofPos,
	})

	return tokens, errs
}

func lineCols(line []byte, startCol int) []int {
	cols := make([]int, len(line)+1)
	col := startCol
	for off := 0; off < len(line); {
		cols[off] = col
		_, sz := utf8.DecodeRune(line[off:])
		for j := 1; j < sz; j++ {
			cols[off+j] = col
		}
		col++
		off += sz
	}
	cols[len(line)] = col
	return cols
}

func lexScanDirective(src []byte, off int) scanResult {
	if off+1 < len(src) && lexIsNameStart(src[off+1]) {
		nameRes := lexScanName(src, off+1)
		totalLen := 1 + nameRes.length
		if nameRes.text == "lkal" {
			return scanResult{kind: tokDirective, length: totalLen, text: "lkal"}
		}
		return scanResult{kind: tokIllegal, length: totalLen, errCode: "L006", errOff: off}
	}
	return scanResult{kind: tokIllegal, length: lexDelimiterRun(src, off), errCode: "L009", errOff: off}
}

func lexScanComment(src []byte, off int) scanResult {
	hashCount := 0
	for off+hashCount < len(src) && src[off+hashCount] == '#' {
		hashCount++
	}
	cut := hashCount
	if cut > 3 {
		cut = 3
	}
	var res scanResult
	res.kind = tokComment
	res.length = len(src) - off
	res.text = lexCommentText(string(src[off+cut:]))
	if hashCount < 3 {
		res.setErr("L001", off)
	}
	return res
}

func lexLine(file string, lineNum int, cols []int, line []byte) ([]token, []*Error) {
	var tokens []token
	var errs []*Error

	first := 0
	for first < len(line) && (line[first] == ' ' || line[first] == '\t') {
		first++
	}

	cursor := 0
	for cursor < len(line) {
		b := line[cursor]
		if b == ' ' || b == '\t' {
			cursor++
			continue
		}
		atStart := (cursor == first)

		var res scanResult

		switch {
		case b == '#' && atStart:
			res = lexScanComment(line, cursor)

		case b == '#' && !atStart:
			errs = append(errs, newError(file, Pos{Line: lineNum, Col: cols[cursor]}, "L002", ""))
			return tokens, errs

		case b == '\\' && atStart && len(line)-cursor >= 2 && line[cursor+1] == '\\':
			res = scanResult{kind: tokRawLine, length: len(line) - cursor, text: string(line[cursor+2:])}

		case b == '@' && atStart:
			res = lexScanDirective(line, cursor)

		case b == '@' && !atStart:
			res = scanResult{kind: tokIllegal, length: 1, errCode: "L008", errOff: cursor}

		case b == '"':
			res = lexScanString(line, cursor)

		case (b >= '0' && b <= '9') || b == '-':
			res = lexScanNumber(line, cursor)

		case symbolTokens[b] != 0:
			res = scanResult{kind: symbolTokens[b], length: 1}

		case b == '.':
			res = lexScanEnum(line, cursor)

		case b == '&':
			res = lexScanRef(line, cursor)

		case lexIsNameStart(b):
			res = lexScanName(line, cursor)

		default:
			_, sz := utf8.DecodeRune(line[cursor:])
			res = scanResult{kind: tokIllegal, length: sz, errCode: "L008", errOff: cursor}
		}

		tokens = append(tokens, token{
			kind: res.kind,
			pos:  Pos{Line: lineNum, Col: cols[cursor]},
			lit:  string(line[cursor : cursor+res.length]),
			text: res.text,
			segs: res.segs,
		})
		if res.errCode != "" {
			errs = append(errs, newError(file, Pos{Line: lineNum, Col: cols[res.errOff]}, res.errCode, ""))
		}
		if res.kind == tokComment || res.kind == tokRawLine {
			errs = append(errs, lexCheckCommentOrRawInvalidChars(file, lineNum, cols, line, cursor)...)
		}
		cursor += res.length
	}

	return tokens, errs
}

func lexIsDelimiter(b byte) bool {
	return b == ' ' || b == '\t' || b == '"' || b == '#' || symbolTokens[b] != 0
}

func lexIsNameStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func lexIsNameChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

func lexIsRefSegmentChar(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.' || b == '-'
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func lexCommentText(rawAfterHashes string) string {
	rawAfterHashes = strings.TrimPrefix(rawAfterHashes, " ")
	return strings.TrimRight(rawAfterHashes, " \t")
}

func lexCheckCommentOrRawInvalidChars(file string, lineNum int, cols []int, line []byte, off int) []*Error {
	var errs []*Error
	for i := off; i < len(line); {
		if line[i] == '\r' {
			errs = append(errs, newError(file, Pos{Line: lineNum, Col: cols[i]}, "L008", ""))
			i++
			continue
		}
		r, sz := utf8.DecodeRune(line[i:])
		if r == utf8.RuneError && sz == 1 {
			errs = append(errs, newError(file, Pos{Line: lineNum, Col: cols[i]}, "L008", ""))
		} else if (r < 0x20 && r != '\t') || r == 0x7f {
			errs = append(errs, newError(file, Pos{Line: lineNum, Col: cols[i]}, "L008", ""))
		}
		i += sz
	}
	return errs
}

func lexValidateNumber(run string) (tokKind, string) {
	s := run
	if s[0] == '-' {
		s = s[1:]
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			return tokIllegal, "L009"
		}
	}

	var intDigits string
	if s[0] == '0' {
		if len(s) > 1 && s[1] >= '0' && s[1] <= '9' {
			return tokIllegal, "L009"
		}
		intDigits = "0"
		s = s[1:]
	} else {
		k := 0
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		intDigits = s[:k]
		s = s[k:]
	}

	hasFraction := false
	var fracDigits string
	if len(s) > 0 && s[0] == '.' {
		hasFraction = true
		s = s[1:]
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			return tokIllegal, "L009"
		}
		k := 0
		for k < len(s) && s[k] >= '0' && s[k] <= '9' {
			k++
		}
		fracDigits = s[:k]
		s = s[k:]
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
			return tokIllegal, "L004"
		}
		return tokIllegal, "L009"
	}

	totalDigits := len(intDigits) + len(fracDigits)
	if totalDigits > 100 {
		return tokIllegal, "L005"
	}

	if !hasFraction && !hasUnit {
		_, err := strconv.ParseInt(run, 10, 64)
		if err != nil {
			return tokIllegal, "L005"
		}
		return tokInt, ""
	}

	if hasUnit {
		return tokQuantity, ""
	}
	return tokDecimal, ""
}

func lexDelimiterRun(src []byte, off int) int {
	k := off
	for k < len(src) && !lexIsDelimiter(src[k]) {
		k++
	}
	return k - off
}

func lexScanName(src []byte, off int) scanResult {
	k := off + 1
	for k < len(src) && lexIsNameChar(src[k]) {
		k++
	}
	text := string(src[off:k])
	kind := tokName
	if text == "true" || text == "false" {
		kind = tokBool
	}
	return scanResult{kind: kind, length: k - off, text: text}
}

func lexScanNumber(src []byte, off int) scanResult {
	length := lexDelimiterRun(src, off)
	run := string(src[off : off+length])
	kind, errCode := lexValidateNumber(run)
	if errCode != "" {
		return scanResult{kind: tokIllegal, length: length, errCode: errCode, errOff: off}
	}
	return scanResult{kind: kind, length: length, text: run}
}

func lexScanEnum(src []byte, off int) scanResult {
	if off+1 < len(src) && lexIsNameStart(src[off+1]) {
		nameRes := lexScanName(src, off+1)
		return scanResult{kind: tokEnum, length: 1 + nameRes.length, text: nameRes.text}
	}
	return scanResult{kind: tokIllegal, length: lexDelimiterRun(src, off), errCode: "L009", errOff: off}
}

func lexScanString(src []byte, off int) scanResult {
	var res scanResult
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
					for i < len(src) && isHexDigit(src[i]) {
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
		if r == utf8.RuneError && sz == 1 {
			res.setErr("L008", i)
			i++
			continue
		}
		if (r < 0x20 && r != '\t') || r == 0x7f {
			res.setErr("L008", i)
			i += sz
			continue
		}

		decoded.WriteRune(r)
		i += sz
	}

	if !closed {
		res.errCode = "L007"
		res.errOff = off
	}

	res.length = i - off
	if res.errCode != "" {
		res.kind = tokIllegal
		res.text = ""
	} else {
		res.kind = tokString
		res.text = decoded.String()
	}
	return res
}

func lexScanRef(src []byte, off int) scanResult {
	var res scanResult
	i := off + 1
	var segs []string

	for {
		if i < len(src) && src[i] == '"' {
			strRes := lexScanString(src, i)
			segs = append(segs, strRes.text)
			if strRes.errCode != "" {
				res.setErr(strRes.errCode, strRes.errOff)
			}
			i += strRes.length
		} else if i < len(src) && lexIsRefSegmentChar(src[i]) {
			start := i
			for i < len(src) && lexIsRefSegmentChar(src[i]) {
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

	res.length = i - off
	if res.errCode != "" {
		res.kind = tokIllegal
		return res
	}
	res.kind = tokRef
	res.text = string(src[off+1 : i])
	res.segs = segs
	return res
}
