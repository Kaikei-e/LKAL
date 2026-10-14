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

	cur := Value(f.Data())
	for _, seg := range segs {
		rec, isRec := cur.(Record)
		if !isRec {
			return nil, notFoundError(f.name, seg)
		}
		found := false
		for _, field := range rec.Fields {
			if field.Key == seg {
				cur = field.Value
				found = true
				break
			}
		}
		if !found {
			return nil, notFoundError(f.name, seg)
		}
	}
	return cur, nil
}

func (f *File) Set(path string, v Value) error {
	segs, ok := parseRefBody(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	targetBlock, remSegs, targetDepth, failSeg, ok := resolveTargetBlock(f.body, segs, 0)
	if !ok {
		return notFoundError(f.name, failSeg)
	}

	for _, seg := range remSegs {
		if strings.HasPrefix(seg, "$") {
			return pathError(f.name, "L403", "rename keys that start with $")
		}
	}

	entryDepth := targetDepth + len(remSegs) - 1
	if entryDepth >= maxBlockDepth {
		return pathError(f.name, "L403", errorHints["L107"])
	}
	if hint := validateValue(v, entryDepth+1); hint != "" {
		return pathError(f.name, "L403", hint)
	}

	vNorm := normalizeValue(v)
	exactIdx, prefixIdxs, _, lastPartialIdx := matchEntriesInBlock(targetBlock, remSegs)

	var oldEnt, layoutEnt *entry
	if exactIdx != -1 {
		oldEnt = targetBlock.entries[exactIdx]
		layoutEnt = oldEnt
	} else if len(prefixIdxs) > 0 {
		oldEnt = targetBlock.entries[prefixIdxs[0]]
	}

	newEnt := buildEntryFromPathAndValue(remSegs, vNorm, layoutEnt)

	switch {
	case exactIdx != -1:
		newEnt.comments = oldEnt.comments
		newEnt.blankBefore = oldEnt.blankBefore
		targetBlock.entries[exactIdx] = newEnt
	case len(prefixIdxs) > 0:
		firstIdx := prefixIdxs[0]
		newEnt.comments = oldEnt.comments
		newEnt.blankBefore = oldEnt.blankBefore
		targetBlock.entries[firstIdx] = newEnt
		for k := len(prefixIdxs) - 1; k >= 1; k-- {
			idx := prefixIdxs[k]
			removeEntries(targetBlock, idx, idx)
		}
	case lastPartialIdx != -1:
		targetBlock.entries = slices.Insert(targetBlock.entries, lastPartialIdx+1, newEnt)
	default:
		if len(targetBlock.entries) == 0 && len(targetBlock.trailing) == 0 {
			targetBlock.oneLine = false
		}
		targetBlock.entries = append(targetBlock.entries, newEnt)
	}
	return nil
}

func (f *File) Delete(path string) error {
	segs, ok := parseRefBody(path)
	if !ok {
		return pathError(f.name, "L401", "")
	}

	targetBlock, remSegs, _, failSeg, ok := resolveTargetBlock(f.body, segs, 0)
	if !ok {
		return notFoundError(f.name, failSeg)
	}

	exactIdx, prefixIdxs, maxPartialLen, _ := matchEntriesInBlock(targetBlock, remSegs)
	switch {
	case exactIdx != -1:
		removeEntries(targetBlock, exactIdx, exactIdx)
		return nil
	case len(prefixIdxs) > 0:
		for k := len(prefixIdxs) - 1; k >= 0; k-- {
			idx := prefixIdxs[k]
			removeEntries(targetBlock, idx, idx)
		}
		return nil
	default:
		return notFoundError(f.name, remSegs[maxPartialLen])
	}
}

func resolveTargetBlock(b *block, segs []string, depth int) (*block, []string, int, string, bool) {
	for {
		advanced := false
		for _, e := range b.entries {
			ep := entryKeyPath(e)
			if len(ep) < len(segs) && slices.Equal(ep, segs[:len(ep)]) {
				if e.kind == entryAssign {
					return nil, nil, 0, segs[len(ep)], false
				}
				b = e.body
				segs = segs[len(ep):]
				depth += len(ep)
				advanced = true
				break
			}
		}
		if !advanced {
			return b, segs, depth, "", true
		}
	}
}

func matchEntriesInBlock(b *block, remSegs []string) (exactIdx int, prefixIdxs []int, maxPartialLen int, lastPartialIdx int) {
	exactIdx = -1
	lastPartialIdx = -1
	n := len(remSegs)
	for i, e := range b.entries {
		ep := entryKeyPath(e)
		if slices.Equal(ep, remSegs) {
			exactIdx = i
			return exactIdx, nil, n, -1
		}
		if len(ep) > n && slices.Equal(ep[:n], remSegs) {
			prefixIdxs = append(prefixIdxs, i)
			continue
		}
		common := 0
		for common < len(ep) && common < n && ep[common] == remSegs[common] {
			common++
		}
		if common > 0 && common < len(ep) {
			if common > maxPartialLen {
				maxPartialLen = common
				lastPartialIdx = i
			} else if common == maxPartialLen {
				lastPartialIdx = i
			}
		}
	}
	return exactIdx, prefixIdxs, maxPartialLen, lastPartialIdx
}

func buildEntryFromPathAndValue(segs []string, vNorm Value, oldEnt *entry) *entry {
	var labels []labelSpan
	if len(segs) > 1 {
		labels = make([]labelSpan, len(segs)-1)
		for i, s := range segs[1:] {
			labels[i] = labelSpan{text: s}
		}
	}
	switch val := vNorm.(type) {
	case Record:
		b := blockFromRecord(val)
		if oldEnt != nil && oldEnt.kind == entryRecord &&
			(len(oldEnt.body.entries) > 0 || len(oldEnt.body.trailing) > 0) {
			b.oneLine = oldEnt.body.oneLine
		}
		return &entry{
			kind:   entryRecord,
			key:    segs[0],
			labels: labels,
			body:   b,
		}
	default:
		n := nodeFromValue(val)
		if oldEnt != nil && oldEnt.kind == entryAssign {
			if lnOld, ok := oldEnt.value.(*listNode); ok {
				if lnNew, ok := n.(*listNode); ok {
					lnNew.oneLine = lnOld.oneLine
				}
			}
		}
		return &entry{
			kind:   entryAssign,
			key:    segs[0],
			labels: labels,
			value:  n,
		}
	}
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
	if tokens[pos].kind == tokLBrace {
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

func scanNumber(s string) (int, bool, bool) {
	s = strings.TrimPrefix(s, "-")
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
		if (val.Number == "0" || val.Number == "-0") && isRadixPrefix(val.Unit[0]) {
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
			case List, Record:
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
	default:
		return val
	}
}
