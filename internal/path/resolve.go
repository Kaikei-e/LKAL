package path

import (
	"slices"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

type parentKind int

const (
	parentRecord parentKind = iota
	parentCollection
)

type parentTarget struct {
	kind    parentKind
	block   *ast.Block
	collKey string
	depth   int
}

func pathError(fileName, code, hint string) error {
	return &diag.ErrorList{Errors: []*diag.Error{diag.NewError(fileName, diag.Pos{}, code, hint)}}
}

func notFoundError(file, seg string) error {
	return pathError(file, "L402", "check the path at segment "+syntax.QuoteString(seg))
}

// KeyRange returns the first and last index of entries with the given key in b,
// or (-1, -1) if not found.
func KeyRange(b *ast.Block, key string) (int, int) {
	first := -1
	last := -1
	for i, e := range b.Entries {
		if e.Key == key {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	return first, last
}

// resolveParent is a pure function that locates the parent block or collection for parentSegs.
func resolveParent(root *ast.Block, parentSegs []string, lastSeg string) (parentTarget, string, bool) {
	target := parentTarget{kind: parentRecord, block: root, depth: 0}
	for i, seg := range parentSegs {
		switch target.kind {
		case parentRecord:
			firstIdx, _ := KeyRange(target.block, seg)
			if firstIdx == -1 {
				return parentTarget{}, seg, false
			}
			first := target.block.Entries[firstIdx]
			switch first.Kind {
			case ast.EntryAssign:
				if i < len(parentSegs)-1 {
					return parentTarget{}, parentSegs[i+1], false
				}
				return parentTarget{}, lastSeg, false
			case ast.EntryRecord:
				target = parentTarget{kind: parentRecord, block: first.Body, depth: target.depth + 1}
			case ast.EntryElem:
				target = parentTarget{kind: parentCollection, block: target.block, collKey: seg, depth: target.depth}
			}
		case parentCollection:
			var elemEnt *ast.Entry
			for _, e := range target.block.Entries {
				if e.Key == target.collKey && e.Label == seg {
					elemEnt = e
					break
				}
			}
			if elemEnt == nil {
				return parentTarget{}, seg, false
			}
			target = parentTarget{kind: parentRecord, block: elemEnt.Body, depth: target.depth + 1}
		}
	}
	return target, "", true
}

// updateAtPath is a pure recursive function that navigates parentSegs and applies fn
// to the target parent, cloning only the modified spine of blocks and entries.
func updateAtPath(
	fileName string,
	root *ast.Block,
	parentSegs []string,
	lastSeg string,
	fn func(target parentTarget, lastSeg string) (*ast.Block, error),
) (*ast.Block, error) {
	return updateTarget(fileName, parentTarget{kind: parentRecord, block: root, depth: 0}, parentSegs, lastSeg, fn)
}

func updateTarget(
	fileName string,
	target parentTarget,
	parentSegs []string,
	lastSeg string,
	fn func(target parentTarget, lastSeg string) (*ast.Block, error),
) (*ast.Block, error) {
	if len(parentSegs) == 0 {
		return fn(target, lastSeg)
	}
	seg := parentSegs[0]
	rest := parentSegs[1:]

	switch target.kind {
	case parentRecord:
		firstIdx, _ := KeyRange(target.block, seg)
		if firstIdx == -1 {
			return nil, notFoundError(fileName, seg)
		}
		first := target.block.Entries[firstIdx]
		switch first.Kind {
		case ast.EntryAssign:
			if len(rest) > 0 {
				return nil, notFoundError(fileName, rest[0])
			}
			return nil, notFoundError(fileName, lastSeg)
		case ast.EntryRecord:
			childTarget := parentTarget{kind: parentRecord, block: first.Body, depth: target.depth + 1}
			newChildBody, err := updateTarget(fileName, childTarget, rest, lastSeg, fn)
			if err != nil {
				return nil, err
			}
			newEnt := ast.CloneEntry(first)
			newEnt.Body = newChildBody
			newEntries := slices.Clone(target.block.Entries)
			newEntries[firstIdx] = newEnt
			return ast.WithEntries(target.block, newEntries), nil
		case ast.EntryElem:
			collTarget := parentTarget{kind: parentCollection, block: target.block, collKey: seg, depth: target.depth}
			return updateTarget(fileName, collTarget, rest, lastSeg, fn)
		}
	case parentCollection:
		elemIdx := -1
		for i, e := range target.block.Entries {
			if e.Key == target.collKey && e.Label == seg {
				elemIdx = i
				break
			}
		}
		if elemIdx == -1 {
			return nil, notFoundError(fileName, seg)
		}
		elemEnt := target.block.Entries[elemIdx]
		childTarget := parentTarget{kind: parentRecord, block: elemEnt.Body, depth: target.depth + 1}
		newElemBody, err := updateTarget(fileName, childTarget, rest, lastSeg, fn)
		if err != nil {
			return nil, err
		}
		newEnt := ast.CloneEntry(elemEnt)
		newEnt.Body = newElemBody
		newEntries := slices.Clone(target.block.Entries)
		newEntries[elemIdx] = newEnt
		return ast.WithEntries(target.block, newEntries), nil
	}
	return nil, notFoundError(fileName, seg)
}
