package lkal

import (
	"slices"
	"strings"
	"unicode/utf8"
)

func (f *File) Get(path string) (Value, error) {
	segs, ok := parseRefBody(path)
	if !ok {
		return nil, pathError(f.name, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parent, failSeg, ok := resolveParent(f, segs[:len(segs)-1], lastSeg)
	if !ok {
		return nil, notFoundError(f.name, failSeg)
	}

	if parent.kind == parentCollection {
		for _, e := range parent.block.entries {
			if e.key == parent.collKey && e.label == lastSeg {
				return buildRecord(e.body), nil
			}
		}
		return nil, notFoundError(f.name, lastSeg)
	}

	firstIdx, lastIdx := keyRange(parent.block, lastSeg)
	if firstIdx == -1 {
		return nil, notFoundError(f.name, lastSeg)
	}

	first := parent.block.entries[firstIdx]
	switch first.kind {
	case entryAssign:
		return buildValue(first.value), nil
	case entryRecord:
		return buildRecord(first.body), nil
	default:
		matching := parent.block.entries[firstIdx : lastIdx+1]
		elems := make([]Elem, len(matching))
		for i, e := range matching {
			elems[i] = Elem{Label: e.label, Body: buildRecord(e.body)}
		}
		return Collection{Elems: elems}, nil
	}
}

func (f *File) Set(path string, v Value) error {
	segs, ok := parseRefBody(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parent, failSeg, ok := resolveParent(f, segs[:len(segs)-1], lastSeg)
	if !ok {
		return notFoundError(f.name, failSeg)
	}

	if parent.kind == parentCollection {
		recVal, isRec := v.(Record)
		if !isRec {
			return pathError(f.name, "L403", "use a record for a collection element")
		}
		if hint := validateValue(recVal, parent.depth+1); hint != "" {
			return pathError(f.name, "L403", hint)
		}
		vNorm := normalizeValue(recVal).(Record)

		var existingElem *entry
		lastCollIdx := -1
		for idx, e := range parent.block.entries {
			if e.key == parent.collKey {
				lastCollIdx = idx
				if e.label == lastSeg {
					existingElem = e
				}
			}
		}

		if existingElem != nil {
			oldBodyOneLine := false
			if len(existingElem.body.entries) > 0 || len(existingElem.body.trailing) > 0 {
				oldBodyOneLine = existingElem.body.oneLine
			}
			newBody := blockFromRecord(vNorm)
			newBody.oneLine = oldBodyOneLine
			existingElem.body = newBody
		} else {
			newElem := &entry{
				kind:  entryElem,
				key:   parent.collKey,
				label: lastSeg,
				body:  blockFromRecord(vNorm),
			}
			parent.block.entries = slices.Insert(parent.block.entries, lastCollIdx+1, newElem)
		}
		return nil
	}

	if strings.HasPrefix(lastSeg, "$") {
		return pathError(f.name, "L403", "rename keys that start with $")
	}

	if hint := validateValue(v, parent.depth+1); hint != "" {
		return pathError(f.name, "L403", hint)
	}

	vNorm := normalizeValue(v)

	firstIdx, lastIdx := keyRange(parent.block, lastSeg)
	var oldEnt *entry
	if firstIdx != -1 {
		oldEnt = parent.block.entries[firstIdx]
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
		if oldEnt != nil && oldEnt.kind == entryRecord &&
			(len(oldEnt.body.entries) > 0 || len(oldEnt.body.trailing) > 0) {
			b.oneLine = oldEnt.body.oneLine
		}
		newEntries = []*entry{{
			kind: entryRecord,
			key:  lastSeg,
			body: b,
		}}
	default:
		n := nodeFromValue(val)
		if oldEnt != nil && oldEnt.kind == entryAssign {
			if lnOld, ok := oldEnt.value.(*listNode); ok {
				if lnNew, ok := n.(*listNode); ok {
					lnNew.oneLine = lnOld.oneLine
				}
			}
		}
		newEntries = []*entry{{
			kind:  entryAssign,
			key:   lastSeg,
			value: n,
		}}
	}

	if oldEnt != nil {
		newEntries[0].comments = oldEnt.comments
		newEntries[0].blankBefore = oldEnt.blankBefore
		parent.block.entries = slices.Replace(parent.block.entries, firstIdx, lastIdx+1, newEntries...)
	} else {
		if len(parent.block.entries) == 0 && len(parent.block.trailing) == 0 {
			parent.block.oneLine = false
		}
		parent.block.entries = append(parent.block.entries, newEntries...)
	}
	return nil
}

func (f *File) Delete(path string) error {
	segs, ok := parseRefBody(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parent, failSeg, ok := resolveParent(f, segs[:len(segs)-1], lastSeg)
	if !ok {
		return notFoundError(f.name, failSeg)
	}

	if parent.kind == parentCollection {
		for idx, e := range parent.block.entries {
			if e.key == parent.collKey && e.label == lastSeg {
				removeEntries(parent.block, idx, idx)
				return nil
			}
		}
		return notFoundError(f.name, lastSeg)
	}

	firstIdx, lastIdx := keyRange(parent.block, lastSeg)
	if firstIdx == -1 {
		return notFoundError(f.name, lastSeg)
	}
	removeEntries(parent.block, firstIdx, lastIdx)
	return nil
}

func keyRange(b *block, key string) (int, int) {
	first := -1
	last := -1
	for i, e := range b.entries {
		if e.key == key {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	return first, last
}

func removeEntries(b *block, fromIdx, toIdx int) {
	first := b.entries[fromIdx]
	hadBlank := first.blankBefore
	if len(first.comments) > 0 {
		hadBlank = first.comments[0].blankBefore
	}
	if hadBlank {
		if toIdx+1 < len(b.entries) {
			next := b.entries[toIdx+1]
			if len(next.comments) > 0 {
				next.comments[0].blankBefore = true
			} else {
				next.blankBefore = true
			}
		} else if len(b.trailing) > 0 {
			b.trailing[0].blankBefore = true
		}
	}
	b.entries = slices.Delete(b.entries, fromIdx, toIdx+1)
}

func ParseValue(src []byte) (Value, error) {
	tokens, lexErrs := lex("", src)

	pos := 0
	for tokens[pos].kind == tokNewline || tokens[pos].kind == tokComment {
		pos++
	}

	if tokens[pos].kind == tokEOF {
		errs := append(lexErrs, newError("", tokens[pos].pos, "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines"))
		return nil, &ErrorList{Errors: sortErrors(errs)}
	}

	p := &parser{
		tokens: tokens,
		pos:    pos,
	}

	var (
		val Value
		ok  bool
	)
	if tokens[pos].kind == tokString && tokens[pos+1].kind == tokLBrace {
		var elems []Elem
		seenLabels := make(map[string]bool)
		ok = true

		for {
			labelTok := p.next()
			body, blockOK := p.parseBracedBlock()
			if !blockOK {
				ok = false
				break
			}
			if seenLabels[labelTok.text] {
				p.errors = append(p.errors, newError("", labelTok.pos, "L201", ""))
			}
			seenLabels[labelTok.text] = true
			p.errors = append(p.errors, validate("", body)...)
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
					ok = false
					break
				}
				continue
			}
			break
		}
		val = Collection{Elems: elems}
	} else if tokens[pos].kind == tokLBrace {
		var body *block
		body, ok = p.parseBracedBlock()
		if ok {
			p.errors = append(p.errors, validate("", body)...)
			val = buildRecord(body)
		}
	} else {
		var node node
		node, ok = p.parseAssignValue()
		if ok {
			p.errors = append(p.errors, validate("", &block{entries: []*entry{{value: node}}})...)
			val = buildValue(node)
		}
	}

	for p.peek().kind == tokNewline || p.peek().kind == tokComment {
		p.next()
	}
	hasSyntaxErr := len(lexErrs) > 0
	for _, err := range p.errors {
		if err.Code < "L201" || err.Code > "L205" {
			hasSyntaxErr = true
			break
		}
	}
	if ok && !hasSyntaxErr && p.peek().kind != tokEOF {
		p.syntaxError(p.peek(), "L106", "write a value: string, number, quantity, boolean, enum, reference, list or \\ lines")
	}

	allErrors := append(lexErrs, p.errors...)
	if len(allErrors) > 0 {
		return nil, &ErrorList{Errors: sortErrors(allErrors)}
	}
	return val, nil
}

func pathError(fileName, code, hint string) error {
	return &ErrorList{Errors: []*Error{newError(fileName, Pos{}, code, hint)}}
}

func notFoundError(file, seg string) error {
	return pathError(file, "L402", "check the path at segment "+quoteString(seg))
}

type parentType int

const (
	parentRecord parentType = iota
	parentCollection
)

type parentTarget struct {
	kind    parentType
	block   *block
	collKey string
	depth   int
}

func resolveParent(f *File, parentSegs []string, lastSeg string) (parentTarget, string, bool) {
	target := parentTarget{kind: parentRecord, block: f.body, depth: 0}
	for i, seg := range parentSegs {
		switch target.kind {
		case parentRecord:
			firstIdx, _ := keyRange(target.block, seg)
			if firstIdx == -1 {
				return parentTarget{}, seg, false
			}
			first := target.block.entries[firstIdx]
			switch first.kind {
			case entryAssign:
				if i < len(parentSegs)-1 {
					return parentTarget{}, parentSegs[i+1], false
				}
				return parentTarget{}, lastSeg, false
			case entryRecord:
				target = parentTarget{kind: parentRecord, block: first.body, depth: target.depth + 1}
			case entryElem:
				target = parentTarget{kind: parentCollection, block: target.block, collKey: seg, depth: target.depth}
			}
		case parentCollection:
			var elemEnt *entry
			for _, e := range target.block.entries {
				if e.key == target.collKey && e.label == seg {
					elemEnt = e
					break
				}
			}
			if elemEnt == nil {
				return parentTarget{}, seg, false
			}
			target = parentTarget{kind: parentRecord, block: elemEnt.body, depth: target.depth + 1}
		}
	}
	return target, "", true
}

func scanNumber(s string) (int, bool, bool) {
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}
	if len(s) == 0 {
		return 0, false, false
	}
	intDigits := 0
	if s[0] == '0' {
		intDigits = 1
		s = s[1:]
		if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			return 0, false, false
		}
	} else if s[0] >= '1' && s[0] <= '9' {
		for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			intDigits++
			s = s[1:]
		}
	} else {
		return 0, false, false
	}
	hasFrac := false
	fracDigits := 0
	if strings.HasPrefix(s, ".") {
		hasFrac = true
		s = s[1:]
		for len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
			fracDigits++
			s = s[1:]
		}
		if fracDigits == 0 {
			return 0, false, false
		}
	}
	if len(s) > 0 {
		return 0, false, false
	}
	return intDigits + fracDigits, hasFrac, true
}

func validateValue(v Value, depth int) string {
	if v == nil {
		return "use a non-nil value"
	}
	switch val := v.(type) {
	case String:
		if !utf8.ValidString(string(val)) {
			return "use valid UTF-8 text"
		}
	case Decimal:
		digits, hasFrac, ok := scanNumber(string(val))
		if !ok || !hasFrac {
			return "write a decimal such as 0.25"
		}
		if digits > 100 {
			return "keep numbers within 100 digits"
		}
	case Quantity:
		digits, _, ok := scanNumber(val.Number)
		if !ok || len(val.Unit) == 0 {
			return "write a quantity such as 512Mi"
		}
		for _, r := range val.Unit {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return "write a quantity such as 512Mi"
			}
		}
		if digits > 100 {
			return "keep numbers within 100 digits"
		}
	case Enum:
		s := string(val)
		if !utf8.ValidString(s) {
			return "use valid UTF-8 text"
		}
		if !isValidEnumName(s) {
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
			switch el.(type) {
			case List, Record, Collection:
				return "use only scalars as list elements"
			}
			if hint := validateValue(el, depth); hint != "" {
				return hint
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
	case Record:
		if depth > maxBlockDepth {
			return errorHints["L107"]
		}
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
		}
		for _, f := range val.Fields {
			if hint := validateValue(f.Value, depth+1); hint != "" {
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
		}
		for _, el := range val.Elems {
			if hint := validateValue(el.Body, depth); hint != "" {
				return hint
			}
		}
	}
	return ""
}

func normalizeValue(v Value) Value {
	switch val := v.(type) {
	case Decimal:
		return Decimal(normalizeNumber(string(val)))
	case Quantity:
		return Quantity{Number: normalizeNumber(val.Number), Unit: val.Unit}
	case Ref:
		return slices.Clone(val)
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
