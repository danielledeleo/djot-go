package djot

import (
	"io"
	"strconv"
	"strings"
)

// RenderOption configures HTML rendering. Pass to [RenderHTML] or [RenderHTMLTo].
type RenderOption func(*renderConfig)

// NodeRenderFunc is a hook that overrides rendering for a specific node kind.
// It receives the node being rendered and a [NodeRenderer] for controlling output.
type NodeRenderFunc func(n *Node, r NodeRenderer)

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
	hooks map[NodeKind]NodeRenderFunc
}

// WithNodeRenderer registers a render hook for the given node kind.
// The hook receives the [Node] and a [NodeRenderer] for full control over output.
// If called multiple times for the same kind, the last one wins.
//
// Use this when you need access to [NodeRenderer.Children] or [NodeRenderer.Default].
// For simpler cases where you just need to return an HTML string, see [WithRenderFunc].
func WithNodeRenderer(kind NodeKind, fn NodeRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		if cfg.hooks == nil {
			cfg.hooks = make(map[NodeKind]NodeRenderFunc)
		}
		cfg.hooks[kind] = fn
	}
}

// WithRenderFunc registers a simple render hook for the given node kind.
// The function receives the node and returns an HTML string to emit.
// If it returns an empty string, the default rendering is used.
//
// This is convenient for leaf nodes like [Symbol] where you don't need
// [NodeRenderer.Children] or [NodeRenderer.Default]:
//
//	html := RenderHTML(doc, WithRenderFunc(Symbol, func(n *Node) string {
//	    if n.Name == "star" {
//	        return "⭐"
//	    }
//	    return "" // fall through to default
//	}))
func WithRenderFunc(kind NodeKind, fn func(n *Node) string) RenderOption {
	return WithNodeRenderer(kind, func(n *Node, r NodeRenderer) {
		if s := fn(n); s != "" {
			r.Write(s)
			return
		}
		r.Default()
	})
}

// RenderHTML renders a parsed document to an HTML string. Optional
// [RenderOption] values can customize rendering via [WithNodeRenderer].
func RenderHTML(doc *Doc, opts ...RenderOption) string {
	var b strings.Builder
	r := newHTMLRenderer(&b, doc, opts...)
	r.renderChildren(doc.Root)
	r.renderFootnotesSection()
	return b.String()
}

// RenderHTMLTo renders a parsed document as HTML to the given writer.
// It returns the first write error encountered, if any.
func RenderHTMLTo(w io.Writer, doc *Doc, opts ...RenderOption) error {
	r := newHTMLRenderer(w, doc, opts...)
	r.renderChildren(doc.Root)
	r.renderFootnotesSection()
	return r.err
}

type footnoteInfo struct {
	num   int
	label string
	node  *Node // may be nil if undefined
}

type htmlRenderer struct {
	w   io.Writer
	err error
	doc *Doc

	hooks map[NodeKind]NodeRenderFunc

	// tight tracks whether we are rendering inside a tight list/definition list.
	// Set by the list container before iterating children and restored after,
	// so list-item and definition default cases can render correctly.
	tight bool

	// Footnote definitions derived from the AST at render time.
	// This ensures correctness even after AST mutations (e.g., include/splice).
	footnotes map[string]*Node
	// Footnote numbering: label → sequential number
	footnoteNums map[string]int
	// Ordered list of referenced footnotes (by first reference order)
	footnoteOrder []*footnoteInfo
	// Counter for assigning numbers
	nextFootnoteNum int
}

func newHTMLRenderer(w io.Writer, doc *Doc, opts ...RenderOption) *htmlRenderer {
	var cfg renderConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	r := &htmlRenderer{
		w:               w,
		doc:             doc,
		hooks:           cfg.hooks,
		footnotes:       make(map[string]*Node),
		footnoteNums:    make(map[string]int),
		nextFootnoteNum: 1,
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
	// First pass: walk the main document tree (skipping Footnote definition nodes)
	// to find all FootnoteReference nodes in order.
	// Collect footnote definitions from the AST so the renderer is
	// independent of Doc.Footnotes (which may be stale after AST mutations).
	Walk(doc.Root, func(n *Node) any {
		if n.Kind == Footnote {
			r.footnotes[n.Label] = n
		}
		return Continue
	})

	var walkForRefs func(n *Node)
	walkForRefs = func(n *Node) {
		if n.Kind == Footnote {
			return // skip footnote definition bodies in first pass
		}
		if n.Kind == FootnoteReference {
			r.getFootnoteNum(n.Label)
		}
		for _, child := range n.Children {
			walkForRefs(child)
		}
	}
	walkForRefs(doc.Root)

	// Now process footnote contents in number order, which may introduce
	// more footnote references (and thus more footnotes to process).
	for i := 0; i < len(r.footnoteOrder); i++ {
		fi := r.footnoteOrder[i]
		if fi.node != nil {
			for _, child := range fi.node.Children {
				walkForRefs(child)
			}
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
	n *Node
}

func (nr *nodeRendererImpl) Children() {
	for _, child := range nr.n.Children {
		nr.r.renderNode(child)
	}
}

func (nr *nodeRendererImpl) Default() {
	nr.r.renderDefault(nr.n)
}

func (nr *nodeRendererImpl) Write(s string) {
	nr.r.write(s)
}

func (r *htmlRenderer) renderNode(n *Node) {
	if fn, ok := r.hooks[n.Kind]; ok {
		fn(n, &nodeRendererImpl{r: r, n: n})
		return
	}
	r.renderDefault(n)
}

func (r *htmlRenderer) renderDefault(n *Node) {
	switch n.Kind {
	case Document:
		r.renderChildren(n)

	case Section:
		r.write("<section")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</section>\n")

	case Paragraph:
		// In tight lists, paragraphs are unwrapped.
		r.write("<p")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</p>\n")

	case Heading:
		level := n.Level
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

	case ThematicBreak:
		r.write("<hr")
		r.renderAttrs(n)
		r.write(">\n")

	case CodeBlock:
		r.write("<pre")
		r.renderAttrs(n)
		r.write("><code")
		if n.Lang != "" {
			r.write(" class=\"language-" + escapeAttr(n.Lang) + "\"")
		}
		r.write(">")
		r.write(escapeHTML(n.Text))
		r.write("</code></pre>\n")

	case RawBlock:
		if n.Format == "html" {
			r.write(n.Text)
		}

	case BlockQuote:
		r.write("<blockquote")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</blockquote>\n")

	case Div:
		r.write("<div")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</div>\n")

	case BulletList:
		r.write("<ul")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(n.tight, func() {
			for _, child := range n.Children {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case OrderedList:
		r.write("<ol")
		if n.ListStart != 1 {
			r.write(" start=\"" + strconv.Itoa(n.ListStart) + "\"")
		}
		switch n.ListStyle {
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
		r.withTight(n.tight, func() {
			for _, child := range n.Children {
				r.renderNode(child)
			}
		})
		r.write("</ol>\n")

	case Table:
		r.write("<table")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</table>\n")

	case Caption:
		r.write("<caption>")
		r.renderInlineChildren(n)
		r.write("</caption>\n")

	case TableRow:
		r.write("<tr")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderChildren(n)
		r.write("</tr>\n")

	case TableCell:
		tag := "td"
		if n.IsHeader {
			tag = "th"
		}
		r.write("<" + tag)
		if n.CellAlign != AlignDefault {
			var alignStr string
			switch n.CellAlign {
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

	case DefinitionList:
		r.write("<dl")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(n.tight, func() {
			for _, child := range n.Children {
				r.renderNode(child)
			}
		})
		r.write("</dl>\n")

	case Term:
		r.write("<dt>")
		r.renderInlineChildren(n)
		r.write("</dt>\n")

	case Definition:
		r.write("<dd>\n")
		r.renderListItemChildren(n)
		r.write("</dd>\n")

	case TaskList:
		r.write("<ul class=\"task-list\"")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		r.withTight(n.tight, func() {
			for _, child := range n.Children {
				r.renderNode(child)
			}
		})
		r.write("</ul>\n")

	case ListItem:
		r.write("<li")
		r.renderAttrs(n)
		r.write(">\n")
		r.renderListItemChildren(n)
		r.write("</li>\n")

	case TaskListItem:
		r.write("<li>\n")
		if n.Checked {
			r.write(`<input disabled="" type="checkbox" checked=""/>`)
		} else {
			r.write(`<input disabled="" type="checkbox"/>`)
		}
		r.write("\n")
		r.renderListItemChildren(n)
		r.write("</li>\n")

	// Inline nodes.
	case Text:
		r.write(escapeHTML(n.Text))

	case SoftBreak:
		r.write("\n")

	case HardBreak:
		r.write("<br>\n")

	case NonBreakingSpace:
		r.write("&nbsp;")

	case Emphasis:
		r.write("<em")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</em>")

	case Strong:
		r.write("<strong")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</strong>")

	case Superscript:
		r.write("<sup")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sup>")

	case Subscript:
		r.write("<sub")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</sub>")

	case Insert:
		r.write("<ins")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</ins>")

	case Delete:
		r.write("<del")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</del>")

	case Mark:
		r.write("<mark")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</mark>")

	case Link:
		r.write("<a")
		if n.Target != "" || n.HasTarget {
			r.write(" href=\"" + escapeAttr(n.Target) + "\"")
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</a>")

	case Image:
		r.write("<img")
		alt := collectText(n)
		if alt != "" {
			r.write(" alt=\"" + escapeAttr(alt) + "\"")
		}
		if n.Target != "" || n.HasTarget {
			r.write(" src=\"" + escapeAttr(n.Target) + "\"")
		}
		r.renderAttrs(n)
		r.write(">")

	case Span:
		r.write("<span")
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</span>")

	case Verbatim:
		r.write("<code>")
		r.write(escapeHTML(n.Text))
		r.write("</code>")

	case InlineMath:
		r.write(`<span class="math inline">\(`)
		r.write(escapeHTML(n.Text))
		r.write(`\)</span>`)

	case DisplayMath:
		r.write(`<span class="math display">\[`)
		r.write(escapeHTML(n.Text))
		r.write(`\]</span>`)

	case RawInline:
		if n.Format == "html" {
			r.write(n.Text)
		}

	case Symbol:
		r.write(":" + escapeHTML(n.Name) + ":")

	case Footnote:
		// Footnote definitions are rendered in the endnotes section, not inline.
		return

	case FootnoteReference:
		num := r.footnoteNums[n.Label]
		ns := strconv.Itoa(num)
		r.write(`<a id="fnref` + ns + `" href="#fn` + ns + `" role="doc-noteref"><sup>` + ns + `</sup></a>`)

	case DoubleQuoted:
		r.write("\u201c")
		r.renderInlineChildren(n)
		r.write("\u201d")

	case SingleQuoted:
		r.write("\u2018")
		r.renderInlineChildren(n)
		r.write("\u2019")

	case Ellipsis:
		r.write("\u2026")
	case EmDash:
		r.write("\u2014")
	case EnDash:
		r.write("\u2013")
	}
}

func (r *htmlRenderer) renderChildren(n *Node) {
	for _, child := range n.Children {
		r.renderNode(child)
	}
}

func (r *htmlRenderer) renderInlineChildren(n *Node) {
	for _, child := range n.Children {
		r.renderNode(child)
	}
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
func (r *htmlRenderer) renderListItemChildren(n *Node) {
	if r.tight {
		for _, child := range n.Children {
			if child.Kind == Paragraph {
				r.renderInlineChildren(child)
				r.write("\n")
			} else {
				r.renderNode(child)
			}
		}
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
		ns := strconv.Itoa(fi.num)
		r.write("<li id=\"fn" + ns + "\">\n")
		if fi.node != nil && len(fi.node.Children) > 0 {
			// Render all children. Append back-reference to the last paragraph.
			children := fi.node.Children
			lastParagraphIdx := -1
			for i, child := range children {
				if child.Kind == Paragraph {
					lastParagraphIdx = i
				}
			}
			backref := `<a href="#fnref` + ns + `" role="doc-backlink">↩︎</a>`
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
			// Empty or undefined footnote: just the back-reference.
			r.write("<p><a href=\"#fnref" + ns + "\" role=\"doc-backlink\">↩︎</a></p>\n")
		}
		r.write("</li>\n")
	}
	r.write("</ol>\n</section>\n")
}

func (r *htmlRenderer) renderAttrs(n *Node) {
	if len(n.Attrs) == 0 {
		return
	}
	// Use insertion order (attrOrder) for deterministic output. All keys in
	// attrOrder are guaranteed valid (parser-derived or SetAttr-validated).
	for _, k := range n.attrOrder {
		if v, ok := n.Attrs[k]; ok {
			r.write(" " + k + "=\"" + escapeAttr(v) + "\"")
		}
	}
}

// renderNonInternalAttrs is an alias for renderAttrs, kept for call-site clarity
// on list containers where internal attributes were historically filtered.
func (r *htmlRenderer) renderNonInternalAttrs(n *Node) {
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
