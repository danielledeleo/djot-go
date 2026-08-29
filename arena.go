package djot

import "github.com/danielledeleo/djot-go/ast"

type typedNodeArena struct {
	texts      typedSlab[ast.Text]
	strongs    typedSlab[ast.Strong]
	paragraphs typedSlab[ast.Paragraph]
	tableCells typedSlab[ast.TableCell]
	listItems  typedSlab[ast.ListItem]
	links      typedSlab[ast.Link]
}

type typedSlab[T any] struct {
	chunk []T
	next  int
}

const (
	typedArenaMinChunk = 4
	typedArenaMaxChunk = 2048
)

func (a *typedSlab[T]) alloc() *T {
	if len(a.chunk) == cap(a.chunk) {
		size := a.next
		if size < typedArenaMinChunk {
			size = typedArenaMinChunk
		}
		a.chunk = make([]T, 0, size)
		if size < typedArenaMaxChunk {
			a.next = size * 2
		}
	}
	a.chunk = append(a.chunk, *new(T))
	return &a.chunk[len(a.chunk)-1]
}
