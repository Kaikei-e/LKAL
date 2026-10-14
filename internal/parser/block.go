package parser

import (
	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/token"
)

// scanHeaderTrivia is a pure function that scans leading newlines and header comments.
func scanHeaderTrivia(tokens []token.Token, pos int) ([]ast.Comment, int) {
	var head []ast.Comment
	newlines := 0
	for {
		tok := tokens[pos]
		switch tok.Kind {
		case token.Newline:
			pos++
			newlines++
		case token.Comment:
			pos++
			head = append(head, ast.Comment{
				Pos:         tok.Pos,
				Text:        tok.Text,
				BlankBefore: (newlines >= 2) || (len(head) == 0 && newlines >= 1),
			})
			newlines = 0
		default:
			return head, pos
		}
	}
}

// scanTrivia is a pure function that consumes newlines, commas, and comments before the next entry.
func scanTrivia(tokens []token.Token, pos int, prevRaw bool) (comments []ast.Comment, blank bool, nextPrevRaw bool, nextPos int) {
	newlines := 0
	nextPrevRaw = prevRaw
	for {
		tok := tokens[pos]
		switch tok.Kind {
		case token.Newline:
			pos++
			newlines++
			blank = newlines >= 2
		case token.Comma:
			pos++
			newlines = 0
			nextPrevRaw = false
		case token.Comment:
			pos++
			comments = append(comments, ast.Comment{
				Pos:         tok.Pos,
				Text:        tok.Text,
				BlankBefore: blank,
			})
			newlines = 0
			blank = false
		default:
			return comments, blank, nextPrevRaw, pos
		}
	}
}

func isRawAssign(ent *ast.Entry) bool {
	if ent == nil || ent.Kind != ast.EntryAssign {
		return false
	}
	_, ok := ent.Value.(*ast.RawNode)
	return ok
}

func (p *parser) parseHeader() (version string, versionPos diag.Pos, head []ast.Comment) {
	head, p.pos = scanHeaderTrivia(p.tokens, p.pos)

	tok := p.peek()
	versionPos = tok.Pos
	if tok.Kind != token.Directive {
		p.syntaxError(tok, "L105", "")
		return "", versionPos, head
	}

	p.next()
	valTok := p.peek()
	if valTok.Kind == token.String {
		p.next()
		version = valTok.Text
		if valTok.Text != "0.1" {
			p.errors = append(p.errors, diag.NewError(p.file, valTok.Pos, "L207", ""))
		}
		tok = p.peek()
		if tok.Kind == token.Newline || tok.Kind == token.EOF {
			return version, versionPos, head
		}
		p.syntaxError(tok, "L106", "end the line after the version declaration")
	} else {
		tok = valTok
		p.syntaxError(tok, "L105", "")
	}

	if tok.Kind != token.Newline && tok.Kind != token.EOF {
		p.recover()
	}
	return version, versionPos, head
}

func (p *parser) parseFileBody() *ast.Block {
	var entries []*ast.Entry
	prevWasRaw := false

	for {
		var comments []ast.Comment
		var blank bool
		comments, blank, prevWasRaw, p.pos = scanTrivia(p.tokens, p.pos, prevWasRaw)
		if p.peek().Kind == token.EOF {
			return &ast.Block{
				Entries:  entries,
				Trailing: comments,
			}
		}

		errCount := len(p.errors)
		ent, ok := p.parseEntry(comments, blank, prevWasRaw)
		if len(p.errors) > errCount+1 {
			p.errors = p.errors[:errCount+1]
		}
		if !ok {
			prevWasRaw = false
			p.recover()
			continue
		}
		if len(p.errors) == errCount {
			entries = append(entries, ent)
			prevWasRaw = isRawAssign(ent)
		} else {
			prevWasRaw = false
		}
	}
}

func (p *parser) parseBracedBlock() (*ast.Block, bool) {
	if p.depth >= diag.MaxBlockDepth {
		p.syntaxError(p.peek(), "L107", "")
		return nil, false
	}
	p.depth++

	lbraceTok := p.next()
	p.track(lbraceTok)
	var entries []*ast.Entry
	prevWasRaw := false

	for {
		var comments []ast.Comment
		var blank bool
		comments, blank, prevWasRaw, p.pos = scanTrivia(p.tokens, p.pos, prevWasRaw)
		if p.peek().Kind == token.RBrace {
			rbraceTok := p.next()
			p.track(rbraceTok)
			p.depth--
			return &ast.Block{
				Entries:  entries,
				Trailing: comments,
				OneLine:  lbraceTok.Pos.Line == rbraceTok.Pos.Line,
				LBrace:   lbraceTok.Pos,
				RBrace:   rbraceTok.Pos,
			}, true
		}

		ent, ok := p.parseEntry(comments, blank, prevWasRaw)
		if !ok {
			p.depth--
			return nil, false
		}
		entries = append(entries, ent)
		prevWasRaw = isRawAssign(ent)
	}
}

func (p *parser) parseEntry(comments []ast.Comment, blankBefore bool, prevWasRawAssign bool) (*ast.Entry, bool) {
	tok := p.peek()
	switch tok.Kind {
	case token.Bool:
		p.syntaxError(tok, "L106", "quote a reserved word to use it as a key")
		return nil, false
	case token.Directive:
		p.syntaxError(tok, "L105", "")
		return nil, false
	case token.RawLine:
		p.parseRaw()
		code := "L106"
		if prevWasRawAssign {
			code = "L104"
		}
		p.syntaxError(tok, code, "")
		return nil, false
	case token.Name, token.String:
	default:
		p.syntaxError(tok, "L106", "")
		return nil, false
	}

	keyTok := p.next()
	ent := &ast.Entry{
		Comments:    comments,
		BlankBefore: blankBefore,
		Key:         keyTok.Text,
		KeyPos:      keyTok.Pos,
	}

	afterKey := p.peek()
	if afterKey.Kind == token.Newline {
		p.syntaxError(afterKey, "L101", "")
		p.next()
		return nil, false
	}

	switch afterKey.Kind {
	case token.Assign:
		ent.Kind = ast.EntryAssign
		p.next()
		p.skipNewlines()
		valNode, ok := p.parseAssignValue()
		if !ok {
			return nil, false
		}
		ent.Value = valNode

	case token.LBrace:
		ent.Kind = ast.EntryRecord
		body, ok := p.parseBracedBlock()
		if !ok {
			return nil, false
		}
		ent.Body = body

	case token.String:
		ent.Kind = ast.EntryElem
		labelTok := p.next()
		ent.Label = labelTok.Text
		ent.LabelPos = labelTok.Pos

		afterLabel := p.peek()
		if afterLabel.Kind == token.Assign {
			p.syntaxError(afterLabel, "L206", "")
			return nil, false
		}
		if afterLabel.Kind == token.Newline {
			p.syntaxError(afterLabel, "L101", "")
			p.next()
			return nil, false
		}
		if afterLabel.Kind != token.LBrace {
			p.syntaxError(afterLabel, "L106", "follow the label with {")
			return nil, false
		}

		body, ok := p.parseBracedBlock()
		if !ok {
			return nil, false
		}
		ent.Body = body

	default:
		p.syntaxError(afterKey, "L106", `follow the key with =, { or "label"`)
		return nil, false
	}

	if !p.checkSeparator() {
		return nil, false
	}
	return ent, true
}

func (p *parser) checkSeparator() bool {
	nxt := p.peek()
	if nxt.Kind == token.Newline || nxt.Kind == token.Comma || nxt.Kind == token.EOF {
		return true
	}
	if nxt.Kind == token.RBrace && len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].Kind == token.LBrace {
		return true
	}
	p.syntaxError(nxt, "L106", "separate entries with a newline or a comma")
	return false
}
