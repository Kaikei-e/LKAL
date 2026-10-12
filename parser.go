package lkal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func Parse(name string, src []byte) (*File, error) {
	tokens, lexErrs := lex(name, src)
	p := &parser{
		file:   name,
		tokens: tokens,
	}

	file, ok := p.parseVersion(name)
	if ok && p.peek().kind != tokEOF {
		file.body, _ = p.parseBlock(true)
	} else if file.body == nil {
		file.body = &block{}
	}

	p.validateBlock(file.body)

	allErrors := append(lexErrs, p.errors...)
	if len(allErrors) > 0 {
		sort.SliceStable(allErrors, func(i, j int) bool {
			if allErrors[i].Pos.Line != allErrors[j].Pos.Line {
				return allErrors[i].Pos.Line < allErrors[j].Pos.Line
			}
			if allErrors[i].Pos.Col != allErrors[j].Pos.Col {
				return allErrors[i].Pos.Col < allErrors[j].Pos.Col
			}
			return allErrors[i].Code < allErrors[j].Code
		})
		if len(allErrors) > 20 {
			allErrors = allErrors[:20]
		}
		return nil, &ErrorList{Errors: allErrors}
	}

	return file, nil
}

func (f *File) Version() string {
	if f == nil {
		return ""
	}
	return f.version
}

func (f *File) Data() Record {
	if f == nil || f.body == nil {
		return Record{}
	}
	return buildRecord(f.body)
}

type bracketInfo struct {
	pos     Pos
	isBrace bool
}

type parser struct {
	file         string
	tokens       []token
	pos          int
	errors       []*Error
	topError     bool
	bracketStack []bracketInfo
}

func (p *parser) peek() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	if len(p.tokens) > 0 {
		return p.tokens[len(p.tokens)-1]
	}
	return token{kind: tokEOF, pos: Pos{Line: 1, Col: 1}}
}

func (p *parser) peekNext() token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	if len(p.tokens) > 0 {
		return p.tokens[len(p.tokens)-1]
	}
	return token{kind: tokEOF, pos: Pos{Line: 1, Col: 1}}
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) skipNewlines() token {
	for p.pos < len(p.tokens) && p.tokens[p.pos].kind == tokNewline {
		p.pos++
	}
	return p.peek()
}

func (p *parser) pushBracket(pos Pos, isBrace bool) {
	p.bracketStack = append(p.bracketStack, bracketInfo{pos: pos, isBrace: isBrace})
}

func (p *parser) popBracket() {
	if len(p.bracketStack) > 0 {
		p.bracketStack = p.bracketStack[:len(p.bracketStack)-1]
	}
}

func (p *parser) reportInnermostL103() {
	if len(p.bracketStack) > 0 {
		top := p.bracketStack[len(p.bracketStack)-1]
		p.errors = append(p.errors, parseNewError(p.file, top.pos, "L103", ""))
		p.bracketStack = nil
	}
}

func (p *parser) syntaxError(pos Pos, code, hint string) {
	if p.topError {
		return
	}
	p.errors = append(p.errors, parseNewError(p.file, pos, code, hint))
	if code != "L103" {
		p.topError = true
	}
}

func isBOM(tok token) bool {
	return tok.kind == tokIllegal && tok.pos.Line == 1 && tok.pos.Col == 1 && tok.lit == "\xef\xbb\xbf"
}

func (p *parser) consumeRawRun() {
	p.next()
	for p.pos < len(p.tokens) {
		if p.peek().kind == tokNewline && p.peekNext().kind == tokRawLine {
			p.next()
			p.next()
		} else {
			break
		}
	}
}

func (p *parser) recoverTopLevel() {
	depth := 0
	for _, b := range p.bracketStack {
		if b.isBrace {
			depth++
		}
	}
	p.bracketStack = nil

	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokLBrace {
			depth++
			p.pos++
			continue
		}
		if tok.kind == tokRBrace {
			if depth > 0 {
				depth--
			}
			p.pos++
			continue
		}
		if tok.kind == tokNewline && depth == 0 {
			p.pos++
			for p.pos < len(p.tokens) && (p.tokens[p.pos].kind == tokNewline || p.tokens[p.pos].kind == tokComma) {
				p.pos++
			}
			break
		}
		p.pos++
	}
}

func (p *parser) parseVersion(file string) (*File, bool) {
	f := &File{name: file}
	var headComments []comment
	newlines := 0
	hasPrevLine := false

	if p.pos < len(p.tokens) && isBOM(p.peek()) {
		p.next()
	}

	for p.pos < len(p.tokens) {
		tok := p.peek()
		if tok.kind == tokNewline {
			p.next()
			newlines++
			continue
		}
		if tok.kind == tokComment {
			p.next()
			blankBefore := (newlines >= 2) || (!hasPrevLine && newlines >= 1)
			headComments = append(headComments, comment{
				pos:         tok.pos,
				text:        tok.text,
				blankBefore: blankBefore,
			})
			newlines = 0
			hasPrevLine = true
			continue
		}
		break
	}
	f.head = headComments

	tok := p.peek()
	if tok.kind == tokEOF {
		p.syntaxError(tok.pos, "L105", "")
		f.version = "0.1"
		f.versionPos = tok.pos
		return f, false
	}

	if tok.kind == tokDirective {
		f.versionPos = tok.pos
		p.next()

		valTok := p.peek()
		if valTok.kind == tokString {
			f.version = valTok.text
			if valTok.text != "0.1" {
				p.errors = append(p.errors, parseNewError(p.file, valTok.pos, "L207", ""))
			}
			p.next()

			nxt := p.peek()
			if nxt.kind != tokNewline && nxt.kind != tokEOF {
				if nxt.kind != tokIllegal {
					p.syntaxError(nxt.pos, "L106", "版の宣言の後ろには改行が必要")
				}
				for p.pos < len(p.tokens) && p.tokens[p.pos].kind != tokNewline && p.tokens[p.pos].kind != tokEOF {
					p.pos++
				}
			}
			return f, true
		}

		if valTok.kind != tokIllegal {
			p.syntaxError(valTok.pos, "L105", "")
		}
		f.version = "0.1"
		for p.pos < len(p.tokens) && p.tokens[p.pos].kind != tokNewline && p.tokens[p.pos].kind != tokEOF {
			p.pos++
		}
		return f, true
	}

	if tok.kind == tokIllegal && strings.HasPrefix(tok.lit, "@") {
		f.versionPos = tok.pos
		f.version = "0.1"
		for p.pos < len(p.tokens) && p.tokens[p.pos].kind != tokNewline && p.tokens[p.pos].kind != tokEOF {
			p.pos++
		}
		return f, true
	}

	if tok.kind != tokIllegal {
		p.syntaxError(tok.pos, "L105", "")
	}
	f.version = "0.1"
	f.versionPos = tok.pos
	return f, true
}

func (p *parser) parseBlock(isTopLevel bool) (*block, bool) {
	blk := &block{}
	var entries []*entry
	var pendingComments []comment
	newlines := 0
	hasPrevLine := true
	prevWasRaw := false

	defer func() {
		blk.entries = entries
		blk.trailing = pendingComments
	}()

	for p.pos < len(p.tokens) {
		tok := p.peek()

		if tok.kind == tokNewline {
			p.next()
			newlines++
			continue
		}

		if tok.kind == tokComma {
			p.next()
			continue
		}

		if tok.kind == tokComment {
			p.next()
			blankBefore := (newlines >= 2) || (!hasPrevLine && newlines >= 1)
			pendingComments = append(pendingComments, comment{
				pos:         tok.pos,
				text:        tok.text,
				blankBefore: blankBefore,
			})
			newlines = 0
			hasPrevLine = true
			continue
		}

		if tok.kind == tokEOF {
			break
		}

		if tok.kind == tokRBrace {
			if !isTopLevel {
				break
			}
			p.syntaxError(tok.pos, "L106", "予期しない閉じ括弧")
			p.next()
			p.recoverTopLevel()
			p.topError = false
			continue
		}

		if isTopLevel {
			p.topError = false
		}

		blankBefore := (newlines >= 2) || (!hasPrevLine && newlines >= 1)
		ent, ok, isRaw := p.parseEntry(pendingComments, blankBefore, prevWasRaw)
		pendingComments = nil
		newlines = 0
		hasPrevLine = true
		prevWasRaw = isRaw

		if !ok {
			if isTopLevel {
				p.recoverTopLevel()
				p.topError = false
				prevWasRaw = false
				continue
			}
			return blk, false
		}

		entries = append(entries, ent)
	}

	return blk, true
}

func (p *parser) parseEntry(comments []comment, blankBefore bool, prevWasRaw bool) (*entry, bool, bool) {
	tok := p.peek()

	if tok.kind == tokBool {
		p.syntaxError(tok.pos, "L106", fmt.Sprintf("予約語をキーにするには %q のように引用符で囲む", tok.lit))
		return nil, false, false
	}

	if tok.kind == tokRawLine {
		if prevWasRaw {
			p.syntaxError(tok.pos, "L104", "")
		} else {
			p.syntaxError(tok.pos, "L106", "複数行文字列は = の後ろに書く")
		}
		p.consumeRawRun()
		return nil, false, false
	}

	if tok.kind == tokDirective {
		p.syntaxError(tok.pos, "L105", "")
		return nil, false, false
	}

	if tok.kind == tokIllegal {
		return nil, false, false
	}

	if tok.kind != tokName && tok.kind != tokString {
		hint := "構文を見直す"
		if tok.kind == tokAssign {
			hint = "キーの前に = は書けない"
		}
		p.syntaxError(tok.pos, "L106", hint)
		return nil, false, false
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
		p.syntaxError(afterKey.pos, "L101", "")
		p.next()
		return nil, false, false
	}

	if afterKey.kind == tokAssign {
		ent.kind = entryAssign
		p.next()

		valTok := p.skipNewlines()
		if valTok.kind == tokComment {
			p.syntaxError(valTok.pos, "L106", "= と値の間にコメントは書けない")
			return nil, false, false
		}
		if valTok.kind == tokLBrace {
			p.syntaxError(valTok.pos, "L106", "レコードは key { … } のように = なしで書く")
			return nil, false, false
		}
		if valTok.kind == tokName {
			p.syntaxError(valTok.pos, "L102", fmt.Sprintf("文字列なら %q、列挙子なら .%s と書く", valTok.text, valTok.text))
			return nil, false, false
		}
		if valTok.kind == tokIllegal {
			return nil, false, false
		}

		valNode, ok := p.parseValue(valTok)
		if !ok {
			return nil, false, false
		}
		ent.value = valNode
		_, isRaw := valNode.(*rawNode)

		if !p.checkSeparator() {
			return nil, false, false
		}
		return ent, true, isRaw
	}

	if afterKey.kind == tokLBrace {
		ent.kind = entryRecord
		if !p.parseEntryBlock(ent) {
			return nil, false, false
		}
		if !p.checkSeparator() {
			return nil, false, false
		}
		return ent, true, false
	}

	if afterKey.kind == tokString {
		ent.kind = entryElem
		labelTok := p.next()
		ent.label = labelTok.text
		ent.labelPos = labelTok.pos

		afterLabel := p.peek()
		if afterLabel.kind == tokAssign {
			p.syntaxError(afterLabel.pos, "L206", "")
			return nil, false, false
		}
		if afterLabel.kind == tokNewline {
			p.syntaxError(afterLabel.pos, "L101", "")
			p.next()
			return nil, false, false
		}
		if afterLabel.kind == tokIllegal {
			return nil, false, false
		}
		if afterLabel.kind != tokLBrace {
			p.syntaxError(afterLabel.pos, "L106", "ラベルの後ろには { が必要")
			return nil, false, false
		}

		if !p.parseEntryBlock(ent) {
			return nil, false, false
		}
		if !p.checkSeparator() {
			return nil, false, false
		}
		return ent, true, false
	}

	if afterKey.kind != tokIllegal {
		p.syntaxError(afterKey.pos, "L106", "= または { が必要")
	}
	return nil, false, false
}

func (p *parser) parseEntryBlock(ent *entry) bool {
	lbraceTok := p.next()
	p.pushBracket(lbraceTok.pos, true)

	body, ok := p.parseBlock(false)
	if !ok {
		return false
	}
	body.lbrace = lbraceTok.pos
	ent.body = body

	if p.peek().kind == tokRBrace {
		rbraceTok := p.next()
		body.rbrace = rbraceTok.pos
		body.oneLine = (body.lbrace.Line == body.rbrace.Line)
		p.popBracket()
		return true
	}
	if p.peek().kind == tokEOF {
		p.reportInnermostL103()
		return false
	}
	if p.peek().kind == tokIllegal {
		return false
	}
	p.syntaxError(p.peek().pos, "L106", "} が必要")
	return false
}

func (p *parser) checkSeparator() bool {
	nxt := p.peek()
	if nxt.kind == tokNewline || nxt.kind == tokComma || nxt.kind == tokRBrace || nxt.kind == tokEOF || nxt.kind == tokComment {
		return true
	}
	if nxt.kind != tokIllegal {
		p.syntaxError(nxt.pos, "L106", "項目の間は改行かカンマで区切る")
	}
	return false
}

func (p *parser) parseValue(valTok token) (node, bool) {
	if getScalarKind(valTok.kind) != -1 {
		p.next()
		return p.parseScalar(valTok), true
	}
	if valTok.kind == tokLBrack {
		return p.parseList(valTok)
	}
	if valTok.kind == tokRawLine {
		return p.parseRaw(valTok)
	}
	if valTok.kind != tokIllegal {
		p.syntaxError(valTok.pos, "L106", "値が必要")
	}
	return nil, false
}

func (p *parser) parseRaw(valTok token) (*rawNode, bool) {
	lines := []string{valTok.text}
	firstPos := valTok.pos
	p.next()

	for p.pos < len(p.tokens) {
		if p.peek().kind == tokNewline && p.peekNext().kind == tokRawLine {
			p.next()
			rawTok := p.next()
			lines = append(lines, rawTok.text)
		} else {
			break
		}
	}
	return &rawNode{pos: firstPos, lines: lines}, true
}

func (p *parser) parseList(lbrackTok token) (*listNode, bool) {
	p.next()
	p.pushBracket(lbrackTok.pos, false)

	var elems []*scalarNode
	firstElemKind := -1
	reportedL204 := false
	expectedComma := false

	for p.pos < len(p.tokens) {
		tok := p.skipNewlines()

		if tok.kind == tokComment {
			p.next()
			p.syntaxError(tok.pos, "L002", "")
			continue
		}

		if tok.kind == tokRBrack {
			break
		}

		if tok.kind == tokEOF {
			p.reportInnermostL103()
			return nil, false
		}

		if expectedComma {
			if tok.kind == tokComma {
				p.next()
				expectedComma = false
				continue
			}
			if tok.kind != tokIllegal {
				p.syntaxError(tok.pos, "L106", "要素の間にはカンマを書く")
			}
			return nil, false
		}

		if tok.kind == tokLBrack {
			p.syntaxError(tok.pos, "L106", "リストの中にリストは書けない")
			return nil, false
		}
		if tok.kind == tokLBrace {
			p.syntaxError(tok.pos, "L106", "リストの中にレコードは書けない")
			return nil, false
		}
		if tok.kind == tokName {
			p.syntaxError(tok.pos, "L102", fmt.Sprintf("文字列なら %q、列挙子なら .%s と書く", tok.text, tok.text))
			return nil, false
		}
		if tok.kind == tokIllegal {
			return nil, false
		}

		sk := getScalarKind(tok.kind)
		if sk == -1 {
			p.syntaxError(tok.pos, "L106", "要素にはスカラー値を書く")
			return nil, false
		}

		if firstElemKind == -1 {
			firstElemKind = sk
		} else if sk != firstElemKind && !reportedL204 {
			p.errors = append(p.errors, parseNewError(p.file, tok.pos, "L204", ""))
			reportedL204 = true
		}

		sn := p.parseScalar(tok)
		p.next()
		elems = append(elems, sn)
		expectedComma = true
	}

	p.skipNewlines()
	if p.peek().kind == tokRBrack {
		rbrackTok := p.next()
		p.popBracket()
		oneLine := (lbrackTok.pos.Line == rbrackTok.pos.Line)
		return &listNode{
			lbrack:  lbrackTok.pos,
			rbrack:  rbrackTok.pos,
			elems:   elems,
			oneLine: oneLine,
		}, true
	}

	if p.peek().kind == tokEOF {
		p.reportInnermostL103()
		return nil, false
	}

	if p.peek().kind != tokIllegal {
		p.syntaxError(p.peek().pos, "L106", "] が必要")
	}
	return nil, false
}

func (p *parser) parseScalar(tok token) *scalarNode {
	var val Value
	switch tok.kind {
	case tokString:
		val = String(tok.text)
	case tokInt:
		n, _ := strconv.ParseInt(tok.text, 10, 64)
		val = Int(n)
	case tokDecimal:
		val = Decimal(normalizeDecimal(tok.text))
	case tokQuantity:
		num, unit := splitQuantity(tok.text)
		val = Quantity{Number: normalizeQuantityNumber(num), Unit: unit}
	case tokBool:
		val = Bool(tok.text == "true")
	case tokEnum:
		val = Enum(tok.text)
	case tokRef:
		val = Ref(tok.segs)
	}
	return &scalarNode{tok: tok, val: val}
}

func (p *parser) validateBlock(b *block) {
	if b == nil {
		return
	}
	seenKindsByKey := make(map[string]map[entryKind]bool)
	seenKeys := make(map[string]bool)
	seenLabelsByCollKey := make(map[string]map[string]bool)
	var prevElemKey string

	for _, e := range b.entries {
		reported := false

		if strings.HasPrefix(e.key, "$") {
			p.errors = append(p.errors, parseNewError(p.file, e.keyPos, "L205", ""))
			reported = true
		}

		if !reported {
			hasDiffKind := false
			for k := range seenKindsByKey[e.key] {
				if k != e.kind {
					hasDiffKind = true
					break
				}
			}
			if hasDiffKind {
				p.errors = append(p.errors, parseNewError(p.file, e.keyPos, "L202", ""))
				reported = true
			}
		}

		if !reported {
			if e.kind == entryAssign || e.kind == entryRecord {
				if seenKeys[e.key] {
					p.errors = append(p.errors, parseNewError(p.file, e.keyPos, "L201", ""))
					reported = true
				}
			} else if e.kind == entryElem {
				labels := seenLabelsByCollKey[e.key]
				if labels != nil && labels[e.label] {
					p.errors = append(p.errors, parseNewError(p.file, e.labelPos, "L201", ""))
					reported = true
				}
			}
		}

		if !reported {
			if e.kind == entryElem {
				if seenKeys[e.key] && prevElemKey != e.key {
					p.errors = append(p.errors, parseNewError(p.file, e.keyPos, "L203", ""))
					reported = true
				}
			}
		}

		if seenKindsByKey[e.key] == nil {
			seenKindsByKey[e.key] = make(map[entryKind]bool)
		}
		seenKindsByKey[e.key][e.kind] = true
		seenKeys[e.key] = true
		if e.kind == entryElem {
			if seenLabelsByCollKey[e.key] == nil {
				seenLabelsByCollKey[e.key] = make(map[string]bool)
			}
			seenLabelsByCollKey[e.key][e.label] = true
			prevElemKey = e.key
		} else {
			prevElemKey = ""
		}

		if e.body != nil {
			p.validateBlock(e.body)
		}
	}
}

func buildRecord(b *block) Record {
	if b == nil || len(b.entries) == 0 {
		return Record{}
	}
	var fields []Field
	collIndexByKey := make(map[string]int)

	for _, e := range b.entries {
		switch e.kind {
		case entryAssign:
			v := buildValue(e.value)
			fields = append(fields, Field{Key: e.key, Value: v})
		case entryRecord:
			sub := buildRecord(e.body)
			fields = append(fields, Field{Key: e.key, Value: sub})
		case entryElem:
			sub := buildRecord(e.body)
			elem := Elem{Label: e.label, Body: sub}
			if idx, ok := collIndexByKey[e.key]; ok {
				coll := fields[idx].Value.(Collection)
				coll.Elems = append(coll.Elems, elem)
				fields[idx].Value = coll
			} else {
				coll := Collection{Elems: []Elem{elem}}
				collIndexByKey[e.key] = len(fields)
				fields = append(fields, Field{Key: e.key, Value: coll})
			}
		}
	}
	return Record{Fields: fields}
}

func buildValue(n node) Value {
	if n == nil {
		return nil
	}
	switch v := n.(type) {
	case *scalarNode:
		return v.val
	case *rawNode:
		return String(strings.Join(v.lines, "\n"))
	case *listNode:
		if len(v.elems) == 0 {
			return List{}
		}
		list := make(List, len(v.elems))
		for i, el := range v.elems {
			list[i] = el.val
		}
		return list
	default:
		return nil
	}
}

func getScalarKind(k tokKind) int {
	switch k {
	case tokString:
		return 1
	case tokInt:
		return 2
	case tokDecimal:
		return 3
	case tokQuantity:
		return 4
	case tokBool:
		return 5
	case tokEnum:
		return 6
	case tokRef:
		return 7
	default:
		return -1
	}
}

func normalizeDecimal(s string) string {
	isNeg := false
	if strings.HasPrefix(s, "-") {
		isNeg = true
		s = s[1:]
	}
	parts := strings.Split(s, ".")
	intPart := parts[0]
	fracPart := parts[1]

	intPart = strings.TrimLeft(intPart, "0")
	if intPart == "" {
		intPart = "0"
	}

	fracPart = strings.TrimRight(fracPart, "0")
	if fracPart == "" {
		fracPart = "0"
	}

	if intPart == "0" && strings.Trim(fracPart, "0") == "" {
		return "0.0"
	}

	res := intPart + "." + fracPart
	if isNeg {
		res = "-" + res
	}
	return res
}

func normalizeQuantityNumber(s string) string {
	if strings.Contains(s, ".") {
		return normalizeDecimal(s)
	}
	isNeg := false
	if strings.HasPrefix(s, "-") {
		isNeg = true
		s = s[1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		s = "0"
	}
	if s == "0" {
		return "0"
	}
	if isNeg {
		return "-" + s
	}
	return s
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

func parseNewError(file string, pos Pos, code, hint string) *Error {
	msg := errorMessages[code]
	if hint == "" {
		hint = errorHints[code]
	}
	return &Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  msg,
		Hint: hint,
	}
}
