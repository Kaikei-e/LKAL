package lexer

import (
	"testing"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/token"
)

func TestLexMultiLineAndRecovery(t *testing.T) {
	t.Run("MultiLineDocumentWithRawAndComments", func(t *testing.T) {
		src := "@lkal \"0.1\"\n### comment\nservice \"web\" {\n  startup =\n    \\\\line1\n    \\\\\n    \\\\line2\n}\n"
		tokens, errs := Lex("test.lkal", []byte(src))
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors, got %d: %v", len(errs), errs)
		}
		wantTokens := []token.Token{
			tok(token.Directive, 1, 1, "@lkal", "lkal"),
			tok(token.String, 1, 7, "\"0.1\"", "0.1"),
			tok(token.Newline, 1, 12, "\n", ""),
			tok(token.Comment, 2, 1, "### comment", "comment"),
			tok(token.Newline, 2, 12, "\n", ""),
			tok(token.Name, 3, 1, "service", "service"),
			tok(token.String, 3, 9, "\"web\"", "web"),
			tok(token.LBrace, 3, 15, "{", ""),
			tok(token.Newline, 3, 16, "\n", ""),
			tok(token.Name, 4, 3, "startup", "startup"),
			tok(token.Assign, 4, 11, "=", ""),
			tok(token.Newline, 4, 12, "\n", ""),
			tok(token.RawLine, 5, 5, "\\\\line1", "line1"),
			tok(token.Newline, 5, 12, "\n", ""),
			tok(token.RawLine, 6, 5, "\\\\", ""),
			tok(token.Newline, 6, 7, "\n", ""),
			tok(token.RawLine, 7, 5, "\\\\line2", "line2"),
			tok(token.Newline, 7, 12, "\n", ""),
			tok(token.RBrace, 8, 1, "}", ""),
			tok(token.Newline, 8, 2, "\n", ""),
			eofTok(9, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.Kind != want.Kind || got.Pos != want.Pos || got.Lit != want.Lit || got.Text != want.Text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.Kind, got.Pos, got.Lit, got.Text, want.Kind, want.Pos, want.Lit, want.Text)
			}
		}
	})

	t.Run("MultiLineIndependentLineErrors", func(t *testing.T) {
		src := "# bad1\na = 1\n\"unclosed\nb = 2\n"
		tokens, errs := Lex("test.lkal", []byte(src))
		if len(errs) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(errs))
		}
		if errs[0].Code != "L001" || errs[0].Pos != (diag.Pos{Line: 1, Col: 1}) {
			t.Errorf("error 0 mismatch: %v", errs[0])
		}
		if errs[1].Code != "L007" || errs[1].Pos != (diag.Pos{Line: 3, Col: 1}) {
			t.Errorf("error 1 mismatch: %v", errs[1])
		}
		wantTokens := []token.Token{
			tok(token.Comment, 1, 1, "# bad1", "bad1"),
			tok(token.Newline, 1, 7, "\n", ""),
			tok(token.Name, 2, 1, "a", "a"),
			tok(token.Assign, 2, 3, "=", ""),
			tok(token.Int, 2, 5, "1", "1"),
			tok(token.Newline, 2, 6, "\n", ""),
			tok(token.Illegal, 3, 1, "\"unclosed", ""),
			tok(token.Newline, 3, 10, "\n", ""),
			tok(token.Name, 4, 1, "b", "b"),
			tok(token.Assign, 4, 3, "=", ""),
			tok(token.Int, 4, 5, "2", "2"),
			tok(token.Newline, 4, 6, "\n", ""),
			eofTok(5, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.Kind != want.Kind || got.Pos != want.Pos || got.Lit != want.Lit || got.Text != want.Text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.Kind, got.Pos, got.Lit, got.Text, want.Kind, want.Pos, want.Lit, want.Text)
			}
		}
	})

	t.Run("MultiLineInlineCommentSuppression", func(t *testing.T) {
		src := "a = 1 # trailing comment\nb = 2\n"
		tokens, errs := Lex("test.lkal", []byte(src))
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d", len(errs))
		}
		if errs[0].Code != "L002" || errs[0].Pos != (diag.Pos{Line: 1, Col: 7}) {
			t.Errorf("error mismatch: %v", errs[0])
		}
		wantTokens := []token.Token{
			tok(token.Name, 1, 1, "a", "a"),
			tok(token.Assign, 1, 3, "=", ""),
			tok(token.Int, 1, 5, "1", "1"),
			tok(token.Newline, 1, 25, "\n", ""),
			tok(token.Name, 2, 1, "b", "b"),
			tok(token.Assign, 2, 3, "=", ""),
			tok(token.Int, 2, 5, "2", "2"),
			tok(token.Newline, 2, 6, "\n", ""),
			eofTok(3, 1),
		}
		if len(tokens) != len(wantTokens) {
			t.Fatalf("token count mismatch: got %d, want %d", len(tokens), len(wantTokens))
		}
		for i, want := range wantTokens {
			got := tokens[i]
			if got.Kind != want.Kind || got.Pos != want.Pos || got.Lit != want.Lit || got.Text != want.Text {
				t.Errorf("token[%d] mismatch: got {kind:%v pos:%v lit:%q text:%q}, want {kind:%v pos:%v lit:%q text:%q}",
					i, got.Kind, got.Pos, got.Lit, got.Text, want.Kind, want.Pos, want.Lit, want.Text)
			}
		}
	})
}

func TestLexControlCharsInCommentAndRawLines(t *testing.T) {
	cases := []lexCase{
		{
			name: "ContractGComment",
			src:  "@lkal \"0.1\"\n### a\x01b\n",
			tokens: []token.Token{
				tok(token.Directive, 1, 1, "@lkal", "lkal"),
				tok(token.String, 1, 7, "\"0.1\"", "0.1"),
				tok(token.Newline, 1, 12, "\n", ""),
				tok(token.Comment, 2, 1, "### a\x01b", "a\x01b"),
				tok(token.Newline, 2, 8, "\n", ""),
				eofTok(3, 1),
			},
			errs: []*diag.Error{errExp("L008", 2, 6)},
		},
		{
			name: "ContractGRawLine",
			src:  "@lkal \"0.1\"\nx =\n  \\\\a\x7fb\n",
			tokens: []token.Token{
				tok(token.Directive, 1, 1, "@lkal", "lkal"),
				tok(token.String, 1, 7, "\"0.1\"", "0.1"),
				tok(token.Newline, 1, 12, "\n", ""),
				tok(token.Name, 2, 1, "x", "x"),
				tok(token.Assign, 2, 3, "=", ""),
				tok(token.Newline, 2, 4, "\n", ""),
				tok(token.RawLine, 3, 3, "\\\\a\x7fb", "a\x7fb"),
				tok(token.Newline, 3, 8, "\n", ""),
				eofTok(4, 1),
			},
			errs: []*diag.Error{errExp("L008", 3, 6)},
		},
		{
			name: "CommentU0000",
			src:  "### a\x00b\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### a\x00b", "a\x00b"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 6)},
		},
		{
			name: "CommentU001B",
			src:  "### a\x1bb\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### a\x1bb", "a\x1bb"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 6)},
		},
		{
			name: "CommentU001F",
			src:  "### a\x1fb\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### a\x1fb", "a\x1fb"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 6)},
		},
		{
			name: "CommentU007F",
			src:  "### a\x7fb\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### a\x7fb", "a\x7fb"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 6)},
		},
		{
			name: "RawLineU0000",
			src:  "\\\\a\x00b\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\a\x00b", "a\x00b"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 4)},
		},
		{
			name: "RawLineU001B",
			src:  "\\\\a\x1bb\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\a\x1bb", "a\x1bb"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 4)},
		},
		{
			name: "RawLineU001F",
			src:  "\\\\a\x1fb\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\a\x1fb", "a\x1fb"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 4)},
		},
		{
			name: "RawLineU007F",
			src:  "\\\\a\x7fb\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\a\x7fb", "a\x7fb"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 4)},
		},
		{
			name: "TwoControlCharsInComment",
			src:  "### \x01 \x02\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### \x01 \x02", "\x01 \x02"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 5), errExp("L008", 1, 7)},
		},
		{
			name: "TwoControlCharsInRawLine",
			src:  "\\\\\x01\x02\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\\x01\x02", "\x01\x02"),
				tok(token.Newline, 1, 5, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L008", 1, 3), errExp("L008", 1, 4)},
		},
		{
			name: "ControlCharInL001Comment",
			src:  "# a\x01b\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "# a\x01b", "a\x01b"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
			errs: []*diag.Error{errExp("L001", 1, 1), errExp("L008", 1, 4)},
		},
		{
			name: "TabAllowedInComment",
			src:  "### a\tb\n",
			tokens: []token.Token{
				tok(token.Comment, 1, 1, "### a\tb", "a\tb"),
				tok(token.Newline, 1, 8, "\n", ""),
				eofTok(2, 1),
			},
		},
		{
			name: "TabAllowedInRawLine",
			src:  "\\\\a\tb\n",
			tokens: []token.Token{
				tok(token.RawLine, 1, 1, "\\\\a\tb", "a\tb"),
				tok(token.Newline, 1, 6, "\n", ""),
				eofTok(2, 1),
			},
		},
	}
	runLexCases(t, cases)
}
