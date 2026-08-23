package djot

import (
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
)

var errNilHTMLWriter = errors.New("djot: RenderHTMLTo called with a nil writer")

// RenderOption configures HTML rendering. Pass to [RenderHTML] or [RenderHTMLTo].
type RenderOption func(*renderConfig)

// NodeRenderFunc is a hook that overrides rendering for a specific node kind.
// It receives the node being rendered and a [NodeRenderer] for controlling output.
type NodeRenderFunc func(n Node, r NodeRenderer)

// NodeRenderer provides methods for controlling output from within a render hook.
//
//   - [NodeRenderer.Write] emits raw HTML to the output.
//   - [NodeRenderer.Children] renders the node's children through the full
//     pipeline (including any hooks registered for child node kinds),
//     without emitting the node's own wrapper element.
//   - [NodeRenderer.Default] renders the node exactly as if no hook were
//     registered. Calling Default does not re-trigger the hook.
type NodeRenderer interface {
	// Write emits a raw HTML string to the output.
	Write(s string)
	// Children renders this node's children through the full rendering
	// pipeline, including any hooks for child node kinds.
	Children()
	// Default renders this node using the built-in renderer, as if no
	// hook were registered. Does not re-trigger the hook.
	Default()
}

// SymbolView is a value view of a Djot symbol. Changing the value does not
// modify the parsed document.
type SymbolView struct {
	Name string
}

// SymbolRenderFunc overrides rendering for a symbol without requiring callers
// to materialize or traverse the typed AST. Use the renderer to write a
// replacement or call [ElementRenderer.Default] to preserve the built-in
// :name: rendering. Returning without doing either suppresses the symbol.
type SymbolRenderFunc func(symbol SymbolView, renderer ElementRenderer)

// AttributeView is a read-only, allocation-free view of an element's ordered
// attributes. The view is valid only for the duration of its render callback.
type AttributeView struct {
	tape       *semanticTape
	attributes *Attributes
	start      uint32
	end        uint32
}

// Len returns the number of attributes.
func (a AttributeView) Len() int {
	if a.tape != nil {
		return int(a.end - a.start)
	}
	return a.attributes.Len()
}

// Get returns the value associated with key, or an empty string when absent.
func (a AttributeView) Get(key string) string {
	value, _ := a.Lookup(key)
	return value
}

// Lookup returns the value associated with key and whether it is present.
func (a AttributeView) Lookup(key string) (string, bool) {
	if a.tape != nil {
		for i := a.start; i < a.end; i++ {
			attribute := a.tape.attributes[i]
			if attribute.key == key {
				return attribute.value, true
			}
		}
		return "", false
	}
	return a.attributes.Lookup(key)
}

// Range calls fn for each attribute in source order, stopping when fn returns
// false.
func (a AttributeView) Range(fn func(Attribute) bool) {
	if a.tape != nil {
		for i := a.start; i < a.end; i++ {
			attribute := a.tape.attributes[i]
			if !fn(Attribute{Key: attribute.key, Value: attribute.value}) {
				return
			}
		}
		return
	}
	a.attributes.Range(fn)
}

// DivView is a value view of a Djot div. It exposes local metadata while
// [ElementRenderer.Children] streams the div's existing children.
type DivView struct {
	attributes AttributeView
	span       SourceSpan
}

// Attributes returns the div's read-only ordered attributes.
func (d DivView) Attributes() AttributeView { return d.attributes }

// Span returns the div's half-open source range.
func (d DivView) Span() SourceSpan { return d.span }

// DivRenderFunc overrides rendering for a div without materializing the typed
// AST. Call [ElementRenderer.Children] to render its children without the
// built-in div wrapper, or [ElementRenderer.Default] to keep that wrapper.
type DivRenderFunc func(div DivView, renderer ElementRenderer)

// ElementView is a compact, read-only view used while inspecting a subtree.
// It exposes metadata shared by every Djot element without exposing mutable
// Nodes. The value is valid only during its subtree callback.
type ElementView struct {
	tape   *semanticTape
	node   Node
	record int
}

// Kind returns the element kind.
func (e ElementView) Kind() Kind {
	if e.tape != nil {
		return Kind(e.tape.records[e.record].kind)
	}
	if e.node == nil {
		return KindDocument
	}
	return e.node.Kind()
}

// Span returns the element's half-open source range.
func (e ElementView) Span() SourceSpan {
	if e.tape != nil {
		position := e.tape.positions[e.record]
		return SourceSpan{
			Start: Pos{Offset: int(position.start)},
			End:   Pos{Offset: int(position.end)},
		}
	}
	if e.node == nil {
		return SourceSpan{}
	}
	return e.node.Span()
}

// Attributes returns the element's read-only ordered attributes.
func (e ElementView) Attributes() AttributeView {
	if e.tape != nil {
		record := e.tape.records[e.record]
		return AttributeView{
			tape: e.tape, start: record.attrStart, end: e.tape.records[e.record+1].attrStart,
		}
	}
	if e.node == nil {
		return AttributeView{}
	}
	return AttributeView{attributes: e.node.Attributes()}
}

// Symbol returns the symbol view when the element is a symbol.
func (e ElementView) Symbol() (SymbolView, bool) {
	if e.Kind() != KindSymbol {
		return SymbolView{}, false
	}
	if e.tape != nil {
		return SymbolView{Name: e.tape.text(e.tape.records[e.record].payload)}, true
	}
	return SymbolView{Name: e.node.(*Symbol).Name}, true
}

// SubtreeView is a bounded, read-only view of one element and all its
// descendants. Traversal reads the compact semantic tape when possible and
// never materializes Nodes solely for inspection. The view is valid only for
// the duration of its render callback.
type SubtreeView struct {
	root ElementView
}

// Root returns the element at the root of the bounded subtree.
func (s SubtreeView) Root() ElementView { return s.root }

// Preorder visits the root and its descendants in document order. Returning
// false stops the traversal.
func (s SubtreeView) Preorder(visit func(ElementView) bool) {
	if s.root.tape != nil {
		end := int(s.root.tape.records[s.root.record].subtreeEnd)
		for i := s.root.record; i < end; i++ {
			if !visit(ElementView{tape: s.root.tape, record: i}) {
				return
			}
		}
		return
	}
	if s.root.node != nil {
		Preorder(s.root.node, func(node Node) bool {
			return visit(ElementView{node: node})
		})
	}
}

// Descendants visits descendants in document order, excluding the subtree
// root. Returning false stops the traversal.
func (s SubtreeView) Descendants(visit func(ElementView) bool) {
	if s.root.tape != nil {
		end := int(s.root.tape.records[s.root.record].subtreeEnd)
		for i := s.root.record + 1; i < end; i++ {
			if !visit(ElementView{tape: s.root.tape, record: i}) {
				return
			}
		}
		return
	}
	if s.root.node != nil {
		keepGoing := true
		forEachChild(s.root.node, func(child Node) {
			if keepGoing {
				Preorder(child, func(node Node) bool {
					keepGoing = visit(ElementView{node: node})
					return keepGoing
				})
			}
		})
	}
}

// Contains reports whether a descendant has kind. The subtree root itself is
// excluded.
func (s SubtreeView) Contains(kind Kind) bool {
	if s.root.tape != nil {
		end := int(s.root.tape.records[s.root.record].subtreeEnd)
		for i := s.root.record + 1; i < end; i++ {
			if Kind(s.root.tape.records[i].kind) == kind {
				return true
			}
		}
		return false
	}
	found := false
	s.Descendants(func(element ElementView) bool {
		found = element.Kind() == kind
		return !found
	})
	return found
}

// SubtreeRenderFunc overrides rendering after inspecting a bounded subtree.
// The view is read-only; use [ElementRenderer.Children] to replay its children
// through the complete hook pipeline.
type SubtreeRenderFunc func(subtree SubtreeView, renderer ElementRenderer)

// ElementRenderer controls output from a compact element rendering hook. It
// uses the semantic tape when possible and the typed tree when required. The
// value is valid only during its callback; its zero value emits no output.
type ElementRenderer struct {
	semantic *semanticHTMLRenderer
	tree     *htmlRenderer
	record   int
	node     Node
}

// Write emits raw HTML.
func (r ElementRenderer) Write(s string) {
	if r.semantic != nil {
		r.semantic.write(s)
	} else if r.tree != nil {
		r.tree.write(s)
	}
}

// Children renders the element's children through the complete hook pipeline.
func (r ElementRenderer) Children() {
	if r.semantic != nil {
		r.semantic.renderChildren(r.record)
	} else if r.tree != nil && r.node != nil {
		forEachChild(r.node, r.tree.renderNode)
	}
}

// Default renders the element using the built-in renderer without invoking
// its hook again.
func (r ElementRenderer) Default() {
	if r.semantic != nil {
		r.semantic.renderDefault(r.record)
	} else if r.tree != nil && r.node != nil {
		r.tree.renderDefault(r.node)
	}
}

type renderConfig struct {
	hooks                 map[Kind]NodeRenderFunc
	elements              elementHooks
	subtrees              map[Kind]SubtreeRenderFunc
	document              DocumentRenderFunc
	multiBacklinks        bool
	footnoteID            func(num int) string
	footnoteRefID         func(num, k int) string
	footnoteBacklinkLabel func(num, k, total int) string
}

type elementHooks struct {
	symbol SymbolRenderFunc
	div    DivRenderFunc
}

// Default footnote id/label producers. Callers can override each independently
// via the WithFootnote* options; these reproduce djot-go's standard output.

func defaultFootnoteID(num int) string { return "fn" + strconv.Itoa(num) }

func defaultFootnoteRefID(num, k int) string { return fnrefID(num, k) }

func defaultFootnoteBacklinkLabel(_, k, total int) string {
	if total <= 1 {
		return "↩︎"
	}
	return backrefLabel(k)
}

// WithFootnoteID overrides the id of a footnote definition — the value used in
// both the <li id="…"> of the endnotes list and the "#…" target of every
// reference to that footnote. num is the footnote's 1-based number. The default
// is fmt.Sprintf("fn%d", num). Use this (with [WithFootnoteRefID]) to namespace
// footnote ids when embedding rendered output in a larger page; see also the
// [WithFootnotePrefix] convenience.
func WithFootnoteID(fn func(num int) string) RenderOption {
	return func(cfg *renderConfig) { cfg.footnoteID = fn }
}

// WithFootnoteRefID overrides the back-anchor id of the k-th reference (1-based)
// to footnote num — the id on the reference's <a> and the "#…" target of the
// matching backlink. The default is "fnrefN" for the first reference and
// "fnrefN-k" for later ones. Both ends of the link use this function, so they
// always agree.
func WithFootnoteRefID(fn func(num, k int) string) RenderOption {
	return func(cfg *renderConfig) { cfg.footnoteRefID = fn }
}

// WithFootnoteBacklinkLabel overrides the visible text of the k-th backlink for
// a footnote, where total is the number of backlinks that footnote has (1 unless
// [WithMultiBacklinks] is set). The default is "↩︎" when total is 1 and the
// letters a, b, c, … otherwise. Use this for numeric or other label schemes.
func WithFootnoteBacklinkLabel(fn func(num, k, total int) string) RenderOption {
	return func(cfg *renderConfig) { cfg.footnoteBacklinkLabel = fn }
}

// WithFootnotePrefix is a convenience that namespaces every footnote id by
// prefixing the defaults: footnote ids become prefix+"fnN" and reference ids
// prefix+"fnrefN"(-k). It is shorthand for setting [WithFootnoteID] and
// [WithFootnoteRefID] together, useful when embedding output in a page that may
// already use fn/fnref ids. A later WithFootnoteID/WithFootnoteRefID option
// overrides the corresponding half.
func WithFootnotePrefix(prefix string) RenderOption {
	return func(cfg *renderConfig) {
		cfg.footnoteID = func(num int) string { return prefix + defaultFootnoteID(num) }
		cfg.footnoteRefID = func(num, k int) string { return prefix + defaultFootnoteRefID(num, k) }
	}
}

// WithMultiBacklinks renders MediaWiki-style backlinks for footnotes that are
// referenced more than once: every reference gets a unique id (fnref1, fnref1-2,
// fnref1-3, …) and the footnote's entry links back to each reference with
// lettered backlinks (a, b, c, …) instead of a single ↩︎ to the first reference.
// Footnotes referenced only once are unchanged (a single ↩︎ backlink).
//
// Without this option the renderer matches djot.js: only the first reference
// carries the id (so the HTML has no duplicate ids) and the footnote has one
// backlink, pointing to that first reference.
func WithMultiBacklinks() RenderOption {
	return func(cfg *renderConfig) {
		cfg.multiBacklinks = true
	}
}

// WithNodeRenderer registers a render hook for the given node kind.
// The hook receives the [Node] and a [NodeRenderer] for full control over output.
// If called multiple times for the same kind, the last one wins.
//
// Use this when you need access to [NodeRenderer.Children] or [NodeRenderer.Default].
// For a symbol hook that can avoid AST materialization, see [WithSymbolRenderer].
func WithNodeRenderer(kind Kind, fn NodeRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		if cfg.hooks == nil {
			cfg.hooks = make(map[Kind]NodeRenderFunc)
		}
		cfg.hooks[kind] = fn
		delete(cfg.subtrees, kind)
		switch kind {
		case KindSymbol:
			cfg.elements.symbol = nil
		case KindDiv:
			cfg.elements.div = nil
		}
	}
}

// WithSymbolRenderer registers a symbol rendering hook. Parser-produced,
// unmodified documents invoke the hook directly from the compact semantic
// representation. If a caller has changed the typed tree, the same hook runs
// against that tree so mutations remain authoritative.
//
// Hooks write raw HTML. Call [ElementRenderer.Default] to retain the built-in
// rendering for a symbol. If multiple symbol or Node hooks for [KindSymbol] are
// registered, the last one wins.
func WithSymbolRenderer(fn SymbolRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		cfg.elements.symbol = fn
		delete(cfg.hooks, KindSymbol)
		delete(cfg.subtrees, KindSymbol)
	}
}

// WithDivRenderer registers a streaming div rendering hook. The callback can
// inspect the div's attributes and source span, write a replacement wrapper,
// and call [ElementRenderer.Children] to stream its children through all
// registered hooks. Call [ElementRenderer.Default] to retain the built-in div
// wrapper. Returning without writing or rendering children suppresses the div
// and its contents.
//
// Parser-produced, unmodified documents invoke the hook directly from the
// compact semantic representation. Mutated or externally constructed trees
// use the same hook through the tree renderer. If multiple Div, subtree, or
// Node hooks for [KindDiv] are registered, the last one wins.
func WithDivRenderer(fn DivRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		cfg.elements.div = fn
		delete(cfg.hooks, KindDiv)
		delete(cfg.subtrees, KindDiv)
	}
}

// WithSubtreeRenderer registers a read-only subtree rendering hook for kind.
// Unlike a streaming element hook, the callback may inspect descendants before
// emitting output. Inspection traverses a bounded semantic-tape range and does
// not materialize the typed AST for an otherwise untouched parsed document.
//
// Call [ElementRenderer.Default] for built-in rendering or
// [ElementRenderer.Children] to stream the existing children without their
// root wrapper. Returning without either suppresses the complete subtree. A
// later subtree, specialized element, or Node hook for the same kind wins.
func WithSubtreeRenderer(kind Kind, fn SubtreeRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		if cfg.subtrees == nil {
			cfg.subtrees = make(map[Kind]SubtreeRenderFunc)
		}
		cfg.subtrees[kind] = fn
		delete(cfg.hooks, kind)
		switch kind {
		case KindSymbol:
			cfg.elements.symbol = nil
		case KindDiv:
			cfg.elements.div = nil
		}
	}
}

// WithDocumentRenderer registers a whole-document rendering hook. The callback
// can inspect focused indexes and summaries such as [DocumentView.Headings] and
// [DocumentView.Contains] before writing.
// Call [DocumentRenderer.Default] to emit the normal document, including
// endnotes, through the remaining hooks. Returning without writing or calling
// Default suppresses all output.
//
// Parser-produced, unmodified documents use the compact semantic tape. Mutated
// or externally constructed trees provide the same view through the tree
// backend. If registered more than once, the last document hook wins.
func WithDocumentRenderer(fn DocumentRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		cfg.document = fn
	}
}

// WithRenderer registers a type-safe render hook. T must be one concrete node
// pointer type; the kind is inferred once when the option is constructed.
//
//	djot.WithRenderer(func(symbol *djot.Symbol, r djot.NodeRenderer) {
//	    r.Write(symbol.Name)
//	})
func WithRenderer[T Node](fn func(T, NodeRenderer)) RenderOption {
	var zero T
	if reflect.TypeOf(zero) == nil {
		panic("djot: WithRenderer requires a concrete node pointer type")
	}
	kind := zero.Kind()
	return WithNodeRenderer(kind, func(node Node, renderer NodeRenderer) {
		typed, ok := node.(T)
		if !ok {
			panic("djot: inferred renderer kind does not match concrete node type")
		}
		fn(typed, renderer)
	})
}

// WithRenderFunc registers a simple render hook for the given node kind.
// The function receives the node and returns an HTML string to emit.
// If it returns an empty string, the default rendering is used.
//
// This is convenient for leaf nodes like [KindSymbol] where you don't need
// [NodeRenderer.Children] or [NodeRenderer.Default]:
//
//	html := RenderHTML(doc, WithRenderFunc(KindSymbol, func(n Node) string {
//	    if symbol, ok := n.(*Symbol); ok && symbol.Name == "star" {
//	        return "⭐"
//	    }
//	    return "" // fall through to default
//	}))
func WithRenderFunc(kind Kind, fn func(n Node) string) RenderOption {
	return WithNodeRenderer(kind, func(n Node, r NodeRenderer) {
		if s := fn(n); s != "" {
			r.Write(s)
			return
		}
		r.Default()
	})
}

// RenderHTML renders a parsed document to an HTML string. Optional
// [RenderOption] values can customize rendering through element, subtree,
// document, and Node hooks.
func RenderHTML(doc *Doc, opts ...RenderOption) string {
	if len(opts) == 0 {
		tape, root, direct := doc.semanticRenderSnapshot()
		if tape != nil && (direct || tape.matchesAST(root)) {
			return renderSemanticHTML(tape)
		}
	}
	cfg := makeRenderConfig(opts)
	if cfg.canRenderSemantic() {
		tape, root, direct := doc.semanticRenderSnapshot()
		if tape != nil && (direct || tape.matchesAST(root)) {
			return renderSemanticHTMLWithHooks(tape, semanticRenderHooks{
				elements: cfg.elements, subtrees: cfg.subtrees, document: cfg.document,
			})
		}
	}
	var b strings.Builder
	r := newHTMLRendererWithConfig(&b, doc, cfg)
	r.renderDocument()
	return b.String()
}

// RenderHTMLTo renders a parsed document as HTML to the given writer.
// It returns the first write error encountered, if any.
func RenderHTMLTo(w io.Writer, doc *Doc, opts ...RenderOption) error {
	if w == nil {
		return errNilHTMLWriter
	}
	if len(opts) == 0 {
		tape, root, direct := doc.semanticRenderSnapshot()
		if tape != nil && (direct || tape.matchesAST(root)) {
			return renderSemanticHTMLTo(w, tape)
		}
	}
	cfg := makeRenderConfig(opts)
	if cfg.canRenderSemantic() {
		tape, root, direct := doc.semanticRenderSnapshot()
		if tape != nil && (direct || tape.matchesAST(root)) {
			return renderSemanticHTMLToWithHooks(w, tape, semanticRenderHooks{
				elements: cfg.elements, subtrees: cfg.subtrees, document: cfg.document,
			})
		}
	}
	r := newHTMLRendererWithConfig(w, doc, cfg)
	r.renderDocument()
	return r.err
}

func makeRenderConfig(opts []RenderOption) renderConfig {
	var cfg renderConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (cfg *renderConfig) canRenderSemantic() bool {
	return len(cfg.hooks) == 0 && !cfg.multiBacklinks && cfg.footnoteID == nil &&
		cfg.footnoteRefID == nil && cfg.footnoteBacklinkLabel == nil
}

type footnoteInfo struct {
	num   int
	label string
	node  *Footnote // may be nil if undefined
}

type htmlRenderer struct {
	w   io.Writer
	err error
	doc *Doc

	hooks    map[Kind]NodeRenderFunc
	elements elementHooks
	subtrees map[Kind]SubtreeRenderFunc
	document DocumentRenderFunc

	// tight tracks whether we are rendering inside a tight list/definition list.
	// Set by the list container before iterating children and restored after,
	// so list-item and definition default cases can render correctly.
	tight bool

	// Footnote definitions derived from the AST at render time.
	// This ensures correctness even after AST mutations (e.g., include/splice).
	footnotes map[string]*Footnote
	// Footnote numbering: label → sequential number
	footnoteNums map[string]int
	// Ordered list of referenced footnotes (by first reference order)
	footnoteOrder []*footnoteInfo
	// Counter for assigning numbers
	nextFootnoteNum int
	// Total references per label, counted up front (for multi-backlink labels)
	fnrefTotal map[string]int
	// References to each label seen so far at render time (for per-ref ids)
	fnrefSeen map[string]int
	// When true, emit a unique id per reference and lettered backlinks
	multiBacklinks bool
	// Footnote id/label producers (resolved to defaults when not overridden)
	footnoteID            func(num int) string
	footnoteRefID         func(num, k int) string
	footnoteBacklinkLabel func(num, k, total int) string
}

func newHTMLRenderer(w io.Writer, doc *Doc, opts ...RenderOption) *htmlRenderer {
	return newHTMLRendererWithConfig(w, doc, makeRenderConfig(opts))
}

func newHTMLRendererWithConfig(w io.Writer, doc *Doc, cfg renderConfig) *htmlRenderer {
	r := &htmlRenderer{
		w:               w,
		doc:             doc,
		hooks:           cfg.hooks,
		elements:        cfg.elements,
		subtrees:        cfg.subtrees,
		document:        cfg.document,
		footnotes:       make(map[string]*Footnote),
		footnoteNums:    make(map[string]int),
		nextFootnoteNum: 1,
		fnrefTotal:      make(map[string]int),
		fnrefSeen:       make(map[string]int),
		multiBacklinks:  cfg.multiBacklinks,

		footnoteID:            cfg.footnoteID,
		footnoteRefID:         cfg.footnoteRefID,
		footnoteBacklinkLabel: cfg.footnoteBacklinkLabel,
	}
	if r.footnoteID == nil {
		r.footnoteID = defaultFootnoteID
	}
	if r.footnoteRefID == nil {
		r.footnoteRefID = defaultFootnoteRefID
	}
	if r.footnoteBacklinkLabel == nil {
		r.footnoteBacklinkLabel = defaultFootnoteBacklinkLabel
	}
	// Walk the entire AST (including footnote definitions) to assign numbers
	// in document order. We need to process the main document first, then
	// footnote contents are processed as we encounter them.
	r.assignFootnoteNumbers(doc)
	return r
}

// assignFootnoteNumbers walks the AST to assign sequential numbers to footnote
// references in document order. Footnote definitions' content is also walked
// (in reference order) to find nested footnote references.
func (r *htmlRenderer) assignFootnoteNumbers(doc *Doc) {
	// First pass: walk the main document tree (skipping footnote definition nodes)
	// to find all footnote-reference nodes in order.
	// Collect footnote definitions from the AST so the renderer is
	// independent of Doc.Footnotes() (which may be stale after AST mutations).
	walkRead(doc.Root(), func(n Node) {
		if footnote, ok := n.(*Footnote); ok {
			r.footnotes[footnote.Label] = footnote
		}
	})

	var walkForRefs func(n Node)
	walkForRefs = func(n Node) {
		if _, ok := n.(*Footnote); ok {
			return // skip footnote definition bodies in first pass
		}
		if reference, ok := n.(*FootnoteReference); ok {
			r.getFootnoteNum(reference.Label)
			r.fnrefTotal[reference.Label]++
		}
		forEachChild(n, walkForRefs)
	}
	walkForRefs(doc.Root())

	// Now process footnote contents in number order, which may introduce
	// more footnote references (and thus more footnotes to process).
	for i := 0; i < len(r.footnoteOrder); i++ {
		fi := r.footnoteOrder[i]
		if fi.node != nil {
			forEachChild(fi.node, walkForRefs)
		}
	}
}

// getFootnoteNum returns the sequential number for a footnote label,
// assigning one if this is the first reference.
func (r *htmlRenderer) getFootnoteNum(label string) int {
	if num, ok := r.footnoteNums[label]; ok {
		return num
	}
	num := r.nextFootnoteNum
	r.nextFootnoteNum++
	r.footnoteNums[label] = num
	fi := &footnoteInfo{num: num, label: label}
	fi.node = r.footnotes[label]
	r.footnoteOrder = append(r.footnoteOrder, fi)
	return num
}

func (r *htmlRenderer) write(s string) {
	if r.err != nil {
		return
	}
	_, r.err = io.WriteString(r.w, s)
}

// nodeRendererImpl implements NodeRenderer for use in hooks.
type nodeRendererImpl struct {
	r *htmlRenderer
	n Node
}

func (nr *nodeRendererImpl) Children() {
	forEachChild(nr.n, nr.r.renderNode)
}

func (nr *nodeRendererImpl) Default() {
	nr.r.renderDefault(nr.n)
}

func (nr *nodeRendererImpl) Write(s string) {
	nr.r.write(s)
}

func (r *htmlRenderer) renderNode(n Node) {
	if r.err != nil {
		return
	}
	if fn, ok := r.hooks[n.Kind()]; ok {
		fn(n, &nodeRendererImpl{r: r, n: n})
		return
	}
	switch n := n.(type) {
	case *Symbol:
		if r.elements.symbol != nil {
			r.elements.symbol(SymbolView{Name: n.Name}, ElementRenderer{tree: r, node: n})
			return
		}
	case *Div:
		if r.elements.div != nil {
			r.elements.div(
				DivView{attributes: AttributeView{attributes: n.Attributes()}, span: n.Span()},
				ElementRenderer{tree: r, node: n},
			)
			return
		}
	}
	if fn, ok := r.subtrees[n.Kind()]; ok {
		fn(
			SubtreeView{root: ElementView{node: n}},
			ElementRenderer{tree: r, node: n},
		)
		return
	}
	r.renderDefault(n)
}

func (r *htmlRenderer) renderDocument() {
	if r.document == nil {
		r.renderDocumentDefault()
		return
	}
	state := documentViewState{root: ElementView{node: r.doc.Root()}}
	r.document(
		DocumentView{state: &state},
		DocumentRenderer{tree: r},
	)
}

func (r *htmlRenderer) renderDocumentDefault() {
	clear(r.fnrefSeen)
	r.tight = false
	r.renderChildren(r.doc.Root())
	r.renderFootnotesSection()
}

func (r *htmlRenderer) renderDefault(n Node) {
	switch n.Kind() {
	case KindDocument:
		r.renderChildren(n)

	case KindSection:
		r.write("<section")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</section>\n")

	case KindParagraph:
		// In tight lists, paragraphs are unwrapped.
		r.write("<p")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</p>\n")

	case KindHeading:
		heading := n.(*Heading)
		level := heading.Level
		if level < 1 {
			level = 1
		} else if level > 6 {
			level = 6
		}
		tag := "h" + strconv.Itoa(level)
		r.write("<" + tag)
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</" + tag + ">\n")

	case KindThematicBreak:
		r.write("<hr")
		r.renderAttrs(n)
		r.write(">\n")

	case KindCodeBlock:
		code := n.(*CodeBlock)
		r.write("<pre")
		r.renderAttrs(n)
		r.write("><code")
		if code.Language != "" {
			r.write(" class=\"language-" + escapeAttr(code.Language) + "\"")
		}
		r.write(">")
		r.write(escapeHTML(code.Text))
		r.write("</code></pre>\n")

	case KindRawBlock:
		raw := n.(*RawBlock)
		if raw.Format == "html" {
			r.write(raw.Text)
		}

	case KindBlockQuote:
		r.write("<blockquote")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</blockquote>\n")

	case KindDiv:
		r.write("<div")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</div>\n")

	case KindBulletList:
		list := n.(*BulletList)
		r.write("<ul")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			for _, child := range list.Items {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case KindOrderedList:
		list := n.(*OrderedList)
		r.write("<ol")
		if list.Start != 1 {
			r.write(" start=\"" + strconv.Itoa(list.Start) + "\"")
		}
		switch list.Style {
		case ListAlphaLower:
			r.write(" type=\"a\"")
		case ListAlphaUpper:
			r.write(" type=\"A\"")
		case ListRomanLower:
			r.write(" type=\"i\"")
		case ListRomanUpper:
			r.write(" type=\"I\"")
		}
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			for _, child := range list.Items {
				r.renderNode(child)
			}
		})
		r.write("</ol>\n")

	case KindTable:
		r.write("<table")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</table>\n")

	case KindCaption:
		r.write("<caption>")
		r.renderInlineChildren(n)
		r.write("</caption>\n")

	case KindTableRow:
		r.write("<tr")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</tr>\n")

	case KindTableCell:
		cell := n.(*TableCell)
		tag := "td"
		if cell.Header {
			tag = "th"
		}
		r.write("<" + tag)
		if cell.Alignment != AlignDefault {
			var alignStr string
			switch cell.Alignment {
			case AlignLeft:
				alignStr = "left"
			case AlignRight:
				alignStr = "right"
			case AlignCenter:
				alignStr = "center"
			}
			r.write(` style="text-align: ` + alignStr + `;"`)

		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</" + tag + ">\n")

	case KindDefinitionList:
		list := n.(*DefinitionList)
		r.write("<dl")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			forEachChild(list, r.renderNode)
		})
		r.write("</dl>\n")

	case KindTerm:
		r.write("<dt>")
		r.renderInlineChildren(n)
		r.write("</dt>\n")

	case KindDefinition:
		r.write("<dd>\n")
		r.renderListItemChildren(n)
		r.write("</dd>\n")

	case KindTaskList:
		list := n.(*TaskList)
		r.write("<ul class=\"task-list\"")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			for _, child := range list.Items {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case KindListItem:
		r.write("<li")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderListItemChildren(n)
		r.write("</li>\n")

	case KindTaskListItem:
		item := n.(*TaskListItem)
		r.write("<li>\n")
		if item.Checked {
			r.write(`<input disabled="" type="checkbox" checked=""/>`)
		} else {
			r.write(`<input disabled="" type="checkbox"/>`)
		}
		r.write("\n")
		r.renderListItemChildren(n)
		r.write("</li>\n")

	// Inline nodes.
	case KindText:
		r.write(escapeHTML(n.(*Text).Value))

	case KindSoftBreak:
		r.write("\n")

	case KindHardBreak:
		r.write("<br>\n")

	case KindNonBreakingSpace:
		r.write("&nbsp;")

	case KindEmphasis:
		r.write("<em")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</em>")

	case KindStrong:
		r.write("<strong")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</strong>")

	case KindSuperscript:
		r.write("<sup")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sup>")

	case KindSubscript:
		r.write("<sub")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sub>")

	case KindInsert:
		r.write("<ins")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</ins>")

	case KindDelete:
		r.write("<del")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</del>")

	case KindMark:
		r.write("<mark")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</mark>")

	case KindLink:
		link := n.(*Link)
		r.write("<a")
		if link.Destination != "" || link.DestinationSet {
			r.write(" href=\"" + escapeAttr(link.Destination) + "\"")
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</a>")

	case KindImage:
		image := n.(*Image)
		r.write("<img")
		alt := collectText(n)
		if alt != "" {
			r.write(" alt=\"" + escapeAttr(alt) + "\"")
		}
		if image.Destination != "" || image.DestinationSet {
			r.write(" src=\"" + escapeAttr(image.Destination) + "\"")
		}
		r.renderAttrs(n)
		r.write(">")

	case KindSpan:
		r.write("<span")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</span>")

	case KindVerbatim:
		verbatim := n.(*Verbatim)
		r.write("<code>")
		r.write(escapeHTML(verbatim.Text))
		r.write("</code>")

	case KindInlineMath:
		math := n.(*InlineMath)
		r.write(`<span class="math inline">\(`)
		r.write(escapeHTML(math.Text))
		r.write(`\)</span>`)

	case KindDisplayMath:
		math := n.(*DisplayMath)
		r.write(`<span class="math display">\[`)
		r.write(escapeHTML(math.Text))
		r.write(`\]</span>`)

	case KindRawInline:
		raw := n.(*RawInline)
		if raw.Format == "html" {
			r.write(raw.Text)
		}

	case KindSymbol:
		r.write(":" + escapeHTML(n.(*Symbol).Name) + ":")

	case KindFootnote:
		// Footnote definitions are rendered in the endnotes section, not inline.
		return

	case KindFootnoteReference:
		reference := n.(*FootnoteReference)
		num := r.footnoteNums[reference.Label]
		ns := strconv.Itoa(num)
		r.fnrefSeen[reference.Label]++
		k := r.fnrefSeen[reference.Label]
		var idAttr string
		// Multi mode gives every reference a unique id; the default emits the id
		// only on the first reference, so the HTML has no duplicate ids.
		if r.multiBacklinks || k == 1 {
			idAttr = ` id="` + r.footnoteRefID(num, k) + `"`
		}
		r.write(`<a` + idAttr + ` href="#` + r.footnoteID(num) + `" role="doc-noteref"><sup>` + ns + `</sup></a>`)

	case KindDoubleQuoted:
		r.write("\u201c")
		r.renderInlineChildren(n)
		r.write("\u201d")

	case KindSingleQuoted:
		r.write("\u2018")
		r.renderInlineChildren(n)
		r.write("\u2019")

	case KindEllipsis:
		r.write("\u2026")
	case KindEmDash:
		r.write("\u2014")
	case KindEnDash:
		r.write("\u2013")
	}
}

func (r *htmlRenderer) renderChildren(n Node) {
	forEachChild(n, r.renderNode)
}

func (r *htmlRenderer) renderInlineChildren(n Node) {
	forEachChild(n, r.renderNode)
}

// withTight runs fn with r.tight set to t, restoring the prior value afterward.
// List containers use this so list-item default rendering can read the parent's
// tight flag without it being passed through every helper.
func (r *htmlRenderer) withTight(t bool, fn func()) {
	prev := r.tight
	r.tight = t
	fn()
	r.tight = prev
}

// renderListItemChildren renders the children of a list item or definition,
// unwrapping paragraph content when r.tight is set.
func (r *htmlRenderer) renderListItemChildren(n Node) {
	if r.tight {
		forEachChild(n, func(child Node) {
			if child.Kind() == KindParagraph {
				r.renderInlineChildren(child)
				r.write("\n")
			} else {
				r.renderNode(child)
			}
		})
		return
	}
	r.renderChildren(n)
}

func (r *htmlRenderer) renderFootnotesSection() {
	if len(r.footnoteOrder) == 0 {
		return
	}
	r.write("<section role=\"doc-endnotes\">\n<hr>\n<ol>\n")
	for _, fi := range r.footnoteOrder {
		r.write("<li id=\"" + r.footnoteID(fi.num) + "\">\n")
		if fi.node != nil && len(fi.node.Children) > 0 {
			// Render all children. Append back-reference to the last paragraph.
			children := fi.node.Children
			lastParagraphIdx := -1
			for i, child := range children {
				if child.Kind() == KindParagraph {
					lastParagraphIdx = i
				}
			}
			backref := r.footnoteBackref(fi)
			for i, child := range children {
				if i == lastParagraphIdx {
					// Render paragraph with backref appended inside <p>.
					r.write("<p")
					r.renderAttrs(child)
					r.write(">")
					r.renderInlineChildren(child)
					r.write(backref)
					r.write("</p>\n")
				} else {
					r.renderNode(child)
				}
			}
			if lastParagraphIdx == -1 {
				// No paragraph found; add backref in its own paragraph.
				r.write("<p>" + backref + "</p>\n")
			}
		} else {
			// Empty or undefined footnote: just the back-reference(s).
			r.write("<p>" + r.footnoteBackref(fi) + "</p>\n")
		}
		r.write("</li>\n")
	}
	r.write("</ol>\n</section>\n")
}

// fnrefID returns the id of the k-th reference (1-based) to footnote number num:
// "fnrefN" for the first reference and "fnrefN-k" for later ones.
func fnrefID(num, k int) string {
	id := "fnref" + strconv.Itoa(num)
	if k > 1 {
		id += "-" + strconv.Itoa(k)
	}
	return id
}

// footnoteBackref builds the back-reference link(s) appended to a footnote's
// entry. The default (and any non-multi mode) emits a single backlink to the
// first reference; multi-backlink mode emits one backlink per reference. Each
// backlink's target id and visible label come from the configured producers,
// so ids stay in sync with the references and labels are customizable.
func (r *htmlRenderer) footnoteBackref(fi *footnoteInfo) string {
	total := 1
	if r.multiBacklinks {
		if t := r.fnrefTotal[fi.label]; t > 1 {
			total = t
		}
	}
	var b strings.Builder
	for k := 1; k <= total; k++ {
		if k > 1 {
			b.WriteByte(' ')
		}
		b.WriteString(`<a href="#` + r.footnoteRefID(fi.num, k) + `" role="doc-backlink">` +
			r.footnoteBacklinkLabel(fi.num, k, total) + `</a>`)
	}
	return b.String()
}

// backrefLabel maps a 1-based index to a spreadsheet-style letter label:
// 1→a, 2→b, …, 26→z, 27→aa, 28→ab, ….
func backrefLabel(i int) string {
	var b []byte
	for i > 0 {
		i--
		b = append([]byte{byte('a' + i%26)}, b...)
		i /= 26
	}
	return string(b)
}

func (r *htmlRenderer) renderAttrs(n Node) {
	if n.Attributes().Len() == 0 {
		return
	}
	for _, attribute := range n.Attributes().items {
		r.write(" " + attribute.Key + "=\"" + escapeAttr(attribute.Value) + "\"")
	}
}

// renderNonInternalAttrs is an alias for renderAttrs, kept for call-site clarity
// on list containers where internal attributes were historically filtered.
func (r *htmlRenderer) renderNonInternalAttrs(n Node) {
	r.renderAttrs(n)
}

func escapeHTML(s string) string {
	// Fast path: no escaping needed.
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>':
			return escapeHTMLSlow(s, i)
		}
	}
	return s
}

func escapeHTMLSlow(s string, first int) string {
	var b strings.Builder
	b.Grow(len(s) + 10)
	b.WriteString(s[:first])
	for i := first; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func escapeAttr(s string) string {
	// Fast path: no escaping needed.
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"':
			return escapeAttrSlow(s, i)
		}
	}
	return s
}

func escapeAttrSlow(s string, first int) string {
	var b strings.Builder
	b.Grow(len(s) + 10)
	b.WriteString(s[:first])
	for i := first; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// collectText extracts visible text from a materialized public node tree.
func collectText(n Node) string {
	switch n.Kind() {
	case KindText:
		return n.(*Text).Value
	case KindSoftBreak, KindHardBreak, KindNonBreakingSpace:
		return " "
	}
	var b strings.Builder
	forEachChild(n, func(child Node) { b.WriteString(collectText(child)) })
	return b.String()
}
