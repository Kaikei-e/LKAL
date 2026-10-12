package lkal

type File struct {
	name       string
	version    string
	versionPos Pos
	head       []comment
	body       *block
}

type block struct {
	entries  []*entry
	trailing []comment
	oneLine  bool
	lbrace   Pos
	rbrace   Pos
}

type entryKind int

const (
	entryAssign entryKind = iota
	entryRecord
	entryElem
)

type entry struct {
	kind        entryKind
	comments    []comment
	blankBefore bool
	key         string
	keyPos      Pos
	label       string
	labelPos    Pos
	value       node
	body        *block
}

type comment struct {
	pos         Pos
	text        string
	blankBefore bool
}

type node interface {
	nodePos() Pos
}

type scalarNode struct {
	tok token
	val Value
}

func (n *scalarNode) nodePos() Pos {
	if n == nil {
		return Pos{}
	}
	return n.tok.pos
}

type listNode struct {
	lbrack  Pos
	rbrack  Pos
	elems   []*scalarNode
	oneLine bool
}

func (n *listNode) nodePos() Pos {
	if n == nil {
		return Pos{}
	}
	return n.lbrack
}

type rawNode struct {
	pos   Pos
	lines []string
}

func (n *rawNode) nodePos() Pos {
	if n == nil {
		return Pos{}
	}
	return n.pos
}
