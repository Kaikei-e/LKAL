package token

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{EOF, "EOF"},
		{Illegal, "ILLEGAL"},
		{Newline, "NEWLINE"},
		{Comment, "COMMENT"},
		{Name, "NAME"},
		{String, "STRING"},
		{RawLine, "RAW_LINE"},
		{Int, "INT"},
		{Decimal, "DECIMAL"},
		{Quantity, "QUANTITY"},
		{Bool, "BOOL"},
		{Enum, "ENUM"},
		{Ref, "REF"},
		{Directive, "DIRECTIVE"},
		{Assign, "="},
		{LBrace, "{"},
		{RBrace, "}"},
		{LBrack, "["},
		{RBrack, "]"},
		{Comma, ","},
		{Kind(-1), ""},
		{Kind(999), ""},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSymbolKind(t *testing.T) {
	tests := []struct {
		b    byte
		want Kind
	}{
		{'=', Assign},
		{'{', LBrace},
		{'}', RBrace},
		{'[', LBrack},
		{']', RBrack},
		{',', Comma},
		{'+', EOF},
		{'a', EOF},
		{0, EOF},
	}

	for _, tt := range tests {
		if got := SymbolKind(tt.b); got != tt.want {
			t.Errorf("SymbolKind(%q) = %v, want %v", tt.b, got, tt.want)
		}
	}
}

func TestIsScalar(t *testing.T) {
	scalars := []Kind{String, Int, Decimal, Quantity, Bool, Enum, Ref}
	for _, k := range scalars {
		if !IsScalar(k) {
			t.Errorf("IsScalar(%v) = false, want true", k)
		}
	}

	nonScalars := []Kind{
		EOF, Illegal, Newline, Comment, Name, RawLine,
		Directive, Assign, LBrace, RBrace, LBrack, RBrack, Comma,
	}
	for _, k := range nonScalars {
		if IsScalar(k) {
			t.Errorf("IsScalar(%v) = true, want false", k)
		}
	}
}

func TestTokenStructFields(t *testing.T) {
	tok := Token{
		Kind: Ref,
		Pos:  diag.Pos{Line: 3, Col: 7},
		Lit:  "&a/b",
		Text: "a/b",
		Segs: []string{"a", "b"},
	}
	if tok.Kind != Ref || tok.Pos.Line != 3 || tok.Pos.Col != 7 || len(tok.Segs) != 2 {
		t.Fatalf("unexpected token fields: %+v", tok)
	}
}
