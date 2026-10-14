package parser

import (
	"strings"

	"github.com/Kaikei-e/LKAL/internal/ast"
	"github.com/Kaikei-e/LKAL/internal/diag"
)

// Validate checks semantic rules (L201-L205) on b without mutating b.
func Validate(file string, b *ast.Block) []*diag.Error {
	if b == nil {
		return nil
	}
	var errs []*diag.Error
	seenKinds := make(map[string]map[ast.EntryKind]bool)
	seenLabels := make(map[string]map[string]bool)
	var prevElemKey string

	for _, e := range b.Entries {
		if err := checkEntryRule(file, e, seenKinds, seenLabels, prevElemKey); err != nil {
			errs = append(errs, err)
		}

		if seenKinds[e.Key] == nil {
			seenKinds[e.Key] = make(map[ast.EntryKind]bool)
		}
		seenKinds[e.Key][e.Kind] = true
		if e.Kind == ast.EntryElem {
			if seenLabels[e.Key] == nil {
				seenLabels[e.Key] = make(map[string]bool)
			}
			seenLabels[e.Key][e.Label] = true
			prevElemKey = e.Key
		} else {
			prevElemKey = ""
		}

		if err := checkListUniformity(file, e.Value); err != nil {
			errs = append(errs, err)
		}

		if e.Body != nil {
			errs = append(errs, Validate(file, e.Body)...)
		}
	}
	return errs
}

func checkEntryRule(
	file string,
	e *ast.Entry,
	seenKinds map[string]map[ast.EntryKind]bool,
	seenLabels map[string]map[string]bool,
	prevElemKey string,
) *diag.Error {
	if strings.HasPrefix(e.Key, "$") {
		return diag.NewError(file, e.KeyPos, "L205", "")
	}
	for k := range seenKinds[e.Key] {
		if k != e.Kind {
			return diag.NewError(file, e.KeyPos, "L202", "")
		}
	}
	if e.Kind == ast.EntryAssign || e.Kind == ast.EntryRecord {
		if seenKinds[e.Key] != nil {
			return diag.NewError(file, e.KeyPos, "L201", "")
		}
	} else if e.Kind == ast.EntryElem {
		if seenLabels[e.Key] != nil && seenLabels[e.Key][e.Label] {
			return diag.NewError(file, e.LabelPos, "L201", "")
		}
	}
	if e.Kind == ast.EntryElem && seenKinds[e.Key] != nil && prevElemKey != e.Key {
		return diag.NewError(file, e.KeyPos, "L203", "")
	}
	return nil
}

func checkListUniformity(file string, n ast.Node) *diag.Error {
	lst, ok := n.(*ast.ListNode)
	if !ok || len(lst.Elems) <= 1 {
		return nil
	}
	firstKind := lst.Elems[0].Tok.Kind
	for _, el := range lst.Elems[1:] {
		if el.Tok.Kind != firstKind {
			return diag.NewError(file, el.Tok.Pos, "L204", "")
		}
	}
	return nil
}
