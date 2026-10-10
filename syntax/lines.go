package syntax

import "sort"

// Lines maps byte offsets in a source file to lines and columns.
type Lines struct {
	starts []int // offset of the first byte of each line
	size   int
}

// NewLines builds the line table for src.
func NewLines(src []byte) *Lines {
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &Lines{starts: starts, size: len(src)}
}

// Position is a 1-based line and column. Columns count bytes, as Go's
// compiler does, so a tab or a multi-byte character can count as one column
// or several.
type Position struct {
	Line, Column int
}

// Position returns the line and column of a byte offset. Offsets past the end
// are clamped to the end of the file.
func (l *Lines) Position(offset int) Position {
	offset = max(0, min(offset, l.size))
	line := sort.Search(len(l.starts), func(i int) bool { return l.starts[i] > offset }) - 1
	return Position{Line: line + 1, Column: offset - l.starts[line] + 1}
}

// Count returns the number of lines. A file that ends with a newline has an
// empty last line.
func (l *Lines) Count() int { return len(l.starts) }

// LineStart returns the offset of the first byte of a 1-based line.
func (l *Lines) LineStart(line int) int { return l.starts[line-1] }
