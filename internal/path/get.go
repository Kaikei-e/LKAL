package path

import (
	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// Get retrieves the Value at pathStr from root without mutating root.
func Get(fileName string, root *ast.Block, pathStr string) (model.Value, error) {
	segs, ok := syntax.ParseRefBody(pathStr)
	if !ok {
		return nil, pathError(fileName, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	parent, failSeg, ok := resolveParent(root, segs[:len(segs)-1], lastSeg)
	if !ok {
		return nil, notFoundError(fileName, failSeg)
	}

	if parent.kind == parentCollection {
		for _, e := range parent.block.Entries {
			if e.Key == parent.collKey && e.Label == lastSeg {
				return ast.BuildRecord(e.Body), nil
			}
		}
		return nil, notFoundError(fileName, lastSeg)
	}

	firstIdx, lastIdx := KeyRange(parent.block, lastSeg)
	if firstIdx == -1 {
		return nil, notFoundError(fileName, lastSeg)
	}

	first := parent.block.Entries[firstIdx]
	switch first.Kind {
	case ast.EntryAssign:
		return ast.BuildValue(first.Value), nil
	case ast.EntryRecord:
		return ast.BuildRecord(first.Body), nil
	default:
		matching := parent.block.Entries[firstIdx : lastIdx+1]
		elems := make([]model.Elem, len(matching))
		for i, e := range matching {
			elems[i] = model.Elem{Label: e.Label, Body: ast.BuildRecord(e.Body)}
		}
		return model.Collection{Elems: elems}, nil
	}
}
