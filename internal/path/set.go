package path

import (
	"slices"
	"strings"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/model"
	"github.com/Kaikei-e/LKAL/internal/syntax"
)

// Set returns a new *ast.Block with v set at pathStr, without mutating root in place.
func Set(fileName string, root *ast.Block, pathStr string, v model.Value) (*ast.Block, error) {
	segs, ok := syntax.ParseRefBody(pathStr)
	if !ok {
		return nil, pathError(fileName, "L401", "")
	}

	lastSeg := segs[len(segs)-1]
	return updateAtPath(fileName, root, segs[:len(segs)-1], lastSeg, func(parent parentTarget, seg string) (*ast.Block, error) {
		if parent.kind == parentCollection {
			return setInCollection(fileName, parent, seg, v)
		}
		return setInRecord(fileName, parent, seg, v)
	})
}

func setInCollection(fileName string, parent parentTarget, label string, v model.Value) (*ast.Block, error) {
	recVal, isRec := v.(model.Record)
	if !isRec {
		return nil, pathError(fileName, "L403", "use a record for a collection element")
	}
	if hint := model.ValidateValue(recVal, parent.depth+1); hint != "" {
		return nil, pathError(fileName, "L403", hint)
	}
	vNorm := model.NormalizeValue(recVal).(model.Record)

	existingIdx := -1
	lastCollIdx := -1
	for idx, e := range parent.block.Entries {
		if e.Key == parent.collKey {
			lastCollIdx = idx
			if e.Label == label {
				existingIdx = idx
			}
		}
	}

	if existingIdx != -1 {
		existingElem := parent.block.Entries[existingIdx]
		oldBodyOneLine := false
		if len(existingElem.Body.Entries) > 0 || len(existingElem.Body.Trailing) > 0 {
			oldBodyOneLine = existingElem.Body.OneLine
		}
		newBody := ast.BlockFromRecord(vNorm)
		newBody.OneLine = oldBodyOneLine

		updatedElem := ast.CloneEntry(existingElem)
		updatedElem.Body = newBody
		newEntries := slices.Clone(parent.block.Entries)
		newEntries[existingIdx] = updatedElem
		return ast.WithEntries(parent.block, newEntries), nil
	}

	newElem := &ast.Entry{
		Kind:  ast.EntryElem,
		Key:   parent.collKey,
		Label: label,
		Body:  ast.BlockFromRecord(vNorm),
	}
	newEntries := slices.Insert(slices.Clone(parent.block.Entries), lastCollIdx+1, newElem)
	return ast.WithEntries(parent.block, newEntries), nil
}

func setInRecord(fileName string, parent parentTarget, key string, v model.Value) (*ast.Block, error) {
	if strings.HasPrefix(key, "$") {
		return nil, pathError(fileName, "L403", "rename keys that start with $")
	}
	if hint := model.ValidateValue(v, parent.depth+1); hint != "" {
		return nil, pathError(fileName, "L403", hint)
	}

	vNorm := model.NormalizeValue(v)
	firstIdx, lastIdx := KeyRange(parent.block, key)
	var oldEnt *ast.Entry
	if firstIdx != -1 {
		oldEnt = parent.block.Entries[firstIdx]
	}

	newEntries := entriesFromValue(key, vNorm, oldEnt)
	if oldEnt != nil {
		newEntries[0].Comments = slices.Clone(oldEnt.Comments)
		newEntries[0].BlankBefore = oldEnt.BlankBefore
		replaced := slices.Replace(slices.Clone(parent.block.Entries), firstIdx, lastIdx+1, newEntries...)
		return ast.WithEntries(parent.block, replaced), nil
	}

	appended := append(slices.Clone(parent.block.Entries), newEntries...)
	res := ast.WithEntries(parent.block, appended)
	if len(parent.block.Entries) == 0 && len(parent.block.Trailing) == 0 {
		res.OneLine = false
	}
	return res, nil
}

func entriesFromValue(key string, vNorm model.Value, oldEnt *ast.Entry) []*ast.Entry {
	switch val := vNorm.(type) {
	case model.Collection:
		entries := make([]*ast.Entry, len(val.Elems))
		for i, elem := range val.Elems {
			entries[i] = &ast.Entry{
				Kind:  ast.EntryElem,
				Key:   key,
				Label: elem.Label,
				Body:  ast.BlockFromRecord(elem.Body),
			}
		}
		return entries
	case model.Record:
		b := ast.BlockFromRecord(val)
		if oldEnt != nil && oldEnt.Kind == ast.EntryRecord &&
			(len(oldEnt.Body.Entries) > 0 || len(oldEnt.Body.Trailing) > 0) {
			b.OneLine = oldEnt.Body.OneLine
		}
		return []*ast.Entry{{
			Kind: ast.EntryRecord,
			Key:  key,
			Body: b,
		}}
	default:
		n := ast.NodeFromValue(val)
		if oldEnt != nil && oldEnt.Kind == ast.EntryAssign {
			if lnOld, ok := oldEnt.Value.(*ast.ListNode); ok {
				if lnNew, ok := n.(*ast.ListNode); ok {
					lnNew.OneLine = lnOld.OneLine
				}
			}
		}
		return []*ast.Entry{{
			Kind:  ast.EntryAssign,
			Key:   key,
			Value: n,
		}}
	}
}
