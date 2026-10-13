package lkal

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func Parse(name string, src []byte) (*File, error) {
	tokens, lexErrs := lex(name, src)
	p := &parser{
		file:   name,
		tokens: tokens,
	}

	file := &File{name: name}
	p.parseHeader(file)
	file.body = p.parseFileBody()

	allErrors := append(lexErrs, p.errors...)
	allErrors = append(allErrors, validate(p.file, file.body)...)
	if len(allErrors) > 0 {
		return nil, &ErrorList{Errors: sortErrors(allErrors)}
	}

	return file, nil
}

func (f *File) Version() string {
	return f.version
}

func (f *File) Data() Record {
	return buildRecord(f.body)
}

type parser struct {
	file     string
	tokens   []token
	pos      int
	errors   []*Error
	brackets []token
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) peekNext() token {
	return p.tokens[p.pos+1]
}

func (p *parser) next() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) skipNewlines() token {
	for p.peek().kind == tokNewline {
		p.pos++
	}
	return p.peek()
}

func (p *parser) syntaxError(tok token, code, hint string) {
	if tok.kind == tokIllegal {
		return
	}
	if tok.kind == tokEOF && len(p.brackets) > 0 {
		b := p.brackets[len(p.brackets)-1]
		p.errors = append(p.errors, newError(p.file, b.pos, "L103", ""))
		return
	}
	p.errors = append(p.errors, newError(p.file, tok.pos, code, hint))
}

func (p *parser) track(tok token) {
	switch tok.kind {
	case tokLBrace, tokLBrack:
		p.brackets = append(p.brackets, tok)
	case tokRBrace:
		if len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].kind == tokLBrace {
			p.brackets = p.brackets[:len(p.brackets)-1]
		}
	case tokRBrack:
		if len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].kind == tokLBrack {
			p.brackets = p.brackets[:len(p.brackets)-1]
		}
	}
}

func (p *parser) recover() {
	for p.peek().kind != tokEOF {
		tok := p.next()
		p.track(tok)
		if tok.kind == tokNewline && len(p.brackets) == 0 {
			break
		}
	}
	p.brackets = nil
}

type bodyState struct {
	comments         []comment
	newlines         int
	blank            bool
	prevWasRawAssign bool
}

func (p *parser) advanceTrivia(s *bodyState) {
	for {
		tok := p.peek()
		switch tok.kind {
		case tokNewline:
			p.next()
			s.newlines++
			s.blank = s.newlines >= 2
		case tokComma:
			p.next()
			s.newlines = 0
			s.prevWasRawAssign = false
		case tokComment:
			p.next()
			s.comments = append(s.comments, comment{
				pos:         tok.pos,
				text:        tok.text,
				blankBefore: s.blank,
			})
			s.newlines = 0
			s.blank = false
		default:
			return
		}
	}
}

func isRaw(n node) bool {
	_, ok := n.(*rawNode)
	return ok
}

func (p *parser) parseHeader(file *File) {
	var head []comment
	newlines := 0

	for {
		tok := p.peek()
		if tok.kind == tokNewline {
			p.next()
			newlines++
			continue
		}
		if tok.kind == tokComment {
			p.next()
			head = append(head, comment{
				pos:         tok.pos,
				text:        tok.text,
				blankBefore: (newlines >= 2) || (len(head) == 0 && newlines >= 1),
			})
			newlines = 0
			continue
		}
		break
	}
	file.head = head

	tok := p.peek()
	file.versionPos = tok.pos
	if tok.kind != tokDirective {
		p.syntaxError(tok, "L105", "")
		return
	}

	p.next()
	valTok := p.peek()
	if valTok.kind == tokString {
		p.next()
		file.version = valTok.text
		if valTok.text != "0.1" {
			p.errors = append(p.errors, newError(p.file, valTok.pos, "L207", ""))
		}
		tok = p.peek()
		if tok.kind == tokNewline || tok.kind == tokEOF {
			return
		}
		p.syntaxError(tok, "L106", "end the line after the version declaration")
	} else {
		tok = valTok
		p.syntaxError(tok, "L105", "")
	}

	if tok.kind != tokNewline && tok.kind != tokEOF {
		p.recover()
	}
}

func (p *parser) parseFileBody() *block {
	var entries []*entry
	var s bodyState

	for {
		p.advanceTrivia(&s)
		if p.peek().kind == tokEOF {
			break
		}

		errCount := len(p.errors)
		ent, ok := p.parseEntry(s.comments, s.blank, s.prevWasRawAssign)
		s.comments = nil
		s.newlines = 0
		s.blank = false

		if len(p.errors) > errCount+1 {
			p.errors = p.errors[:errCount+1]
		}
		if !ok {
			s.prevWasRawAssign = false
			p.recover()
			continue
		}
		if len(p.errors) == errCount {
			entries = append(entries, ent)
			s.prevWasRawAssign = (ent.kind == entryAssign && isRaw(ent.value))
		} else {
			s.prevWasRawAssign = false
		}
	}

	return &block{
		entries:  entries,
		trailing: s.comments,
	}
}

func (p *parser) parseBracedBlock() (*block, bool) {
	lbraceTok := p.next()
	p.track(lbraceTok)
	var entries []*entry
	var s bodyState

	for {
		p.advanceTrivia(&s)
		if p.peek().kind == tokRBrace {
			break
		}

		ent, ok := p.parseEntry(s.comments, s.blank, s.prevWasRawAssign)
		s.comments = nil
		s.newlines = 0
		s.blank = false

		if !ok {
			return nil, false
		}
		entries = append(entries, ent)
		s.prevWasRawAssign = (ent.kind == entryAssign && isRaw(ent.value))
	}

	rbraceTok := p.next()
	p.track(rbraceTok)
	return &block{
		entries:  entries,
		trailing: s.comments,
		oneLine:  lbraceTok.pos.Line == rbraceTok.pos.Line,
		lbrace:   lbraceTok.pos,
		rbrace:   rbraceTok.pos,
	}, true
}

func (p *parser) parseEntry(comments []comment, blankBefore bool, prevWasRawAssign bool) (*entry, bool) {
	tok := p.peek()
	if tok.kind == tokBool {
		p.syntaxError(tok, "L106", "quote a reserved word to use it as a key")
		return nil, false
	}
	if tok.kind == tokDirective {
		p.syntaxError(tok, "L105", "")
		return nil, false
	}
	if tok.kind == tokRawLine {
		p.parseRaw()
		code := "L106"
		if prevWasRawAssign {
			code = "L104"
		}
		p.syntaxError(tok, code, "")
		return nil, false
	}
	if tok.kind != tokName && tok.kind != tokString {
		p.syntaxError(tok, "L106", "")
		return nil, false
	}

	keyTok := p.next()
	ent := &entry{
		comments:    comments,
		blankBefore: blankBefore,
		key:         keyTok.text,
		keyPos:      keyTok.pos,
	}

	afterKey := p.peek()
	if afterKey.kind == tokNewline {
		p.syntaxError(afterKey, "L101", "")
		p.next()
		return nil, false
	}

	switch afterKey.kind {
	case tokAssign:
		ent.kind = entryAssign
		p.next()
		p.skipNewlines()
		valNode, ok := p.parseAssignValue()
		if !ok {
			return nil, false
		}
		ent.value = valNode

	case tokLBrace:
		ent.kind = entryRecord
		body, ok := p.parseBracedBlock()
		if !ok {
			return nil, false
		}
		ent.body = body

	case tokString:
		ent.kind = entryElem
		labelTok := p.next()
		ent.label = labelTok.text
		ent.labelPos = labelTok.pos

		afterLabel := p.peek()
		if afterLabel.kind == tokAssign {
			p.syntaxError(afterLabel, "L206", "")
			return nil, false
		}
		if afterLabel.kind == tokNewline {
			p.syntaxError(afterLabel, "L101", "")
			p.next()
			return nil, false
		}
		if afterLabel.kind != tokLBrace {
			p.syntaxError(afterLabel, "L106", "follow the label with {")
			return nil, false
		}

		body, ok := p.parseBracedBlock()
		if !ok {
			return nil, false
		}
		ent.body = body

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
	if nxt.kind == tokNewline || nxt.kind == tokComma || nxt.kind == tokEOF {
		return true
	}
	if nxt.kind == tokRBrace && len(p.brackets) > 0 && p.brackets[len(p.brackets)-1].kind == tokLBrace {
		return true
	}
	p.syntaxError(nxt, "L106", "separate entries with a newline or a comma")
	return false
}

func isScalar(k tokKind) bool {
	switch k {
	case tokString, tokInt, tokDecimal, tokQuantity, tokBool, tokEnum, tokRef:
		return true
	default:
		return false
	}
}

func parseScalar(tok token) *scalarNode {
	var val Value
	switch tok.kind {
	case tokString:
		val = String(tok.text)
	case tokInt:
		n, _ := strconv.ParseInt(tok.text, 10, 64)
		val = Int(n)
	case tokDecimal:
		val = Decimal(normalizeNumber(tok.text))
	case tokQuantity:
		num, unit := splitQuantity(tok.text)
		val = Quantity{Number: normalizeNumber(num), Unit: unit}
	case tokBool:
		val = Bool(tok.text == "true")
	case tokEnum:
		val = Enum(tok.text)
	case tokRef:
		val = Ref(tok.segs)
	}
	return &scalarNode{tok: tok, val: val}
}

func (p *parser) parseAssignValue() (node, bool) {
	tok := p.peek()
	if tok.kind == tokLBrack {
		return p.parseList()
	}
	if tok.kind == tokRawLine {
		return p.parseRaw(), true
	}
	if tok.kind == tokComment {
		p.syntaxError(tok, "L106", "move the comment above the entry")
		return nil, false
	}
	if tok.kind == tokLBrace {
		p.syntaxError(tok, "L106", "write a record without =: key { ... }")
		return nil, false
	}
	return p.parseScalarOrError()
}

func (p *parser) parseListElem() (*scalarNode, bool) {
	tok := p.peek()
	if tok.kind == tokLBrack || tok.kind == tokLBrace {
		p.syntaxError(tok, "L106", "use only scalars as list elements")
		return nil, false
	}
	return p.parseScalarOrError()
}

func (p *parser) parseScalarOrError() (*scalarNode, bool) {
	tok := p.peek()
	if isScalar(tok.kind) {
		p.next()
		return parseScalar(tok), true
	}
	if tok.kind == tokName {
		p.syntaxError(tok, "L102", fmt.Sprintf("write %q for a string or .%s for an enum", tok.text, tok.text))
		return nil, false
	}
	if tok.kind == tokDirective {
		p.syntaxError(tok, "L105", "")
		return nil, false
	}
	p.syntaxError(tok, "L106", `write a value: string, number, quantity, boolean, enum, reference, list or \\ lines`)
	return nil, false
}

func (p *parser) skipListWhitespace() token {
	for {
		tok := p.peek()
		if tok.kind == tokNewline {
			p.next()
			continue
		}
		if tok.kind == tokComment {
			p.syntaxError(tok, "L002", "")
			p.next()
			continue
		}
		return tok
	}
}

func (p *parser) parseList() (*listNode, bool) {
	lbrackTok := p.next()
	p.track(lbrackTok)
	var elems []*scalarNode

	tok := p.skipListWhitespace()
	if tok.kind != tokRBrack {
		for {
			elem, ok := p.parseListElem()
			if !ok {
				return nil, false
			}
			elems = append(elems, elem)

			sep := p.skipListWhitespace()
			if sep.kind == tokComma {
				p.next()
				if p.skipListWhitespace().kind == tokRBrack {
					break
				}
				continue
			}
			if sep.kind == tokRBrack {
				break
			}
			p.syntaxError(sep, "L106", "separate list elements with ,")
			return nil, false
		}
	}

	rbrackTok := p.next()
	p.track(rbrackTok)
	return &listNode{
		lbrack:  lbrackTok.pos,
		rbrack:  rbrackTok.pos,
		elems:   elems,
		oneLine: lbrackTok.pos.Line == rbrackTok.pos.Line,
	}, true
}

func (p *parser) parseRaw() *rawNode {
	firstTok := p.next()
	lines := []string{firstTok.text}
	for p.peek().kind == tokNewline && p.peekNext().kind == tokRawLine {
		p.next()
		lines = append(lines, p.next().text)
	}
	return &rawNode{pos: firstTok.pos, lines: lines}
}

func validate(file string, b *block) []*Error {
	var errs []*Error
	seenKinds := make(map[string]map[entryKind]bool)
	seenLabels := make(map[string]map[string]bool)
	var prevElemKey string

	for _, e := range b.entries {
		checkEntry := func() *Error {
			if strings.HasPrefix(e.key, "$") {
				return newError(file, e.keyPos, "L205", "")
			}
			for k := range seenKinds[e.key] {
				if k != e.kind {
					return newError(file, e.keyPos, "L202", "")
				}
			}
			if e.kind == entryAssign || e.kind == entryRecord {
				if seenKinds[e.key] != nil {
					return newError(file, e.keyPos, "L201", "")
				}
			} else if e.kind == entryElem {
				if seenLabels[e.key] != nil && seenLabels[e.key][e.label] {
					return newError(file, e.labelPos, "L201", "")
				}
			}
			if e.kind == entryElem && seenKinds[e.key] != nil && prevElemKey != e.key {
				return newError(file, e.keyPos, "L203", "")
			}
			return nil
		}

		if err := checkEntry(); err != nil {
			errs = append(errs, err)
		}

		if seenKinds[e.key] == nil {
			seenKinds[e.key] = make(map[entryKind]bool)
		}
		seenKinds[e.key][e.kind] = true
		if e.kind == entryElem {
			if seenLabels[e.key] == nil {
				seenLabels[e.key] = make(map[string]bool)
			}
			seenLabels[e.key][e.label] = true
			prevElemKey = e.key
		} else {
			prevElemKey = ""
		}

		if lst, ok := e.value.(*listNode); ok && len(lst.elems) > 1 {
			firstKind := lst.elems[0].tok.kind
			for _, el := range lst.elems[1:] {
				if el.tok.kind != firstKind {
					errs = append(errs, newError(file, el.tok.pos, "L204", ""))
					break
				}
			}
		}

		if e.body != nil {
			errs = append(errs, validate(file, e.body)...)
		}
	}
	return errs
}

func buildRecord(b *block) Record {
	var fields []Field
	collIndexByKey := make(map[string]int)

	for _, e := range b.entries {
		switch e.kind {
		case entryAssign:
			fields = append(fields, Field{Key: e.key, Value: buildValue(e.value)})
		case entryRecord:
			fields = append(fields, Field{Key: e.key, Value: buildRecord(e.body)})
		case entryElem:
			elem := Elem{Label: e.label, Body: buildRecord(e.body)}
			if idx, ok := collIndexByKey[e.key]; ok {
				coll := fields[idx].Value.(Collection)
				coll.Elems = append(coll.Elems, elem)
				fields[idx].Value = coll
			} else {
				collIndexByKey[e.key] = len(fields)
				fields = append(fields, Field{Key: e.key, Value: Collection{Elems: []Elem{elem}}})
			}
		}
	}
	return Record{Fields: fields}
}

func buildValue(n node) Value {
	switch v := n.(type) {
	case *scalarNode:
		if r, ok := v.val.(Ref); ok {
			return slices.Clone(r)
		}
		return v.val
	case *rawNode:
		return String(strings.Join(v.lines, "\n"))
	case *listNode:
		list := make(List, len(v.elems))
		for i, el := range v.elems {
			if r, ok := el.val.(Ref); ok {
				list[i] = slices.Clone(r)
			} else {
				list[i] = el.val
			}
		}
		return list
	}
	return nil
}

func normalizeNumber(s string) string {
	if s == "-0" {
		return "0"
	}
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if !hasDot {
		return s
	}
	isNeg := false
	if strings.HasPrefix(intPart, "-") {
		isNeg = true
		intPart = intPart[1:]
	}
	fracPart = strings.TrimRight(fracPart, "0")
	if fracPart == "" {
		fracPart = "0"
	}
	if intPart == "0" && fracPart == "0" {
		return "0.0"
	}
	if isNeg {
		return "-" + intPart + "." + fracPart
	}
	return intPart + "." + fracPart
}

func splitQuantity(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return s[:i], s[i:]
		}
	}
	return s, ""
}
