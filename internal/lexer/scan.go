package lexer

import (
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/syntax"
	"github.com/Kaikei-e/LKAL/internal/token"
)

type scanResult struct {
	kind    token.Kind
	length  int
	text    string
	segs    []string
	errCode string
	errOff  int
}

func scanToken(line []byte, cursor int, atStart bool) scanResult {
	b := line[cursor]
	switch {
	case b == '#' && atStart:
		return scanComment(line, cursor)
	case b == '\\' && atStart && len(line)-cursor >= 2 && line[cursor+1] == '\\':
		return scanResult{kind: token.RawLine, length: len(line) - cursor, text: string(line[cursor+2:])}
	case b == '@' && atStart:
		return scanDirective(line, cursor)
	case b == '@' && !atStart:
		return scanResult{kind: token.Illegal, length: 1, errCode: "L008", errOff: cursor}
	case b == '"':
		return scanString(line, cursor)
	case (b >= '0' && b <= '9') || b == '-':
		return scanNumber(line, cursor)
	case token.SymbolKind(b) != token.EOF:
		return scanResult{kind: token.SymbolKind(b), length: 1}
	case b == '.':
		return scanEnum(line, cursor)
	case b == '&':
		return scanRef(line, cursor)
	case syntax.IsNameStart(b):
		return scanName(line, cursor)
	default:
		_, sz := utf8.DecodeRune(line[cursor:])
		return scanResult{kind: token.Illegal, length: sz, errCode: "L008", errOff: cursor}
	}
}

func scanDirective(src []byte, off int) scanResult {
	if off+1 < len(src) && syntax.IsNameStart(src[off+1]) {
		nameRes := scanName(src, off+1)
		totalLen := 1 + nameRes.length
		if nameRes.text == "lkal" {
			return scanResult{kind: token.Directive, length: totalLen, text: "lkal"}
		}
		return scanResult{kind: token.Illegal, length: totalLen, errCode: "L006", errOff: off}
	}
	return scanResult{kind: token.Illegal, length: syntax.DelimiterRun(src, off), errCode: "L009", errOff: off}
}

func scanComment(src []byte, off int) scanResult {
	hashCount := 0
	for off+hashCount < len(src) && src[off+hashCount] == '#' {
		hashCount++
	}
	cut := min(hashCount, 3)
	res := scanResult{
		kind:   token.Comment,
		length: len(src) - off,
		text:   commentText(string(src[off+cut:])),
	}
	if hashCount < 3 {
		res.errCode = "L001"
		res.errOff = off
	}
	return res
}

func commentText(rawAfterHashes string) string {
	rawAfterHashes = strings.TrimPrefix(rawAfterHashes, " ")
	return strings.TrimRight(rawAfterHashes, " \t")
}

func scanName(src []byte, off int) scanResult {
	k := off + 1
	for k < len(src) && syntax.IsNameChar(src[k]) {
		k++
	}
	text := string(src[off:k])
	kind := token.Name
	if text == "true" || text == "false" {
		kind = token.Bool
	}
	return scanResult{kind: kind, length: k - off, text: text}
}

func scanNumber(src []byte, off int) scanResult {
	length := syntax.DelimiterRun(src, off)
	run := string(src[off : off+length])
	numKind, errCode := syntax.ValidateNumber(run)
	if errCode != "" {
		return scanResult{kind: token.Illegal, length: length, errCode: errCode, errOff: off}
	}
	var kind token.Kind
	switch numKind {
	case syntax.NumberInt:
		kind = token.Int
	case syntax.NumberDecimal:
		kind = token.Decimal
	case syntax.NumberQuantity:
		kind = token.Quantity
	}
	return scanResult{kind: kind, length: length, text: run}
}

func scanEnum(src []byte, off int) scanResult {
	if off+1 < len(src) && syntax.IsNameStart(src[off+1]) {
		nameRes := scanName(src, off+1)
		return scanResult{kind: token.Enum, length: 1 + nameRes.length, text: nameRes.text}
	}
	return scanResult{kind: token.Illegal, length: syntax.DelimiterRun(src, off), errCode: "L009", errOff: off}
}

func scanString(src []byte, off int) scanResult {
	s := syntax.ScanString(src, off)
	if s.ErrCode != "" {
		return scanResult{kind: token.Illegal, length: s.Length, errCode: s.ErrCode, errOff: s.ErrOff}
	}
	return scanResult{kind: token.String, length: s.Length, text: s.Text}
}

func scanRef(src []byte, off int) scanResult {
	r := syntax.ScanRef(src, off)
	if r.ErrCode != "" {
		return scanResult{kind: token.Illegal, length: r.Length, errCode: r.ErrCode, errOff: r.ErrOff}
	}
	return scanResult{kind: token.Ref, length: r.Length, text: r.Text, segs: r.Segs}
}

func checkInvalidChars(file string, lineNum int, cols []int, line []byte, off int) []*diag.Error {
	var errs []*diag.Error
	for i := off; i < len(line); {
		r, sz := utf8.DecodeRune(line[i:])
		if (r == utf8.RuneError && sz == 1) || syntax.IsControlChar(r) {
			errs = append(errs, diag.NewError(file, diag.Pos{Line: lineNum, Col: cols[i]}, "L008", ""))
		}
		i += sz
	}
	return errs
}
