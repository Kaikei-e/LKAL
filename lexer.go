package lkal

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

func lex(name string, src []byte) ([]token, []*Error) {
	var tokens []token
	var errs []*Error

	var lines [][]byte
	var terminators []string
	start := 0
	numLFs := 0
	for {
		idx := -1
		for j := start; j < len(src); j++ {
			if src[j] == '\n' {
				idx = j
				break
			}
		}
		if idx == -1 {
			lines = append(lines, src[start:])
			terminators = append(terminators, "")
			break
		}
		numLFs++
		if idx > start && src[idx-1] == '\r' {
			lines = append(lines, src[start:idx-1])
			terminators = append(terminators, "\r\n")
		} else {
			lines = append(lines, src[start:idx])
			terminators = append(terminators, "\n")
		}
		start = idx + 1
	}

	for lineIdx, lineBytes := range lines {
		lineNum := lineIdx + 1
		term := terminators[lineIdx]
		hasLF := term != ""

		if lineIdx == len(lines)-1 && !hasLF && len(lineBytes) == 0 && lineIdx > 0 {
			break
		}

		col := 1
		rem := lineBytes

		if lineNum == 1 && len(rem) >= 3 && rem[0] == 0xef && rem[1] == 0xbb && rem[2] == 0xbf {
			errs = append(errs, lexNewError(name, Pos{Line: 1, Col: 1}, "L008"))
			tokens = append(tokens, token{
				kind: tokIllegal,
				pos:  Pos{Line: 1, Col: 1},
				lit:  string(rem[:3]),
				text: "",
			})
			col++
			rem = rem[3:]
		}

		for len(rem) > 0 && (rem[0] == ' ' || rem[0] == '\t') {
			col++
			rem = rem[1:]
		}

		if len(rem) > 0 {
			if rem[0] == '#' {
				hashCount := 0
				for hashCount < len(rem) && rem[hashCount] == '#' {
					hashCount++
				}
				if hashCount >= 3 {
					tokLit := string(rem)
					tokText := lexCommentText(string(rem[3:]))
					tokens = append(tokens, token{
						kind: tokComment,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  tokLit,
						text: tokText,
					})
					errs = append(errs, lexCheckCommentOrRawInvalidChars(name, lineNum, col, rem)...)
				} else {
					errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L001"))
					tokLit := string(rem)
					tokText := lexCommentText(string(rem[hashCount:]))
					tokens = append(tokens, token{
						kind: tokComment,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  tokLit,
						text: tokText,
					})
					errs = append(errs, lexCheckCommentOrRawInvalidChars(name, lineNum, col, rem)...)
				}
				rem = nil
			} else if len(rem) >= 2 && rem[0] == '\\' && rem[1] == '\\' {
				tokLit := string(rem)
				tokText := string(rem[2:])
				tokens = append(tokens, token{
					kind: tokRawLine,
					pos:  Pos{Line: lineNum, Col: col},
					lit:  tokLit,
					text: tokText,
				})
				errs = append(errs, lexCheckCommentOrRawInvalidChars(name, lineNum, col, rem)...)
				rem = nil
			} else if rem[0] == '@' {
				dirCol := col
				if len(rem) > 1 && lexIsNameStart(rem[1]) {
					k := 1
					for k < len(rem) && lexIsNameChar(rem[k]) {
						k++
					}
					nameStr := string(rem[1:k])
					fullLit := string(rem[:k])
					col += k
					rem = rem[k:]
					if nameStr == "lkal" {
						tokens = append(tokens, token{
							kind: tokDirective,
							pos:  Pos{Line: lineNum, Col: dirCol},
							lit:  fullLit,
							text: "lkal",
						})
					} else {
						errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: dirCol}, "L006"))
						tokens = append(tokens, token{
							kind: tokIllegal,
							pos:  Pos{Line: lineNum, Col: dirCol},
							lit:  fullLit,
							text: "",
						})
					}
				} else {
					errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: dirCol}, "L009"))
					k := 0
					for k < len(rem) && !lexIsDelimiter(rem[k]) {
						k++
					}
					runLit := string(rem[:k])
					col += lexCountCodePoints(rem[:k])
					rem = rem[k:]
					tokens = append(tokens, token{
						kind: tokIllegal,
						pos:  Pos{Line: lineNum, Col: dirCol},
						lit:  runLit,
						text: "",
					})
				}
			}
		}

		for len(rem) > 0 {
			if rem[0] == ' ' || rem[0] == '\t' {
				col++
				rem = rem[1:]
				continue
			}

			if rem[0] == '#' {
				errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L002"))
				break
			}

			if rem[0] == '"' {
				litStr, decodedStr, strErr, consumed, cp := lexScanString(name, lineNum, col, rem)
				if strErr != nil {
					errs = append(errs, strErr)
					tokens = append(tokens, token{
						kind: tokIllegal,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  litStr,
						text: "",
					})
				} else {
					tokens = append(tokens, token{
						kind: tokString,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  litStr,
						text: decodedStr,
					})
				}
				col += cp
				rem = rem[consumed:]
				continue
			}

			if (rem[0] >= '0' && rem[0] <= '9') || rem[0] == '-' {
				numStartCol := col
				k := 0
				for k < len(rem) && !lexIsDelimiter(rem[k]) {
					k++
				}
				runBytes := rem[:k]
				runLit := string(runBytes)
				numKind, numErr := lexValidateNumber(runLit)
				if numErr != "" {
					errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: numStartCol}, numErr))
					tokens = append(tokens, token{
						kind: tokIllegal,
						pos:  Pos{Line: lineNum, Col: numStartCol},
						lit:  runLit,
						text: "",
					})
				} else {
					tokens = append(tokens, token{
						kind: numKind,
						pos:  Pos{Line: lineNum, Col: numStartCol},
						lit:  runLit,
						text: runLit,
					})
				}
				col += lexCountCodePoints(runBytes)
				rem = rem[k:]
				continue
			}

			if rem[0] == '=' {
				tokens = append(tokens, token{kind: tokAssign, pos: Pos{Line: lineNum, Col: col}, lit: "=", text: ""})
				col++
				rem = rem[1:]
				continue
			}
			if rem[0] == '{' {
				tokens = append(tokens, token{kind: tokLBrace, pos: Pos{Line: lineNum, Col: col}, lit: "{", text: ""})
				col++
				rem = rem[1:]
				continue
			}
			if rem[0] == '}' {
				tokens = append(tokens, token{kind: tokRBrace, pos: Pos{Line: lineNum, Col: col}, lit: "}", text: ""})
				col++
				rem = rem[1:]
				continue
			}
			if rem[0] == '[' {
				tokens = append(tokens, token{kind: tokLBrack, pos: Pos{Line: lineNum, Col: col}, lit: "[", text: ""})
				col++
				rem = rem[1:]
				continue
			}
			if rem[0] == ']' {
				tokens = append(tokens, token{kind: tokRBrack, pos: Pos{Line: lineNum, Col: col}, lit: "]", text: ""})
				col++
				rem = rem[1:]
				continue
			}
			if rem[0] == ',' {
				tokens = append(tokens, token{kind: tokComma, pos: Pos{Line: lineNum, Col: col}, lit: ",", text: ""})
				col++
				rem = rem[1:]
				continue
			}

			if lexIsNameStart(rem[0]) {
				k := 0
				for k < len(rem) && lexIsNameChar(rem[k]) {
					k++
				}
				nameLit := string(rem[:k])
				if nameLit == "true" || nameLit == "false" {
					tokens = append(tokens, token{
						kind: tokBool,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  nameLit,
						text: nameLit,
					})
				} else {
					tokens = append(tokens, token{
						kind: tokName,
						pos:  Pos{Line: lineNum, Col: col},
						lit:  nameLit,
						text: nameLit,
					})
				}
				col += k
				rem = rem[k:]
				continue
			}

			if rem[0] == '.' {
				enumStartCol := col
				if len(rem) >= 2 && lexIsNameStart(rem[1]) {
					k := 2
					for k < len(rem) && lexIsNameChar(rem[k]) {
						k++
					}
					runLit := string(rem[:k])
					tokens = append(tokens, token{
						kind: tokEnum,
						pos:  Pos{Line: lineNum, Col: enumStartCol},
						lit:  runLit,
						text: runLit[1:],
					})
					col += k
					rem = rem[k:]
					continue
				}

				errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: enumStartCol}, "L009"))
				k := 0
				for k < len(rem) && !lexIsDelimiter(rem[k]) {
					k++
				}
				runBytes := rem[:k]
				runLit := string(runBytes)
				tokens = append(tokens, token{
					kind: tokIllegal,
					pos:  Pos{Line: lineNum, Col: enumStartCol},
					lit:  runLit,
					text: "",
				})
				col += lexCountCodePoints(runBytes)
				rem = rem[k:]
				continue
			}

			if rem[0] == '&' {
				refTok, refErr, consumed, cp := lexScanRef(name, lineNum, col, rem)
				if refErr != nil {
					errs = append(errs, refErr)
				}
				tokens = append(tokens, refTok)
				col += cp
				rem = rem[consumed:]
				continue
			}

			if rem[0] == '\r' {
				errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L008"))
				tokens = append(tokens, token{
					kind: tokIllegal,
					pos:  Pos{Line: lineNum, Col: col},
					lit:  "\r",
					text: "",
				})
				col++
				rem = rem[1:]
				continue
			}

			if rem[0] == '@' {
				errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L008"))
				tokens = append(tokens, token{
					kind: tokIllegal,
					pos:  Pos{Line: lineNum, Col: col},
					lit:  "@",
					text: "",
				})
				col++
				rem = rem[1:]
				continue
			}

			if rem[0] == '\\' {
				errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L008"))
				tokens = append(tokens, token{
					kind: tokIllegal,
					pos:  Pos{Line: lineNum, Col: col},
					lit:  "\\",
					text: "",
				})
				col++
				rem = rem[1:]
				continue
			}

			_, sz := utf8.DecodeRune(rem)
			errs = append(errs, lexNewError(name, Pos{Line: lineNum, Col: col}, "L008"))
			tokens = append(tokens, token{
				kind: tokIllegal,
				pos:  Pos{Line: lineNum, Col: col},
				lit:  string(rem[:sz]),
				text: "",
			})
			col++
			rem = rem[sz:]
		}

		if hasLF {
			newlineCol := 1 + lexCountCodePoints(lineBytes)
			tokens = append(tokens, token{
				kind: tokNewline,
				pos:  Pos{Line: lineNum, Col: newlineCol},
				lit:  term,
				text: "",
			})
		}
	}

	eofLine := numLFs + 1
	eofCol := 1 + lexCountCodePoints(lines[len(lines)-1])
	tokens = append(tokens, token{
		kind: tokEOF,
		pos:  Pos{Line: eofLine, Col: eofCol},
		lit:  "",
		text: "",
	})

	return tokens, errs
}

func lexNewError(file string, pos Pos, code string) *Error {
	return &Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  errorMessages[code],
		Hint: errorHints[code],
	}
}

func lexCountCodePoints(b []byte) int {
	n := 0
	for len(b) > 0 {
		_, sz := utf8.DecodeRune(b)
		n++
		b = b[sz:]
	}
	return n
}

func lexIsDelimiter(b byte) bool {
	switch b {
	case ' ', '\t', '=', '{', '}', '[', ']', ',', '"', '#':
		return true
	default:
		return false
	}
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
	if strings.HasPrefix(rawAfterHashes, " ") {
		rawAfterHashes = rawAfterHashes[1:]
	}
	return strings.TrimRight(rawAfterHashes, " \t")
}

func lexCheckCommentOrRawInvalidChars(file string, lineNum, startCol int, b []byte) []*Error {
	var errs []*Error
	currCol := startCol
	for len(b) > 0 {
		if b[0] == '\r' {
			errs = append(errs, lexNewError(file, Pos{Line: lineNum, Col: currCol}, "L008"))
			currCol++
			b = b[1:]
			continue
		}
		r, sz := utf8.DecodeRune(b)
		if r == utf8.RuneError && sz == 1 {
			errs = append(errs, lexNewError(file, Pos{Line: lineNum, Col: currCol}, "L008"))
			currCol++
			b = b[1:]
			continue
		}
		currCol++
		b = b[sz:]
	}
	return errs
}

func lexValidateNumber(run string) (tokKind, string) {
	s := run
	if len(s) == 0 {
		return tokIllegal, "L009"
	}

	if s[0] == '-' {
		s = s[1:]
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			return tokIllegal, "L009"
		}
	} else if s[0] < '0' || s[0] > '9' {
		return tokIllegal, "L009"
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

func lexScanString(file string, lineNum, startCol int, src []byte) (string, string, *Error, int, int) {
	var litBuilder strings.Builder
	var decoded strings.Builder
	var firstErr *Error

	litBuilder.WriteByte('"')
	i := 1
	currCol := startCol + 1
	closed := false

	for i < len(src) {
		if src[i] == '"' {
			closed = true
			litBuilder.WriteByte('"')
			currCol++
			i++
			break
		}
		if src[i] == '\\' {
			backslashCol := currCol
			litBuilder.WriteByte('\\')
			currCol++
			i++
			if i >= len(src) {
				break
			}
			switch src[i] {
			case '"':
				litBuilder.WriteByte('"')
				decoded.WriteByte('"')
				currCol++
				i++
			case '\\':
				litBuilder.WriteByte('\\')
				decoded.WriteByte('\\')
				currCol++
				i++
			case 'n':
				litBuilder.WriteByte('n')
				decoded.WriteByte('\n')
				currCol++
				i++
			case 'r':
				litBuilder.WriteByte('r')
				decoded.WriteByte('\r')
				currCol++
				i++
			case 't':
				litBuilder.WriteByte('t')
				decoded.WriteByte('\t')
				currCol++
				i++
			case 'u':
				litBuilder.WriteByte('u')
				currCol++
				i++
				if i < len(src) && src[i] == '{' {
					litBuilder.WriteByte('{')
					currCol++
					i++
					hexStart := i
					for i < len(src) && isHexDigit(src[i]) {
						litBuilder.WriteByte(src[i])
						currCol++
						i++
					}
					if i < len(src) && src[i] == '}' {
						litBuilder.WriteByte('}')
						currCol++
						hexStr := string(src[hexStart:i])
						i++
						val, err := strconv.ParseUint(hexStr, 16, 32)
						if err != nil || len(hexStr) < 1 || len(hexStr) > 6 || val > 0x10FFFF || (val >= 0xD800 && val <= 0xDFFF) {
							if firstErr == nil {
								firstErr = lexNewError(file, Pos{Line: lineNum, Col: backslashCol}, "L003")
							}
						} else {
							decoded.WriteRune(rune(val))
						}
					} else {
						if firstErr == nil {
							firstErr = lexNewError(file, Pos{Line: lineNum, Col: backslashCol}, "L003")
						}
					}
				} else {
					if firstErr == nil {
						firstErr = lexNewError(file, Pos{Line: lineNum, Col: backslashCol}, "L003")
					}
				}
			default:
				_, sz := utf8.DecodeRune(src[i:])
				litBuilder.WriteString(string(src[i : i+sz]))
				currCol++
				i += sz
				if firstErr == nil {
					firstErr = lexNewError(file, Pos{Line: lineNum, Col: backslashCol}, "L003")
				}
			}
			continue
		}

		r, sz := utf8.DecodeRune(src[i:])
		if r == utf8.RuneError && sz == 1 {
			litBuilder.WriteByte(src[i])
			if firstErr == nil {
				firstErr = lexNewError(file, Pos{Line: lineNum, Col: currCol}, "L008")
			}
			currCol++
			i++
			continue
		}
		if (r < 0x20 && r != '\t') || r == 0x7f {
			litBuilder.WriteString(string(src[i : i+sz]))
			if firstErr == nil {
				firstErr = lexNewError(file, Pos{Line: lineNum, Col: currCol}, "L008")
			}
			currCol++
			i += sz
			continue
		}

		litBuilder.WriteString(string(src[i : i+sz]))
		decoded.WriteRune(r)
		currCol++
		i += sz
	}

	if !closed {
		firstErr = lexNewError(file, Pos{Line: lineNum, Col: startCol}, "L007")
	}

	return litBuilder.String(), decoded.String(), firstErr, i, currCol - startCol
}

func lexScanRef(file string, lineNum, startCol int, src []byte) (token, *Error, int, int) {
	i := 1
	currCol := startCol + 1
	var segs []string
	var firstErr *Error
	hasError := false
	segCount := 0

	for {
		if i >= len(src) {
			break
		}
		if src[i] != '"' && lexIsDelimiter(src[i]) {
			break
		}

		if src[i] == '/' {
			hasError = true
			i++
			currCol++
			continue
		}

		if src[i] == '"' {
			_, decodedStr, strErr, consumed, cp := lexScanString(file, lineNum, currCol, src[i:])
			segCount++
			segs = append(segs, decodedStr)
			if strErr != nil {
				hasError = true
				if firstErr == nil {
					firstErr = strErr
				}
			}
			i += consumed
			currCol += cp
		} else if lexIsRefSegmentChar(src[i]) {
			segStart := i
			for i < len(src) && lexIsRefSegmentChar(src[i]) {
				currCol++
				i++
			}
			segLit := string(src[segStart:i])
			segCount++
			segs = append(segs, segLit)
		} else {
			break
		}

		if i < len(src) && src[i] == '/' {
			i++
			currCol++
			if i >= len(src) || (!lexIsRefSegmentChar(src[i]) && src[i] != '"' && src[i] != '/') {
				hasError = true
				break
			}
		} else {
			break
		}
	}

	fullLit := string(src[:i])
	if segCount == 0 || hasError {
		if firstErr == nil {
			firstErr = lexNewError(file, Pos{Line: lineNum, Col: startCol}, "L009")
		}
		tok := token{
			kind: tokIllegal,
			pos:  Pos{Line: lineNum, Col: startCol},
			lit:  fullLit,
			text: "",
		}
		return tok, firstErr, i, currCol - startCol
	}

	tok := token{
		kind: tokRef,
		pos:  Pos{Line: lineNum, Col: startCol},
		lit:  fullLit,
		text: string(src[1:i]),
		segs: segs,
	}
	return tok, nil, i, currCol - startCol
}
