package diag

// MaxBlockDepth is the maximum allowed nesting depth for blocks and objects.
const MaxBlockDepth = 1000

// MaxErrors is the maximum number of errors reported in an ErrorList.
const MaxErrors = 20

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
	"L107": "blocks nested too deeply",
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
	"L107": "nest blocks at most 1000 levels deep",
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

// Message returns the standard message for an error code.
func Message(code string) string {
	return errorMessages[code]
}

// DefaultHint returns the default hint for an error code.
func DefaultHint(code string) string {
	return errorHints[code]
}
