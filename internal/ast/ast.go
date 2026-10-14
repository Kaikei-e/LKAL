package ast

import (
	"slices"

	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/token"
)

// File represents a parsed LKAL document syntax tree, preserving comments and layout metadata.
type File struct {
	Name       string
	Version    string
	VersionPos diag.Pos
	Head       []Comment
	Body       *Block
}

// Block represents a top-level document body or a braced block.
type Block struct {
	Entries  []*Entry
	Trailing []Comment
	OneLine  bool
	LBrace   diag.Pos
	RBrace   diag.Pos
}

// EntryKind classifies an entry inside a Block.
type EntryKind int

const (
	EntryAssign EntryKind = iota
	EntryRecord
	EntryElem
)

// Entry represents a key-value assignment, a nested record block, or a collection element block.
type Entry struct {
	Kind        EntryKind
	Comments    []Comment
	BlankBefore bool
	Key         string
	KeyPos      diag.Pos
	Label       string
	LabelPos    diag.Pos
	Value       Node
	Body        *Block
}

// Comment represents a single ### comment line.
type Comment struct {
	Pos         diag.Pos
	Text        string
	BlankBefore bool
}

// Node is implemented by AST value nodes on the right-hand side of an assignment.
type Node interface {
	NodePos() diag.Pos
}

// ScalarNode represents a single scalar literal value.
type ScalarNode struct {
	Tok token.Token
	Val model.Value
}

func (n *ScalarNode) NodePos() diag.Pos {
	return n.Tok.Pos
}

// ListNode represents a bracketed list of scalars.
type ListNode struct {
	LBrack  diag.Pos
	RBrack  diag.Pos
	Elems   []*ScalarNode
	OneLine bool
}

func (n *ListNode) NodePos() diag.Pos {
	return n.LBrack
}

// RawNode represents a multi-line raw string (\\ lines).
type RawNode struct {
	Pos   diag.Pos
	Lines []string
}

func (n *RawNode) NodePos() diag.Pos {
	return n.Pos
}

// CloneEntry returns a shallow copy of e with cloned Comments slice.
func CloneEntry(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	cp := *e
	cp.Comments = slices.Clone(e.Comments)
	return &cp
}

// WithEntries returns a new Block copying b's metadata and trailing comments with the given entries.
func WithEntries(b *Block, entries []*Entry) *Block {
	return &Block{
		Entries:  entries,
		Trailing: slices.Clone(b.Trailing),
		OneLine:  b.OneLine,
		LBrace:   b.LBrace,
		RBrace:   b.RBrace,
	}
}
