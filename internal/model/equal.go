package model

import "slices"

// Equal reports whether two LKAL values are semantically equal.
// Record fields and collection elements are compared without regard to order;
// list elements and reference segments are compared in order.
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
