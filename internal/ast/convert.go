package ast

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
	"github.com/Kaikei-e/LKAL/internal/token"
)

// BuildRecord constructs a pure model.Record from an AST Block.
func BuildRecord(b *Block) model.Record {
	if b == nil {
		return model.Record{}
	}
	var fields []model.Field
	collIndexByKey := make(map[string]int)

	for _, e := range b.Entries {
		switch e.Kind {
		case EntryAssign:
			fields = append(fields, model.Field{Key: e.Key, Value: BuildValue(e.Value)})
		case EntryRecord:
			fields = append(fields, model.Field{Key: e.Key, Value: BuildRecord(e.Body)})
		case EntryElem:
			elem := model.Elem{Label: e.Label, Body: BuildRecord(e.Body)}
			if idx, ok := collIndexByKey[e.Key]; ok {
				coll := fields[idx].Value.(model.Collection)
				coll.Elems = append(coll.Elems, elem)
				fields[idx].Value = coll
			} else {
				collIndexByKey[e.Key] = len(fields)
				fields = append(fields, model.Field{Key: e.Key, Value: model.Collection{Elems: []model.Elem{elem}}})
			}
		}
	}
	return model.Record{Fields: fields}
}

// BuildValue constructs a pure model.Value from an AST value Node.
func BuildValue(n Node) model.Value {
	switch v := n.(type) {
	case *ScalarNode:
		if r, ok := v.Val.(model.Ref); ok {
			return slices.Clone(r)
		}
		return v.Val
	case *RawNode:
		return model.String(strings.Join(v.Lines, "\n"))
	case *ListNode:
		list := make(model.List, len(v.Elems))
		for i, el := range v.Elems {
			if r, ok := el.Val.(model.Ref); ok {
				list[i] = slices.Clone(r)
			} else {
				list[i] = el.Val
			}
		}
		return list
	}
	return nil
}

// NodeFromValue constructs a pure AST Node from a model.Value.
func NodeFromValue(v model.Value) Node {
	if lst, ok := v.(model.List); ok {
		elems := make([]*ScalarNode, len(lst))
		for i, el := range lst {
			elems[i] = &ScalarNode{Val: el}
		}
		return &ListNode{Elems: elems, OneLine: true}
	}
	return &ScalarNode{Val: v}
}

// BlockFromRecord constructs a pure AST Block from a model.Record.
func BlockFromRecord(r model.Record) *Block {
	b := &Block{}
	for _, f := range r.Fields {
		switch val := f.Value.(type) {
		case model.Record:
			b.Entries = append(b.Entries, &Entry{
				Kind: EntryRecord,
				Key:  f.Key,
				Body: BlockFromRecord(val),
			})
		case model.Collection:
			for _, el := range val.Elems {
				b.Entries = append(b.Entries, &Entry{
					Kind:  EntryElem,
					Key:   f.Key,
					Label: el.Label,
					Body:  BlockFromRecord(el.Body),
				})
			}
		default:
			b.Entries = append(b.Entries, &Entry{
				Kind:  EntryAssign,
				Key:   f.Key,
				Value: NodeFromValue(val),
			})
		}
	}
	return b
}

// ScalarFromToken constructs a pure ScalarNode from a scalar token.Token.
func ScalarFromToken(tok token.Token) *ScalarNode {
	var val model.Value
	switch tok.Kind {
	case token.String:
		val = model.String(tok.Text)
	case token.Int:
		n, _ := strconv.ParseInt(tok.Text, 10, 64)
		val = model.Int(n)
	case token.Decimal:
		val = model.Decimal(syntax.NormalizeNumber(tok.Text))
	case token.Quantity:
		num, unit := syntax.SplitQuantity(tok.Text)
		val = model.Quantity{Number: syntax.NormalizeNumber(num), Unit: unit}
	case token.Bool:
		val = model.Bool(tok.Text == "true")
	case token.Enum:
		val = model.Enum(tok.Text)
	case token.Ref:
		val = model.Ref(tok.Segs)
	}
	return &ScalarNode{Tok: tok, Val: val}
}
