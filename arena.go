package djot

// nodeArena bump-allocates Node values from chunks, so the parser makes one
// heap allocation per chunk of nodes instead of one per node.
//
// Pointers returned by new stay valid for the life of the document. A chunk is
// only ever appended to while it has spare capacity, so its backing array is
// never reallocated out from under the pointers we have handed out; when a
// chunk fills, new starts a fresh chunk and the old one stays reachable through
// the nodes that point into it.
//
// Chunks are plain GC-managed slices, not off-heap memory: a Node holds pointers
// (Children, Attrs, the strings), which the garbage collector must keep scanning.
// That rules out Go's experimental runtime arena, whose values are invisible to
// the GC. Chunk sizes grow geometrically from arenaMinChunk to arenaMaxChunk so
// small documents don't pay for a large first chunk while large documents still
// amortize down to a handful of allocations.
//
// An arena is not safe for concurrent use, but each Parse call owns its own, so
// concurrent Parse calls remain independent.
type nodeArena struct {
	chunk []Node
	next  int // capacity for the next chunk
}

const (
	arenaMinChunk = 32
	arenaMaxChunk = 2048
)

// new copies n into the arena and returns a stable pointer to the stored value.
func (a *nodeArena) new(n Node) *Node {
	if len(a.chunk) == cap(a.chunk) {
		size := a.next
		if size < arenaMinChunk {
			size = arenaMinChunk
		}
		a.chunk = make([]Node, 0, size)
		if size < arenaMaxChunk {
			a.next = size * 2
		}
	}
	a.chunk = append(a.chunk, n)
	return &a.chunk[len(a.chunk)-1]
}
