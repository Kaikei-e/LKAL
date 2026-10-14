package diag

import (
	"slices"
	"sort"
	"strings"
)

// Error represents a single LKAL diagnostic with file, position, code, message, and hint.
type Error struct {
	File string
	Pos  Pos
	Code string
	Msg  string
	Hint string
}

func (e *Error) Error() string {
	var s string
	if e.File != "" {
		s += e.File + ":"
	}
	if e.Pos.Line > 0 {
		s += e.Pos.String() + ":"
	}
	if s != "" {
		s += " "
	}
	s += e.Code + " " + e.Msg
	if e.Hint != "" {
		s += "\n  hint: " + e.Hint
	}
	return s
}

// ErrorList holds up to MaxErrors errors sorted by source position and code.
type ErrorList struct {
	Errors []*Error
}

func (l *ErrorList) Error() string {
	if len(l.Errors) == 0 {
		return ""
	}
	var b strings.Builder
	for i, err := range l.Errors {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

// NewError constructs an Error, using the catalog default hint if hint is empty.
func NewError(file string, pos Pos, code, hint string) *Error {
	if hint == "" {
		hint = DefaultHint(code)
	}
	return &Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  Message(code),
		Hint: hint,
	}
}

// SortErrors returns a new slice of errors sorted by Line, Col, and Code,
// capped at MaxErrors, without mutating the input slice.
func SortErrors(errs []*Error) []*Error {
	sorted := slices.Clone(errs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Pos.Line != sorted[j].Pos.Line {
			return sorted[i].Pos.Line < sorted[j].Pos.Line
		}
		if sorted[i].Pos.Col != sorted[j].Pos.Col {
			return sorted[i].Pos.Col < sorted[j].Pos.Col
		}
		return sorted[i].Code < sorted[j].Code
	})
	if len(sorted) > MaxErrors {
		return sorted[:MaxErrors]
	}
	return sorted
}
