package lexer

import (
	"bytes"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/token"
)

// Lex tokenizes src into an immutable slice of tokens ending with token.EOF,
// along with any lexical errors in source order.
func Lex(name string, src []byte) ([]token.Token, []*diag.Error) {
	var tokens []token.Token
	var errs []*diag.Error

	startCol := 1
	if bytes.HasPrefix(src, []byte("\xef\xbb\xbf")) {
		errs = append(errs, diag.NewError(name, diag.Pos{Line: 1, Col: 1}, "L008", ""))
		src = src[3:]
		startCol = 2
	}

	lineNum := 1
	eofPos := diag.Pos{Line: 1, Col: startCol}
	for len(src) > 0 {
		line, term, rest := splitLine(src)
		src = rest

		cols := diag.LineCols(line, startCol)
		lineTokens, lineErrs := lexLine(name, lineNum, cols, line)
		tokens = append(tokens, lineTokens...)
		errs = append(errs, lineErrs...)

		if term != "" {
			tokens = append(tokens, token.Token{
				Kind: token.Newline,
				Pos:  diag.Pos{Line: lineNum, Col: cols[len(line)]},
				Lit:  term,
			})
			eofPos = diag.Pos{Line: lineNum + 1, Col: 1}
		} else {
			eofPos = diag.Pos{Line: lineNum, Col: cols[len(line)]}
		}

		lineNum++
		startCol = 1
	}

	tokens = append(tokens, token.Token{
		Kind: token.EOF,
		Pos:  eofPos,
	})

	return tokens, errs
}

func splitLine(src []byte) (line []byte, term string, rest []byte) {
	idx := bytes.IndexByte(src, '\n')
	if idx < 0 {
		return src, "", nil
	}
	if idx > 0 && src[idx-1] == '\r' {
		return src[:idx-1], "\r\n", src[idx+1:]
	}
	return src[:idx], "\n", src[idx+1:]
}

func lexLine(file string, lineNum int, cols []int, line []byte) ([]token.Token, []*diag.Error) {
	var tokens []token.Token
	var errs []*diag.Error

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
		atStart := cursor == first
		if b == '#' && !atStart {
			errs = append(errs, diag.NewError(file, diag.Pos{Line: lineNum, Col: cols[cursor]}, "L002", ""))
			return tokens, errs
		}

		res := scanToken(line, cursor, atStart)
		tokens = append(tokens, token.Token{
			Kind: res.kind,
			Pos:  diag.Pos{Line: lineNum, Col: cols[cursor]},
			Lit:  string(line[cursor : cursor+res.length]),
			Text: res.text,
			Segs: res.segs,
		})
		if res.errCode != "" {
			errs = append(errs, diag.NewError(file, diag.Pos{Line: lineNum, Col: cols[res.errOff]}, res.errCode, ""))
		}
		if res.kind == token.Comment || res.kind == token.RawLine {
			errs = append(errs, checkInvalidChars(file, lineNum, cols, line, cursor)...)
		}
		cursor += res.length
	}

	return tokens, errs
}
