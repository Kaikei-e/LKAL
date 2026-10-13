package lkal

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func ToJSON(f *File, plain bool) ([]byte, error) {
	if f == nil {
		return nil, errors.New("lkal: nil File")
	}
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf, jsontext.WithIndent("  "))
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return nil, err
	}
	if !plain {
		if err := enc.WriteToken(jsontext.String("$lkal")); err != nil {
			return nil, err
		}
		if err := enc.WriteToken(jsontext.String(f.Version())); err != nil {
			return nil, err
		}
	}
	for _, field := range f.Data().Fields {
		if err := enc.WriteToken(jsontext.String(field.Key)); err != nil {
			return nil, err
		}
		if err := writeJSONValue(enc, field.Value, plain); err != nil {
			return nil, err
		}
	}
	if err := enc.WriteToken(jsontext.EndObject); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeJSONValue(enc *jsontext.Encoder, v Value, plain bool) error {
	switch val := v.(type) {
	case String:
		return enc.WriteToken(jsontext.String(string(val)))
	case Int:
		return enc.WriteToken(jsontext.Int(int64(val)))
	case Decimal:
		return enc.WriteValue(jsontext.Value(val))
	case Bool:
		return enc.WriteToken(jsontext.Bool(bool(val)))
	case Enum:
		if plain {
			return enc.WriteToken(jsontext.String(string(val)))
		}
		return writeTag(enc, "$enum", string(val))
	case Quantity:
		payload := val.Number + val.Unit
		if plain {
			return enc.WriteToken(jsontext.String(payload))
		}
		return writeTag(enc, "$qty", payload)
	case Ref:
		payload := refBody(val)
		if plain {
			return enc.WriteToken(jsontext.String(payload))
		}
		return writeTag(enc, "$ref", payload)
	case List:
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		for _, el := range val {
			if err := writeJSONValue(enc, el, plain); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndArray)
	case Record:
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}
		for _, f := range val.Fields {
			if err := enc.WriteToken(jsontext.String(f.Key)); err != nil {
				return err
			}
			if err := writeJSONValue(enc, f.Value, plain); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndObject)
	case Collection:
		if !plain {
			if err := enc.WriteToken(jsontext.BeginObject); err != nil {
				return err
			}
			if err := enc.WriteToken(jsontext.String("$coll")); err != nil {
				return err
			}
		}
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}
		for _, el := range val.Elems {
			if err := enc.WriteToken(jsontext.String(el.Label)); err != nil {
				return err
			}
			if err := writeJSONValue(enc, el.Body, plain); err != nil {
				return err
			}
		}
		if err := enc.WriteToken(jsontext.EndObject); err != nil {
			return err
		}
		if !plain {
			return enc.WriteToken(jsontext.EndObject)
		}
	}
	return nil
}

func writeTag(enc *jsontext.Encoder, tag, payload string) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	if err := enc.WriteToken(jsontext.String(tag)); err != nil {
		return err
	}
	if err := enc.WriteToken(jsontext.String(payload)); err != nil {
		return err
	}
	return enc.WriteToken(jsontext.EndObject)
}

func nodeFromValue(v Value) node {
	if lst, ok := v.(List); ok {
		elems := make([]*scalarNode, len(lst))
		for i, el := range lst {
			elems[i] = &scalarNode{val: el}
		}
		return &listNode{elems: elems, oneLine: true}
	}
	return &scalarNode{val: v}
}

func blockFromRecord(r Record) *block {
	var entries []*entry
	for _, f := range r.Fields {
		switch val := f.Value.(type) {
		case Record:
			entries = append(entries, &entry{
				kind: entryRecord,
				key:  f.Key,
				body: blockFromRecord(val),
			})
		case Collection:
			for _, el := range val.Elems {
				entries = append(entries, &entry{
					kind:  entryElem,
					key:   f.Key,
					label: el.Label,
					body:  blockFromRecord(el.Body),
				})
			}
		default:
			entries = append(entries, &entry{
				kind:  entryAssign,
				key:   f.Key,
				value: nodeFromValue(val),
			})
		}
	}
	return &block{
		entries: entries,
		oneLine: false,
	}
}

func scalarType(v Value) string {
	switch v.(type) {
	case String:
		return "String"
	case Int:
		return "Int"
	case Decimal:
		return "Decimal"
	case Bool:
		return "Bool"
	case Enum:
		return "Enum"
	case Quantity:
		return "Quantity"
	case Ref:
		return "Ref"
	default:
		return ""
	}
}

func isValidEnumName(s string) bool {
	if len(s) == 0 || !lexIsNameStart(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !lexIsNameChar(s[i]) {
			return false
		}
	}
	return true
}

func parseRefBody(s string) (Ref, bool) {
	res := lexScanRef([]byte("&"+s), 0)
	if res.kind == tokRef && res.length == len(s)+1 && len(res.segs) > 0 {
		return Ref(res.segs), true
	}
	return nil, false
}

type rawError struct {
	offset int
	code   string
	hint   string
}

type jsonDecoder struct {
	src       []byte
	dec       *jsontext.Decoder
	errors    []rawError
	syntaxErr bool
}

func (p *jsonDecoder) recordError(offset int, code, hint string) {
	p.errors = append(p.errors, rawError{offset: offset, code: code, hint: hint})
}

func (p *jsonDecoder) nextTokenStart() int {
	off := int(p.dec.InputOffset())
	for off < len(p.src) {
		b := p.src[off]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == ',' || b == ':' {
			off++
			continue
		}
		break
	}
	return off
}

func tagHint(tag string) string {
	switch tag {
	case "$enum":
		return `write the enum name as a string, for example {"$enum": "tcp"}`
	case "$qty":
		return `write the quantity as a string, for example {"$qty": "512Mi"}`
	case "$ref":
		return `write the reference as a string, for example {"$ref": "secret/db-creds"}`
	default:
		return `write {"$coll": {"label": {...}}} with at least one element`
	}
}

func (p *jsonDecoder) handleSyntaxError(err error) {
	if p.syntaxErr {
		return
	}
	p.syntaxErr = true
	var synErr *jsontext.SyntacticError
	if errors.As(err, &synErr) {
		hint := ""
		if synErr.Err != nil && strings.Contains(synErr.Err.Error(), "depth") {
			hint = "nest objects and arrays at most 10000 levels deep"
		}
		p.recordError(int(synErr.ByteOffset), "L301", hint)
	} else if err == io.EOF || err == io.ErrUnexpectedEOF {
		p.recordError(len(p.src), "L301", "")
	}
}

func (p *jsonDecoder) readToken() (jsontext.Token, int, bool) {
	if p.syntaxErr {
		return jsontext.Token{}, 0, false
	}
	off := p.nextTokenStart()
	tok, err := p.dec.ReadToken()
	if err != nil {
		p.handleSyntaxError(err)
		return jsontext.Token{}, off, false
	}
	return tok, off, true
}

func (p *jsonDecoder) skipValue() bool {
	if p.syntaxErr {
		return false
	}
	if err := p.dec.SkipValue(); err != nil {
		p.handleSyntaxError(err)
		return false
	}
	return true
}

func (p *jsonDecoder) skipContainerRemainder(targetDepth int) {
	for !p.syntaxErr && p.dec.StackDepth() >= targetDepth {
		if _, _, ok := p.readToken(); !ok {
			return
		}
	}
}

func (p *jsonDecoder) checkTrailing() {
	if p.syntaxErr {
		return
	}
	for off := int(p.dec.InputOffset()); off < len(p.src); off++ {
		b := p.src[off]
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			p.handleSyntaxError(&jsontext.SyntacticError{ByteOffset: int64(off)})
			break
		}
	}
}

func (p *jsonDecoder) buildErrorList() *ErrorList {
	sort.SliceStable(p.errors, func(i, j int) bool {
		if p.errors[i].offset != p.errors[j].offset {
			return p.errors[i].offset < p.errors[j].offset
		}
		return p.errors[i].code < p.errors[j].code
	})

	n := len(p.errors)
	if n > 20 {
		n = 20
	}
	kept := p.errors[:n]

	res := make([]*Error, len(kept))
	cur := 0
	curLine := 1
	curCol := 1

	for idx, rErr := range kept {
		targetOff := rErr.offset
		if targetOff > len(p.src) {
			targetOff = len(p.src)
		}
		for cur < targetOff && cur < len(p.src) {
			if p.src[cur] == '\n' {
				curLine++
				curCol = 1
				cur++
			} else {
				_, sz := utf8.DecodeRune(p.src[cur:])
				if cur+sz > targetOff {
					sz = 1
				}
				curCol++
				cur += sz
			}
		}
		res[idx] = newError("", Pos{Line: curLine, Col: curCol}, rErr.code, rErr.hint)
	}

	return &ErrorList{Errors: res}
}

func FromJSON(src []byte) (*File, error) {
	dec := jsontext.NewDecoder(bytes.NewReader(src), jsontext.AllowDuplicateNames(true))
	p := &jsonDecoder{src: src, dec: dec}

	if len(bytes.Trim(src, " \t\r\n")) == 0 {
		p.recordError(len(src), "L301", "")
		return nil, p.buildErrorList()
	}

	kind := dec.PeekKind()
	if kind == jsontext.KindInvalid {
		p.readToken()
		return nil, p.buildErrorList()
	}

	topOff := p.nextTokenStart()
	if kind != '{' {
		if _, _, ok := p.readToken(); !ok {
			return nil, p.buildErrorList()
		}
		p.recordError(topOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		if kind == '[' {
			p.skipContainerRemainder(p.dec.StackDepth())
		}
		p.checkTrailing()
		return nil, p.buildErrorList()
	}

	if _, _, ok := p.readToken(); !ok {
		return nil, p.buildErrorList()
	}

	if dec.PeekKind() == '}' {
		closeOff := p.nextTokenStart()
		p.readToken()
		p.recordError(closeOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		p.checkTrailing()
		return nil, p.buildErrorList()
	}

	firstKeyTok, firstKeyOff, ok := p.readToken()
	if !ok {
		return nil, p.buildErrorList()
	}
	firstKey := firstKeyTok.String()

	seenKeys := make(map[string]bool)
	var topFields []Field

	if firstKey == "$lkal" {
		valOff := p.nextTokenStart()
		switch dec.PeekKind() {
		case 'n':
			if _, _, ok := p.readToken(); ok {
				p.recordError(valOff, "L105", `start the top-level object with "$lkal": "0.1"`)
			}
		case '"':
			verTok, verOff, ok := p.readToken()
			if !ok {
				return nil, p.buildErrorList()
			}
			if verTok.String() != "0.1" {
				p.recordError(verOff, "L207", "")
			}
		default:
			p.recordError(valOff, "L105", `start the top-level object with "$lkal": "0.1"`)
			if !p.skipValue() {
				return nil, p.buildErrorList()
			}
		}
	} else {
		p.recordError(firstKeyOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		hasKeyError := false
		if strings.HasPrefix(firstKey, "$") {
			hasKeyError = true
			p.recordError(firstKeyOff, "L205", "")
		}
		seenKeys[firstKey] = true
		if hasKeyError {
			if !p.skipValue() {
				return nil, p.buildErrorList()
			}
		} else {
			val, ok := p.parseValue()
			if ok {
				topFields = append(topFields, Field{Key: firstKey, Value: val})
			}
		}
	}

	if p.syntaxErr {
		return nil, p.buildErrorList()
	}

	for !p.syntaxErr {
		kind := dec.PeekKind()
		if kind == jsontext.KindInvalid {
			p.readToken()
			return nil, p.buildErrorList()
		}
		if kind == '}' {
			p.readToken()
			break
		}

		kTok, kOff, ok := p.readToken()
		if !ok {
			return nil, p.buildErrorList()
		}
		k := kTok.String()

		hasKeyError := false
		if k == "$lkal" {
			hasKeyError = true
			p.recordError(kOff, "L105", `start the top-level object with "$lkal": "0.1"`)
		} else if strings.HasPrefix(k, "$") {
			hasKeyError = true
			p.recordError(kOff, "L205", "")
		} else if seenKeys[k] {
			hasKeyError = true
			p.recordError(kOff, "L201", "")
		}
		seenKeys[k] = true

		if hasKeyError {
			if !p.skipValue() {
				return nil, p.buildErrorList()
			}
		} else {
			val, ok := p.parseValue()
			if ok {
				topFields = append(topFields, Field{Key: k, Value: val})
			}
		}
	}

	if p.syntaxErr {
		return nil, p.buildErrorList()
	}

	p.checkTrailing()

	if len(p.errors) > 0 {
		return nil, p.buildErrorList()
	}

	return &File{version: "0.1", body: blockFromRecord(Record{Fields: topFields})}, nil
}

func (p *jsonDecoder) parseValue() (Value, bool) {
	if p.syntaxErr {
		return nil, false
	}
	kind := p.dec.PeekKind()
	if kind == jsontext.KindInvalid {
		p.readToken()
		return nil, false
	}

	off := p.nextTokenStart()

	switch kind {
	case 'n':
		if _, _, ok := p.readToken(); ok {
			p.recordError(off, "L302", "")
		}
		return nil, false

	case 't', 'f':
		tok, _, ok := p.readToken()
		if !ok {
			return nil, false
		}
		return Bool(tok.Bool()), true

	case '"':
		tok, _, ok := p.readToken()
		if !ok {
			return nil, false
		}
		return String(tok.String()), true

	case '0':
		tok, _, ok := p.readToken()
		if !ok {
			return nil, false
		}
		raw := tok.String()
		if strings.ContainsAny(raw, "eE") {
			p.recordError(off, "L303", "")
			return nil, false
		}
		valKind, errCode := lexValidateNumber(raw)
		if errCode == "L005" {
			p.recordError(off, "L005", "")
			return nil, false
		}
		if valKind == tokDecimal {
			return Decimal(normalizeNumber(raw)), true
		}
		n, _ := strconv.ParseInt(raw, 10, 64)
		return Int(n), true

	case '[':
		p.readToken()
		return p.parseArray()

	case '{':
		p.readToken()
		return p.parseObject()

	default:
		p.readToken()
		return nil, false
	}
}

func (p *jsonDecoder) parseArray() (Value, bool) {
	if p.syntaxErr {
		return nil, false
	}
	if p.dec.PeekKind() == ']' {
		p.readToken()
		return List{}, true
	}

	var elements []Value
	var elemOffsets []int
	hadElementError := false

	for !p.syntaxErr {
		elemKind := p.dec.PeekKind()
		if elemKind == jsontext.KindInvalid {
			p.readToken()
			return nil, false
		}
		elemOff := p.nextTokenStart()

		if elemKind == '[' {
			p.recordError(elemOff, "L304", "")
			hadElementError = true
			if !p.skipValue() {
				return nil, false
			}
		} else if elemKind == '{' {
			braceOff := elemOff
			if _, _, ok := p.readToken(); !ok {
				return nil, false
			}
			recDepth := p.dec.StackDepth()
			if p.dec.PeekKind() == '}' {
				p.readToken()
				p.recordError(braceOff, "L304", "")
				hadElementError = true
			} else {
				firstKeyTok, _, ok := p.readToken()
				if !ok {
					return nil, false
				}
				firstKey := firstKeyTok.String()
				if firstKey == "$enum" || firstKey == "$qty" || firstKey == "$ref" {
					tagVal, ok := p.parseScalarTag(firstKey)
					if !ok {
						hadElementError = true
					} else {
						elements = append(elements, tagVal)
						elemOffsets = append(elemOffsets, braceOff)
					}
				} else {
					p.recordError(braceOff, "L304", "")
					hadElementError = true
					p.skipContainerRemainder(recDepth)
					if p.syntaxErr {
						return nil, false
					}
				}
			}
		} else {
			val, ok := p.parseValue()
			if !ok {
				hadElementError = true
			} else {
				elements = append(elements, val)
				elemOffsets = append(elemOffsets, elemOff)
			}
		}

		if p.syntaxErr {
			return nil, false
		}

		nextKind := p.dec.PeekKind()
		if nextKind == jsontext.KindInvalid {
			p.readToken()
			return nil, false
		}
		if nextKind == ']' {
			p.readToken()
			break
		}
	}

	if p.syntaxErr || hadElementError {
		return nil, false
	}

	if len(elements) > 1 {
		firstType := scalarType(elements[0])
		for i := 1; i < len(elements); i++ {
			if scalarType(elements[i]) != firstType {
				p.recordError(elemOffsets[i], "L204", "")
				return nil, false
			}
		}
	}

	return List(elements), true
}

func (p *jsonDecoder) parseObject() (Value, bool) {
	if p.syntaxErr {
		return nil, false
	}
	if p.dec.PeekKind() == '}' {
		p.readToken()
		return Record{}, true
	}

	firstKeyTok, firstKeyOff, ok := p.readToken()
	if !ok {
		return nil, false
	}
	firstKey := firstKeyTok.String()

	switch firstKey {
	case "$enum", "$qty", "$ref":
		return p.parseScalarTag(firstKey)
	case "$coll":
		return p.parseCollTag()
	default:
		return p.parseRecordMembers(firstKey, firstKeyOff)
	}
}

func (p *jsonDecoder) parseScalarTag(tag string) (Value, bool) {
	if p.syntaxErr {
		return nil, false
	}
	depth := p.dec.StackDepth()
	payloadKind := p.dec.PeekKind()
	if payloadKind == jsontext.KindInvalid {
		p.readToken()
		return nil, false
	}
	payloadOff := p.nextTokenStart()

	if payloadKind == 'n' {
		if _, _, ok := p.readToken(); ok {
			p.recordError(payloadOff, "L302", "")
		}
		p.skipContainerRemainder(depth)
		return nil, false
	}

	var val Value
	if payloadKind == '"' {
		tok, _, ok := p.readToken()
		if !ok {
			return nil, false
		}
		s := tok.String()
		switch tag {
		case "$enum":
			if isValidEnumName(s) {
				val = Enum(s)
			} else {
				p.recordError(payloadOff, "L305", tagHint(tag))
			}
		case "$qty":
			if len(s) == 0 || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) {
				p.recordError(payloadOff, "L305", tagHint(tag))
			} else {
				kind, errCode := lexValidateNumber(s)
				if kind == tokQuantity && errCode == "" {
					num, unit := splitQuantity(s)
					val = Quantity{Number: normalizeNumber(num), Unit: unit}
				} else if errCode == "L005" {
					p.recordError(payloadOff, "L005", "")
				} else {
					p.recordError(payloadOff, "L305", tagHint(tag))
				}
			}
		case "$ref":
			if ref, ok := parseRefBody(s); ok {
				val = ref
			} else {
				p.recordError(payloadOff, "L305", tagHint(tag))
			}
		}
	} else {
		p.recordError(payloadOff, "L305", tagHint(tag))
		p.skipContainerRemainder(depth)
		return nil, false
	}

	if val == nil {
		p.skipContainerRemainder(depth)
		return nil, false
	}

	if p.syntaxErr {
		return nil, false
	}

	nextKind := p.dec.PeekKind()
	if nextKind == jsontext.KindInvalid {
		p.readToken()
		return nil, false
	}
	if nextKind == '}' {
		p.readToken()
		return val, true
	}

	_, kOff, ok := p.readToken()
	if !ok {
		return nil, false
	}
	p.recordError(kOff, "L305", "write a tag object with only one key")
	p.skipContainerRemainder(depth)
	return nil, false
}

func (p *jsonDecoder) parseCollTag() (Value, bool) {
	if p.syntaxErr {
		return nil, false
	}
	depth := p.dec.StackDepth()
	payloadKind := p.dec.PeekKind()
	if payloadKind == jsontext.KindInvalid {
		p.readToken()
		return nil, false
	}
	payloadOff := p.nextTokenStart()

	if payloadKind == 'n' {
		if _, _, ok := p.readToken(); ok {
			p.recordError(payloadOff, "L302", "")
		}
		p.skipContainerRemainder(depth)
		return nil, false
	}

	collHint := tagHint("$coll")
	if payloadKind != '{' {
		p.recordError(payloadOff, "L305", collHint)
		p.skipContainerRemainder(depth)
		return nil, false
	}

	collObjOff := payloadOff
	if _, _, ok := p.readToken(); !ok {
		return nil, false
	}

	if p.dec.PeekKind() == '}' {
		p.readToken()
		p.recordError(collObjOff, "L305", collHint)
		p.skipContainerRemainder(depth)
		return nil, false
	}

	seenLabels := make(map[string]bool)
	var elems []Elem
	collHadError := false

	for !p.syntaxErr {
		kind := p.dec.PeekKind()
		if kind == jsontext.KindInvalid {
			p.readToken()
			return nil, false
		}
		if kind == '}' {
			p.readToken()
			break
		}

		labelTok, labelOff, ok := p.readToken()
		if !ok {
			return nil, false
		}
		label := labelTok.String()

		isDup := seenLabels[label]
		if isDup {
			p.recordError(labelOff, "L201", "")
			collHadError = true
			if !p.skipValue() {
				return nil, false
			}
			continue
		}
		seenLabels[label] = true

		valKind := p.dec.PeekKind()
		if valKind == jsontext.KindInvalid {
			p.readToken()
			return nil, false
		}
		valOff := p.nextTokenStart()

		if valKind == 'n' {
			if _, _, ok := p.readToken(); ok {
				p.recordError(valOff, "L302", "")
			}
			collHadError = true
			continue
		}

		if valKind != '{' {
			p.recordError(valOff, "L305", collHint)
			collHadError = true
			if !p.skipValue() {
				return nil, false
			}
			continue
		}

		recBraceOff := valOff
		if _, _, ok := p.readToken(); !ok {
			return nil, false
		}
		recDepth := p.dec.StackDepth()
		if p.dec.PeekKind() == '}' {
			p.readToken()
			if !collHadError {
				elems = append(elems, Elem{Label: label, Body: Record{}})
			}
			continue
		}

		firstKeyTok, firstKeyOff, ok := p.readToken()
		if !ok {
			return nil, false
		}
		firstKey := firstKeyTok.String()
		if firstKey == "$enum" || firstKey == "$qty" || firstKey == "$ref" || firstKey == "$coll" {
			p.recordError(recBraceOff, "L305", collHint)
			collHadError = true
			p.skipContainerRemainder(recDepth)
			if p.syntaxErr {
				return nil, false
			}
			continue
		}

		recVal, ok := p.parseRecordMembers(firstKey, firstKeyOff)
		if !ok {
			collHadError = true
		} else if !collHadError {
			elems = append(elems, Elem{Label: label, Body: recVal})
		}
	}

	if p.syntaxErr {
		return nil, false
	}

	nextKind := p.dec.PeekKind()
	if nextKind == jsontext.KindInvalid {
		p.readToken()
		return nil, false
	}
	if nextKind == '}' {
		p.readToken()
		if collHadError {
			return nil, false
		}
		return Collection{Elems: elems}, true
	}

	_, secondKeyOff, ok := p.readToken()
	if !ok {
		return nil, false
	}
	if !collHadError {
		p.recordError(secondKeyOff, "L305", "write a tag object with only one key")
	}
	p.skipContainerRemainder(depth)
	return nil, false
}

func (p *jsonDecoder) parseRecordMembers(firstKey string, firstKeyOff int) (Record, bool) {
	if p.syntaxErr {
		return Record{}, false
	}
	seenKeys := make(map[string]bool)
	var fields []Field
	recordHadError := false

	curKey := firstKey
	curKeyOff := firstKeyOff

	for !p.syntaxErr {
		hasKeyError := false
		if strings.HasPrefix(curKey, "$") {
			hasKeyError = true
			p.recordError(curKeyOff, "L205", "")
		} else if seenKeys[curKey] {
			hasKeyError = true
			p.recordError(curKeyOff, "L201", "")
		}
		seenKeys[curKey] = true

		if hasKeyError {
			recordHadError = true
			if !p.skipValue() {
				return Record{}, false
			}
		} else {
			val, ok := p.parseValue()
			if !ok {
				recordHadError = true
			} else {
				fields = append(fields, Field{Key: curKey, Value: val})
			}
		}

		if p.syntaxErr {
			return Record{}, false
		}

		nextKind := p.dec.PeekKind()
		if nextKind == jsontext.KindInvalid {
			p.readToken()
			return Record{}, false
		}
		if nextKind == '}' {
			p.readToken()
			break
		}

		nextKeyTok, nextKeyOff, ok := p.readToken()
		if !ok {
			return Record{}, false
		}
		curKey = nextKeyTok.String()
		curKeyOff = nextKeyOff
	}

	if p.syntaxErr || recordHadError {
		return Record{}, false
	}
	return Record{Fields: fields}, true
}
