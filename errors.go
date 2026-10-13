package lkal

import (
	"sort"
	"strconv"
	"strings"
)

type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}

type Error struct {
	File string
	Pos  Pos
	Code string
	Msg  string
	Hint string
}

func (e *Error) Error() string {
	var prefix string
	if e.File != "" && e.Pos.Line > 0 {
		prefix = e.File + ":" + e.Pos.String() + ": "
	} else if e.File == "" && e.Pos.Line > 0 {
		prefix = e.Pos.String() + ": "
	} else if e.File != "" && e.Pos.Line == 0 {
		prefix = e.File + ": "
	}
	s := prefix + e.Code + " " + e.Msg
	if e.Hint != "" {
		s += "\n  hint: " + e.Hint
	}
	return s
}

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

var errorMessages = map[string]string{
	"L001": "comment starts with # or ##",
	"L002": "comment at the end of a line or inside a list",
	"L003": "invalid escape sequence",
	"L004": "digits after a unit",
	"L005": "number out of range",
	"L006": "unknown directive",
	"L007": "unterminated string",
	"L008": "invalid character",
	"L009": "malformed token",
	"L101": "entry header broken across lines",
	"L102": "bare word in value position",
	"L103": "unclosed bracket",
	"L104": "blank line or comment inside a multi-line string",
	"L105": "missing, misplaced or repeated version declaration",
	"L106": "unexpected token",
	"L201": "duplicate key or label",
	"L202": "key used for more than one kind of entry",
	"L203": "collection elements are not contiguous",
	"L204": "list mixes types",
	"L205": "key starts with $",
	"L206": "value assigned to a collection element",
	"L207": "unsupported version",
	"L301": "invalid JSON",
	"L302": "null value",
	"L303": "number in exponent notation",
	"L304": "list element is not a scalar",
	"L305": "malformed tag",
	"L401": "malformed path",
	"L402": "path not found",
	"L403": "invalid value",
}

var errorHints = map[string]string{
	"L001": "start the comment with ###",
	"L002": "move the comment to its own line outside the list",
	"L003": `use \u{...}`,
	"L004": "use a single unit (for example 90m)",
	"L005": "keep integers within int64 and numbers within 100 digits",
	"L006": "v0.1 supports only @lkal",
	"L007": `close the string on the same line; write multi-line strings as \\ lines`,
	"L008": `remove the character; write control characters as \u{...}`,
	"L009": "follow the syntax for numbers, enums, references and directives",
	"L101": "keep the key, label and { on one line",
	"L102": `write "WORD" for a string or .WORD for an enum`,
	"L103": "add the matching closing bracket",
	"L104": `remove the gap; write an empty line as a lone \\`,
	"L105": `write @lkal "0.1" once, at the top`,
	"L106": `start an entry with key = value, key { ... } or key "label" { ... }`,
	"L201": "remove one of them",
	"L202": "use the key for one kind only: assignment, record or collection",
	"L203": "write the elements next to each other",
	"L204": "use one type for all elements",
	"L205": "rename the key",
	"L206": `write a block instead: key "label" { ... }`,
	"L207": `this implementation accepts only "0.1"`,
	"L301": "fix the JSON syntax",
	"L302": "omit the key instead of writing null",
	"L303": "write the number without an exponent",
	"L304": "use only scalars as list elements",
	"L401": "separate segments with / and quote segments that contain other characters",
}

func newError(file string, pos Pos, code, hint string) *Error {
	if hint == "" {
		hint = errorHints[code]
	}
	return &Error{
		File: file,
		Pos:  pos,
		Code: code,
		Msg:  errorMessages[code],
		Hint: hint,
	}
}

func sortErrors(errs []*Error) []*Error {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Pos.Line != errs[j].Pos.Line {
			return errs[i].Pos.Line < errs[j].Pos.Line
		}
		if errs[i].Pos.Col != errs[j].Pos.Col {
			return errs[i].Pos.Col < errs[j].Pos.Col
		}
		return errs[i].Code < errs[j].Code
	})
	if len(errs) > 20 {
		return errs[:20]
	}
	return errs
}
