package model

// Value is the sealed interface implemented by all 10 LKAL data model types.
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

// ScalarType returns the canonical type name of a scalar Value, or "" if v is not a scalar.
func ScalarType(v Value) string {
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
