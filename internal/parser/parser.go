package parser

import (
	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/lexer"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/token"
)

// Parse lexes, parses, and semantically validates src, returning an *ast.File or *diag.ErrorList.
func Parse(name string, src []byte) (*ast.File, error) {
	tokens, lexErrs := lexer.Lex(name, src)
	p := &parser{
		file:   name,
		tokens: tokens,
	}

	version, versionPos, head := p.parseHeader()
	body := p.parseFileBody()

	var allErrors []*diag.Error
	allErrors = append(allErrors, lexErrs...)
	allErrors = append(allErrors, p.errors...)
	allErrors = append(allErrors, Validate(name, body)...)
	if len(allErrors) > 0 {
		return nil, &diag.ErrorList{Errors: diag.SortErrors(allErrors)}
	}

	return &ast.File{
		Name:       name,
		Version:    version,
		VersionPos: versionPos,
		Head:       head,
		Body:       body,
	}, nil
}

// ParseValue parses a standalone LKAL value expression into a model.Value.
func ParseValue(src []byte) (model.Value, error) {
	tokens, lexErrs := lexer.Lex("", src)

	pos := skipTriviaAndComments(tokens, 0)
	if tokens[pos].Kind == token.EOF {
		var errs []*diag.Error
		errs = append(errs, lexErrs...)
		errs = append(errs, diag.NewError("", tokens[pos].Pos, "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines"))
		return nil, &diag.ErrorList{Errors: diag.SortErrors(errs)}
	}

	p := &parser{
		tokens: tokens,
		pos:    pos,
	}

	val, ok := p.parseTopValue()
	p.pos = skipTriviaAndComments(p.tokens, p.pos)

	if ok && !hasSyntaxErrors(lexErrs, p.errors) && p.peek().Kind != token.EOF {
		p.syntaxError(p.peek(), "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines")
	}

	var allErrors []*diag.Error
	allErrors = append(allErrors, lexErrs...)
	allErrors = append(allErrors, p.errors...)
	if len(allErrors) > 0 {
		return nil, &diag.ErrorList{Errors: diag.SortErrors(allErrors)}
	}
	return val, nil
}

func skipTriviaAndComments(tokens []token.Token, pos int) int {
	for tokens[pos].Kind == token.Newline || tokens[pos].Kind == token.Comment {
		pos++
	}
	return pos
}

func hasSyntaxErrors(lexErrs, parseErrs []*diag.Error) bool {
	if len(lexErrs) > 0 {
		return true
	}
	for _, err := range parseErrs {
		if err.Code < "L201" || err.Code > "L205" {
			return true
		}
	}
	return false
}

type parser struct {
	file     string
	tokens   []token.Token
	pos      int
	errors   []*diag.Error
	brackets []token.Token
	depth    int
}

func (p *parser) peek() token.Token {
	return p.tokens[p.pos]
}

func (p *parser) peekNext() token.Token {
	return p.tokens[p.pos+1]
}

func (p *parser) next() token.Token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) skipNewlines() token.Token {
	for p.peek().Kind == token.Newline {
		p.pos++
	}
	return p.peek()
}

func (p *parser) syntaxError(tok token.Token, code, hint string) {
	if tok.Kind == token.Illegal {
		return
	}
	if tok.Kind == token.EOF && len(p.brackets) > 0 {
		b := p.brackets[len(p.brackets)-1]
		p.errors = append(p.errors, diag.NewError(p.file, b.Pos, "L103", ""))
		return
	}
	p.errors = append(p.errors, diag.NewError(p.file, tok.Pos, code, hint))
}

func (p *parser) track(tok token.Token) {
	switch tok.Kind {
	case token.LBrace, token.LBrack:
		p.brackets = append(p.brackets, tok)
	case token.RBrace:
		if len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].Kind == token.LBrace {
			p.brackets = p.brackets[:len(p.brackets)-1]
		}
	case token.RBrack:
		if len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].Kind == token.LBrack {
			p.brackets = p.brackets[:len(p.brackets)-1]
		}
	}
}

func (p *parser) recover() {
	for p.peek().Kind != token.EOF {
		tok := p.next()
		p.track(tok)
		if tok.Kind == token.Newline && len(p.brackets) == 0 {
			break
		}
	}
	p.brackets = nil
}
