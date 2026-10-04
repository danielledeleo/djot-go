package djot

import (
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

var errNilHTMLWriter = errors.New("djot: RenderHTMLTo called with a nil writer")

// RenderOption configures HTML rendering. Pass to [RenderHTML] or [RenderHTMLTo].
type RenderOption func(*renderConfig)

// NodeRenderFunc is a hook that overrides rendering for a specific node kind.
// It receives the node being rendered and a [NodeRenderer] for controlling output.
type NodeRenderFunc func(n ast.Node, r NodeRenderer)

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

type renderConfig struct {
	hooks                 map[ast.Kind]NodeRenderFunc
	multiBacklinks        bool
	footnoteID            func(num int) string
	footnoteRefID         func(num, k int) string
	footnoteBacklinkLabel func(num, k, total int) string
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
// [WithFootnotePrefix] convenience. The returned value is HTML-attribute
// escaped.
func WithFootnoteID(fn func(num int) string) RenderOption {
	requireRenderCallback(fn != nil, "WithFootnoteID")
	return func(cfg *renderConfig) { cfg.footnoteID = fn }
}

// WithFootnoteRefID overrides the back-anchor id of the k-th reference (1-based)
// to footnote num — the id on the reference's <a> and the "#…" target of the
// matching backlink. The default is "fnrefN" for the first reference and
// "fnrefN-k" for later ones. Both ends of the link use this function, so they
// always agree. The returned value is HTML-attribute escaped.
func WithFootnoteRefID(fn func(num, k int) string) RenderOption {
	requireRenderCallback(fn != nil, "WithFootnoteRefID")
	return func(cfg *renderConfig) { cfg.footnoteRefID = fn }
}

// WithFootnoteBacklinkLabel overrides the visible text of the k-th backlink for
// a footnote, where total is the number of backlinks that footnote has (1 unless
// [WithMultiBacklinks] is set). The default is "↩︎" when total is 1 and the
// letters a, b, c, … otherwise. Use this for numeric or other label schemes.
// The returned value is escaped as HTML text.
func WithFootnoteBacklinkLabel(fn func(num, k, total int) string) RenderOption {
	requireRenderCallback(fn != nil, "WithFootnoteBacklinkLabel")
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
// The hook receives the [ast.Node] and a [NodeRenderer] for full control over output.
// If called multiple times for the same kind, the last one wins.
//
// KindDocument and KindFootnote are not directly rendered elements; use
// [WithDocumentRenderer] for the document and render hooks for a footnote's
// block children. WithNodeRenderer panics for either synthetic root kind, an
// invalid kind, or a nil callback.
//
// Use this when you need access to [NodeRenderer.Children] or [NodeRenderer.Default].
// For a symbol hook that can avoid AST materialization, see [WithSymbolRenderer].
func WithNodeRenderer(kind ast.Kind, fn NodeRenderFunc) RenderOption {
	requireElementHook(kind, fn != nil, "WithNodeRenderer")
	return func(cfg *renderConfig) {
		if cfg.hooks == nil {
			cfg.hooks = make(map[ast.Kind]NodeRenderFunc)
		}
		cfg.hooks[kind] = fn
	}
}

func requireElementHook(kind ast.Kind, callbackPresent bool, option string) {
	requireRenderCallback(callbackPresent, option)
	if kind < 0 || kind > ast.KindEnDash {
		panic("djot: " + option + " requires a valid node kind")
	}
	if kind == ast.KindDocument || kind == ast.KindFootnote {
		panic("djot: " + option + " cannot target a synthetic rendering root")
	}
}

func requireRenderCallback(present bool, option string) {
	if !present {
		panic("djot: " + option + " requires a non-nil callback")
	}
}

// WithRenderer registers a type-safe render hook. T must be one concrete node
// pointer type; the kind is inferred once when the option is constructed.
//
//	djot.WithRenderer(func(symbol *ast.Symbol, r djot.NodeRenderer) {
//	    r.Write(symbol.Name)
//	})
func WithRenderer[T ast.Node](fn func(T, NodeRenderer)) RenderOption {
	requireRenderCallback(fn != nil, "WithRenderer")
	var zero T
	if reflect.TypeOf(zero) == nil {
		panic("djot: WithRenderer requires a concrete node pointer type")
	}
	kind := zero.Kind()
	return WithNodeRenderer(kind, func(node ast.Node, renderer NodeRenderer) {
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
// This is convenient for leaf nodes like [ast.KindSymbol] where you don't need
// [NodeRenderer.Children] or [NodeRenderer.Default]:
//
//	html := RenderHTML(doc, WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
//	    if symbol, ok := n.(*ast.Symbol); ok && symbol.Name == "star" {
//	        return "⭐"
//	    }
//	    return "" // fall through to default
//	}))
func WithRenderFunc(kind ast.Kind, fn func(n ast.Node) string) RenderOption {
	requireRenderCallback(fn != nil, "WithRenderFunc")
	return WithNodeRenderer(kind, func(n ast.Node, r NodeRenderer) {
		if s := fn(n); s != "" {
			r.Write(s)
			return
		}
		r.Default()
	})
}

// RenderHTML renders a parsed document to an HTML string. Optional
// [RenderOption] values customize rendering through node hooks and footnote
// options.
func RenderHTML(doc *Doc, opts ...RenderOption) string {
	cfg := makeRenderConfig(opts)
	if cfg.canRenderSemantic() {
		return renderSemanticHTMLWithHooks(doc.tape(), cfg.semanticHooks())
	}
	var b strings.Builder
	r := newHTMLRendererWithConfig(&b, doc.AST(), cfg)
	r.renderDocument()
	return b.String()
}

// RenderHTMLTo renders a parsed document as HTML to the given writer.
// It returns the first write error encountered, if any.
func RenderHTMLTo(w io.Writer, doc *Doc, opts ...RenderOption) error {
	if w == nil {
		return errNilHTMLWriter
	}
	cfg := makeRenderConfig(opts)
	if cfg.canRenderSemantic() {
		return renderSemanticHTMLToWithHooks(w, doc.tape(), cfg.semanticHooks())
	}
	r := newHTMLRendererWithConfig(w, doc.AST(), cfg)
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
	return len(cfg.hooks) == 0
}

func (cfg *renderConfig) semanticHooks() semanticRenderHooks {
	return semanticRenderHooks{
		multiBacklinks: cfg.multiBacklinks, footnoteID: cfg.footnoteID,
		footnoteRefID: cfg.footnoteRefID, footnoteBacklinkLabel: cfg.footnoteBacklinkLabel,
	}
}

type footnoteInfo struct {
	num   int
	label string
	node  *ast.Footnote // may be nil if undefined
}

type htmlRenderer struct {
	w    io.Writer
	err  error
	root *ast.Document

	hooks map[ast.Kind]NodeRenderFunc

	// tight tracks whether we are rendering inside a tight list/definition list.
	// Set by the list container before iterating children and restored after,
	// so list-item and definition default cases can render correctly.
	tight bool

	// Footnote definitions derived from the AST at render time.
	// This ensures correctness even after AST mutations (e.g., include/splice).
	footnotes map[string]*ast.Footnote
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
	footnoteParagraph     ast.Node
	footnoteParagraphInfo *footnoteInfo
}

func newHTMLRendererWithConfig(w io.Writer, root *ast.Document, cfg renderConfig) *htmlRenderer {
	r := &htmlRenderer{
		w:               w,
		root:            root,
		hooks:           cfg.hooks,
		footnotes:       make(map[string]*ast.Footnote),
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
	r.assignFootnoteNumbers()
	return r
}

// assignFootnoteNumbers walks the AST to assign sequential numbers to footnote
// references in document order. Footnote definitions' content is also walked
// (in reference order) to find nested footnote references.
func (r *htmlRenderer) assignFootnoteNumbers() {
	// First pass: walk the main document tree (skipping footnote definition nodes)
	// to find all footnote-reference nodes in order.
	// Collect footnote definitions from the AST so rendering reflects mutations
	// without allocating a separate public index.
	ast.Preorder(r.root, func(n ast.Node) bool {
		if footnote, ok := n.(*ast.Footnote); ok {
			r.footnotes[footnote.Label] = footnote
		}
		return true
	})

	var walkForRefs func(n ast.Node)
	walkForRefs = func(n ast.Node) {
		if _, ok := n.(*ast.Footnote); ok {
			return // skip footnote definition bodies in first pass
		}
		if reference, ok := n.(*ast.FootnoteReference); ok {
			r.getFootnoteNum(reference.Label)
			r.fnrefTotal[reference.Label]++
		}
		ast.ForEachChild(n, walkForRefs)
	}
	walkForRefs(r.root)

	// Now process footnote contents in number order, which may introduce
	// more footnote references (and thus more footnotes to process).
	for i := 0; i < len(r.footnoteOrder); i++ {
		fi := r.footnoteOrder[i]
		if fi.node != nil {
			ast.ForEachChild(fi.node, walkForRefs)
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
	n ast.Node
}

func (nr *nodeRendererImpl) Children() {
	ast.ForEachChild(nr.n, nr.r.renderNode)
}

func (nr *nodeRendererImpl) Default() {
	nr.r.renderDefault(nr.n)
}

func (nr *nodeRendererImpl) Write(s string) {
	nr.r.write(s)
}

func (r *htmlRenderer) renderNode(n ast.Node) {
	if r.err != nil {
		return
	}
	// Tightness applies to list-item paragraphs, not nested block containers.
	switch n.Kind() {
	case ast.KindBlockQuote, ast.KindDiv, ast.KindSection:
		previous := r.tight
		r.tight = false
		defer func() { r.tight = previous }()
	}

	if fn, ok := r.hooks[n.Kind()]; ok {
		fn(n, &nodeRendererImpl{r: r, n: n})
		return
	}
	r.renderDefault(n)
}

func (r *htmlRenderer) renderDocument() {
	clear(r.fnrefSeen)
	r.tight = false
	r.renderChildren(r.root)
	r.renderFootnotesSection()
}

func (r *htmlRenderer) renderDefault(n ast.Node) {
	switch n.Kind() {
	case ast.KindDocument:
		r.renderChildren(n)

	case ast.KindSection:
		r.write("<section")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</section>\n")

	case ast.KindParagraph:
		if r.tight {
			r.renderInlineChildren(n)
			r.write("\n")
			return
		}
		r.write("<p")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		if n == r.footnoteParagraph && r.footnoteParagraphInfo != nil {
			r.write(r.footnoteBackref(r.footnoteParagraphInfo))
		}
		r.write("</p>\n")

	case ast.KindHeading:
		heading := n.(*ast.Heading)
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

	case ast.KindThematicBreak:
		r.write("<hr")
		r.renderAttrs(n)
		r.write(">\n")

	case ast.KindCodeBlock:
		code := n.(*ast.CodeBlock)
		r.write("<pre")
		r.renderAttrs(n)
		r.write("><code")
		if code.Language != "" {
			r.write(" class=\"language-" + escapeAttr(code.Language) + "\"")
		}
		r.write(">")
		r.write(escapeHTML(code.Text))
		r.write("</code></pre>\n")

	case ast.KindRawBlock:
		raw := n.(*ast.RawBlock)
		if raw.Format == "html" {
			r.write(raw.Text)
		}

	case ast.KindBlockQuote:
		r.write("<blockquote")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</blockquote>\n")

	case ast.KindDiv:
		r.write("<div")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</div>\n")

	case ast.KindBulletList:
		list := n.(*ast.BulletList)
		r.write("<ul")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			for _, child := range list.Items {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case ast.KindOrderedList:
		list := n.(*ast.OrderedList)
		r.write("<ol")
		if list.Start != 1 {
			r.write(" start=\"" + strconv.Itoa(list.Start) + "\"")
		}
		switch list.Style {
		case ast.ListAlphaLower:
			r.write(" type=\"a\"")
		case ast.ListAlphaUpper:
			r.write(" type=\"A\"")
		case ast.ListRomanLower:
			r.write(" type=\"i\"")
		case ast.ListRomanUpper:
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

	case ast.KindTable:
		r.write("<table")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</table>\n")

	case ast.KindCaption:
		r.write("<caption>")
		r.renderInlineChildren(n)
		r.write("</caption>\n")

	case ast.KindTableRow:
		r.write("<tr")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</tr>\n")

	case ast.KindTableCell:
		cell := n.(*ast.TableCell)
		tag := "td"
		if cell.Header {
			tag = "th"
		}
		r.write("<" + tag)
		if cell.Alignment != ast.AlignDefault {
			var alignStr string
			switch cell.Alignment {
			case ast.AlignLeft:
				alignStr = "left"
			case ast.AlignRight:
				alignStr = "right"
			case ast.AlignCenter:
				alignStr = "center"
			}
			r.write(` style="text-align: ` + alignStr + `;"`)

		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</" + tag + ">\n")

	case ast.KindDefinitionList:
		list := n.(*ast.DefinitionList)
		r.write("<dl")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			ast.ForEachChild(list, r.renderNode)
		})
		r.write("</dl>\n")

	case ast.KindTerm:
		r.write("<dt>")
		r.renderInlineChildren(n)
		r.write("</dt>\n")

	case ast.KindDefinition:
		r.write("<dd>\n")
		r.renderListItemChildren(n)
		r.write("</dd>\n")

	case ast.KindTaskList:
		list := n.(*ast.TaskList)
		r.write("<ul class=\"task-list\"")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(list.Tight, func() {
			for _, child := range list.Items {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case ast.KindListItem:
		r.write("<li")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderListItemChildren(n)
		r.write("</li>\n")

	case ast.KindTaskListItem:
		item := n.(*ast.TaskListItem)
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
	case ast.KindText:
		r.write(escapeHTML(n.(*ast.Text).Value))

	case ast.KindSoftBreak:
		r.write("\n")

	case ast.KindHardBreak:
		r.write("<br>\n")

	case ast.KindNonBreakingSpace:
		r.write("&nbsp;")

	case ast.KindEmphasis:
		r.write("<em")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</em>")

	case ast.KindStrong:
		r.write("<strong")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</strong>")

	case ast.KindSuperscript:
		r.write("<sup")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sup>")

	case ast.KindSubscript:
		r.write("<sub")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sub>")

	case ast.KindInsert:
		r.write("<ins")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</ins>")

	case ast.KindDelete:
		r.write("<del")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</del>")

	case ast.KindMark:
		r.write("<mark")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</mark>")

	case ast.KindLink:
		link := n.(*ast.Link)
		r.write("<a")
		if link.Destination != "" || link.DestinationSet {
			r.write(" href=\"" + escapeAttr(link.Destination) + "\"")
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</a>")

	case ast.KindImage:
		image := n.(*ast.Image)
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

	case ast.KindSpan:
		r.write("<span")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</span>")

	case ast.KindVerbatim:
		verbatim := n.(*ast.Verbatim)
		r.write("<code>")
		r.write(escapeHTML(verbatim.Text))
		r.write("</code>")

	case ast.KindInlineMath:
		math := n.(*ast.InlineMath)
		r.write(`<span class="math inline">\(`)
		r.write(escapeHTML(math.Text))
		r.write(`\)</span>`)

	case ast.KindDisplayMath:
		math := n.(*ast.DisplayMath)
		r.write(`<span class="math display">\[`)
		r.write(escapeHTML(math.Text))
		r.write(`\]</span>`)

	case ast.KindRawInline:
		raw := n.(*ast.RawInline)
		if raw.Format == "html" {
			r.write(raw.Text)
		}

	case ast.KindSymbol:
		r.write(":" + escapeHTML(n.(*ast.Symbol).Name) + ":")

	case ast.KindFootnote:
		// Footnote definitions are rendered in the endnotes section, not inline.
		return

	case ast.KindFootnoteReference:
		reference := n.(*ast.FootnoteReference)
		num := r.footnoteNums[reference.Label]
		ns := strconv.Itoa(num)
		r.fnrefSeen[reference.Label]++
		k := r.fnrefSeen[reference.Label]
		var idAttr string
		// Multi mode gives every reference a unique id; the default emits the id
		// only on the first reference, so the HTML has no duplicate ids.
		if r.multiBacklinks || k == 1 {
			idAttr = ` id="` + escapeAttr(r.footnoteRefID(num, k)) + `"`
		}
		r.write(`<a` + idAttr + ` href="` + escapeAttr("#"+r.footnoteID(num)) + `" role="doc-noteref"><sup>` + ns + `</sup></a>`)

	case ast.KindDoubleQuoted:
		r.write("\u201c")
		r.renderInlineChildren(n)
		r.write("\u201d")

	case ast.KindSingleQuoted:
		r.write("\u2018")
		r.renderInlineChildren(n)
		r.write("\u2019")

	case ast.KindEllipsis:
		r.write("\u2026")
	case ast.KindEmDash:
		r.write("\u2014")
	case ast.KindEnDash:
		r.write("\u2013")
	}
}

func (r *htmlRenderer) renderChildren(n ast.Node) {
	ast.ForEachChild(n, r.renderNode)
}

func (r *htmlRenderer) renderInlineChildren(n ast.Node) {
	ast.ForEachChild(n, r.renderNode)
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
func (r *htmlRenderer) renderListItemChildren(n ast.Node) {
	if r.tight {
		r.renderChildren(n)
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
		r.write("<li id=\"" + escapeAttr(r.footnoteID(fi.num)) + "\">\n")
		if fi.node != nil && len(fi.node.Children) > 0 {
			// Render all children. Append back-reference to the last paragraph.
			children := fi.node.Children
			lastParagraphIdx := -1
			for i, child := range children {
				if child.Kind() == ast.KindParagraph {
					lastParagraphIdx = i
				}
			}
			for i, child := range children {
				if i == lastParagraphIdx {
					previousParagraph, previousInfo := r.footnoteParagraph, r.footnoteParagraphInfo
					r.footnoteParagraph, r.footnoteParagraphInfo = child, fi
					r.renderNode(child)
					r.footnoteParagraph, r.footnoteParagraphInfo = previousParagraph, previousInfo
				} else {
					r.renderNode(child)
				}
			}
			if lastParagraphIdx == -1 {
				// No paragraph found; add backref in its own paragraph.
				r.write("<p>" + r.footnoteBackref(fi) + "</p>\n")
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
		b.WriteString(`<a href="` + escapeAttr("#"+r.footnoteRefID(fi.num, k)) + `" role="doc-backlink">` +
			escapeHTML(r.footnoteBacklinkLabel(fi.num, k, total)) + `</a>`)
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

func (r *htmlRenderer) renderAttrs(n ast.Node) {
	if n.Attributes().Len() == 0 {
		return
	}
	for _, attribute := range n.Attributes().Entries() {
		r.write(" " + attribute.Key + "=\"" + escapeAttr(attribute.Value) + "\"")
	}
}

// renderNonInternalAttrs is an alias for renderAttrs, kept for call-site clarity
// on list containers where internal attributes were historically filtered.
func (r *htmlRenderer) renderNonInternalAttrs(n ast.Node) {
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
func collectText(n ast.Node) string {
	return collectDocumentText(textElement{node: n})
}
