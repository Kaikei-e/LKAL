package schema

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/Kaikei-e/LKAL"
)

//go:embed meta.lkal
var metaBytes []byte

var metaSchema *Schema

func init() {
	f, err := lkal.Parse("meta.lkal", metaBytes)
	if err != nil {
		panic("schema: embedded meta.lkal failed to parse: " + err.Error())
	}
	s, err := compileFile(f, false)
	if err != nil {
		panic("schema: embedded meta.lkal failed to compile: " + err.Error())
	}
	metaSchema = s
}

type valueKind uint8

const (
	kindNone valueKind = iota
	kindString
	kindInt
	kindDecimal
	kindBool
	kindEnum
	kindQuantity
	kindRef
	kindList
	kindRecord
	kindMap
	kindAny
)

func kindBit(k valueKind) uint16 {
	return 1 << k
}

const scalarKindMask uint16 = (1 << kindString) |
	(1 << kindInt) |
	(1 << kindDecimal) |
	(1 << kindBool) |
	(1 << kindEnum) |
	(1 << kindQuantity) |
	(1 << kindRef)

var enumToKind = map[string]valueKind{
	"string":   kindString,
	"int":      kindInt,
	"decimal":  kindDecimal,
	"bool":     kindBool,
	"enum":     kindEnum,
	"quantity": kindQuantity,
	"ref":      kindRef,
	"list":     kindList,
	"record":   kindRecord,
	"map":      kindMap,
	"any":      kindAny,
}

var kindNames = map[valueKind]string{
	kindString:   "string",
	kindInt:      "int",
	kindDecimal:  "decimal",
	kindBool:     "bool",
	kindEnum:     "enum",
	kindQuantity: "quantity",
	kindRef:      "ref",
	kindList:     "list",
	kindRecord:   "record",
	kindMap:      "map",
	kindAny:      "any",
}

// Schema is a compiled, immutable LKAL v0.2 schema.
type Schema struct {
	id      string
	version string
	root    *typeSpec
	defs    map[string]*typeSpec
}

// ID returns the schema's optional top-level id, or "" if unset.
func (s *Schema) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// Version returns the schema's optional top-level version, or "" if unset.
func (s *Schema) Version() string {
	if s == nil {
		return ""
	}
	return s.version
}

type typeSpec struct {
	hasUse   bool
	useName  string
	resolved *typeSpec
	broken   bool

	isUnion  bool
	kind     valueKind
	kinds    []valueKind
	kindMask uint16

	// .string constraints
	hasMinLen bool
	minLen    int64
	hasMaxLen bool
	maxLen    int64
	hasPat    bool
	patSrc    string
	patRe     *regexp.Regexp
	strOneOf  []string

	// .int / .decimal constraints
	hasMin   bool
	minVal   string
	hasMax   bool
	maxVal   string
	intOneOf []int64
	decOneOf []string

	// .enum constraints
	enumValues []string
	enumSet    map[string]bool

	// .quantity constraints
	units   []string
	unitSet map[string]bool

	// .ref constraints
	targets    []lkal.Ref
	hasMinSegs bool
	minSegs    int64
	hasMaxSegs bool
	maxSegs    int64

	// .list constraints
	items       *typeSpec
	hasMinItems bool
	minItems    int64
	hasMaxItems bool
	maxItems    int64
	unique      bool

	// .record constraints
	fields     map[string]*fieldSpec
	fieldOrder []string
	open       bool
	reserved   map[string]bool

	// .map constraints
	mapValue      *typeSpec
	mapKey        *keySpec
	hasMinEntries bool
	minEntries    int64
	hasMaxEntries bool
	maxEntries    int64
	uniqueBy      []string
}

func (ts *typeSpec) deref() *typeSpec {
	for ts != nil && ts.resolved != nil {
		ts = ts.resolved
	}
	return ts
}

func (ts *typeSpec) expectedTypeNames() string {
	parts := make([]string, len(ts.kinds))
	for i, k := range ts.kinds {
		parts[i] = kindNames[k]
	}
	return joinOr(parts)
}

type fieldSpec struct {
	name       string
	spec       *typeSpec
	required   bool
	hasDefault bool
	defVal     lkal.Value
}

type keySpec struct {
	hasMinLen bool
	minLen    int64
	hasMaxLen bool
	maxLen    int64
	hasPat    bool
	patSrc    string
	patRe     *regexp.Regexp
	oneOf     []string
}

// Compile parses and compiles an LKAL v0.2 schema document.
func Compile(name string, src []byte) (*Schema, error) {
	f, err := lkal.Parse(name, src)
	if err != nil {
		return nil, err
	}
	return CompileFile(f)
}

// CompileFile compiles a parsed *lkal.File as an LKAL v0.2 schema.
func CompileFile(f *lkal.File) (*Schema, error) {
	if f == nil {
		return nil, toErrorList([]*lkal.Error{
			newError("", lkal.Pos{}, "", "L601", "provide a non-nil file"),
		})
	}
	if _, ok := f.Locate(nil); !ok {
		return nil, toErrorList([]*lkal.Error{
			newError(f.Name(), lkal.Pos{}, "", "L601", "provide a parsed file"),
		})
	}
	return compileFile(f, true)
}

type pendingUse struct {
	spec *typeSpec
	path []string // path to "use" field
}

type pendingListItems struct {
	listSpec  *typeSpec
	itemsPath []string // path to "items" field
}

type pendingUniqueBy struct {
	mapSpec  *typeSpec
	mapPath  []string
	elemIdxs []int
}

type pendingDefault struct {
	field *fieldSpec
	path  []string // path to "default" field
}

type compiler struct {
	file     *lkal.File
	name     string
	errs     []*lkal.Error
	defs     map[string]*typeSpec
	defOrder []string
	defUse   map[string][]string // def name -> path to its "use" field

	uses      []pendingUse
	listItems []pendingListItems
	uniqueBys []pendingUniqueBy
	defaults  []pendingDefault
}

func compileFile(f *lkal.File, useMeta bool) (*Schema, error) {
	data := f.Data()

	// Check lkal-schema version immediately before meta-validation so unknown
	// versions do not trigger spurious v0.2 keyword errors.
	if verVal, ok := findField(data, "lkal-schema"); ok {
		if verStr, isStr := verVal.(lkal.String); isStr && string(verStr) != "0.2" {
			loc, _ := f.Locate([]string{"lkal-schema"})
			return nil, toErrorList([]*lkal.Error{
				newError(f.Name(), loc.Value, "lkal-schema", "L607", `this implementation accepts only "0.2"`),
			})
		}
	}

	c := &compiler{
		file:   f,
		name:   f.Name(),
		defs:   make(map[string]*typeSpec),
		defUse: make(map[string][]string),
	}

	if useMeta {
		c.runMetaValidation(f)
	}

	var id, version string
	if v, ok := findField(data, "id"); ok {
		if s, isStr := v.(lkal.String); isStr {
			id = string(s)
		}
	}
	if v, ok := findField(data, "version"); ok {
		if s, isStr := v.(lkal.String); isStr {
			version = string(s)
		}
	}

	if defVal, ok := findField(data, "def"); ok {
		if defRec, isRec := defVal.(lkal.Record); isRec {
			for _, df := range defRec.Fields {
				defPath := []string{"def", df.Key}
				if specRec, isSpecRec := df.Value.(lkal.Record); isSpecRec {
					ts := c.compileTypeSpec(specRec, defPath, false)
					c.defs[df.Key] = ts
					c.defOrder = append(c.defOrder, df.Key)
					if ts.hasUse {
						c.defUse[df.Key] = appendSeg(defPath, "use")
					}
				}
			}
		}
	}

	var rootSpec *typeSpec
	if rootVal, ok := findField(data, "root"); ok {
		if rootRec, isRec := rootVal.(lkal.Record); isRec {
			rootSpec = c.compileRootSpec(rootRec, []string{"root"})
		}
	}

	// Pass 2: resolve `use` references and detect alias cycles.
	c.resolveDefsAndUses()

	// Pass 3: post-resolution checks (list.items, map.unique-by, field.default).
	c.checkListItems()
	c.checkUniqueBys()
	c.checkDefaults()

	if len(c.errs) > 0 {
		return nil, toErrorList(c.errs)
	}
	return &Schema{
		id:      id,
		version: version,
		root:    rootSpec,
		defs:    c.defs,
	}, nil
}

func (c *compiler) runMetaValidation(f *lkal.File) {
	v := &validator{file: f, name: f.Name()}
	v.validateNode(metaSchema.root, f.Data(), nil, -1)
	for _, mErr := range v.errs {
		code := "L601"
		hint := mErr.Hint
		if mErr.Code == "L504" {
			switch lastPathSegment(mErr.Path) {
			case "max-segments":
				code = "L606"
				hint = "max-segments must be >= 1"
			case "one-of":
				code = "L606"
				hint = "one-of must not be empty"
			case "units":
				code = "L606"
				hint = "units must not be empty"
			case "targets":
				code = "L606"
				hint = "targets must not be empty"
			case "unique-by":
				code = "L606"
				hint = "unique-by must not be empty"
			}
		}
		c.errs = append(c.errs, newError(mErr.File, mErr.Pos, mErr.Path, code, hint))
	}
}

func lastPathSegment(path string) string {
	segs, ok := parsePath(path)
	if !ok || len(segs) == 0 {
		return ""
	}
	return segs[len(segs)-1]
}

func (c *compiler) locate(path []string) lkal.Location {
	loc, _ := c.file.Locate(path)
	return loc
}

func (c *compiler) addErr(pos lkal.Pos, path []string, code, hint string) {
	c.errs = append(c.errs, newError(c.name, pos, formatPath(path), code, hint))
}

func (c *compiler) compileRootSpec(rec lkal.Record, path []string) *typeSpec {
	ts := &typeSpec{
		kind:     kindRecord,
		kinds:    []valueKind{kindRecord},
		kindMask: kindBit(kindRecord),
		fields:   make(map[string]*fieldSpec),
	}
	c.compileRecordConstraints(ts, rec, path)
	return ts
}

var useConflictKeys = map[string]bool{
	"type":         true,
	"types":        true,
	"min-length":   true,
	"max-length":   true,
	"pattern":      true,
	"one-of":       true,
	"min":          true,
	"max":          true,
	"values":       true,
	"units":        true,
	"targets":      true,
	"min-segments": true,
	"max-segments": true,
	"items":        true,
	"min-items":    true,
	"max-items":    true,
	"unique":       true,
	"field":        true,
	"open":         true,
	"reserved":     true,
	"value":        true,
	"key":          true,
	"min-entries":  true,
	"max-entries":  true,
	"unique-by":    true,
}

func (c *compiler) compileFieldSpec(name string, rec lkal.Record, path []string) *fieldSpec {
	fs := &fieldSpec{
		name: name,
	}
	if v, ok := findField(rec, "required"); ok {
		if b, isBool := v.(lkal.Bool); isBool {
			fs.required = bool(b)
		}
	}
	if defVal, ok := findField(rec, "default"); ok {
		fs.hasDefault = true
		fs.defVal = normalizeAndClone(defVal)
		if fs.required {
			defPath := appendSeg(path, "default")
			c.addErr(c.locate(defPath).Key, defPath, "L606", "cannot specify default on a required field")
		}
	}

	fs.spec = c.compileTypeSpec(rec, path, true)

	if fs.hasDefault {
		c.defaults = append(c.defaults, pendingDefault{
			field: fs,
			path:  appendSeg(path, "default"),
		})
	}
	return fs
}

func (c *compiler) compileTypeSpec(rec lkal.Record, path []string, isField bool) *typeSpec {
	ts := &typeSpec{}

	typeVal, hasType := findField(rec, "type")
	typesVal, hasTypes := findField(rec, "types")
	useVal, hasUse := findField(rec, "use")

	if !hasType && !hasTypes && !hasUse {
		loc := c.locate(path)
		pos := loc.Value
		if pos.Line == 0 {
			pos = loc.Key
		}
		c.addErr(pos, path, "L601", "specify one of type, types or use")
		ts.broken = true
		return ts
	}

	if hasUse {
		for _, f := range rec.Fields {
			if useConflictKeys[f.Key] {
				kp := appendSeg(path, f.Key)
				c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("cannot combine %s with use", formatKey(f.Key)))
			}
		}
		if ref, ok := useVal.(lkal.Ref); ok && len(ref) == 2 && ref[0] == "def" {
			ts.hasUse = true
			ts.useName = ref[1]
			c.uses = append(c.uses, pendingUse{
				spec: ts,
				path: appendSeg(path, "use"),
			})
		} else {
			ts.broken = true
		}
		return ts
	}

	if hasType && hasTypes {
		tp := appendSeg(path, "types")
		c.addErr(c.locate(tp).Key, tp, "L606", "cannot specify both type and types")
		ts.broken = true
		return ts
	}

	if hasType {
		e, ok := typeVal.(lkal.Enum)
		if !ok {
			ts.broken = true
			return ts
		}
		k, known := enumToKind[string(e)]
		if !known {
			ts.broken = true
			return ts
		}
		ts.kind = k
		ts.kinds = []valueKind{k}
		ts.kindMask = kindBit(k)
	} else {
		lst, ok := typesVal.(lkal.List)
		if !ok {
			ts.broken = true
			return ts
		}
		ts.isUnion = true
		seen := uint16(0)
		for _, el := range lst {
			e, isEnum := el.(lkal.Enum)
			if !isEnum {
				continue
			}
			k, known := enumToKind[string(e)]
			if !known || (kindBit(k)&scalarKindMask) == 0 {
				continue
			}
			if seen&kindBit(k) == 0 {
				seen |= kindBit(k)
				ts.kinds = append(ts.kinds, k)
			}
		}
		ts.kindMask = seen
		if len(ts.kinds) == 0 {
			ts.broken = true
			return ts
		}
	}

	c.compileConstraints(ts, rec, path, isField)
	return ts
}

func (c *compiler) compileConstraints(ts *typeSpec, rec lkal.Record, path []string, isField bool) {
	_ = isField
	hasString := ts.kindMask&kindBit(kindString) != 0
	hasInt := ts.kindMask&kindBit(kindInt) != 0
	hasDecimal := ts.kindMask&kindBit(kindDecimal) != 0
	hasEnum := ts.kindMask&kindBit(kindEnum) != 0
	hasQuantity := ts.kindMask&kindBit(kindQuantity) != 0
	hasRef := ts.kindMask&kindBit(kindRef) != 0
	isList := !ts.isUnion && ts.kind == kindList
	isRecord := !ts.isUnion && ts.kind == kindRecord
	isMap := !ts.isUnion && ts.kind == kindMap

	hasValues := false
	hasItems := false
	hasMapValue := false

	for _, f := range rec.Fields {
		kp := appendSeg(path, f.Key)
		switch f.Key {
		case "doc", "type", "types", "use", "required", "default", "deprecated", "ext":
			// Handled elsewhere or metadata.

		// .string constraints
		case "min-length":
			if !hasString {
				c.addErr(c.locate(kp).Key, kp, "L601", "min-length is only allowed for .string")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMinLen = true
				ts.minLen = int64(n)
			}
		case "max-length":
			if !hasString {
				c.addErr(c.locate(kp).Key, kp, "L601", "max-length is only allowed for .string")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMaxLen = true
				ts.maxLen = int64(n)
			}
		case "pattern":
			if !hasString {
				c.addErr(c.locate(kp).Key, kp, "L601", "pattern is only allowed for .string")
				continue
			}
			if s, ok := f.Value.(lkal.String); ok {
				ts.hasPat = true
				ts.patSrc = string(s)
				if re, ok := c.compilePattern(string(s), kp); ok {
					ts.patRe = re
				}
			}

		// .int / .decimal constraints
		case "min":
			if !hasInt && !hasDecimal {
				c.addErr(c.locate(kp).Key, kp, "L601", "min is only allowed for .int or .decimal")
				continue
			}
			switch v := f.Value.(type) {
			case lkal.Int:
				ts.hasMin = true
				ts.minVal = strconv.FormatInt(int64(v), 10)
			case lkal.Decimal:
				if !hasDecimal {
					c.addErr(c.locate(kp).Value, kp, "L601", "min for .int must be an int")
				} else {
					ts.hasMin = true
					ts.minVal = normalizeNumber(string(v))
				}
			}
		case "max":
			if !hasInt && !hasDecimal {
				c.addErr(c.locate(kp).Key, kp, "L601", "max is only allowed for .int or .decimal")
				continue
			}
			switch v := f.Value.(type) {
			case lkal.Int:
				ts.hasMax = true
				ts.maxVal = strconv.FormatInt(int64(v), 10)
			case lkal.Decimal:
				if !hasDecimal {
					c.addErr(c.locate(kp).Value, kp, "L601", "max for .int must be an int")
				} else {
					ts.hasMax = true
					ts.maxVal = normalizeNumber(string(v))
				}
			}

		// shared one-of (.string, .int, .decimal)
		case "one-of":
			if !hasString && !hasInt && !hasDecimal {
				c.addErr(c.locate(kp).Key, kp, "L601", "one-of is only allowed for .string, .int or .decimal")
				continue
			}
			lst, ok := f.Value.(lkal.List)
			if !ok || len(lst) == 0 {
				continue
			}
			switch lst[0].(type) {
			case lkal.String:
				if !hasString {
					c.addErr(c.locate(kp).Value, kp, "L601", "one-of elements must match "+ts.expectedTypeNames())
					continue
				}
				vals := make([]string, 0, len(lst))
				for _, el := range lst {
					if s, ok := el.(lkal.String); ok {
						vals = append(vals, string(s))
					}
				}
				ts.strOneOf = vals
			case lkal.Int:
				if !hasInt {
					c.addErr(c.locate(kp).Value, kp, "L601", "one-of elements must match "+ts.expectedTypeNames())
					continue
				}
				vals := make([]int64, 0, len(lst))
				for _, el := range lst {
					if n, ok := el.(lkal.Int); ok {
						vals = append(vals, int64(n))
					}
				}
				ts.intOneOf = vals
			case lkal.Decimal:
				if !hasDecimal {
					c.addErr(c.locate(kp).Value, kp, "L601", "one-of elements must match "+ts.expectedTypeNames())
					continue
				}
				vals := make([]string, 0, len(lst))
				for _, el := range lst {
					if d, ok := el.(lkal.Decimal); ok {
						vals = append(vals, normalizeNumber(string(d)))
					}
				}
				ts.decOneOf = vals
			}

		// .enum constraints
		case "values":
			hasValues = true
			if !hasEnum {
				c.addErr(c.locate(kp).Key, kp, "L601", "values is only allowed for .enum")
				continue
			}
			lst, ok := f.Value.(lkal.List)
			if !ok {
				c.addErr(c.locate(kp).Value, kp, "L601", "values must be a list of enums")
				continue
			}
			if len(lst) == 0 {
				c.addErr(c.locate(kp).Value, kp, "L606", "values must not be empty")
				continue
			}
			vals := make([]string, 0, len(lst))
			set := make(map[string]bool, len(lst))
			allValid := true
			var (
				valLoc    lkal.Location
				valLocSet bool
			)
			for i, el := range lst {
				e, isEnum := el.(lkal.Enum)
				if !isEnum {
					allValid = false
					if !valLocSet {
						valLoc = c.locate(kp)
						valLocSet = true
					}
					c.addErr(locElemPos(valLoc, i), kp, "L601", fmt.Sprintf("element %d: expected enum, got %s", i+1, actualTypeName(el)))
					continue
				}
				name := string(e)
				if !set[name] {
					set[name] = true
					vals = append(vals, name)
				}
			}
			if allValid {
				ts.enumValues = vals
				ts.enumSet = set
			}

		// .quantity constraints
		case "units":
			if !hasQuantity {
				c.addErr(c.locate(kp).Key, kp, "L601", "units is only allowed for .quantity")
				continue
			}
			if lst, ok := f.Value.(lkal.List); ok && len(lst) > 0 {
				units := make([]string, 0, len(lst))
				set := make(map[string]bool, len(lst))
				for _, el := range lst {
					if s, ok := el.(lkal.String); ok {
						u := string(s)
						if !set[u] {
							set[u] = true
							units = append(units, u)
						}
					}
				}
				ts.units = units
				ts.unitSet = set
			}

		// .ref constraints
		case "targets":
			if !hasRef {
				c.addErr(c.locate(kp).Key, kp, "L601", "targets is only allowed for .ref")
				continue
			}
			if lst, ok := f.Value.(lkal.List); ok && len(lst) > 0 {
				targets := make([]lkal.Ref, 0, len(lst))
				for _, el := range lst {
					if r, ok := el.(lkal.Ref); ok {
						targets = append(targets, slices.Clone(r))
					}
				}
				ts.targets = targets
			}
		case "min-segments":
			if !hasRef {
				c.addErr(c.locate(kp).Key, kp, "L601", "min-segments is only allowed for .ref")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMinSegs = true
				ts.minSegs = int64(n)
			}
		case "max-segments":
			if !hasRef {
				c.addErr(c.locate(kp).Key, kp, "L601", "max-segments is only allowed for .ref")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 1 {
				ts.hasMaxSegs = true
				ts.maxSegs = int64(n)
			}

		// .list constraints
		case "items":
			hasItems = true
			if !isList {
				c.addErr(c.locate(kp).Key, kp, "L601", "items is only allowed for .list")
				continue
			}
			if itemRec, ok := f.Value.(lkal.Record); ok {
				ts.items = c.compileTypeSpec(itemRec, kp, false)
				c.listItems = append(c.listItems, pendingListItems{
					listSpec:  ts,
					itemsPath: kp,
				})
			}
		case "min-items":
			if !isList {
				c.addErr(c.locate(kp).Key, kp, "L601", "min-items is only allowed for .list")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMinItems = true
				ts.minItems = int64(n)
			}
		case "max-items":
			if !isList {
				c.addErr(c.locate(kp).Key, kp, "L601", "max-items is only allowed for .list")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMaxItems = true
				ts.maxItems = int64(n)
			}
		case "unique":
			if !isList {
				c.addErr(c.locate(kp).Key, kp, "L601", "unique is only allowed for .list")
				continue
			}
			if b, ok := f.Value.(lkal.Bool); ok {
				ts.unique = bool(b)
			}

		// .record constraints
		case "field", "open", "reserved":
			if !isRecord {
				c.addErr(c.locate(kp).Key, kp, "L601", fmt.Sprintf("%s is only allowed for .record", f.Key))
			}

		// .map constraints
		case "value":
			hasMapValue = true
			if !isMap {
				c.addErr(c.locate(kp).Key, kp, "L601", "value is only allowed for .map")
				continue
			}
			if valRec, ok := f.Value.(lkal.Record); ok {
				ts.mapValue = c.compileTypeSpec(valRec, kp, false)
			}
		case "key":
			if !isMap {
				c.addErr(c.locate(kp).Key, kp, "L601", "key is only allowed for .map")
				continue
			}
			if keyRec, ok := f.Value.(lkal.Record); ok {
				ts.mapKey = c.compileKeySpec(keyRec, kp)
			}
		case "min-entries":
			if !isMap {
				c.addErr(c.locate(kp).Key, kp, "L601", "min-entries is only allowed for .map")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMinEntries = true
				ts.minEntries = int64(n)
			}
		case "max-entries":
			if !isMap {
				c.addErr(c.locate(kp).Key, kp, "L601", "max-entries is only allowed for .map")
				continue
			}
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ts.hasMaxEntries = true
				ts.maxEntries = int64(n)
			}
		case "unique-by":
			if !isMap {
				c.addErr(c.locate(kp).Key, kp, "L601", "unique-by is only allowed for .map")
				continue
			}
			if lst, ok := f.Value.(lkal.List); ok && len(lst) > 0 {
				ub := make([]string, 0, len(lst))
				idxs := make([]int, 0, len(lst))
				seen := make(map[string]bool, len(lst))
				for i, el := range lst {
					if s, ok := el.(lkal.String); ok {
						name := string(s)
						if !seen[name] {
							seen[name] = true
							ub = append(ub, name)
							idxs = append(idxs, i)
						}
					}
				}
				ts.uniqueBy = ub
				c.uniqueBys = append(c.uniqueBys, pendingUniqueBy{
					mapSpec:  ts,
					mapPath:  path,
					elemIdxs: idxs,
				})
			}
		}
	}

	if isRecord {
		ts.fields = make(map[string]*fieldSpec)
		c.compileRecordConstraints(ts, rec, path)
	}

	// Required constraint checks.
	if hasEnum && !hasValues {
		loc := c.locate(path)
		pos := loc.Value
		if pos.Line == 0 {
			pos = loc.Key
		}
		c.addErr(pos, path, "L601", "add values for .enum")
	}
	if isList && !hasItems {
		loc := c.locate(path)
		pos := loc.Value
		if pos.Line == 0 {
			pos = loc.Key
		}
		c.addErr(pos, path, "L601", "add items for .list")
	}
	if isMap && !hasMapValue {
		loc := c.locate(path)
		pos := loc.Value
		if pos.Line == 0 {
			pos = loc.Key
		}
		c.addErr(pos, path, "L601", "add value for .map")
	}

	// Range conflict checks (L606).
	if ts.hasMinLen && ts.hasMaxLen && ts.minLen > ts.maxLen {
		kp := appendSeg(path, "min-length")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min-length (%d) must be <= max-length (%d)", ts.minLen, ts.maxLen))
	}
	if ts.hasMin && ts.hasMax && compareBase10(ts.minVal, ts.maxVal) > 0 {
		kp := appendSeg(path, "min")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min (%s) must be <= max (%s)", ts.minVal, ts.maxVal))
	}
	if ts.hasMinSegs && ts.hasMaxSegs && ts.minSegs > ts.maxSegs {
		kp := appendSeg(path, "min-segments")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min-segments (%d) must be <= max-segments (%d)", ts.minSegs, ts.maxSegs))
	}
	if ts.hasMinItems && ts.hasMaxItems && ts.minItems > ts.maxItems {
		kp := appendSeg(path, "min-items")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min-items (%d) must be <= max-items (%d)", ts.minItems, ts.maxItems))
	}
	if ts.hasMinEntries && ts.hasMaxEntries && ts.minEntries > ts.maxEntries {
		kp := appendSeg(path, "min-entries")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min-entries (%d) must be <= max-entries (%d)", ts.minEntries, ts.maxEntries))
	}

	// Check one-of elements against sibling constraints on the same spec (L606).
	if len(ts.strOneOf) > 0 {
		c.checkStringOneOfConflicts(ts.strOneOf, ts.hasMinLen, ts.minLen, ts.hasMaxLen, ts.maxLen, ts.hasPat, ts.patSrc, ts.patRe, appendSeg(path, "one-of"))
	}
	if len(ts.intOneOf) > 0 && (ts.hasMin || ts.hasMax) {
		oneOfPath := appendSeg(path, "one-of")
		var (
			loc    lkal.Location
			locSet bool
		)
		getElemPos := func(i int) lkal.Pos {
			if !locSet {
				loc = c.locate(oneOfPath)
				locSet = true
			}
			return locElemPos(loc, i)
		}
		for i, n := range ts.intOneOf {
			numStr := strconv.FormatInt(n, 10)
			if ts.hasMin && compareBase10(numStr, ts.minVal) < 0 {
				c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates min (%s)", numStr, ts.minVal))
			}
			if ts.hasMax && compareBase10(numStr, ts.maxVal) > 0 {
				c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates max (%s)", numStr, ts.maxVal))
			}
		}
	}
	if len(ts.decOneOf) > 0 && (ts.hasMin || ts.hasMax) {
		oneOfPath := appendSeg(path, "one-of")
		var (
			loc    lkal.Location
			locSet bool
		)
		getElemPos := func(i int) lkal.Pos {
			if !locSet {
				loc = c.locate(oneOfPath)
				locSet = true
			}
			return locElemPos(loc, i)
		}
		for i, d := range ts.decOneOf {
			if ts.hasMin && compareBase10(d, ts.minVal) < 0 {
				c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates min (%s)", d, ts.minVal))
			}
			if ts.hasMax && compareBase10(d, ts.maxVal) > 0 {
				c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates max (%s)", d, ts.maxVal))
			}
		}
	}
}

func (c *compiler) checkStringOneOfConflicts(vals []string, hasMinLen bool, minLen int64, hasMaxLen bool, maxLen int64, hasPat bool, patSrc string, patRe *regexp.Regexp, oneOfPath []string) {
	if !hasMinLen && !hasMaxLen && (!hasPat || patRe == nil) {
		return
	}
	var (
		loc    lkal.Location
		locSet bool
	)
	getElemPos := func(i int) lkal.Pos {
		if !locSet {
			loc = c.locate(oneOfPath)
			locSet = true
		}
		return locElemPos(loc, i)
	}
	for i, s := range vals {
		runeLen := int64(utf8.RuneCountInString(s))
		if hasMinLen && runeLen < minLen {
			c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates min-length (%d)", quoteString(s), minLen))
		}
		if hasMaxLen && runeLen > maxLen {
			c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s violates max-length (%d)", quoteString(s), maxLen))
		}
		if hasPat && patRe != nil && !patRe.MatchString(s) {
			c.addErr(getElemPos(i), oneOfPath, "L606", fmt.Sprintf("one-of value %s does not match pattern %s", quoteString(s), patSrc))
		}
	}
}

func locElemPos(loc lkal.Location, idx int) lkal.Pos {
	if idx >= 0 && idx < len(loc.Elems) {
		return loc.Elems[idx]
	}
	return loc.Value
}

func (c *compiler) compileRecordConstraints(ts *typeSpec, rec lkal.Record, path []string) {
	if openVal, ok := findField(rec, "open"); ok {
		if b, isBool := openVal.(lkal.Bool); isBool {
			ts.open = bool(b)
		}
	}
	if fieldVal, ok := findField(rec, "field"); ok {
		if fieldRec, isRec := fieldVal.(lkal.Record); isRec {
			fieldMapPath := appendSeg(path, "field")
			for _, f := range fieldRec.Fields {
				fp := appendSeg(fieldMapPath, f.Key)
				if specRec, isSpecRec := f.Value.(lkal.Record); isSpecRec {
					fs := c.compileFieldSpec(f.Key, specRec, fp)
					ts.fields[f.Key] = fs
					ts.fieldOrder = append(ts.fieldOrder, f.Key)
				}
			}
		}
	}
	if resVal, ok := findField(rec, "reserved"); ok {
		if lst, isList := resVal.(lkal.List); isList && len(lst) > 0 {
			ts.reserved = make(map[string]bool, len(lst))
			resPath := appendSeg(path, "reserved")
			var (
				resLoc    lkal.Location
				resLocSet bool
			)
			for i, el := range lst {
				s, isStr := el.(lkal.String)
				if !isStr {
					continue
				}
				name := string(s)
				ts.reserved[name] = true
				if _, declared := ts.fields[name]; declared {
					if !resLocSet {
						resLoc = c.locate(resPath)
						resLocSet = true
					}
					c.addErr(locElemPos(resLoc, i), resPath, "L606", fmt.Sprintf("field %s cannot be both declared and reserved", quoteString(name)))
				}
			}
		}
	}
}

func (c *compiler) compileKeySpec(rec lkal.Record, path []string) *keySpec {
	ks := &keySpec{}
	for _, f := range rec.Fields {
		kp := appendSeg(path, f.Key)
		switch f.Key {
		case "min-length":
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ks.hasMinLen = true
				ks.minLen = int64(n)
			}
		case "max-length":
			if n, ok := f.Value.(lkal.Int); ok && n >= 0 {
				ks.hasMaxLen = true
				ks.maxLen = int64(n)
			}
		case "pattern":
			if s, ok := f.Value.(lkal.String); ok {
				ks.hasPat = true
				ks.patSrc = string(s)
				if re, ok := c.compilePattern(string(s), kp); ok {
					ks.patRe = re
				}
			}
		case "one-of":
			if lst, ok := f.Value.(lkal.List); ok && len(lst) > 0 {
				vals := make([]string, 0, len(lst))
				for _, el := range lst {
					if s, ok := el.(lkal.String); ok {
						vals = append(vals, string(s))
					}
				}
				ks.oneOf = vals
			}
		}
	}
	if ks.hasMinLen && ks.hasMaxLen && ks.minLen > ks.maxLen {
		kp := appendSeg(path, "min-length")
		c.addErr(c.locate(kp).Key, kp, "L606", fmt.Sprintf("min-length (%d) must be <= max-length (%d)", ks.minLen, ks.maxLen))
	}
	if len(ks.oneOf) > 0 {
		c.checkStringOneOfConflicts(ks.oneOf, ks.hasMinLen, ks.minLen, ks.hasMaxLen, ks.maxLen, ks.hasPat, ks.patSrc, ks.patRe, appendSeg(path, "one-of"))
	}
	return ks
}

func (c *compiler) compilePattern(pat string, path []string) (*regexp.Regexp, bool) {
	if _, err := regexp.Compile(pat); err != nil {
		c.addErr(c.locate(path).Value, path, "L604", err.Error())
		return nil, false
	}
	re, err := regexp.Compile(`\A(?:` + pat + `)\z`)
	if err != nil {
		c.addErr(c.locate(path).Value, path, "L604", err.Error())
		return nil, false
	}
	return re, true
}

func (c *compiler) resolveDefsAndUses() {
	// First check all `use` targets for existence (L602).
	for _, u := range c.uses {
		if _, ok := c.defs[u.spec.useName]; !ok {
			c.addErr(c.locate(u.path).Value, u.path, "L602", fmt.Sprintf("define def %s", quoteString(u.spec.useName)))
			u.spec.broken = true
		}
	}

	// Three-color walk over def aliases to detect cycles (L603) and resolve chains.
	const (
		stateUnvisited = 0
		stateVisiting  = 1
		stateDone      = 2
	)
	state := make(map[string]int, len(c.defs))
	for _, startName := range c.defOrder {
		if state[startName] != stateUnvisited {
			continue
		}
		var chain []string
		chainIndex := make(map[string]int)
		curName := startName
		for {
			ts := c.defs[curName]
			if ts == nil {
				for _, n := range chain {
					c.defs[n].broken = true
					state[n] = stateDone
				}
				break
			}
			if state[curName] == stateDone {
				target := ts.deref()
				for _, n := range chain {
					if ts.broken || target == nil {
						c.defs[n].broken = true
					} else {
						c.defs[n].resolved = target
					}
					state[n] = stateDone
				}
				break
			}
			if idx, inChain := chainIndex[curName]; inChain {
				cycle := append(slices.Clone(chain[idx:]), curName)
				cycleFirst := cycle[0]
				usePath := c.defUse[cycleFirst]
				c.addErr(c.locate(usePath).Value, usePath, "L603", "cycle: "+joinCycle(cycle))
				for _, n := range chain {
					c.defs[n].broken = true
					state[n] = stateDone
				}
				break
			}

			state[curName] = stateVisiting
			chainIndex[curName] = len(chain)
			chain = append(chain, curName)

			if !ts.hasUse {
				// Reached a concrete def (not a `use` alias).
				for _, n := range chain[:len(chain)-1] {
					if ts.broken {
						c.defs[n].broken = true
					} else {
						c.defs[n].resolved = ts
					}
					state[n] = stateDone
				}
				state[curName] = stateDone
				break
			}
			if ts.broken {
				for _, n := range chain {
					c.defs[n].broken = true
					state[n] = stateDone
				}
				break
			}
			curName = ts.useName
		}
	}

	// Resolve all non-def `use` nodes (field, items, map value).
	for _, u := range c.uses {
		if u.spec.broken || u.spec.resolved != nil {
			continue
		}
		target, ok := c.defs[u.spec.useName]
		if !ok || target.broken {
			u.spec.broken = true
			continue
		}
		u.spec.resolved = target.deref()
	}
}

func joinCycle(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += " -> "
		}
		out += formatKey(n)
	}
	return out
}

func (c *compiler) checkListItems() {
	for _, p := range c.listItems {
		if p.listSpec.items == nil || p.listSpec.items.broken {
			continue
		}
		resolved := p.listSpec.items.deref()
		if resolved == nil || resolved.broken || resolved.kindMask == 0 {
			continue
		}
		if (resolved.kindMask &^ scalarKindMask) != 0 {
			errPath := p.itemsPath
			pos := c.locate(p.itemsPath).Value
			if p.listSpec.items.hasUse {
				up := appendSeg(p.itemsPath, "use")
				if loc := c.locate(up); loc.Value.Line > 0 {
					pos = loc.Value
				}
			} else {
				tp := appendSeg(p.itemsPath, "type")
				if loc := c.locate(tp); loc.Value.Line > 0 {
					pos = loc.Value
				}
			}
			c.addErr(pos, errPath, "L601", "list items must be a scalar type")
		}
	}
}

func (c *compiler) checkUniqueBys() {
	for _, p := range c.uniqueBys {
		if p.mapSpec.mapValue == nil || p.mapSpec.mapValue.broken {
			continue
		}
		valSpec := p.mapSpec.mapValue.deref()
		if valSpec == nil || valSpec.broken || valSpec.kindMask == 0 {
			continue
		}
		ubPath := appendSeg(p.mapPath, "unique-by")
		if valSpec.isUnion || valSpec.kind != kindRecord {
			c.addErr(c.locate(ubPath).Key, ubPath, "L606", "unique-by requires map value to be .record")
			continue
		}
		var (
			ubLoc    lkal.Location
			ubLocSet bool
		)
		getElemPos := func(i int) lkal.Pos {
			if !ubLocSet {
				ubLoc = c.locate(ubPath)
				ubLocSet = true
			}
			return locElemPos(ubLoc, i)
		}
		for i, fieldName := range p.mapSpec.uniqueBy {
			elemIdx := i
			if i < len(p.elemIdxs) {
				elemIdx = p.elemIdxs[i]
			}
			if !valSpec.open {
				if _, exists := valSpec.fields[fieldName]; !exists {
					c.addErr(getElemPos(elemIdx), ubPath, "L606", fmt.Sprintf("unique-by field %s is not declared in record", quoteString(fieldName)))
				}
			} else if valSpec.reserved[fieldName] {
				c.addErr(getElemPos(elemIdx), ubPath, "L606", fmt.Sprintf("unique-by field %s is reserved", quoteString(fieldName)))
			}
		}
	}
}

func (c *compiler) checkDefaults() {
	for _, p := range c.defaults {
		if p.field.spec == nil || p.field.spec.broken {
			continue
		}
		target := p.field.spec.deref()
		if target == nil || target.broken || target.kindMask == 0 {
			continue
		}
		v := &validator{}
		v.validateNode(target, p.field.defVal, nil, -1)
		if len(v.errs) > 0 {
			first := sortErrors(v.errs)[0]
			hint := first.Hint
			if first.Path != "" {
				hint = first.Path + ": " + first.Hint
			}
			c.addErr(c.locate(p.path).Value, p.path, "L605", hint)
		}
	}
}

func findField(rec lkal.Record, key string) (lkal.Value, bool) {
	for _, f := range rec.Fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return nil, false
}

func appendSeg(path []string, seg string) []string {
	out := make([]string, len(path)+1)
	copy(out, path)
	out[len(path)] = seg
	return out
}
