package lkal

type tokKind int

const (
	tokEOF tokKind = iota
	tokIllegal
	tokNewline
	tokComment
	tokName
	tokString
	tokRawLine
	tokInt
	tokDecimal
	tokQuantity
	tokBool
	tokEnum
	tokRef
	tokDirective
	tokAssign
	tokLBrace
	tokRBrace
	tokLBrack
	tokRBrack
	tokComma
)

func (k tokKind) String() string {
	switch k {
	case tokEOF:
		return "EOF"
	case tokIllegal:
		return "ILLEGAL"
	case tokNewline:
		return "NEWLINE"
	case tokComment:
		return "COMMENT"
	case tokName:
		return "NAME"
	case tokString:
		return "STRING"
	case tokRawLine:
		return "RAW_LINE"
	case tokInt:
		return "INT"
	case tokDecimal:
		return "DECIMAL"
	case tokQuantity:
		return "QUANTITY"
	case tokBool:
		return "BOOL"
	case tokEnum:
		return "ENUM"
	case tokRef:
		return "REF"
	case tokDirective:
		return "DIRECTIVE"
	case tokAssign:
		return "="
	case tokLBrace:
		return "{"
	case tokRBrace:
		return "}"
	case tokLBrack:
		return "["
	case tokRBrack:
		return "]"
	case tokComma:
		return ","
	default:
		return ""
	}
}

type token struct {
	kind tokKind
	pos  Pos
	lit  string
	text string
	segs []string
}
