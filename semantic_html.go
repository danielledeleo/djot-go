package djot

import (
	"io"
	"strconv"
	"strings"
)

type semanticFootnote struct {
	num   int
	label string
	node  int
}

type semanticHTMLRenderer struct {
	tape   *semanticTape
	out    strings.Builder
	writer io.Writer
	err    error

	tight         bool
	footnotes     map[string]int
	footnoteNums  map[string]int
	footnoteOrder []semanticFootnote
	fnrefTotal    map[string]int
	fnrefSeen     map[string]int
}

func renderSemanticHTML(tape *semanticTape) string {
	r := renderSemanticHTMLInto(tape, nil)
	return r.out.String()
}

func renderSemanticHTMLTo(w io.Writer, tape *semanticTape) error {
	r := renderSemanticHTMLInto(tape, w)
	return r.err
}

func renderSemanticHTMLInto(tape *semanticTape, writer io.Writer) *semanticHTMLRenderer {
	r := &semanticHTMLRenderer{
		tape:         tape,
		writer:       writer,
		footnotes:    make(map[string]int),
		footnoteNums: make(map[string]int),
		fnrefTotal:   make(map[string]int),
		fnrefSeen:    make(map[string]int),
	}
	for i := 0; i+1 < len(tape.records); i++ {
		if r.kind(i) == KindFootnote {
			r.footnotes[r.label(i)] = i
		}
	}
	r.walkRefs(0)
	for i := 0; i < len(r.footnoteOrder); i++ {
		if node := r.footnoteOrder[i].node; node >= 0 {
			r.children(node, r.walkRefs)
		}
	}
	r.renderChildren(0)
	r.renderFootnotes()
	return r
}

func (r *semanticHTMLRenderer) write(value string) {
	if r.err != nil {
		return
	}
	if r.writer == nil {
		r.out.WriteString(value)
		return
	}
	_, r.err = io.WriteString(r.writer, value)
}

func (r *semanticHTMLRenderer) writeByte(value byte) {
	if r.writer == nil {
		r.out.WriteByte(value)
		return
	}
	switch value {
	case ' ':
		r.write(" ")
	case '\n':
		r.write("\n")
	case '"':
		r.write("\"")
	case '<':
		r.write("<")
	case '>':
		r.write(">")
	case ':':
		r.write(":")
	default:
		var one [1]byte
		one[0] = value
		if r.err == nil {
			_, r.err = r.writer.Write(one[:])
		}
	}
}

func (r *semanticHTMLRenderer) kind(i int) Kind {
	return Kind(r.tape.records[i].kind)
}

func (r *semanticHTMLRenderer) label(i int) string {
	id := r.tape.records[i].payload
	if id == 0 {
		return ""
	}
	return r.tape.labels[id]
}

func (r *semanticHTMLRenderer) target(i int) string {
	id := r.tape.records[i].payload
	if id == 0 {
		return ""
	}
	return r.tape.targets[id]
}

func (r *semanticHTMLRenderer) textExtra(i int) semanticTextExtra {
	id := r.tape.records[i].payload
	if id == 0 {
		return semanticTextExtra{}
	}
	return r.tape.textExtras[id]
}

func (r *semanticHTMLRenderer) children(i int, fn func(int)) {
	for child, end := i+1, int(r.tape.records[i].subtreeEnd); child < end; {
		fn(child)
		child = int(r.tape.records[child].subtreeEnd)
	}
}

func (r *semanticHTMLRenderer) walkRefs(i int) {
	if r.kind(i) == KindFootnote {
		return
	}
	if r.kind(i) == KindFootnoteReference {
		label := r.label(i)
		r.footnoteNum(label)
		r.fnrefTotal[label]++
	}
	r.children(i, r.walkRefs)
}

func (r *semanticHTMLRenderer) footnoteNum(label string) int {
	if num, ok := r.footnoteNums[label]; ok {
		return num
	}
	num := len(r.footnoteOrder) + 1
	r.footnoteNums[label] = num
	node := -1
	if i, ok := r.footnotes[label]; ok {
		node = i
	}
	r.footnoteOrder = append(r.footnoteOrder, semanticFootnote{num: num, label: label, node: node})
	return num
}

func (r *semanticHTMLRenderer) renderAttrs(i int) {
	start := r.tape.records[i].attrStart
	end := r.tape.records[i+1].attrStart
	for j := start; j < end; j++ {
		attr := r.tape.attributes[j]
		r.writeByte(' ')
		r.write(attr.key)
		r.write(`="`)
		r.write(escapeAttr(attr.value))
		r.writeByte('"')
	}
}

func (r *semanticHTMLRenderer) renderChildren(i int) {
	r.children(i, r.renderNode)
}

func (r *semanticHTMLRenderer) withTight(tight bool, fn func()) {
	previous := r.tight
	r.tight = tight
	fn()
	r.tight = previous
}

func (r *semanticHTMLRenderer) renderListItemChildren(i int) {
	if !r.tight {
		r.renderChildren(i)
		return
	}
	r.children(i, func(child int) {
		if r.kind(child) == KindParagraph {
			r.renderChildren(child)
			r.writeByte('\n')
		} else {
			r.renderNode(child)
		}
	})
}

func (r *semanticHTMLRenderer) renderContainer(i int, tag string) {
	r.writeByte('<')
	r.write(tag)
	r.renderAttrs(i)
	r.writeByte('>')
	r.renderChildren(i)
	r.write("</")
	r.write(tag)
	r.writeByte('>')
}

func (r *semanticHTMLRenderer) renderNode(i int) {
	record := r.tape.records[i]
	switch r.kind(i) {
	case KindDocument:
		r.renderChildren(i)
	case KindSection:
		r.write("<section")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</section>\n")
	case KindParagraph:
		r.write("<p")
		r.renderAttrs(i)
		r.writeByte('>')
		r.renderChildren(i)
		r.write("</p>\n")
	case KindHeading:
		level := int(record.small)
		if level < 1 {
			level = 1
		} else if level > 6 {
			level = 6
		}
		tag := "h" + strconv.Itoa(level)
		r.writeByte('<')
		r.write(tag)
		r.renderAttrs(i)
		r.writeByte('>')
		r.renderChildren(i)
		r.write("</")
		r.write(tag)
		r.write(">\n")
	case KindThematicBreak:
		r.write("<hr")
		r.renderAttrs(i)
		r.write(">\n")
	case KindCodeBlock:
		payload := r.textExtra(i)
		r.write("<pre")
		r.renderAttrs(i)
		r.write("><code")
		if payload.extra != "" {
			r.write(` class="language-`)
			r.write(escapeAttr(payload.extra))
			r.write(`"`)
		}
		r.writeByte('>')
		r.write(escapeHTML(payload.text))
		r.write("</code></pre>\n")
	case KindRawBlock:
		payload := r.textExtra(i)
		if payload.extra == "html" {
			r.write(payload.text)
		}
	case KindBlockQuote, KindDiv:
		tag := "blockquote"
		if r.kind(i) == KindDiv {
			tag = "div"
		}
		r.writeByte('<')
		r.write(tag)
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</")
		r.write(tag)
		r.write(">\n")
	case KindBulletList, KindTaskList, KindDefinitionList:
		tag := "ul"
		if r.kind(i) == KindDefinitionList {
			tag = "dl"
		}
		r.writeByte('<')
		r.write(tag)
		if r.kind(i) == KindTaskList {
			r.write(` class="task-list"`)
		}
		r.renderAttrs(i)
		r.write(">\n")
		r.withTight(record.flags&semanticTight != 0, func() { r.renderChildren(i) })
		r.write("</")
		r.write(tag)
		r.write(">\n")
	case KindOrderedList:
		r.write("<ol")
		start := r.tape.listStarts[record.payload]
		if start != 1 {
			r.write(` start="`)
			r.write(strconv.Itoa(start))
			r.write(`"`)
		}
		switch ListStyle(record.small) {
		case ListAlphaLower:
			r.write(` type="a"`)
		case ListAlphaUpper:
			r.write(` type="A"`)
		case ListRomanLower:
			r.write(` type="i"`)
		case ListRomanUpper:
			r.write(` type="I"`)
		}
		r.renderAttrs(i)
		r.write(">\n")
		r.withTight(record.flags&semanticTight != 0, func() { r.renderChildren(i) })
		r.write("</ol>\n")
	case KindTable:
		r.write("<table")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</table>\n")
	case KindCaption:
		r.write("<caption>")
		r.renderChildren(i)
		r.write("</caption>\n")
	case KindTableRow:
		r.write("<tr")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</tr>\n")
	case KindTableCell:
		tag := "td"
		if record.flags&semanticHeader != 0 {
			tag = "th"
		}
		r.writeByte('<')
		r.write(tag)
		if alignment := CellAlign(record.small); alignment != AlignDefault {
			name := "left"
			if alignment == AlignRight {
				name = "right"
			} else if alignment == AlignCenter {
				name = "center"
			}
			r.write(` style="text-align: `)
			r.write(name)
			r.write(`;"`)
		}
		r.renderAttrs(i)
		r.writeByte('>')
		r.renderChildren(i)
		r.write("</")
		r.write(tag)
		r.write(">\n")
	case KindTerm:
		r.write("<dt>")
		r.renderChildren(i)
		r.write("</dt>\n")
	case KindDefinition:
		r.write("<dd>\n")
		r.renderListItemChildren(i)
		r.write("</dd>\n")
	case KindListItem:
		r.write("<li")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderListItemChildren(i)
		r.write("</li>\n")
	case KindTaskListItem:
		r.write("<li>\n")
		if record.flags&semanticChecked != 0 {
			r.write(`<input disabled="" type="checkbox" checked=""/>`)
		} else {
			r.write(`<input disabled="" type="checkbox"/>`)
		}
		r.writeByte('\n')
		r.renderListItemChildren(i)
		r.write("</li>\n")
	case KindText:
		r.write(escapeHTML(r.tape.text(record.payload)))
	case KindSoftBreak:
		r.writeByte('\n')
	case KindHardBreak:
		r.write("<br>\n")
	case KindNonBreakingSpace:
		r.write("&nbsp;")
	case KindEmphasis, KindStrong, KindSuperscript, KindSubscript, KindInsert, KindDelete, KindMark, KindSpan:
		tag := ""
		switch r.kind(i) {
		case KindEmphasis:
			tag = "em"
		case KindStrong:
			tag = "strong"
		case KindSuperscript:
			tag = "sup"
		case KindSubscript:
			tag = "sub"
		case KindInsert:
			tag = "ins"
		case KindDelete:
			tag = "del"
		case KindMark:
			tag = "mark"
		case KindSpan:
			tag = "span"
		}
		r.renderContainer(i, tag)
	case KindLink:
		r.write("<a")
		target := r.target(i)
		if target != "" || record.flags&semanticHasTarget != 0 {
			r.write(` href="`)
			r.write(escapeAttr(target))
			r.write(`"`)
		}
		r.renderAttrs(i)
		r.writeByte('>')
		r.renderChildren(i)
		r.write("</a>")
	case KindImage:
		r.write("<img")
		if alt := r.collectText(i); alt != "" {
			r.write(` alt="`)
			r.write(escapeAttr(alt))
			r.write(`"`)
		}
		target := r.target(i)
		if target != "" || record.flags&semanticHasTarget != 0 {
			r.write(` src="`)
			r.write(escapeAttr(target))
			r.write(`"`)
		}
		r.renderAttrs(i)
		r.writeByte('>')
	case KindVerbatim:
		r.write("<code>")
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write("</code>")
	case KindInlineMath:
		r.write(`<span class="math inline">\(`)
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write(`\)</span>`)
	case KindDisplayMath:
		r.write(`<span class="math display">\[`)
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write(`\]</span>`)
	case KindRawInline:
		payload := r.textExtra(i)
		if payload.extra == "html" {
			r.write(payload.text)
		}
	case KindSymbol:
		r.writeByte(':')
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.writeByte(':')
	case KindFootnote:
		return
	case KindFootnoteReference:
		label := r.label(i)
		num := r.footnoteNums[label]
		r.fnrefSeen[label]++
		r.write(`<a`)
		if r.fnrefSeen[label] == 1 {
			r.write(` id="fnref`)
			r.write(strconv.Itoa(num))
			r.write(`"`)
		}
		r.write(` href="#fn`)
		r.write(strconv.Itoa(num))
		r.write(`" role="doc-noteref"><sup>`)
		r.write(strconv.Itoa(num))
		r.write(`</sup></a>`)
	case KindDoubleQuoted:
		r.write("“")
		r.renderChildren(i)
		r.write("”")
	case KindSingleQuoted:
		r.write("‘")
		r.renderChildren(i)
		r.write("’")
	case KindEllipsis:
		r.write("…")
	case KindEmDash:
		r.write("—")
	case KindEnDash:
		r.write("–")
	}
}

func (r *semanticHTMLRenderer) collectText(i int) string {
	switch r.kind(i) {
	case KindText:
		return r.tape.text(r.tape.records[i].payload)
	case KindSoftBreak, KindHardBreak, KindNonBreakingSpace:
		return " "
	}
	var out strings.Builder
	r.children(i, func(child int) { out.WriteString(r.collectText(child)) })
	return out.String()
}

func (r *semanticHTMLRenderer) renderFootnotes() {
	if len(r.footnoteOrder) == 0 {
		return
	}
	r.write("<section role=\"doc-endnotes\">\n<hr>\n<ol>\n")
	for _, footnote := range r.footnoteOrder {
		r.write(`<li id="fn`)
		r.write(strconv.Itoa(footnote.num))
		r.write(`">` + "\n")
		if footnote.node >= 0 && int(r.tape.records[footnote.node].subtreeEnd) > footnote.node+1 {
			lastParagraph := -1
			r.children(footnote.node, func(child int) {
				if r.kind(child) == KindParagraph {
					lastParagraph = child
				}
			})
			r.children(footnote.node, func(child int) {
				if child == lastParagraph {
					r.write("<p")
					r.renderAttrs(child)
					r.writeByte('>')
					r.renderChildren(child)
					r.renderBackref(footnote)
					r.write("</p>\n")
				} else {
					r.renderNode(child)
				}
			})
			if lastParagraph < 0 {
				r.write("<p>")
				r.renderBackref(footnote)
				r.write("</p>\n")
			}
		} else {
			r.write("<p>")
			r.renderBackref(footnote)
			r.write("</p>\n")
		}
		r.write("</li>\n")
	}
	r.write("</ol>\n</section>\n")
}

func (r *semanticHTMLRenderer) renderBackref(footnote semanticFootnote) {
	r.write(`<a href="#fnref`)
	r.write(strconv.Itoa(footnote.num))
	r.write(`" role="doc-backlink">↩︎</a>`)
}
