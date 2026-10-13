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
		return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(failSeg))
	}

	if parent.kind == parentCollection {
		for _, e := range parent.collBlock.entries {
			if e.key == parent.collKey && e.label == lastSeg {
				return buildRecord(e.body), nil
			}
		}
		return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
	}

	firstIdx, lastIdx := keyRange(parent.recordBlock, lastSeg)
	if firstIdx == -1 {
		return nil, pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
	}

	first := parent.recordBlock.entries[firstIdx]
	switch first.kind {
	case entryAssign:
		return buildValue(first.value), nil
	case entryRecord:
		return buildRecord(first.body), nil
	case entryElem:
		matching := parent.recordBlock.entries[firstIdx : lastIdx+1]
		elems := make([]Elem, len(matching))
		for i, e := range matching {
			elems[i] = Elem{Label: e.label, Body: buildRecord(e.body)}
		}
		return Collection{Elems: elems}, nil
	}
	return nil, nil
}

func (f *File) Set(path string, v Value) error {
	segs, ok := parseRefBody(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parent, failSeg, ok := resolveParent(f, segs[:len(segs)-1], lastSeg)
	if !ok {
		return pathError(f.name, "L402", "check the path at segment "+quoteString(failSeg))
	}

	if parent.kind == parentCollection {
		recVal, isRec := v.(Record)
		if !isRec {
			return pathError(f.name, "L403", "use a record for a collection element")
		}
		if hint := validateValue(recVal); hint != "" {
			return pathError(f.name, "L403", hint)
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
				kind:  entryElem,
				key:   parent.collKey,
				label: lastSeg,
				body:  blockFromRecord(vNorm),
			}
			parent.collBlock.entries = slices.Insert(parent.collBlock.entries, lastCollIdx+1, newElem)
		}
		return nil
	}

	if strings.HasPrefix(lastSeg, "$") {
		return pathError(f.name, "L403", "rename keys that start with $")
	}

	if hint := validateValue(v); hint != "" {
		return pathError(f.name, "L403", hint)
	}

	vNorm := normalizeValue(v)

	firstIdx, lastIdx := keyRange(parent.recordBlock, lastSeg)
	var oldEnt *entry
	if firstIdx != -1 {
		oldEnt = parent.recordBlock.entries[firstIdx]
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
		if oldEnt != nil && oldEnt.kind == entryRecord && oldEnt.body != nil &&
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
		parent.recordBlock.entries = slices.Replace(parent.recordBlock.entries, firstIdx, lastIdx+1, newEntries...)
	} else {
		if len(parent.recordBlock.entries) == 0 && len(parent.recordBlock.trailing) == 0 {
			parent.recordBlock.oneLine = false
		}
		parent.recordBlock.entries = append(parent.recordBlock.entries, newEntries...)
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
		return pathError(f.name, "L402", "check the path at segment "+quoteString(failSeg))
	}

	if parent.kind == parentCollection {
		for idx, e := range parent.collBlock.entries {
			if e.key == parent.collKey && e.label == lastSeg {
				removeEntries(parent.collBlock, idx, idx)
				return nil
			}
		}
		return pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
	}

	firstIdx, lastIdx := keyRange(parent.recordBlock, lastSeg)
	if firstIdx == -1 {
		return pathError(f.name, "L402", "check the path at segment "+quoteString(lastSeg))
	}
	removeEntries(parent.recordBlock, firstIdx, lastIdx)
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

	var val Value
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
					break
				}
				continue
			}
			break
		}
		val = Collection{Elems: elems}
	} else if tokens[pos].kind == tokLBrace {
		body, ok := p.parseBracedBlock()
		if ok {
			p.errors = append(p.errors, validate("", body)...)
			val = buildRecord(body)
		}
	} else {
		node, ok := p.parseAssignValue()
		if ok {
			p.errors = append(p.errors, validate("", &block{entries: []*entry{{value: node}}})...)
			val = buildValue(node)
		}
	}

	for p.peek().kind == tokNewline || p.peek().kind == tokComment {
		p.next()
	}
	if p.peek().kind != tokEOF && len(p.errors) == 0 {
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

func resolveParent(f *File, parentSegs []string, lastSeg string) (parentTarget, string, bool) {
	target := parentTarget{kind: parentRecord, recordBlock: f.body}
	for i := 0; i < len(parentSegs); i++ {
		seg := parentSegs[i]
		switch target.kind {
		case parentRecord:
			firstIdx, _ := keyRange(target.recordBlock, seg)
			if firstIdx == -1 {
				return parentTarget{}, seg, false
			}
			first := target.recordBlock.entries[firstIdx]
			switch first.kind {
			case entryAssign:
				if i < len(parentSegs)-1 {
					return parentTarget{}, parentSegs[i+1], false
				}
				return parentTarget{}, lastSeg, false
			case entryRecord:
				target = parentTarget{kind: parentRecord, recordBlock: first.body}
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
				return parentTarget{}, seg, false
			}
			target = parentTarget{kind: parentRecord, recordBlock: elemEnt.body}
		}
	}
	return target, "", true
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
		if len(s) == 0 || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) {
			return "write a decimal such as 0.25"
		}
		kind, errCode := lexValidateNumber(s)
		if errCode == "L005" {
			return "keep numbers within 100 digits"
		}
		if kind != tokDecimal || errCode != "" {
			return "write a decimal such as 0.25"
		}
	case Quantity:
		if len(val.Number) == 0 || (val.Number[0] != '-' && (val.Number[0] < '0' || val.Number[0] > '9')) {
			return "write a quantity such as 512Mi"
		}
		kind, errCode := lexValidateNumber(val.Number)
		if errCode == "L005" {
			return "keep numbers within 100 digits"
		}
		if (kind != tokInt && kind != tokDecimal) || errCode != "" {
			return "write a quantity such as 512Mi"
		}
		if len(val.Unit) == 0 {
			return "write a quantity such as 512Mi"
		}
		for i := 0; i < len(val.Unit); i++ {
			c := val.Unit[i]
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return "write a quantity such as 512Mi"
			}
		}
	case Bool:
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
			if el == nil {
				return "use a non-nil value"
			}
			switch el.(type) {
			case List, Record, Collection:
				return "use only scalars as list elements"
			}
			if hint := validateValue(el); hint != "" {
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
