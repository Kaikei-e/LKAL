package parser

import (
	"fmt"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/token"
)

func (p *parser) parseTopValue() (model.Value, bool) {
	if p.peek().Kind == token.String && p.peekNext().Kind == token.LBrace {
		return p.parseCollectionValue()
	}
	if p.peek().Kind == token.LBrace {
		body, ok := p.parseBracedBlock()
		if !ok {
			return nil, false
		}
		p.errors = append(p.errors, Validate("", body)...)
		return ast.BuildRecord(body), true
	}
	n, ok := p.parseAssignValue()
	if !ok {
		return nil, false
	}
	p.errors = append(p.errors, Validate("", &ast.Block{Entries: []*ast.Entry{{Value: n}}})...)
	return ast.BuildValue(n), true
}

func (p *parser) parseCollectionValue() (model.Value, bool) {
	var elems []model.Elem
	seenLabels := make(map[string]bool)
	ok := true

	for {
		labelTok := p.next()
		body, blockOK := p.parseBracedBlock()
		if !blockOK {
			ok = false
			break
		}
		if seenLabels[labelTok.Text] {
			p.errors = append(p.errors, diag.NewError("", labelTok.Pos, "L201", ""))
		}
		seenLabels[labelTok.Text] = true
		p.errors = append(p.errors, Validate("", body)...)
		elems = append(elems, model.Elem{Label: labelTok.Text, Body: ast.BuildRecord(body)})

		hasSep := false
		for p.peek().Kind == token.Newline || p.peek().Kind == token.Comma || p.peek().Kind == token.Comment {
			if p.peek().Kind == token.Newline || p.peek().Kind == token.Comma {
				hasSep = true
			}
			p.next()
		}

		if p.peek().Kind == token.EOF {
			break
		}
		if p.peek().Kind == token.String && p.peekNext().Kind == token.LBrace {
			if !hasSep {
				p.syntaxError(p.peek(), "L106", "separate entries with a newline or a comma")
				ok = false
				break
			}
			continue
		}
		break
	}
	return model.Collection{Elems: elems}, ok
}

func (p *parser) parseAssignValue() (ast.Node, bool) {
	tok := p.peek()
	switch tok.Kind {
	case token.LBrack:
		return p.parseList()
	case token.RawLine:
		return p.parseRaw(), true
	case token.Comment:
		p.syntaxError(tok, "L106", "move the comment above the entry")
		return nil, false
	case token.LBrace:
		p.syntaxError(tok, "L106", "write a record without =: key { ... }")
		return nil, false
	default:
		return p.parseScalarOrError()
	}
}

func (p *parser) parseListElem() (*ast.ScalarNode, bool) {
	tok := p.peek()
	if tok.Kind == token.LBrack || tok.Kind == token.LBrace {
		p.syntaxError(tok, "L106", "use only scalars as list elements")
		return nil, false
	}
	return p.parseScalarOrError()
}

func (p *parser) parseScalarOrError() (*ast.ScalarNode, bool) {
	tok := p.peek()
	if token.IsScalar(tok.Kind) {
		p.next()
		return ast.ScalarFromToken(tok), true
	}
	if tok.Kind == token.Name {
		p.syntaxError(tok, "L102", fmt.Sprintf("write %q for a string or .%s for an enum", tok.Text, tok.Text))
		return nil, false
	}
	if tok.Kind == token.Directive {
		p.syntaxError(tok, "L105", "")
		return nil, false
	}
	p.syntaxError(tok, "L106", `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`)
	return nil, false
}

func (p *parser) skipListWhitespace() token.Token {
	for {
		tok := p.peek()
		if tok.Kind == token.Newline {
			p.next()
			continue
		}
		if tok.Kind == token.Comment {
			p.syntaxError(tok, "L002", "")
			p.next()
			continue
		}
		return tok
	}
}

func (p *parser) parseList() (*ast.ListNode, bool) {
	lbrackTok := p.next()
	p.track(lbrackTok)
	var elems []*ast.ScalarNode

	tok := p.skipListWhitespace()
	if tok.Kind != token.RBrack {
		for {
			elem, ok := p.parseListElem()
			if !ok {
				return nil, false
			}
			elems = append(elems, elem)

			sep := p.skipListWhitespace()
			if sep.Kind == token.Comma {
				p.next()
				if p.skipListWhitespace().Kind == token.RBrack {
					break
				}
				continue
			}
			if sep.Kind == token.RBrack {
				break
			}
			p.syntaxError(sep, "L106", "separate list elements with ,")
			return nil, false
		}
	}

	rbrackTok := p.next()
	p.track(rbrackTok)
	return &ast.ListNode{
		LBrack:  lbrackTok.Pos,
		RBrack:  rbrackTok.Pos,
		Elems:   elems,
		OneLine: lbrackTok.Pos.Line == rbrackTok.Pos.Line,
	}, true
}

func (p *parser) parseRaw() *ast.RawNode {
	firstTok := p.next()
	lines := []string{firstTok.Text}
	for p.peek().Kind == token.Newline && p.peekNext().Kind == token.RawLine {
		p.next()
		lines = append(lines, p.next().Text)
	}
	return &ast.RawNode{Pos: firstTok.Pos, Lines: lines}
}
