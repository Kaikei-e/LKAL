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
	return false
}
