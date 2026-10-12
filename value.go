package lkal

type Value interface {
	lkalValue()
}

type (
	String     string
	Int        int64
	Decimal    string
	Bool       bool
	Enum       string
	Quantity   struct{ Number, Unit string }
	Ref        []string
	List       []Value
	Record     struct{ Fields []Field }
	Collection struct{ Elems []Elem }
)

type Field struct {
	Key   string
	Value Value
}

type Elem struct {
	Label string
	Body  Record
}

func (String) lkalValue()     {}
func (Int) lkalValue()        {}
func (Decimal) lkalValue()    {}
func (Bool) lkalValue()       {}
func (Enum) lkalValue()       {}
func (Quantity) lkalValue()   {}
func (Ref) lkalValue()        {}
func (List) lkalValue()       {}
func (Record) lkalValue()     {}
func (Collection) lkalValue() {}

func Equal(a, b Value) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch va := a.(type) {
	case String:
		vb, ok := b.(String)
		return ok && va == vb
	case Int:
		vb, ok := b.(Int)
		return ok && va == vb
	case Decimal:
		vb, ok := b.(Decimal)
		return ok && va == vb
	case Bool:
		vb, ok := b.(Bool)
		return ok && va == vb
	case Enum:
		vb, ok := b.(Enum)
		return ok && va == vb
	case Quantity:
		vb, ok := b.(Quantity)
		return ok && va == vb
	case Ref:
		vb, ok := b.(Ref)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if va[i] != vb[i] {
				return false
			}
		}
		return true
	case List:
		vb, ok := b.(List)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !Equal(va[i], vb[i]) {
				return false
			}
		}
		return true
	case Record:
		vb, ok := b.(Record)
		if !ok || len(va.Fields) != len(vb.Fields) {
			return false
		}
		if len(va.Fields) == 0 {
			return true
		}
		used := make([]bool, len(vb.Fields))
		for _, fa := range va.Fields {
			found := false
			for j, fb := range vb.Fields {
				if !used[j] && fa.Key == fb.Key && Equal(fa.Value, fb.Value) {
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case Collection:
		vb, ok := b.(Collection)
		if !ok || len(va.Elems) != len(vb.Elems) {
			return false
		}
		if len(va.Elems) == 0 {
			return true
		}
		used := make([]bool, len(vb.Elems))
		for _, ea := range va.Elems {
			found := false
			for j, eb := range vb.Elems {
				if !used[j] && ea.Label == eb.Label && Equal(ea.Body, eb.Body) {
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return false
	}
}
