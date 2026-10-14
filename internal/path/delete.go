package path

import (
	"slices"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// Delete returns a new *ast.Block with the target at pathStr removed,
// without mutating root in place.
func Delete(fileName string, root *ast.Block, pathStr string) (*ast.Block, error) {
	segs, ok := syntax.ParseRefBody(pathStr)
	if !ok {
		return nil, pathError(fileName, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	return updateAtPath(fileName, root, segs[:len(segs)-1], lastSeg, func(parent parentTarget, seg string) (*ast.Block, error) {
		if parent.kind == parentCollection {
			for idx, e := range parent.block.Entries {
				if e.Key == parent.collKey && e.Label == seg {
					return removeEntriesPure(parent.block, idx, idx), nil
				}
			}
			return nil, notFoundError(fileName, seg)
		}

		firstIdx, lastIdx := KeyRange(parent.block, seg)
		if firstIdx == -1 {
			return nil, notFoundError(fileName, seg)
		}
		return removeEntriesPure(parent.block, firstIdx, lastIdx), nil
	})
}

func removeEntriesPure(b *ast.Block, fromIdx, toIdx int) *ast.Block {
	first := b.Entries[fromIdx]
	hadBlank := first.BlankBefore
	if len(first.Comments) > 0 {
		hadBlank = first.Comments[0].BlankBefore
	}

	newEntries := slices.Delete(slices.Clone(b.Entries), fromIdx, toIdx+1)
	newTrailing := slices.Clone(b.Trailing)

	if hadBlank {
		if fromIdx < len(newEntries) {
			next := ast.CloneEntry(newEntries[fromIdx])
			if len(next.Comments) > 0 {
				next.Comments[0].BlankBefore = true
			} else {
				next.BlankBefore = true
			}
			newEntries[fromIdx] = next
		} else if len(newTrailing) > 0 {
			newTrailing[0].BlankBefore = true
		}
	}

	return &ast.Block{
		Entries:  newEntries,
		Trailing: newTrailing,
		OneLine:  b.OneLine,
		LBrace:   b.LBrace,
		RBrace:   b.RBrace,
	}
}
