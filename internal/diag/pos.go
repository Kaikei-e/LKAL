package diag

import (
	"strconv"
	"unicode/utf8"
)

// Pos is a 1-based source position where Col counts Unicode code points.
type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}

// LineCols returns a byte-offset-to-column lookup table for a single line,
// starting at startCol (1-based). It is a pure function.
func LineCols(line []byte, startCol int) []int {
	cols := make([]int, len(line)+1)
	col := startCol
	for off := 0; off < len(line); {
		cols[off] = col
		_, sz := utf8.DecodeRune(line[off:])
		for j := 1; j < sz; j++ {
			cols[off+j] = col
		}
		col++
		off += sz
	}
	cols[len(line)] = col
	return cols
}
