package djot

import (
	"fmt"
	"io"
	"strings"
)

// RenderOption configures HTML rendering.
type RenderOption func(*renderConfig)

// NodeRenderFunc is a hook that overrides rendering for a specific node kind.
type NodeRenderFunc func(n *Node, r NodeRenderer)

// NodeRenderer provides access to the rendering pipeline from within a hook.
type NodeRenderer interface {
	Children() // render this node's children through the full pipeline
	Default()  // render this node as if no hook were registered
	Write(s string)
}

type renderConfig struct {
	hooks map[NodeKind]NodeRenderFunc
}

// WithNodeRenderer registers a render hook for the given node kind.
// If called multiple times for the same kind, the last one wins.
func WithNodeRenderer(kind NodeKind, fn NodeRenderFunc) RenderOption {
	return func(cfg *renderConfig) {
		if cfg.hooks == nil {
			cfg.hooks = make(map[NodeKind]NodeRenderFunc)
		}
		cfg.hooks[kind] = fn
	}
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

	hooks      map[NodeKind]NodeRenderFunc
	bypassHook bool // when true, renderNode skips hook lookup (used by Default)

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
	if r.doc != nil {
		fi.node = r.doc.Footnotes[label]
	}
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
	nr.r.bypassHook = true
	nr.r.renderNode(nr.n)
	nr.r.bypassHook = false
}

func (nr *nodeRendererImpl) Write(s string) {
	nr.r.write(s)
}

func (r *htmlRenderer) renderNode(n *Node) {
	if !r.bypassHook {
		if fn, ok := r.hooks[n.Kind]; ok {
			fn(n, &nodeRendererImpl{r: r, n: n})
			return
		}
	}
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
		tag := fmt.Sprintf("h%d", n.Level)
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
			r.write(fmt.Sprintf(" class=\"language-%s\"", escapeAttr(n.Lang)))
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
		tight := n.Attr("tight") == "true"
		for _, child := range n.Children {
			r.renderListItem(child, tight)
		}
		r.write("</ul>\n")

	case OrderedList:
		r.write("<ol")
		if n.ListStart != 1 {
			r.write(fmt.Sprintf(" start=\"%d\"", n.ListStart))
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
		tight := n.Attr("tight") == "true"
		for _, child := range n.Children {
			r.renderListItem(child, tight)
		}
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
			r.write(fmt.Sprintf(` style="text-align: %s;"`, alignStr))
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</" + tag + ">\n")

	case DefinitionList:
		r.write("<dl")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		tight := n.Attr("tight") == "true"
		for _, child := range n.Children {
			switch child.Kind {
			case Term:
				r.write("<dt>")
				r.renderInlineChildren(child)
				r.write("</dt>\n")
			case Definition:
				r.write("<dd>\n")
				if tight {
					for _, c := range child.Children {
						if c.Kind == Paragraph {
							r.renderInlineChildren(c)
							r.write("\n")
						} else {
							r.renderNode(c)
						}
					}
				} else {
					r.renderChildren(child)
				}
				r.write("</dd>\n")
			}
		}
		r.write("</dl>\n")

	case TaskList:
		r.write("<ul class=\"task-list\"")
		r.renderNonInternalAttrs(n)
		r.write(">\n")
		tight := n.Attr("tight") == "true"
		for _, child := range n.Children {
			r.renderTaskListItem(child, tight)
		}
		r.write("</ul>\n")

	case TaskListItem:
		// Handled by TaskList rendering
		r.write("<li>\n")
		r.renderChildren(n)
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
			r.write(fmt.Sprintf(" href=\"%s\"", escapeAttr(n.Target)))
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</a>")

	case Image:
		r.write("<img")
		alt := collectText(n)
		if alt != "" {
			r.write(fmt.Sprintf(" alt=\"%s\"", escapeAttr(alt)))
		}
		if n.Target != "" || n.HasTarget {
			r.write(fmt.Sprintf(" src=\"%s\"", escapeAttr(n.Target)))
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
		r.write(":" + n.Name + ":")

	case Footnote:
		// Footnote definitions are rendered in the endnotes section, not inline.
		return

	case FootnoteReference:
		num := r.footnoteNums[n.Label]
		r.write(fmt.Sprintf(`<a id="fnref%d" href="#fn%d" role="doc-noteref"><sup>%d</sup></a>`,
			num, num, num))

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

func (r *htmlRenderer) renderTaskListItem(n *Node, tight bool) {
	r.write("<li>\n")
	if n.Checked {
		r.write(`<input disabled="" type="checkbox" checked=""/>`)
	} else {
		r.write(`<input disabled="" type="checkbox"/>`)
	}
	r.write("\n")
	if tight {
		for _, child := range n.Children {
			if child.Kind == Paragraph {
				r.renderInlineChildren(child)
				r.write("\n")
			} else {
				r.renderNode(child)
			}
		}
	} else {
		r.renderChildren(n)
	}
	r.write("</li>\n")
}

func (r *htmlRenderer) renderListItem(n *Node, tight bool) {
	r.write("<li")
	r.renderAttrs(n)
	r.write(">\n")
	if tight {
		// In tight lists, render paragraph content directly without <p> tags.
		for _, child := range n.Children {
			if child.Kind == Paragraph {
				r.renderInlineChildren(child)
				r.write("\n")
			} else {
				r.renderNode(child)
			}
		}
	} else {
		r.renderChildren(n)
	}
	r.write("</li>\n")
}

func (r *htmlRenderer) renderFootnotesSection() {
	if len(r.footnoteOrder) == 0 {
		return
	}
	r.write("<section role=\"doc-endnotes\">\n<hr>\n<ol>\n")
	for _, fi := range r.footnoteOrder {
		r.write(fmt.Sprintf("<li id=\"fn%d\">\n", fi.num))
		if fi.node != nil && len(fi.node.Children) > 0 {
			// Render all children. Append back-reference to the last paragraph.
			children := fi.node.Children
			lastParagraphIdx := -1
			for i, child := range children {
				if child.Kind == Paragraph {
					lastParagraphIdx = i
				}
			}
			backref := fmt.Sprintf(`<a href="#fnref%d" role="doc-backlink">↩︎</a>`, fi.num)
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
				r.write(fmt.Sprintf("<p>%s</p>\n", backref))
			}
		} else {
			// Empty or undefined footnote: just the back-reference.
			r.write(fmt.Sprintf("<p><a href=\"#fnref%d\" role=\"doc-backlink\">↩︎</a></p>\n", fi.num))
		}
		r.write("</li>\n")
	}
	r.write("</ol>\n</section>\n")
}

func (r *htmlRenderer) renderAttrs(n *Node) {
	if n.Attrs == nil || len(n.Attrs) == 0 {
		return
	}
	// Use insertion order (AttrOrder) for deterministic output.
	for _, k := range n.AttrOrder {
		if k == "tight" {
			continue // internal attribute
		}
		if v, ok := n.Attrs[k]; ok {
			r.write(fmt.Sprintf(` %s="%s"`, k, escapeAttr(v)))
		}
	}
}

func (r *htmlRenderer) renderNonInternalAttrs(n *Node) {
	if n.Attrs == nil || len(n.Attrs) == 0 {
		return
	}
	for _, k := range n.AttrOrder {
		if k == "tight" {
			continue
		}
		if v, ok := n.Attrs[k]; ok {
			r.write(fmt.Sprintf(` %s="%s"`, k, escapeAttr(v)))
		}
	}
}

func escapeHTML(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
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
	var b strings.Builder
	for i := 0; i < len(s); i++ {
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
