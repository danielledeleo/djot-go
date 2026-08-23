# v1 parser and typed AST refactor plan

Status: compact parser, lazy document, direct HTML, and typed Node milestones
complete; tiered extensions pending

Baseline: v0.3.2 (`c5c4ead`)

## Goal

Make a compact, immutable semantic event/tape representation the parser core,
and replace the tagged, universal `Node` struct with an optional materialized
view built from closed interfaces and focused concrete types.

The direct rendering path should consume semantic records without materializing
an AST. Extensions that require a subtree or document receive a typed tree only
for the required scope. The refactor should make invalid states harder to
represent, make node-specific APIs discoverable, and reduce retained memory
without giving up djot-go's traversal and transformation model.

No memory or speed improvement will be claimed until it is measured. The event
pipeline and typed AST are separate experiments: either must justify itself
before production parser code is converted.

## Architecture

The intended dependency direction is:

```text
block/inline scanners -> semantic tape + document indexes
                              |-> direct HTML/event consumers
                              |-> stream-local hooks
                              |-> typed subtree materializer
                              `-> typed document materializer
```

The semantic tape is a compact preorder sequence with subtree boundaries and
side tables for uncommon payloads, attributes, references, and footnotes. It is
not the public mutable AST and it must not contain renderer-specific HTML.

The benchmark-only first representation uses one 16-byte record per logical
node. A record stores its kind, compact flags/scalars, the first index after its
subtree, an optional payload index, and an attribute start index. Source spans
can be an optional parallel table: direct HTML does not need them, while AST and
source-mapping consumers do. This layout is provisional until direct rendering
proves that it contains every required semantic distinction.

### Prototype evidence (Go 1.27.0, Apple M5 Pro)

The first tape adapter mirrors the existing AST; it does not yet bypass AST
construction. On the 1 MB corpus it produces 137,095 records, 79,716 payloads,
and 1,752 ordered attributes:

| Measurement | Result |
| --- | ---: |
| Logical tape storage | 6,713,752 bytes |
| Retained tape capacity with source-size hints | 7,389,250 bytes |
| Build tape from an already parsed AST | 1.31 ms, 7,389,265 B/op, 4 allocs |
| Sequential tape scan | 117.8 us, 0 B/op, 0 allocs |
| Render HTML from current AST | 6.39 ms, 11,480,852 B/op, 39,917 allocs |
| Render HTML from prebuilt tape | 3.53 ms, 11,070,199 B/op, 7,065 allocs |

The tape renderer is about 45% faster than the current renderer on this corpus,
and the tape scan is about 5.6 times faster than the current pointer-tree walk.
The renderer covers tight lists, ordered attributes, image text, raw content,
references, deferred footnotes, and all node kinds. Its output is checked
byte-for-byte against the current renderer for every official HTML fixture.

These results establish that the compact representation can preserve rendering
semantics and is cheap to consume. They do **not** establish end-to-end parsing
speed: the adapter still pays for the old AST first. The next performance proof
must emit the tape directly from the parser and make AST construction an
optional consumer.

### Layout and scanner strategy matrix

Additional benchmark-only variants tested AoS versus SoA records, generic
versus kind-specific payloads, source-backed text ranges, and five inline text
scanners. Every scanner is checked against the current switch loop on prose,
dense markup, Unicode, and all byte values. Every payload renderer is checked
byte-for-byte against the production HTML renderer.

Payload layouts on the 1 MB corpus:

| Payload strategy | Logical tape storage | Rendering result |
| --- | ---: | ---: |
| Fat generic payload | 6,713,752 B | ~3.58 ms |
| Sparse generic payload | 4,800,576 B | Extra indirection |
| Sparse generic + source ranges | 4,408,136 B | Extra indirection |
| Kind-specific semantic union | 3,525,136 B | ~3.50–3.59 ms |
| Kind-specific + source ranges | 3,132,696 B | ~3.50–3.53 ms |

The kind-specific design gives `record.payload` a meaning determined by
`record.kind`: text nodes index a text table, links index destinations,
footnotes index labels, text-plus-metadata nodes index a focused pair, and an
ordered-list record stores its start value directly. It removes the generic
payload-ref object. Of the tested text payloads, 49,056 can reference exact
source ranges and 22,776 require stored values. The resulting 3.13 MB tape is
about 53% smaller than the first tape prototype and about 94% smaller than the
current retained AST, before output storage.

Record layout results:

| Scan | 16-byte AoS record | Pure SoA columns |
| --- | ---: | ---: |
| Kind-only | ~36.6 us | ~34.2 us |
| Full semantic scan | ~117.8 us | ~149.3 us |

SoA helps a narrow classification pass by roughly 7%, but hurts a consumer that
needs the full record by roughly 27%. Keep the compact AoS record with SoA-style
hot/cold side tables; do not split the record into columns.

Inline scanning microbenchmarks found:

- the current switch loop: ~95.7 us on long ASCII prose and ~130 us on dense
  markup;
- a 256-byte classification table: ~29.7 us on prose and ~70 us on dense
  markup;
- an eight-byte unrolled table: ~20.1 us on prose but ~107 us on dense markup;
- `strings.IndexAny`: competitive on long prose but ~483 us on dense markup;
- equality-based SWAR across all 21 delimiter bytes: ~136 us on prose and
  ~435 us on dense markup.

Despite winning the isolated scan, substituting the classification table in
the production parser regressed the mixed 1 MB parse by about 3% and the
delimiter-heavy parse by about 5–6%; it improved the attribute-heavy corpus by
about 2%. The change was removed. Retain the scanner matrix for reconsideration
after direct tape emission changes the overall cost distribution, but do not
optimize `parseText` now. In particular, naive multi-byte-equality SWAR is a
clear loss for Djot's large, irregular delimiter set.

### Production compact-document milestone

The parser no longer constructs exported `Node` values. It uses a private
128-byte common working node with 144-byte rare payloads allocated from typed
slabs, resolves document semantics there, and finalizes the winning 16-byte
record/kind-specific/source-range tape. The private workspace is released before
`Parse` returns.

`Doc.Root()`, `Doc.Footnotes()`, and `Doc.References()` now materialize the
legacy mutable Node view on demand. Default `RenderHTML` reads an untouched tape
directly. If the AST has been requested, an exact render-semantic comparison
detects mutation and selects the legacy renderer. Existing render options and
hooks currently select the legacy renderer directly; tiered tape-native hooks
remain the next public API milestone.

Lazy document state is synchronized, so concurrent read-only calls to `Root`,
`Footnotes`, `References`, and the renderers share one materialized view without
races. `NewDoc` constructs a document from an external tree, and `SetRoot`
provides the migration path for replacing the old exported `Root` field.

Final tape capacity comes from an exact counting walk over the completed private
parser tree, rather than a source-size estimate. This keeps dense finalization
allocation-efficient without overallocating for long, low-node-density prose.
Ordered-list starts live in a full-width `int` side table, and all narrowed tape
indexes and source positions are checked before conversion so oversized inputs
fail explicitly instead of silently corrupting records.

`RenderHTMLTo` consumes the tape incrementally and stops writing after the first
writer error. On a 10 MB single-paragraph document it uses 336 B/op and five
allocations when writing to `io.Discard`; the parsed document retains about
10.49 MB instead of the 47 MB retained by the source-size capacity heuristic.

Production results on the frozen 1 MB corpus:

| Benchmark | v0.3.2 baseline | Compact production path | Change |
| --- | ---: | ---: | ---: |
| Parse | 19.423 ms | 17.652 ms | 9% faster |
| Parse B/op | 54,860,160 | 43,593,137 | 21% lower |
| Parse allocs/op | 189,848 | 191,268 | 0.7% higher |
| RenderHTML | 6.492 ms | 2.848 ms | 56% faster |
| RenderHTML allocs/op | 39,917 | 6,190 | 84% lower |
| Parse + RenderHTML | ~24.4 ms | 20.988 ms | ~14% faster |
| Retained parsed document | 48,694,248 B | ~6,836,000 B | 86% lower |

The retained figure includes semantic records, source positions, source-backed
text tables, reference metadata, and the source file. Requesting `Root()` adds a
materialized legacy AST; the typed Node milestone will reduce that worst-case
representation.

All repository tests, official fixtures, race tests, vet checks, differential
mutation/fallback tests, and 10-second parser and tape-versus-AST fuzz runs pass.
The permanent parity fuzz target checks direct tape output byte-for-byte against
the materialized AST renderer.

The typed AST remains the public representation for inspection, mutation,
render-many workflows, and extensions that need arbitrary tree operations. It
is built mechanically from the same semantic records used by the fast renderer,
so there is one parser and one set of Djot semantics rather than a fast parser
and a customizable parser that can drift apart.

### Production typed-AST milestone

The universal exported struct has been replaced by the closed `Node`, `Block`,
and `Inline` interfaces and 43 focused concrete pointer types. Child storage is
grammar-specific (`[]Block`, `[]Inline`, `[]*ListItem`, `[]*TaskListItem`, and
`[]*TableCell`), while `Kind` remains available for generic tools and hook
registration. Source ranges live in `SourceSpan`; `Span` is the concrete Djot
inline-container type.

The six dominant concrete shapes use typed slabs during tape materialization.
Their shallow sizes are 72–104 bytes on arm64, down from 264 bytes for every
legacy node. `Attributes` is an ordered slice-backed value rather than a map
plus a separate order slice.

Measurements on the unchanged frozen corpora:

| Measurement | Legacy v0.3.2 | Typed production tree | Change |
| --- | ---: | ---: | ---: |
| Huge materialized document retained | 48,694,248 B | ~20,585,000 B | 58% lower |
| Attribute-heavy document retained | 2,620,704 B | ~1,227,000 B | 53% lower |
| Huge `Preorder` | n/a | ~0.79 ms, 0 allocs | allocation-free |
| Huge transforming `Walk` | ~0.666 ms, 0 allocs | ~1.00 ms, 0 allocs | interface dispatch cost |

The materialized figures include the compact tape retained by parsed documents;
the untouched fast path still retains only about 6.84 MB on the Huge corpus.
Parsing and default rendering do not materialize nodes, so their performance
remains governed by the compact milestone above. Typed rendering is selected
when a tree is constructed or mutated or a current render hook is registered.

`Walk` now visits and may replace or remove its supplied root, returns the new
root, and rejects category-invalid replacements. `Preorder` is the faster
read-only traversal. Generic `WithRenderer` infers a concrete node kind once at
registration, while `WithNodeRenderer` remains available for kind-driven code.

### Extension planning without capability declarations

Extensions select a constrained interface instead of asserting a capability
flag:

```go
OnEvent(func(Event) Event)             // local and streamable by construction
OnSubtree(func(Subtree) Subtree)       // materializes the selected subtree
OnDocument(func(*Document) *Document)  // materializes the entire document
```

The execution planner infers its plan from the registered hook types. A local
hook cannot accidentally inspect future descendants because they are absent
from its API. Advanced dynamic hooks may eventually request transactional
escalation, but output already committed to an `io.Writer` cannot be rewound;
that feature is not required for the initial v1 design.

## Previous public shape

Every AST element was a `*Node` tagged by `Node.Kind`. The struct contained common
tree metadata, parser-only state, and the payload fields for every node kind.

This produced a convenient homogeneous `[]*Node` tree and enabled a common
chunk allocator, but it also:

- exposes many meaningless zero-valued fields on every node;
- permits invalid combinations such as a text node with a list start;
- makes node-specific behavior discoverable only through documentation;
- exposes parser implementation details (`tight`, attribute ordering, and
  temporary text) through the layout of the public type;
- makes every node pay for every possible payload.

The affected public surface includes `Node`, `NodeKind`, `Doc.Root`,
`Doc.Footnotes`, `Doc.References`, `Walk`, `WalkBottomUp`, `Replace`, render
hooks, examples, and direct external AST construction.

## Proposed public model

### Small, closed interfaces

```go
type Node interface {
    Span() SourceSpan
    Attributes() *Attributes
    Kind() Kind
    node()
}

type Block interface {
    Node
    block()
}

type Inline interface {
    Node
    inline()
}
```

`node`, `block`, and `inline` are package-private marker methods. The renderer
and serializers therefore have a closed set of node types and can treat an
unknown implementation as an internal bug rather than an extension point.

`Kind` is retained for generic tooling, diagnostics, stable names, and hook
registration. Ordinary code should prefer type switches. Constants must be
renamed to avoid colliding with concrete types (`KindHeading`, `KindLink`, and
so on).

The interface should not grow tree mutation, sibling navigation, rendering, or
node-specific getters. Those belong in traversal functions or concrete types.

### Shared metadata

Concrete nodes embed an unexported base implementation:

```go
type nodeBase struct {
    span  SourceSpan
    attrs Attributes
}

type SourceSpan struct {
    Start Pos
    End   Pos
}
```

Keeping `nodeBase` private permits direct construction of useful zero-position
nodes while preventing source positions and attribute storage from becoming
layout commitments:

```go
text := &djot.Text{Value: "hello"}
strong := &djot.Strong{Children: []djot.Inline{text}}
strong.Attributes().Set("class", "warning")
```

All concrete node implementations are pointers. A typed nil must never be
stored in the tree.

### Attributes

Replace the public map plus private ordering slice with an ordered `Attributes`
value. The leading candidate is a small slice of `Attribute{Key, Value string}`
with `Get`, `Set`, `Delete`, `Len`, `Clone`, and iteration support.

Djot nodes normally have few attributes, so linear lookup may use less memory
than maintaining both a map and an order slice. This must be benchmarked with
attribute-heavy inputs. Attribute key validation remains centralized in `Set`.

### Concrete type taxonomy

Root:

- `Document { Children []Block }`

Block nodes:

- `Section`, `Paragraph`, `Heading`, `ThematicBreak`, `CodeBlock`, `RawBlock`
- `BlockQuote`, `Div`
- `BulletList`, `OrderedList`, `TaskList`, `ListItem`, `TaskListItem`
- `DefinitionList`, `Term`, `Definition`
- `Table`, `TableRow`, `TableCell`, `Caption`
- `Footnote`

Inline nodes:

- `Text`, `SoftBreak`, `HardBreak`, `NonBreakingSpace`
- `Emphasis`, `Strong`, `Superscript`, `Subscript`, `Insert`, `Delete`, `Mark`
- `Link`, `Image`, `Span`
- `Verbatim`, `InlineMath`, `DisplayMath`, `RawInline`, `Symbol`
- `FootnoteReference`, `DoubleQuoted`, `SingleQuoted`
- `Ellipsis`, `EmDash`, `EnDash`

Use typed child slices where the grammar is clear (`[]Block` or `[]Inline`).
Do not add a `Children() []Node` interface method: converting typed slices to
`[]Node` allocates, and leaf types should not pretend to be containers.

Representative payload names:

- `Text.Value`
- `Heading.Level`, `Heading.Children []Inline`
- `Link.Destination`, `Link.DestinationSet`, `Link.Children []Inline`
- `CodeBlock.Language`, `CodeBlock.Text`
- `RawBlock.Format`, `RawBlock.Text`
- `OrderedList.Style`, `OrderedList.Start`, `OrderedList.Tight`
- `TaskListItem.Checked`
- `TableCell.Alignment`, `TableCell.Header`
- `Footnote.Label`, `FootnoteReference.Label`
- `Symbol.Name`

`DestinationSet` preserves the current distinction between an absent target and
an explicitly empty target. Before freezing v1, consider a small optional value
type instead of a parallel boolean.

### Document indexes

Use specific types instead of generic nodes:

```go
type Doc struct {
    Root       *Document
    Files      []FileInfo
    Footnotes  map[string]*Footnote
    References map[string]*Reference
}

type Reference struct {
    Destination string
    Attributes  Attributes
}
```

Reference definitions are metadata, not `Link` nodes in the document tree.
Separating them removes the current synthetic-node special case.

## Traversal and transformation

Provide a simple allocation-free read API without raising the Go 1.22 minimum:

```go
func Preorder(root Node, visit func(Node) bool)
```

Retain an action-based mutation API, but replace the current `any` return with
one concrete action type:

```go
type Action struct { /* unexported */ }

var (
    Continue     Action
    SkipChildren Action
    Remove       Action
)

func Replace(Node) Action
func Walk(Node, func(Node) Action) Node
func WalkBottomUp(Node, func(Node))
```

`Walk` should document whether it visits the supplied root; the v1 API should
visit it for consistency with `Preorder`. Root removal/replacement needs either
a returned root or a separate `Transform` API. Decide this before implementation
rather than preserving the current accidental behavior where `Walk` only visits
children of its argument.

Replacement must preserve the grammar category of the occupied slot. Replacing
an `Inline` with a block should panic with a precise programmer-error message or
return an error. Prefer preventing the mismatch through separate internal block
and inline walkers where practical.

## Rendering hooks

The minimum compatible change is:

```go
type NodeRenderFunc func(Node, NodeRenderer)
func WithNodeRenderer(Kind, NodeRenderFunc) RenderOption
```

Investigate an inferred generic convenience:

```go
func WithRenderer[T Node](func(T, NodeRenderer)) RenderOption

djot.WithRenderer(func(n *djot.Symbol, r djot.NodeRenderer) {
    // n is already concrete.
})
```

Only ship the generic form if it can be implemented without surprising
reflection behavior, ambiguous registrations, or a second competing hook
system. The non-generic API remains sufficient and explicit.

`NodeRenderer.Children` and `Default` remain valuable and should keep their
current semantics.

## Compatibility requirements

The refactor may break Go source compatibility, but must preserve observable
Djot behavior:

- official specification AST output;
- HTML output, including footnote IDs and backlinks;
- text AST output and source-position formatting;
- JSON AST schema, key names, key order, and newline behavior;
- attribute insertion order;
- reference and heading resolution;
- section construction and auto IDs;
- render-hook fallback and child rendering semantics;
- all pathological and fuzz-test invariants.

A migration guide must cover:

- `n.Kind == KindLink` to `link, ok := n.(*Link)`;
- direct `Children` access and typed child slices;
- direct node construction;
- `Target` to `Destination`;
- attribute map access to `Attributes` methods;
- walker callback and replacement changes;
- render-hook registration changes;
- `Doc` root, footnote, and reference index types.

## Benchmark baseline and gates

Record baseline results on the same machine and Go version before code changes.
The current v0.3.2 snapshot was recorded on an Apple M5 Pro running
darwin/arm64 and Go 1.27.0. Median parse results from three serial one-second
runs are:

| Benchmark | Time | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: |
| Parse/Small | 3.020 us | 14,392 | 47 |
| Parse/Medium | 10.243 us | 38,752 | 154 |
| Parse/Huge_1MB | 19.423 ms | 54,860,160 | 189,848 |
| Parse/Attributes_100KB | 1.224 ms | 3,422,099 | 28,512 |
| RenderHTML/Huge_1MB | 6.492 ms | 11,480,843 | 39,917 |
| Walk/Huge_1MB | 665.936 us | 0 | 0 |

Representation baselines:

| Corpus | Nodes | Attributed nodes | Attributes | Retained bytes |
| --- | ---: | ---: | ---: | ---: |
| Huge_1MB | 137,095 | 1,752 | 1,752 | 48,694,248 |
| Attributes_100KB | 4,591 | 1,836 | 5,508 | 2,620,704 |

The current universal `Node` is 264 shallow bytes on arm64. Child backing
arrays, maps, and string storage are additional. Multiplying the shallow node
size by the Huge corpus node count accounts for roughly 36.2 MB before those
additional allocations, confirming that node layout is a primary retained-heap
cost rather than a speculative micro-optimization.

The largest node populations in the Huge corpus are `Text` (67,890), `Strong`
(15,768), `Paragraph` (11,826), `TableCell` (7,884), `ListItem` (6,570), and
`Link` (5,256). These six shapes account for about 84% of nodes and are the
first representation prototype set.

Add measurements for:

- concrete type sizes using `unsafe.Sizeof` in a benchmark-only report;
- total node count and count by concrete type;
- parse-only, render-only, parse-and-render, and traversal;
- attribute-heavy documents;
- retained heap after parsing, separate from temporary parser allocation;
- peak heap for large inputs;
- GC scan cost where it can be measured reproducibly.

Initial acceptance gates:

- `Preorder` and `Walk` remain zero-allocation after parsing;
- no more than a 10% parse-time regression on the 1 MB corpus;
- no more than a 10% render-time regression;
- retained AST memory improves materially (target at least 20%);
- parse `B/op` does not regress without an understood temporary-allocation
  tradeoff and a follow-up plan;
- all correctness and fuzz tests pass.

Allocation count may increase if retained memory decreases substantially, but a
one-allocation-per-node design should not be accepted without comparing typed
slab allocators.

## Implementation sequence

### 0. Freeze the baseline

1. Save benchmark output, Go version, OS, architecture, and commit.
2. Add node-count, retained-memory, and attribute-heavy benchmarks.
3. Add compile-time examples covering construction, inspection, replacement,
   attributes, and render hooks.

### 1. Validate representation choices

1. Prototype a compact semantic tape and measure build, retained storage, and
   sequential scan costs.
2. Render the official HTML fixtures from the tape byte-for-byte, including
   forward references, sections, attributes, and footnotes.
3. Prototype constrained event, subtree, and document hook registrations and
   infer an execution plan from their types.
4. Prototype representative typed leaf, inline-container, block-container, and
   rich payload types.
5. Compare ordinary allocation with family-specific typed slabs.
6. Prototype ordered-slice attributes against the current map/order pair.
7. Test typed traversal and tape-to-AST materialization without avoidable
   conversion allocations.
8. Stop and review results before converting the production parser.

### 2. Introduce the typed AST foundation

1. Add `SourceSpan`, `Attributes`, `Node`, `Block`, `Inline`, and concrete types.
2. Add compile-time interface assertions for every concrete type.
3. Add exhaustive internal child iteration and kind mapping.
4. Add constructors only where they materially improve common transformations;
   keep simple zero-value structs useful.

### 3. Convert behavior behind equivalence tests

1. Convert text and JSON AST renderers first; compare output byte-for-byte.
2. Convert HTML rendering and hook dispatch.
3. Convert section building, reference resolution, and footnote collection.
4. Convert traversal and replacement with block/inline category validation.
5. Convert block parsing, then inline parsing, using centralized node factories
   so allocation strategy remains replaceable.
6. Remove the temporary old-to-new adapter and old `Node` implementation.

### 4. Stabilize the public API

1. Rewrite package documentation and all examples using type switches.
2. Add the migration guide.
3. Run `go test`, `go test -race`, `go vet`, fuzz smoke tests, and benchmarks.
4. Review every exported identifier with `go doc -all`.
5. Tag a prerelease or v0.4 for external feedback before v1.0.0.

## Explicit non-goals

- Allowing third-party node implementations.
- Mixing parser internals into the public node interfaces.
- Reproducing a DOM-style parent/sibling mutation API.
- Changing the Djot syntax, HTML semantics, or serialized AST schema.
- Promising lower memory before retained-heap measurements exist.

## Resolved API decisions

1. Retain Go 1.22; `Preorder` takes a callback rather than exposing `iter.Seq`.
2. Keep `Kind()` on `Node` for diagnostics and generic tooling.
3. Use ordered `Attributes` with explicit `Range` and copying `Entries` APIs.
4. Preserve optional destinations with `Destination` plus `DestinationSet`.
5. Make `Walk` visit its root and return the replacement or nil on removal.
6. Use typed slabs for the six dominant shapes and ordinary allocation for
   rare types.
7. Ship generic `WithRenderer` alongside explicit `WithNodeRenderer`.
