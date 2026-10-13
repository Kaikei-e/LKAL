package lkal

import "slices"

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
		return ok && slices.Equal(va, vb)
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
		return ok && matchUnordered(len(va.Fields), len(vb.Fields), func(i, j int) bool {
			return va.Fields[i].Key == vb.Fields[j].Key && Equal(va.Fields[i].Value, vb.Fields[j].Value)
		})
	case Collection:
		vb, ok := b.(Collection)
		return ok && matchUnordered(len(va.Elems), len(vb.Elems), func(i, j int) bool {
			return va.Elems[i].Label == vb.Elems[j].Label && Equal(va.Elems[i].Body, vb.Elems[j].Body)
		})
	default:
		return false
	}
}

func matchUnordered(n, m int, same func(i, j int) bool) bool {
	if n != m {
		return false
	}
	used := make([]bool, m)
	for i := 0; i < n; i++ {
		found := false
		for j := 0; j < m; j++ {
			if !used[j] && same(i, j) {
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
}
