package lkal

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	decimalRegex = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]+$`)
	intRegex     = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	unitRegex    = regexp.MustCompile(`^[A-Za-z]+$`)
	enumRegex    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

func (f *File) Get(path string) (Value, error) {
	segs, ok := parsePath(path)
	if !ok {
		return nil, pathError(f.name, "L401", "")
	}
	if f == nil || f.body == nil {
		return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(segs[0]))
	}

	currBlock := f.body
	for i := 0; i < len(segs); i++ {
		seg := segs[i]
		var matching []*entry
		for _, e := range currBlock.entries {
			if e.key == seg {
				matching = append(matching, e)
			}
		}
		if len(matching) == 0 {
			return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(seg))
		}

		first := matching[0]
		switch first.kind {
		case entryAssign:
			if i == len(segs)-1 {
				return buildValue(first.value), nil
			}
			return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(segs[i+1]))

		case entryRecord:
			b := first.body
			if b == nil {
				b = &block{}
			}
			if i == len(segs)-1 {
				return buildRecord(b), nil
			}
			currBlock = b

		case entryElem:
			if i == len(segs)-1 {
				coll := Collection{Elems: make([]Elem, len(matching))}
				for idx, e := range matching {
					b := e.body
					if b == nil {
						b = &block{}
					}
					coll.Elems[idx] = Elem{Label: e.label, Body: buildRecord(b)}
				}
				return coll, nil
			}
			nextSeg := segs[i+1]
			var elemEnt *entry
			for _, e := range matching {
				if e.label == nextSeg {
					elemEnt = e
					break
				}
			}
			if elemEnt == nil {
				return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(nextSeg))
			}
			i++
			b := elemEnt.body
			if b == nil {
				b = &block{}
			}
			if i == len(segs)-1 {
				return buildRecord(b), nil
			}
			currBlock = b
		}
	}

	return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(segs[len(segs)-1]))
}

func (f *File) Set(path string, v Value) error {
	segs, ok := parsePath(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	if hint := validateValue(v); hint != "" {
		return pathError(f.name, "L403", hint)
	}

	lastSeg := segs[len(segs)-1]
	parentSegs := segs[:len(segs)-1]
	parent, failSeg := resolveParent(f, parentSegs, lastSeg)
	if failSeg != "" {
		return pathError(f.name, "L402", "check the path at segment "+quoteString(failSeg))
	}

	if parent.kind == parentCollection {
		recVal, isRec := v.(Record)
		if !isRec {
			return pathError(f.name, "L403", "use a record for a collection element")
		}
		vNorm := normalizeValue(recVal).(Record)

		var existingElem *entry
		lastCollIdx := -1
		for idx, e := range parent.collBlock.entries {
			if e.key == parent.collKey {
				lastCollIdx = idx
				if e.label == lastSeg {
					existingElem = e
				}
			}
		}

		if existingElem != nil {
			oldBodyOneLine := false
			if existingElem.body != nil && (len(existingElem.body.entries) > 0 || len(existingElem.body.trailing) > 0) {
				oldBodyOneLine = existingElem.body.oneLine
			}
			newBody := blockFromRecord(vNorm)
			newBody.oneLine = oldBodyOneLine
			existingElem.body = newBody
		} else {
			newElem := &entry{
				kind:        entryElem,
				key:         parent.collKey,
				label:       lastSeg,
				body:        blockFromRecord(vNorm),
				blankBefore: false,
			}
			insertIdx := lastCollIdx + 1
			parent.collBlock.entries = append(parent.collBlock.entries[:insertIdx], append([]*entry{newElem}, parent.collBlock.entries[insertIdx:]...)...)
		}
		return nil
	}

	if strings.HasPrefix(lastSeg, "$") {
		return pathError(f.name, "L403", "rename keys that start with $")
	}

	vNorm := normalizeValue(v)

	firstIdx := -1
	lastIdx := -1
	for idx, e := range parent.recordBlock.entries {
		if e.key == lastSeg {
			if firstIdx == -1 {
				firstIdx = idx
			}
			lastIdx = idx
		}
	}

	var oldComments []comment
	oldBlankBefore := false
	wasRecord := false
	oldBodyOneLine := false
	wasList := false
	oldListOneLine := false

	if firstIdx != -1 {
		firstEnt := parent.recordBlock.entries[firstIdx]
		oldComments = firstEnt.comments
		oldBlankBefore = firstEnt.blankBefore
		if firstEnt.kind == entryRecord {
			wasRecord = true
			if firstEnt.body != nil && (len(firstEnt.body.entries) > 0 || len(firstEnt.body.trailing) > 0) {
				oldBodyOneLine = firstEnt.body.oneLine
			}
		}
		if firstEnt.kind == entryAssign {
			if ln, ok := firstEnt.value.(*listNode); ok {
				wasList = true
				oldListOneLine = ln.oneLine
			}
		}
	}

	var newEntries []*entry
	switch val := vNorm.(type) {
	case Collection:
		for _, elem := range val.Elems {
			newEntries = append(newEntries, &entry{
				kind:  entryElem,
				key:   lastSeg,
				label: elem.Label,
				body:  blockFromRecord(elem.Body),
			})
		}
	case Record:
		b := blockFromRecord(val)
		if wasRecord {
			b.oneLine = oldBodyOneLine
		}
		newEntries = append(newEntries, &entry{
			kind: entryRecord,
			key:  lastSeg,
			body: b,
		})
	default:
		n := nodeFromValue(val)
		if wasList {
			if ln, ok := n.(*listNode); ok {
				ln.oneLine = oldListOneLine
			}
		}
		newEntries = append(newEntries, &entry{
			kind:  entryAssign,
			key:   lastSeg,
			value: n,
		})
	}

	if firstIdx != -1 {
		newEntries[0].comments = oldComments
		newEntries[0].blankBefore = oldBlankBefore
		parent.recordBlock.entries = append(parent.recordBlock.entries[:firstIdx], append(newEntries, parent.recordBlock.entries[lastIdx+1:]...)...)
	} else {
		for _, e := range newEntries {
			e.blankBefore = false
		}
		parent.recordBlock.entries = append(parent.recordBlock.entries, newEntries...)
	}

	return nil
}

func (f *File) Delete(path string) error {
	segs, ok := parsePath(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parentSegs := segs[:len(segs)-1]
	parent, failSeg := resolveParent(f, parentSegs, lastSeg)
	if failSeg != "" {
		return pathError(f.name, "L402", "check the path at segment "+quoteString(failSeg))
	}

	if parent.kind == parentCollection {
		targetIdx := -1
		for idx, e := range parent.collBlock.entries {
			if e.key == parent.collKey && e.label == lastSeg {
				targetIdx = idx
				break
			}
		}
		if targetIdx == -1 {
			return pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
		}

		firstRemoved := parent.collBlock.entries[targetIdx]
		hadBlank := firstRemoved.blankBefore
		if len(firstRemoved.comments) > 0 {
			hadBlank = firstRemoved.comments[0].blankBefore
		}

		if hadBlank {
			nextIdx := targetIdx + 1
			if nextIdx < len(parent.collBlock.entries) {
				nextEnt := parent.collBlock.entries[nextIdx]
				if len(nextEnt.comments) > 0 {
					nextEnt.comments[0].blankBefore = true
				} else {
					nextEnt.blankBefore = true
				}
			} else if len(parent.collBlock.trailing) > 0 {
				parent.collBlock.trailing[0].blankBefore = true
			}
		}

		parent.collBlock.entries = append(parent.collBlock.entries[:targetIdx], parent.collBlock.entries[targetIdx+1:]...)
		return nil
	}

	firstIdx := -1
	lastIdx := -1
	for idx, e := range parent.recordBlock.entries {
		if e.key == lastSeg {
			if firstIdx == -1 {
				firstIdx = idx
			}
			lastIdx = idx
		}
	}
	if firstIdx == -1 {
		return pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
	}

	firstRemoved := parent.recordBlock.entries[firstIdx]
	hadBlank := firstRemoved.blankBefore
	if len(firstRemoved.comments) > 0 {
		hadBlank = firstRemoved.comments[0].blankBefore
	}

	if hadBlank {
		nextIdx := lastIdx + 1
		if nextIdx < len(parent.recordBlock.entries) {
			nextEnt := parent.recordBlock.entries[nextIdx]
			if len(nextEnt.comments) > 0 {
				nextEnt.comments[0].blankBefore = true
			} else {
				nextEnt.blankBefore = true
			}
		} else if len(parent.recordBlock.trailing) > 0 {
			parent.recordBlock.trailing[0].blankBefore = true
		}
	}

	parent.recordBlock.entries = append(parent.recordBlock.entries[:firstIdx], parent.recordBlock.entries[lastIdx+1:]...)
	return nil
}

func ParseValue(src []byte) (Value, error) {
	tokens, lexErrs := lex("", src)

	pos := 0
	for pos < len(tokens) && (tokens[pos].kind == tokNewline || tokens[pos].kind == tokComment) {
		pos++
	}

	if pos >= len(tokens) || tokens[pos].kind == tokEOF {
		eofPos := Pos{Line: 1, Col: 1}
		if pos < len(tokens) {
			eofPos = tokens[pos].pos
		}
		errs := append(lexErrs, newError("", eofPos, "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines"))
		return nil, &ErrorList{Errors: sortErrors(errs)}
	}

	p := &parser{
		file:   "",
		tokens: tokens,
		pos:    pos,
	}

	if tokens[pos].kind == tokString && pos+1 < len(tokens) && tokens[pos+1].kind == tokLBrace {
		var elems []Elem
		seenLabels := make(map[string]bool)

		for {
			labelTok := p.next()
			body, ok := p.parseBracedBlock()
			if !ok {
				break
			}
			if seenLabels[labelTok.text] {
				p.errors = append(p.errors, newError("", labelTok.pos, "L201", ""))
			}
			seenLabels[labelTok.text] = true
			valErrs := validate("", body)
			p.errors = append(p.errors, valErrs...)
			elems = append(elems, Elem{Label: labelTok.text, Body: buildRecord(body)})

			hasSep := false
			for p.peek().kind == tokNewline || p.peek().kind == tokComma || p.peek().kind == tokComment {
				if p.peek().kind == tokNewline || p.peek().kind == tokComma {
					hasSep = true
				}
				p.next()
			}

			if p.peek().kind == tokEOF {
				break
			}
			if p.peek().kind == tokString && p.peekNext().kind == tokLBrace {
				if !hasSep {
					p.syntaxError(p.peek(), "L106", "separate entries with a newline or a comma")
					break
				}
				continue
			}
			p.syntaxError(p.peek(), "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines")
			break
		}

		allErrors := append(lexErrs, p.errors...)
		if len(allErrors) > 0 {
			return nil, &ErrorList{Errors: sortErrors(allErrors)}
		}
		return Collection{Elems: elems}, nil
	}

	if tokens[pos].kind == tokLBrace {
		body, ok := p.parseBracedBlock()
		if ok {
			valErrs := validate("", body)
			p.errors = append(p.errors, valErrs...)
			for p.peek().kind == tokNewline || p.peek().kind == tokComment {
				p.next()
			}
			if p.peek().kind != tokEOF {
				p.syntaxError(p.peek(), "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines")
			}
		}

		allErrors := append(lexErrs, p.errors...)
		if len(allErrors) > 0 {
			return nil, &ErrorList{Errors: sortErrors(allErrors)}
		}
		return buildRecord(body), nil
	}

	node, ok := p.parseAssignValue()
	if ok {
		if lst, okLst := node.(*listNode); okLst && len(lst.elems) > 1 {
			firstKind := lst.elems[0].tok.kind
			for _, el := range lst.elems[1:] {
				if el.tok.kind != firstKind {
					p.errors = append(p.errors, newError("", el.tok.pos, "L204", ""))
					break
				}
			}
		}
		for p.peek().kind == tokNewline || p.peek().kind == tokComment {
			p.next()
		}
		if p.peek().kind != tokEOF {
			p.syntaxError(p.peek(), "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines")
		}
	}

	allErrors := append(lexErrs, p.errors...)
	if len(allErrors) > 0 {
		return nil, &ErrorList{Errors: sortErrors(allErrors)}
	}
	return buildValue(node), nil
}

func parsePath(path string) ([]string, bool) {
	if path == "" {
		return nil, false
	}
	src := []byte("&" + path)
	res := lexScanRef(src, 0)
	if res.errCode != "" || res.length != len(src) || len(res.segs) == 0 {
		return nil, false
	}
	return res.segs, true
}

func pathError(fileName, code, hint string) error {
	return &ErrorList{Errors: []*Error{newError(fileName, Pos{}, code, hint)}}
}

type parentType int

const (
	parentRecord parentType = iota
	parentCollection
)

type parentTarget struct {
	kind        parentType
	recordBlock *block
	collBlock   *block
	collKey     string
}

func resolveParent(f *File, parentSegs []string, lastSeg string) (parentTarget, string) {
	if f.body == nil {
		f.body = &block{}
	}
	if len(parentSegs) == 0 {
		return parentTarget{kind: parentRecord, recordBlock: f.body}, ""
	}

	target := parentTarget{kind: parentRecord, recordBlock: f.body}
	for i := 0; i < len(parentSegs); i++ {
		seg := parentSegs[i]
		switch target.kind {
		case parentRecord:
			var matching []*entry
			for _, e := range target.recordBlock.entries {
				if e.key == seg {
					matching = append(matching, e)
				}
			}
			if len(matching) == 0 {
				return parentTarget{}, seg
			}
			first := matching[0]
			switch first.kind {
			case entryAssign:
				if i < len(parentSegs)-1 {
					return parentTarget{}, parentSegs[i+1]
				}
				return parentTarget{}, lastSeg
			case entryRecord:
				b := first.body
				if b == nil {
					b = &block{}
				}
				target = parentTarget{kind: parentRecord, recordBlock: b}
			case entryElem:
				target = parentTarget{kind: parentCollection, collBlock: target.recordBlock, collKey: seg}
			}
		case parentCollection:
			var elemEnt *entry
			for _, e := range target.collBlock.entries {
				if e.key == target.collKey && e.label == seg {
					elemEnt = e
					break
				}
			}
			if elemEnt == nil {
				return parentTarget{}, seg
			}
			b := elemEnt.body
			if b == nil {
				b = &block{}
			}
			target = parentTarget{kind: parentRecord, recordBlock: b}
		}
	}

	return target, ""
}

func validateValue(v Value) string {
	if v == nil {
		return "use a non-nil value"
	}
	switch val := v.(type) {
	case String:
		if !utf8.ValidString(string(val)) {
			return "use valid UTF-8 text"
		}
	case Int:
	case Decimal:
		s := string(val)
		if !decimalRegex.MatchString(s) {
			return "write a decimal such as 0.25"
		}
		if countDigits(s) > 100 {
			return "keep numbers within 100 digits"
		}
	case Quantity:
		if !(intRegex.MatchString(val.Number) || decimalRegex.MatchString(val.Number)) || !unitRegex.MatchString(val.Unit) {
			return "write a quantity such as 512Mi"
		}
		if countDigits(val.Number) > 100 {
			return "keep numbers within 100 digits"
		}
	case Bool:
	case Enum:
		s := string(val)
		if !utf8.ValidString(s) {
			return "use valid UTF-8 text"
		}
		if !enumRegex.MatchString(s) {
			return "write an enum name that starts with a letter or _"
		}
	case Ref:
		if len(val) == 0 {
			return "write a reference with at least one segment"
		}
		for _, seg := range val {
			if !utf8.ValidString(seg) {
				return "use valid UTF-8 text"
			}
		}
	case List:
		for _, el := range val {
			if el == nil {
				return "use a non-nil value"
			}
		}
		for _, el := range val {
			switch el.(type) {
			case List, Record, Collection:
				return "use only scalars as list elements"
			}
		}
		if len(val) > 1 {
			t0 := scalarType(val[0])
			for _, el := range val[1:] {
				if scalarType(el) != t0 {
					return "use one type for all list elements"
				}
			}
		}
		for _, el := range val {
			if hint := validateValue(el); hint != "" {
				return hint
			}
		}
	case Record:
		seenKeys := make(map[string]bool)
		for _, f := range val.Fields {
			if !utf8.ValidString(f.Key) {
				return "use valid UTF-8 text"
			}
			if strings.HasPrefix(f.Key, "$") {
				return "rename keys that start with $"
			}
			if seenKeys[f.Key] {
				return "remove the duplicate key"
			}
			seenKeys[f.Key] = true
			if f.Value == nil {
				return "use a non-nil value"
			}
			if hint := validateValue(f.Value); hint != "" {
				return hint
			}
		}
	case Collection:
		if len(val.Elems) == 0 {
			return "write at least one collection element"
		}
		seenLabels := make(map[string]bool)
		for _, el := range val.Elems {
			if !utf8.ValidString(el.Label) {
				return "use valid UTF-8 text"
			}
			if seenLabels[el.Label] {
				return "remove the duplicate label"
			}
			seenLabels[el.Label] = true
			if hint := validateValue(el.Body); hint != "" {
				return hint
			}
		}
	}
	return ""
}

func countDigits(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			count++
		}
	}
	return count
}

func normalizeValue(v Value) Value {
	switch val := v.(type) {
	case Decimal:
		return Decimal(normalizeNumber(string(val)))
	case Quantity:
		return Quantity{Number: normalizeNumber(val.Number), Unit: val.Unit}
	case List:
		res := make(List, len(val))
		for i, el := range val {
			res[i] = normalizeValue(el)
		}
		return res
	case Record:
		fields := make([]Field, len(val.Fields))
		for i, f := range val.Fields {
			fields[i] = Field{Key: f.Key, Value: normalizeValue(f.Value)}
		}
		return Record{Fields: fields}
	case Collection:
		elems := make([]Elem, len(val.Elems))
		for i, el := range val.Elems {
			elems[i] = Elem{Label: el.Label, Body: normalizeValue(el.Body).(Record)}
		}
		return Collection{Elems: elems}
	default:
		return val
	}
}
