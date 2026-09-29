package djot

import "sync"

// sourceFile holds the parsed input and resolves byte offsets in it to line
// and column. The line index is built on first use.
type sourceFile struct {
	text           string
	lineStarts     []int
	lineStartsOnce sync.Once
}

// position resolves a byte offset to a one-based line and column.
func (s *sourceFile) position(offset int) (line, col int) {
	s.lineStartsOnce.Do(func() {
		s.lineStarts = []int{0}
		for i := 0; i < len(s.text); i++ {
			if s.text[i] == '\n' {
				s.lineStarts = append(s.lineStarts, i+1)
			}
		}
	})
	// Binary search for the line containing offset.
	lo, hi := 0, len(s.lineStarts)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if s.lineStarts[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	line = lo // 1-based: lineStarts[0] is line 1
	col = offset - s.lineStarts[line-1] + 1
	return line, col
}
