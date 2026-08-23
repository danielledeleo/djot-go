package djot_test

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/danielledeleo/djot-go"
)

// This file is a representation experiment, not a proposed second AST. It
// mirrors a parsed tree into small interface-backed node shapes so heap and
// typed-slab allocation can be compared before production code is disturbed.

type prototypeNode interface {
	isPrototypeNode()
}

type prototypeAttribute struct {
	key   string
	value string
}

type prototypeBase struct {
	start djot.Pos
	end   djot.Pos
	attrs []prototypeAttribute
}

func (*prototypeBase) isPrototypeNode() {}

// The six explicit shapes below account for about 84% of hugeDoc's nodes.

type prototypeText struct {
	prototypeBase
	value string
}

type prototypeStrong struct {
	prototypeBase
	children []prototypeNode
}

type prototypeParagraph struct {
	prototypeBase
	children []prototypeNode
}

type prototypeTableCell struct {
	prototypeBase
	children  []prototypeNode
	alignment djot.CellAlign
	header    bool
}

type prototypeListItem struct {
	prototypeBase
	children []prototypeNode
}

type prototypeLink struct {
	prototypeBase
	children       []prototypeNode
	destination    string
	destinationSet bool
}

// Remaining kinds are represented by payload family rather than by semantic
// type. This avoids undercounting their data while keeping the experiment
// focused on the dominant concrete shapes.

type prototypeLeaf struct {
	prototypeBase
}

type prototypeTextLeaf struct {
	prototypeBase
	value string
}

type prototypeBlockText struct {
	prototypeBase
	value string
	extra string
}

type prototypeContainer struct {
	prototypeBase
	children []prototypeNode
}

type prototypeRichContainer struct {
	prototypeBase
	children []prototypeNode
	text     string
	first    int
	second   int
	flag     bool
}

type prototypeSlab[T any] struct {
	chunk []T
	next  int
}

func (a *prototypeSlab[T]) alloc() *T {
	if len(a.chunk) == cap(a.chunk) {
		size := a.next
		if size < arenaMinChunkPrototype {
			size = arenaMinChunkPrototype
		}
		a.chunk = make([]T, 0, size)
		if size < arenaMaxChunkPrototype {
			a.next = size * 2
		}
	}
	a.chunk = append(a.chunk, *new(T))
	return &a.chunk[len(a.chunk)-1]
}

const (
	// Per-type slabs start smaller than the current universal arena. Rare node
	// kinds should not each reserve 32 slots, while dominant kinds quickly grow
	// through the same geometric sequence.
	arenaMinChunkPrototype = 4
	arenaMaxChunkPrototype = 2048
)

type prototypeArena struct {
	dominantOnly  bool
	text          prototypeSlab[prototypeText]
	strong        prototypeSlab[prototypeStrong]
	paragraph     prototypeSlab[prototypeParagraph]
	tableCell     prototypeSlab[prototypeTableCell]
	listItem      prototypeSlab[prototypeListItem]
	link          prototypeSlab[prototypeLink]
	leaf          prototypeSlab[prototypeLeaf]
	textLeaf      prototypeSlab[prototypeTextLeaf]
	blockText     prototypeSlab[prototypeBlockText]
	container     prototypeSlab[prototypeContainer]
	richContainer prototypeSlab[prototypeRichContainer]
}

type prototypeAllocation uint8

const (
	prototypeHeap prototypeAllocation = iota
	prototypeDominantSlabs
	prototypeAllSlabs
)

func newPrototypeArena(mode prototypeAllocation) *prototypeArena {
	if mode == prototypeHeap {
		return nil
	}
	return &prototypeArena{dominantOnly: mode == prototypeDominantSlabs}
}

func prototypeBaseFrom(src *djot.Node) prototypeBase {
	base := prototypeBase{start: src.Start, end: src.End}
	if len(src.Attrs) > 0 {
		base.attrs = make([]prototypeAttribute, 0, len(src.Attrs))
		for key, value := range src.Attrs {
			base.attrs = append(base.attrs, prototypeAttribute{key: key, value: value})
		}
	}
	return base
}

func prototypeChildren(src *djot.Node, arena *prototypeArena) []prototypeNode {
	if len(src.Children) == 0 {
		return nil
	}
	children := make([]prototypeNode, len(src.Children))
	for i, child := range src.Children {
		children[i] = convertPrototypeNode(child, arena)
	}
	return children
}

// A nil arena selects ordinary heap allocation. A non-nil arena selects a
// separate geometrically grown slab for each concrete representation.
func convertPrototypeNode(src *djot.Node, arena *prototypeArena) prototypeNode {
	base := prototypeBaseFrom(src)

	switch src.Kind {
	case djot.Text:
		var dst *prototypeText
		if arena == nil {
			dst = new(prototypeText)
		} else {
			dst = arena.text.alloc()
		}
		dst.prototypeBase = base
		dst.value = src.Text
		return dst

	case djot.Strong:
		var dst *prototypeStrong
		if arena == nil {
			dst = new(prototypeStrong)
		} else {
			dst = arena.strong.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		return dst

	case djot.Paragraph:
		var dst *prototypeParagraph
		if arena == nil {
			dst = new(prototypeParagraph)
		} else {
			dst = arena.paragraph.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		return dst

	case djot.TableCell:
		var dst *prototypeTableCell
		if arena == nil {
			dst = new(prototypeTableCell)
		} else {
			dst = arena.tableCell.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		dst.alignment = src.CellAlign
		dst.header = src.IsHeader
		return dst

	case djot.ListItem:
		var dst *prototypeListItem
		if arena == nil {
			dst = new(prototypeListItem)
		} else {
			dst = arena.listItem.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		return dst

	case djot.Link:
		var dst *prototypeLink
		if arena == nil {
			dst = new(prototypeLink)
		} else {
			dst = arena.link.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		dst.destination = src.Target
		dst.destinationSet = src.HasTarget
		return dst

	case djot.ThematicBreak, djot.SoftBreak, djot.HardBreak,
		djot.NonBreakingSpace, djot.Ellipsis, djot.EmDash, djot.EnDash:
		var dst *prototypeLeaf
		if arena == nil || arena.dominantOnly {
			dst = new(prototypeLeaf)
		} else {
			dst = arena.leaf.alloc()
		}
		dst.prototypeBase = base
		return dst

	case djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol,
		djot.FootnoteReference:
		var dst *prototypeTextLeaf
		if arena == nil || arena.dominantOnly {
			dst = new(prototypeTextLeaf)
		} else {
			dst = arena.textLeaf.alloc()
		}
		dst.prototypeBase = base
		switch src.Kind {
		case djot.Symbol:
			dst.value = src.Name
		case djot.FootnoteReference:
			dst.value = src.Label
		default:
			dst.value = src.Text
		}
		return dst

	case djot.CodeBlock, djot.RawBlock, djot.RawInline:
		var dst *prototypeBlockText
		if arena == nil || arena.dominantOnly {
			dst = new(prototypeBlockText)
		} else {
			dst = arena.blockText.alloc()
		}
		dst.prototypeBase = base
		dst.value = src.Text
		if src.Kind == djot.CodeBlock {
			dst.extra = src.Lang
		} else {
			dst.extra = src.Format
		}
		return dst

	case djot.Document, djot.Section, djot.BlockQuote, djot.Div,
		djot.Emphasis, djot.Superscript, djot.Subscript, djot.Insert,
		djot.Delete, djot.Mark, djot.Span, djot.DoubleQuoted,
		djot.SingleQuoted:
		var dst *prototypeContainer
		if arena == nil || arena.dominantOnly {
			dst = new(prototypeContainer)
		} else {
			dst = arena.container.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		return dst

	default:
		var dst *prototypeRichContainer
		if arena == nil || arena.dominantOnly {
			dst = new(prototypeRichContainer)
		} else {
			dst = arena.richContainer.alloc()
		}
		dst.prototypeBase = base
		dst.children = prototypeChildren(src, arena)
		dst.text = src.Label
		dst.first = src.Level
		dst.second = src.ListStart
		dst.flag = src.Checked || src.IsHeader
		return dst
	}
}

func prototypeNodeChildren(node prototypeNode) []prototypeNode {
	switch node := node.(type) {
	case *prototypeStrong:
		return node.children
	case *prototypeParagraph:
		return node.children
	case *prototypeTableCell:
		return node.children
	case *prototypeListItem:
		return node.children
	case *prototypeLink:
		return node.children
	case *prototypeContainer:
		return node.children
	case *prototypeRichContainer:
		return node.children
	default:
		return nil
	}
}

func walkPrototype(node prototypeNode) int {
	count := 1
	for _, child := range prototypeNodeChildren(node) {
		count += walkPrototype(child)
	}
	return count
}

func BenchmarkTypedASTPrototypeSizes(b *testing.B) {
	b.ReportMetric(float64(unsafe.Sizeof(prototypeText{})), "text-B")
	b.ReportMetric(float64(unsafe.Sizeof(prototypeStrong{})), "strong-B")
	b.ReportMetric(float64(unsafe.Sizeof(prototypeParagraph{})), "paragraph-B")
	b.ReportMetric(float64(unsafe.Sizeof(prototypeTableCell{})), "table-cell-B")
	b.ReportMetric(float64(unsafe.Sizeof(prototypeListItem{})), "list-item-B")
	b.ReportMetric(float64(unsafe.Sizeof(prototypeLink{})), "link-B")
	for i := 0; i < b.N; i++ {
	}
}

func BenchmarkTypedASTPrototypeConvert(b *testing.B) {
	src := djot.Parse(hugeDoc())
	for _, tc := range []struct {
		name string
		mode prototypeAllocation
	}{
		{"Heap", prototypeHeap},
		{"DominantSlabs", prototypeDominantSlabs},
		{"AllSlabs", prototypeAllSlabs},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				arena := newPrototypeArena(tc.mode)
				root := convertPrototypeNode(src.Root(), arena)
				runtime.KeepAlive(root)
				runtime.KeepAlive(arena)
			}
		})
	}
}

func BenchmarkTypedASTPrototypeWalk(b *testing.B) {
	src := djot.Parse(hugeDoc())
	for _, tc := range []struct {
		name string
		mode prototypeAllocation
	}{
		{"Heap", prototypeHeap},
		{"DominantSlabs", prototypeDominantSlabs},
		{"AllSlabs", prototypeAllSlabs},
	} {
		b.Run(tc.name, func(b *testing.B) {
			arena := newPrototypeArena(tc.mode)
			root := convertPrototypeNode(src.Root(), arena)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := walkPrototype(root); got != 137095 {
					b.Fatalf("walked %d nodes, want 137095", got)
				}
			}
			runtime.KeepAlive(arena)
		})
	}
}

func BenchmarkTypedASTPrototypeRetained(b *testing.B) {
	input := hugeDoc()
	for _, tc := range []struct {
		name string
		mode prototypeAllocation
	}{
		{"Heap", prototypeHeap},
		{"DominantSlabs", prototypeDominantSlabs},
		{"AllSlabs", prototypeAllSlabs},
	} {
		b.Run(tc.name, func(b *testing.B) {
			var retained uint64
			for i := 0; i < b.N; i++ {
				retained += measureRetainedPrototype(input, tc.mode)
			}
			b.ReportMetric(float64(retained)/float64(b.N), "retained-B/doc")
		})
	}
}

func measureRetainedPrototype(input string, mode prototypeAllocation) uint64 {
	// Warm conversion and runtime paths before taking the heap baseline.
	warmDoc := djot.Parse(input)
	warmArena := newPrototypeArena(mode)
	warmRoot := convertPrototypeNode(warmDoc.Root(), warmArena)
	runtime.KeepAlive(warmRoot)
	runtime.KeepAlive(warmArena)
	warmRoot = nil
	warmDoc = nil
	warmArena = nil
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	src := djot.Parse(input)
	arena := newPrototypeArena(mode)
	root := convertPrototypeNode(src.Root(), arena)
	src = nil
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(root)
	runtime.KeepAlive(arena)

	if after.HeapAlloc <= before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}
