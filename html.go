package djot

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// RenderHTML renders a parsed document to HTML.
func RenderHTML(doc *Doc) string {
	var b strings.Builder
	r := &htmlRenderer{w: &b}
	r.renderChildren(doc.Root)
	return b.String()
}

// RenderHTMLTo renders a parsed document to the given writer.
func RenderHTMLTo(w io.Writer, doc *Doc) error {
	r := &htmlRenderer{w: w}
	r.renderChildren(doc.Root)
	return r.err
}

type htmlRenderer struct {
	w   io.Writer
	err error
}

func (r *htmlRenderer) write(s string) {
	if r.err != nil {
		return
	}
	_, r.err = io.WriteString(r.w, s)
}

func (r *htmlRenderer) renderNode(n *Node) {
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
		r.write("<pre><code")
		if n.Lang != "" {
			r.write(fmt.Sprintf(" class=\"language-%s\"", escapeAttr(n.Lang)))
		}
		r.renderAttrs(n)
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
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</" + tag + ">\n")

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
		if n.Target != "" {
			r.write(fmt.Sprintf(" href=\"%s\"", escapeAttr(n.Target)))
		}
		r.renderAttrs(n)
		r.write(">")
		r.renderInlineChildren(n)
		r.write("</a>")

	case Image:
		r.write("<img")
		if n.Target != "" {
			r.write(fmt.Sprintf(" src=\"%s\"", escapeAttr(n.Target)))
		}
		alt := collectText(n)
		if alt != "" {
			r.write(fmt.Sprintf(" alt=\"%s\"", escapeAttr(alt)))
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

	case FootnoteReference:
		// Simplified footnote reference rendering.
		r.write(fmt.Sprintf(`<a id="fnref%s" href="#fn%s" role="doc-noteref"><sup>%s</sup></a>`,
			n.Label, n.Label, n.Label))

	case Ellipsis:
		r.write("…")
	case EmDash:
		r.write("—")
	case EnDash:
		r.write("–")
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

func (r *htmlRenderer) renderAttrs(n *Node) {
	if n.Attrs == nil || len(n.Attrs) == 0 {
		return
	}
	// Sort attributes alphabetically for deterministic output.
	keys := make([]string, 0, len(n.Attrs))
	for k := range n.Attrs {
		if k == "tight" {
			continue // internal attribute
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.write(fmt.Sprintf(` %s="%s"`, k, escapeAttr(n.Attrs[k])))
	}
}

func (r *htmlRenderer) renderNonInternalAttrs(n *Node) {
	if n.Attrs == nil || len(n.Attrs) == 0 {
		return
	}
	keys := make([]string, 0, len(n.Attrs))
	for k := range n.Attrs {
		if k == "tight" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.write(fmt.Sprintf(` %s="%s"`, k, escapeAttr(n.Attrs[k])))
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
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func escapeAttr(s string) string {
	return escapeHTML(s)
}
