package token

import "github.com/Kaikei-e/LKAL/internal/diag"

// Kind identifies the lexical category of a Token.
type Kind int

const (
	EOF Kind = iota
	Illegal
	Newline
	Comment
	Name
	String
	RawLine
	Int
	Decimal
	Quantity
	Bool
	Enum
	Ref
	Directive
	Assign
	LBrace
	RBrace
	LBrack
	RBrack
	Comma
)

var kindNames = [...]string{
	EOF:       "EOF",
	Illegal:   "ILLEGAL",
	Newline:   "NEWLINE",
	Comment:   "COMMENT",
	Name:      "NAME",
	String:    "STRING",
	RawLine:   "RAW_LINE",
	Int:       "INT",
	Decimal:   "DECIMAL",
	Quantity:  "QUANTITY",
	Bool:      "BOOL",
	Enum:      "ENUM",
	Ref:       "REF",
	Directive: "DIRECTIVE",
	Assign:    "=",
	LBrace:    "{",
	RBrace:    "}",
	LBrack:    "[",
	RBrack:    "]",
	Comma:     ",",
}

func (k Kind) String() string {
	if int(k) >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return ""
}

// Token is an immutable lexical token produced by the lexer.
type Token struct {
	Kind Kind
	Pos  diag.Pos
	Lit  string
	Text string
	Segs []string
}

// SymbolKind returns the punctuation token Kind for a single byte, or EOF (0) if none.
func SymbolKind(b byte) Kind {
	switch b {
	case '=':
		return Assign
	case '{':
		return LBrace
	case '}':
		return RBrace
	case '[':
		return LBrack
	case ']':
		return RBrack
	case ',':
		return Comma
	default:
		return EOF
	}
}

// IsScalar reports whether k is one of the 7 scalar value token kinds.
func IsScalar(k Kind) bool {
	switch k {
	case String, Int, Decimal, Quantity, Bool, Enum, Ref:
		return true
	default:
		return false
	}
}
