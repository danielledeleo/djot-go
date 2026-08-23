package djot

// typedNodeArena uses small, independently growing slabs for the six dominant
// materialized node shapes. Rare shapes use ordinary allocation; giving every
// concrete type a slab retained more empty capacity without reducing tree size.
type typedNodeArena struct {
	texts      typedSlab[Text]
	strongs    typedSlab[Strong]
	paragraphs typedSlab[Paragraph]
	tableCells typedSlab[TableCell]
	listItems  typedSlab[ListItem]
	links      typedSlab[Link]
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
