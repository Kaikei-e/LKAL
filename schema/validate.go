package schema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL"
)

// Validate validates a parsed *lkal.File against s, reporting all violations
// with source positions, paths, and hints (sorted and capped at 20 errors).
func (s *Schema) Validate(f *lkal.File) error {
	if s == nil || s.root == nil {
		return toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L601", "use a non-nil schema"),
		})
	}
	if f == nil {
		return toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L403", "use a non-nil file"),
		})
	}
	if _, ok := f.Locate(nil); !ok {
		return toErrorList([]*lkal.Error{
			newError(f.Name(), lkal.Pos{}, "", "L403", "use a parsed file"),
		})
	}
	v := &validator{
		file: f,
		name: f.Name(),
	}
	v.validateNode(s.root, f.Data(), nil, -1)
	return toErrorList(v.errs)
}

// ValidateValue structurally validates v per LKAL's value invariants (returning
// L403 on structural invalidity), then validates v against s without source
// positions (File: "", Pos: Pos{}, Path populated).
func (s *Schema) ValidateValue(v lkal.Value) error {
	if s == nil || s.root == nil {
		return toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L601", "use a non-nil schema"),
		})
	}
	if hint := validateStructuralValue(v, 0); hint != "" {
		return toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L403", hint),
		})
	}
	norm := normalizeAndClone(v)
	val := &validator{}
	val.validateNode(s.root, norm, nil, -1)
	return toErrorList(val.errs)
}

type listLocCache struct {
	loaded bool
	loc    lkal.Location
}

type validator struct {
	file      *lkal.File
	name      string
	errs      []*lkal.Error
	listCache *listLocCache
}

func (v *validator) locate(path []string) lkal.Location {
	if v.file == nil {
		return lkal.Location{}
	}
	loc, _ := v.file.Locate(path)
	return loc
}

func (v *validator) elemPos(path []string, idx int) lkal.Pos {
	var loc lkal.Location
	if v.listCache != nil {
		if !v.listCache.loaded {
			v.listCache.loc = v.locate(path)
			v.listCache.loaded = true
		}
		loc = v.listCache.loc
	} else {
		loc = v.locate(path)
	}
	if idx >= 0 && idx < len(loc.Elems) && loc.Elems[idx].Line > 0 {
		return loc.Elems[idx]
	}
	return posOrKey(loc)
}

// posOrKey is the value's source position, or the key position when the value
// was created by Set and has none. ADR 0012.
func posOrKey(loc lkal.Location) lkal.Pos {
	if loc.Value.Line > 0 {
		return loc.Value
	}
	return loc.Key
}

func (v *validator) addNodeErr(path []string, elemIdx int, code, hint string) {
	var pos lkal.Pos
	if elemIdx >= 0 {
		pos = v.elemPos(path, elemIdx)
		hint = fmt.Sprintf("element %d: %s", elemIdx+1, hint)
	} else {
		pos = posOrKey(v.locate(path))
	}
	v.errs = append(v.errs, newError(v.name, pos, formatPath(path), code, hint))
}

func (v *validator) addErr(pos lkal.Pos, path []string, code, hint string) {
	v.errs = append(v.errs, newError(v.name, pos, formatPath(path), code, hint))
}

func (v *validator) validateNode(spec *typeSpec, val lkal.Value, path []string, elemIdx int) {
	if spec == nil {
		return
	}
	spec = spec.deref()
	if spec == nil || spec.broken || spec.kindMask == 0 {
		return
	}
	if !spec.isUnion && spec.kind == kindAny {
		return
	}

	// Type check (L501).
	if !matchesSpecType(spec, val) {
		v.addNodeErr(path, elemIdx, "L501", fmt.Sprintf("expected %s, got %s", spec.expectedTypeNames(), actualTypeName(val)))
		return
	}

	switch tv := val.(type) {
	case lkal.String:
		v.validateString(spec, string(tv), path, elemIdx)
	case lkal.Int:
		v.validateInt(spec, int64(tv), path, elemIdx)
	case lkal.Decimal:
		v.validateDecimal(spec, string(tv), path, elemIdx)
	case lkal.Bool:
		// No constraints on .bool.
	case lkal.Enum:
		v.validateEnum(spec, string(tv), path, elemIdx)
	case lkal.Quantity:
		v.validateQuantity(spec, tv, path, elemIdx)
	case lkal.Ref:
		v.validateRef(spec, tv, path, elemIdx)
	case lkal.List:
		v.validateList(spec, tv, path)
	case lkal.Record:
		if spec.kind == kindMap {
			v.validateMap(spec, tv, path)
		} else {
			v.validateRecord(spec, tv, path)
		}
	}
}

func matchesSpecType(spec *typeSpec, val lkal.Value) bool {
	switch val.(type) {
	case lkal.String:
		return spec.kindMask&kindBit(kindString) != 0
	case lkal.Int:
		return spec.kindMask&kindBit(kindInt) != 0
	case lkal.Decimal:
		return spec.kindMask&kindBit(kindDecimal) != 0
	case lkal.Bool:
		return spec.kindMask&kindBit(kindBool) != 0
	case lkal.Enum:
		return spec.kindMask&kindBit(kindEnum) != 0
	case lkal.Quantity:
		return spec.kindMask&kindBit(kindQuantity) != 0
	case lkal.Ref:
		return spec.kindMask&kindBit(kindRef) != 0
	case lkal.List:
		return !spec.isUnion && spec.kind == kindList
	case lkal.Record:
		return !spec.isUnion && (spec.kind == kindRecord || spec.kind == kindMap)
	default:
		return false
	}
}

func actualTypeName(val lkal.Value) string {
	switch val.(type) {
	case lkal.String:
		return "string"
	case lkal.Int:
		return "int"
	case lkal.Decimal:
		return "decimal"
	case lkal.Bool:
		return "bool"
	case lkal.Enum:
		return "enum"
	case lkal.Quantity:
		return "quantity"
	case lkal.Ref:
		return "ref"
	case lkal.List:
		return "list"
	case lkal.Record:
		return "record"
	default:
		return "unknown"
	}
}

func (v *validator) validateString(spec *typeSpec, s string, path []string, elemIdx int) {
	runeLen := int64(utf8.RuneCountInString(s))
	if spec.hasMinLen && runeLen < spec.minLen {
		v.addNodeErr(path, elemIdx, "L504", fmt.Sprintf("length must be >= %d", spec.minLen))
	}
	if spec.hasMaxLen && runeLen > spec.maxLen {
		v.addNodeErr(path, elemIdx, "L504", fmt.Sprintf("length must be <= %d", spec.maxLen))
	}
	if spec.hasPat && spec.patRe != nil && !spec.patRe.MatchString(s) {
		v.addNodeErr(path, elemIdx, "L505", fmt.Sprintf("must match %s", spec.patSrc))
	}
	if len(spec.strOneOf) > 0 && !slices.Contains(spec.strOneOf, s) {
		v.addNodeErr(path, elemIdx, "L506", "use one of "+formatStringList(spec.strOneOf))
	}
}

func (v *validator) validateInt(spec *typeSpec, n int64, path []string, elemIdx int) {
	numStr := strconv.FormatInt(n, 10)
	if spec.hasMin && compareBase10(numStr, spec.minVal) < 0 {
		v.addNodeErr(path, elemIdx, "L504", "must be >= "+spec.minVal)
	}
	if spec.hasMax && compareBase10(numStr, spec.maxVal) > 0 {
		v.addNodeErr(path, elemIdx, "L504", "must be <= "+spec.maxVal)
	}
	if spec.hasNumericOneOf() && !spec.numericOneOfMatch(numStr) {
		v.addNodeErr(path, elemIdx, "L506", spec.numericOneOfHint())
	}
}

func (v *validator) validateDecimal(spec *typeSpec, dec string, path []string, elemIdx int) {
	if spec.hasMin && compareBase10(dec, spec.minVal) < 0 {
		v.addNodeErr(path, elemIdx, "L504", "must be >= "+spec.minVal)
	}
	if spec.hasMax && compareBase10(dec, spec.maxVal) > 0 {
		v.addNodeErr(path, elemIdx, "L504", "must be <= "+spec.maxVal)
	}
	if spec.hasNumericOneOf() && !spec.numericOneOfMatch(dec) {
		v.addNodeErr(path, elemIdx, "L506", spec.numericOneOfHint())
	}
}

func (v *validator) validateEnum(spec *typeSpec, e string, path []string, elemIdx int) {
	if len(spec.enumValues) > 0 && !spec.enumSet[e] {
		v.addNodeErr(path, elemIdx, "L506", "use one of "+formatEnumList(spec.enumValues))
	}
}

func (v *validator) validateQuantity(spec *typeSpec, q lkal.Quantity, path []string, elemIdx int) {
	if len(spec.units) > 0 && !spec.unitSet[q.Unit] {
		v.addNodeErr(path, elemIdx, "L507", "use one of "+strings.Join(spec.units, ", "))
	}
}

func (v *validator) validateRef(spec *typeSpec, r lkal.Ref, path []string, elemIdx int) {
	if len(spec.targets) > 0 {
		matched := false
		for _, target := range spec.targets {
			if len(r) >= len(target) && slices.Equal(r[:len(target)], target) {
				matched = true
				break
			}
		}
		if !matched {
			v.addNodeErr(path, elemIdx, "L508", "reference must start with "+formatRefList(spec.targets))
		}
	}
	n := int64(len(r))
	if spec.hasMinSegs && n < spec.minSegs {
		v.addNodeErr(path, elemIdx, "L504", formatCountHint("at least", spec.minSegs, "segment", "segments"))
	}
	if spec.hasMaxSegs && n > spec.maxSegs {
		v.addNodeErr(path, elemIdx, "L504", formatCountHint("at most", spec.maxSegs, "segment", "segments"))
	}
}

func (v *validator) validateList(spec *typeSpec, lst lkal.List, path []string) {
	prevCache := v.listCache
	v.listCache = &listLocCache{}
	defer func() { v.listCache = prevCache }()

	n := int64(len(lst))
	if spec.hasMinItems && n < spec.minItems {
		v.addNodeErr(path, -1, "L504", formatCountHint("at least", spec.minItems, "item", "items"))
	}
	if spec.hasMaxItems && n > spec.maxItems {
		v.addNodeErr(path, -1, "L504", formatCountHint("at most", spec.maxItems, "item", "items"))
	}
	if spec.items != nil {
		for i, el := range lst {
			v.validateNode(spec.items, el, path, i)
		}
	}
	if spec.unique && len(lst) > 1 {
		seen := make(map[string]int, len(lst))
		for i, el := range lst {
			k := valueEqualKey(el)
			if firstIdx, dup := seen[k]; dup {
				hint := fmt.Sprintf("element %d: duplicate of element %d (%s)", i+1, firstIdx+1, formatCanonicalValue(el))
				v.addErr(v.elemPos(path, i), path, "L509", hint)
			} else {
				seen[k] = i
			}
		}
	}
}

func (v *validator) validateRecord(spec *typeSpec, rec lkal.Record, path []string) {
	present := make(map[string]bool, len(rec.Fields))
	for _, f := range rec.Fields {
		present[f.Key] = true
		childPath := appendSeg(path, f.Key)
		if fs, declared := spec.fields[f.Key]; declared {
			v.validateNode(fs.spec, f.Value, childPath, -1)
			continue
		}
		if spec.reserved[f.Key] {
			v.addErr(v.locate(childPath).Key, childPath, "L503", fmt.Sprintf("field %s is reserved", quoteString(f.Key)))
		} else if !spec.open {
			v.addErr(v.locate(childPath).Key, childPath, "L503", unknownFieldHint(f.Key, spec.fieldOrder))
		}
	}
	var (
		parentKeyPos lkal.Pos
		parentLoced  bool
	)
	for _, name := range spec.fieldOrder {
		fs := spec.fields[name]
		if fs.required && !present[name] {
			if !parentLoced {
				parentKeyPos = v.locate(path).Key
				parentLoced = true
			}
			missingPath := appendSeg(path, name)
			v.addErr(parentKeyPos, missingPath, "L502", "add "+formatKey(name))
		}
	}
}

func (v *validator) validateMap(spec *typeSpec, rec lkal.Record, path []string) {
	n := int64(len(rec.Fields))
	if spec.hasMinEntries && n < spec.minEntries {
		v.addNodeErr(path, -1, "L504", formatCountHint("at least", spec.minEntries, "entry", "entries"))
	}
	if spec.hasMaxEntries && n > spec.maxEntries {
		v.addNodeErr(path, -1, "L504", formatCountHint("at most", spec.maxEntries, "entry", "entries"))
	}

	for _, entry := range rec.Fields {
		entryPath := appendSeg(path, entry.Key)
		if spec.mapKey != nil {
			v.validateMapKey(spec.mapKey, entry.Key, entryPath)
		}
		if spec.mapValue != nil {
			v.validateNode(spec.mapValue, entry.Value, entryPath, -1)
		}
	}

	if len(spec.uniqueBy) > 0 {
		mapName := "entry"
		if len(path) > 0 {
			mapName = formatKey(path[len(path)-1])
		}
		for _, fieldName := range spec.uniqueBy {
			seen := make(map[string]string, len(rec.Fields))
			for _, entry := range rec.Fields {
				entryRec, ok := entry.Value.(lkal.Record)
				if !ok {
					continue
				}
				fv, exists := findField(entryRec, fieldName)
				if !exists {
					continue
				}
				vk := valueEqualKey(fv)
				if firstKey, dup := seen[vk]; dup {
					dupPath := appendSeg(appendSeg(path, entry.Key), fieldName)
					hint := fmt.Sprintf("%s %s is also used by %s %s",
						formatKey(fieldName), formatCanonicalValue(fv), mapName, quoteString(firstKey))
					v.addErr(posOrKey(v.locate(dupPath)), dupPath, "L509", hint)
				} else {
					seen[vk] = entry.Key
				}
			}
		}
	}
}

func (v *validator) validateMapKey(ks *keySpec, key string, entryPath []string) {
	var (
		keyPos  lkal.Pos
		located bool
	)
	getKeyPos := func() lkal.Pos {
		if !located {
			keyPos = v.locate(entryPath).Key
			located = true
		}
		return keyPos
	}
	runeLen := int64(utf8.RuneCountInString(key))
	if ks.hasMinLen && runeLen < ks.minLen {
		v.addErr(getKeyPos(), entryPath, "L510", fmt.Sprintf("key length must be >= %d", ks.minLen))
	}
	if ks.hasMaxLen && runeLen > ks.maxLen {
		v.addErr(getKeyPos(), entryPath, "L510", fmt.Sprintf("key length must be <= %d", ks.maxLen))
	}
	if ks.hasPat && ks.patRe != nil && !ks.patRe.MatchString(key) {
		v.addErr(getKeyPos(), entryPath, "L510", fmt.Sprintf("key must match %s", ks.patSrc))
	}
	if len(ks.oneOf) > 0 && !slices.Contains(ks.oneOf, key) {
		v.addErr(getKeyPos(), entryPath, "L510", "use one of "+formatStringList(ks.oneOf))
	}
}

func (ks *keySpec) matches(key string) bool {
	if ks == nil {
		return true
	}
	runeLen := int64(utf8.RuneCountInString(key))
	if ks.hasMinLen && runeLen < ks.minLen {
		return false
	}
	if ks.hasMaxLen && runeLen > ks.maxLen {
		return false
	}
	if ks.hasPat && ks.patRe != nil && !ks.patRe.MatchString(key) {
		return false
	}
	if len(ks.oneOf) > 0 && !slices.Contains(ks.oneOf, key) {
		return false
	}
	return true
}

func formatCountHint(bound string, n int64, singular, plural string) string {
	unit := plural
	if n == 1 {
		unit = singular
	}
	return fmt.Sprintf("must have %s %d %s", bound, n, unit)
}

// valueEqualKey produces a collision-free, length-prefixed canonical key for
// any valid LKAL value such that valueEqualKey(a) == valueEqualKey(b) iff
// lkal.Equal(a, b).
func valueEqualKey(v lkal.Value) string {
	var b strings.Builder
	writeValueEqualKey(&b, v)
	return b.String()
}

func writeLenPrefixed(b *strings.Builder, s string) {
	b.WriteString(strconv.Itoa(len(s)))
	b.WriteByte(':')
	b.WriteString(s)
}

func writeValueEqualKey(b *strings.Builder, v lkal.Value) {
	switch val := v.(type) {
	case nil:
		b.WriteString("N;")
	case lkal.String:
		b.WriteByte('S')
		writeLenPrefixed(b, string(val))
	case lkal.Int:
		b.WriteByte('I')
		b.WriteString(strconv.FormatInt(int64(val), 10))
		b.WriteByte(';')
	case lkal.Decimal:
		b.WriteByte('D')
		writeLenPrefixed(b, string(val))
	case lkal.Bool:
		if val {
			b.WriteString("Bt")
		} else {
			b.WriteString("Bf")
		}
	case lkal.Enum:
		b.WriteByte('E')
		writeLenPrefixed(b, string(val))
	case lkal.Quantity:
		b.WriteByte('Q')
		writeLenPrefixed(b, val.Number)
		writeLenPrefixed(b, val.Unit)
	case lkal.Ref:
		b.WriteByte('R')
		b.WriteString(strconv.Itoa(len(val)))
		b.WriteByte(':')
		for _, seg := range val {
			writeLenPrefixed(b, seg)
		}
	case lkal.List:
		b.WriteByte('L')
		b.WriteString(strconv.Itoa(len(val)))
		b.WriteByte(':')
		for _, el := range val {
			writeValueEqualKey(b, el)
		}
	case lkal.Record:
		b.WriteByte('M')
		b.WriteString(strconv.Itoa(len(val.Fields)))
		b.WriteByte(':')
		if len(val.Fields) == 0 {
			return
		}
		idx := make([]int, len(val.Fields))
		for i := range idx {
			idx[i] = i
		}
		slices.SortFunc(idx, func(i, j int) int {
			return strings.Compare(val.Fields[i].Key, val.Fields[j].Key)
		})
		for _, i := range idx {
			writeLenPrefixed(b, val.Fields[i].Key)
			writeValueEqualKey(b, val.Fields[i].Value)
		}
	}
}

// compareBase10 compares two valid base-10 integer or decimal strings exactly,
// returning -1 if a < b, 0 if a == b, and +1 if a > b. Never converts to float64.
func compareBase10(a, b string) int {
	negA, intA, fracA := splitBase10(a)
	negB, intB, fracB := splitBase10(b)

	if negA != negB {
		if negA {
			return -1
		}
		return 1
	}

	var cmp int
	switch {
	case len(intA) < len(intB):
		cmp = -1
	case len(intA) > len(intB):
		cmp = 1
	case intA < intB:
		cmp = -1
	case intA > intB:
		cmp = 1
	default:
		cmp = strings.Compare(fracA, fracB)
	}

	if negA {
		return -cmp
	}
	return cmp
}

func (ts *typeSpec) hasNumericOneOf() bool {
	return len(ts.intOneOf) > 0 || len(ts.decOneOf) > 0
}

func (ts *typeSpec) numericOneOfMatch(numStr string) bool {
	for _, n := range ts.intOneOf {
		if compareBase10(numStr, strconv.FormatInt(n, 10)) == 0 {
			return true
		}
	}
	for _, d := range ts.decOneOf {
		if compareBase10(numStr, d) == 0 {
			return true
		}
	}
	return false
}

func (ts *typeSpec) numericOneOfHint() string {
	parts := make([]string, 0, len(ts.intOneOf)+len(ts.decOneOf))
	for _, n := range ts.intOneOf {
		parts = append(parts, strconv.FormatInt(n, 10))
	}
	parts = append(parts, ts.decOneOf...)
	return "use one of " + strings.Join(parts, ", ")
}

func splitBase10(s string) (neg bool, intPart, fracPart string) {
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	intPart, fracPart, _ = strings.Cut(s, ".")
	intPart = strings.TrimLeft(intPart, "0")
	fracPart = strings.TrimRight(fracPart, "0")
	if intPart == "" && fracPart == "" {
		neg = false
	}
	return neg, intPart, fracPart
}

// validateStructuralValue checks LKAL's value invariants (the same rules as Set).
// depth is accepted for call-site compatibility; ValueHint counts depth itself.
func validateStructuralValue(v lkal.Value, depth int) string {
	_ = depth
	return lkal.ValueHint(v)
}
