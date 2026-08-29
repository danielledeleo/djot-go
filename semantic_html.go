package djot

import (
	"io"
	"strconv"
	"strings"

	"github.com/danielledeleo/djot-go/ast"
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

	hooks *semanticRenderHooks

	tight                 bool
	footnotes             map[string]int
	footnoteNums          map[string]int
	footnoteOrder         []semanticFootnote
	fnrefTotal            map[string]int
	fnrefSeen             map[string]int
	footnoteParagraph     int
	footnoteParagraphInfo *semanticFootnote
	multiBacklinks        bool
	footnoteID            func(int) string
	footnoteRefID         func(int, int) string
	footnoteBacklinkLabel func(int, int, int) string
}

func renderSemanticHTML(tape *semanticTape) string {
	r := renderSemanticHTMLInto(tape, nil, nil)
	return r.out.String()
}

func renderSemanticHTMLTo(w io.Writer, tape *semanticTape) error {
	r := renderSemanticHTMLInto(tape, w, nil)
	return r.err
}

type semanticRenderHooks struct {
	elements              elementHooks
	subtrees              map[ast.Kind]SubtreeRenderFunc
	document              DocumentRenderFunc
	doc                   *Doc
	multiBacklinks        bool
	footnoteID            func(int) string
	footnoteRefID         func(int, int) string
	footnoteBacklinkLabel func(int, int, int) string
}

func renderSemanticHTMLWithHooks(tape *semanticTape, hooks semanticRenderHooks) string {
	r := renderSemanticHTMLInto(tape, nil, &hooks)
	return r.out.String()
}

func renderSemanticHTMLToWithHooks(w io.Writer, tape *semanticTape, hooks semanticRenderHooks) error {
	r := renderSemanticHTMLInto(tape, w, &hooks)
	return r.err
}

func renderSemanticHTMLInto(tape *semanticTape, writer io.Writer, hooks *semanticRenderHooks) *semanticHTMLRenderer {
	r := &semanticHTMLRenderer{
		tape:              tape,
		writer:            writer,
		hooks:             hooks,
		footnotes:         make(map[string]int),
		footnoteNums:      make(map[string]int),
		fnrefTotal:        make(map[string]int),
		fnrefSeen:         make(map[string]int),
		footnoteParagraph: -1,
	}
	if hooks != nil {
		r.multiBacklinks = hooks.multiBacklinks
		r.footnoteID = hooks.footnoteID
		r.footnoteRefID = hooks.footnoteRefID
		r.footnoteBacklinkLabel = hooks.footnoteBacklinkLabel
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
	for i := 0; i+1 < len(tape.records); i++ {
		if r.kind(i) == ast.KindFootnote {
			r.footnotes[r.label(i)] = i
		}
	}
	r.walkRefs(0)
	for i := 0; i < len(r.footnoteOrder); i++ {
		if node := r.footnoteOrder[i].node; node >= 0 {
			r.children(node, r.walkRefs)
		}
	}
	r.renderDocument()
	return r
}

func (r *semanticHTMLRenderer) renderDocument() {
	if r.hooks == nil || r.hooks.document == nil {
		r.renderDocumentDefault()
		return
	}
	state := documentViewState{root: ElementView{tape: r.tape, record: 0}, semantic: r, doc: r.hooks.doc}
	r.hooks.document(
		DocumentView{state: &state},
		DocumentRenderer{semantic: r},
	)
}

func (r *semanticHTMLRenderer) renderDocumentDefault() {
	clear(r.fnrefSeen)
	r.tight = false
	r.renderChildren(0)
	r.renderFootnotes()
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

func (r *semanticHTMLRenderer) kind(i int) ast.Kind {
	return ast.Kind(r.tape.records[i].kind)
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
	if r.kind(i) == ast.KindFootnote {
		return
	}
	if r.kind(i) == ast.KindFootnoteReference {
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
	r.renderChildren(i)
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
	if r.err != nil {
		return
	}
	switch r.kind(i) {
	case ast.KindSymbol:
		if r.hooks != nil && r.hooks.elements.symbol != nil {
			r.hooks.elements.symbol(
				SymbolView{Name: r.tape.text(r.tape.records[i].payload)},
				ElementRenderer{semantic: r, record: i},
			)
			return
		}
	case ast.KindDiv:
		if r.hooks != nil && r.hooks.elements.div != nil {
			record := r.tape.records[i]
			position := r.tape.positions[i]
			r.hooks.elements.div(
				DivView{
					attributes: AttributeView{
						tape: r.tape, start: record.attrStart, end: r.tape.records[i+1].attrStart,
					},
					span: ast.SourceSpan{
						Start: ast.Pos{Offset: int(position.start)},
						End:   ast.Pos{Offset: int(position.end)},
					},
				},
				ElementRenderer{semantic: r, record: i},
			)
			return
		}
	}
	if r.hooks != nil {
		if fn, ok := r.hooks.subtrees[r.kind(i)]; ok {
			fn(
				SubtreeView{root: ElementView{tape: r.tape, record: i}},
				ElementRenderer{semantic: r, record: i},
			)
			return
		}
	}
	r.renderDefault(i)
}

func (r *semanticHTMLRenderer) renderDefault(i int) {
	record := r.tape.records[i]
	switch r.kind(i) {
	case ast.KindDocument:
		r.renderChildren(i)
	case ast.KindSection:
		r.write("<section")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</section>\n")
	case ast.KindParagraph:
		if r.tight {
			r.renderChildren(i)
			r.writeByte('\n')
			return
		}
		r.write("<p")
		r.renderAttrs(i)
		r.writeByte('>')
		r.renderChildren(i)
		if i == r.footnoteParagraph && r.footnoteParagraphInfo != nil {
			r.renderBackref(*r.footnoteParagraphInfo)
		}
		r.write("</p>\n")
	case ast.KindHeading:
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
	case ast.KindThematicBreak:
		r.write("<hr")
		r.renderAttrs(i)
		r.write(">\n")
	case ast.KindCodeBlock:
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
	case ast.KindRawBlock:
		payload := r.textExtra(i)
		if payload.extra == "html" {
			r.write(payload.text)
		}
	case ast.KindBlockQuote, ast.KindDiv:
		tag := "blockquote"
		if r.kind(i) == ast.KindDiv {
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
	case ast.KindBulletList, ast.KindTaskList, ast.KindDefinitionList:
		tag := "ul"
		if r.kind(i) == ast.KindDefinitionList {
			tag = "dl"
		}
		r.writeByte('<')
		r.write(tag)
		if r.kind(i) == ast.KindTaskList {
			r.write(` class="task-list"`)
		}
		r.renderAttrs(i)
		r.write(">\n")
		r.withTight(record.flags&semanticTight != 0, func() { r.renderChildren(i) })
		r.write("</")
		r.write(tag)
		r.write(">\n")
	case ast.KindOrderedList:
		r.write("<ol")
		start := r.tape.listStarts[record.payload]
		if start != 1 {
			r.write(` start="`)
			r.write(strconv.Itoa(start))
			r.write(`"`)
		}
		switch ast.ListStyle(record.small) {
		case ast.ListAlphaLower:
			r.write(` type="a"`)
		case ast.ListAlphaUpper:
			r.write(` type="A"`)
		case ast.ListRomanLower:
			r.write(` type="i"`)
		case ast.ListRomanUpper:
			r.write(` type="I"`)
		}
		r.renderAttrs(i)
		r.write(">\n")
		r.withTight(record.flags&semanticTight != 0, func() { r.renderChildren(i) })
		r.write("</ol>\n")
	case ast.KindTable:
		r.write("<table")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</table>\n")
	case ast.KindCaption:
		r.write("<caption>")
		r.renderChildren(i)
		r.write("</caption>\n")
	case ast.KindTableRow:
		r.write("<tr")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderChildren(i)
		r.write("</tr>\n")
	case ast.KindTableCell:
		tag := "td"
		if record.flags&semanticHeader != 0 {
			tag = "th"
		}
		r.writeByte('<')
		r.write(tag)
		if alignment := ast.CellAlign(record.small); alignment != ast.AlignDefault {
			name := "left"
			if alignment == ast.AlignRight {
				name = "right"
			} else if alignment == ast.AlignCenter {
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
	case ast.KindTerm:
		r.write("<dt>")
		r.renderChildren(i)
		r.write("</dt>\n")
	case ast.KindDefinition:
		r.write("<dd>\n")
		r.renderListItemChildren(i)
		r.write("</dd>\n")
	case ast.KindListItem:
		r.write("<li")
		r.renderAttrs(i)
		r.write(">\n")
		r.renderListItemChildren(i)
		r.write("</li>\n")
	case ast.KindTaskListItem:
		r.write("<li>\n")
		if record.flags&semanticChecked != 0 {
			r.write(`<input disabled="" type="checkbox" checked=""/>`)
		} else {
			r.write(`<input disabled="" type="checkbox"/>`)
		}
		r.writeByte('\n')
		r.renderListItemChildren(i)
		r.write("</li>\n")
	case ast.KindText:
		r.write(escapeHTML(r.tape.text(record.payload)))
	case ast.KindSoftBreak:
		r.writeByte('\n')
	case ast.KindHardBreak:
		r.write("<br>\n")
	case ast.KindNonBreakingSpace:
		r.write("&nbsp;")
	case ast.KindEmphasis, ast.KindStrong, ast.KindSuperscript, ast.KindSubscript, ast.KindInsert, ast.KindDelete, ast.KindMark, ast.KindSpan:
		tag := ""
		switch r.kind(i) {
		case ast.KindEmphasis:
			tag = "em"
		case ast.KindStrong:
			tag = "strong"
		case ast.KindSuperscript:
			tag = "sup"
		case ast.KindSubscript:
			tag = "sub"
		case ast.KindInsert:
			tag = "ins"
		case ast.KindDelete:
			tag = "del"
		case ast.KindMark:
			tag = "mark"
		case ast.KindSpan:
			tag = "span"
		}
		r.renderContainer(i, tag)
	case ast.KindLink:
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
	case ast.KindImage:
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
	case ast.KindVerbatim:
		r.write("<code>")
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write("</code>")
	case ast.KindInlineMath:
		r.write(`<span class="math inline">\(`)
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write(`\)</span>`)
	case ast.KindDisplayMath:
		r.write(`<span class="math display">\[`)
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.write(`\]</span>`)
	case ast.KindRawInline:
		payload := r.textExtra(i)
		if payload.extra == "html" {
			r.write(payload.text)
		}
	case ast.KindSymbol:
		r.writeByte(':')
		r.write(escapeHTML(r.tape.text(record.payload)))
		r.writeByte(':')
	case ast.KindFootnote:
		return
	case ast.KindFootnoteReference:
		label := r.label(i)
		num := r.footnoteNums[label]
		r.fnrefSeen[label]++
		k := r.fnrefSeen[label]
		r.write(`<a`)
		if r.multiBacklinks || k == 1 {
			r.write(` id="`)
			r.write(escapeAttr(r.footnoteRefID(num, k)))
			r.write(`"`)
		}
		r.write(` href="`)
		r.write(escapeAttr("#" + r.footnoteID(num)))
		r.write(`" role="doc-noteref"><sup>`)
		r.write(strconv.Itoa(num))
		r.write(`</sup></a>`)
	case ast.KindDoubleQuoted:
		r.write("“")
		r.renderChildren(i)
		r.write("”")
	case ast.KindSingleQuoted:
		r.write("‘")
		r.renderChildren(i)
		r.write("’")
	case ast.KindEllipsis:
		r.write("…")
	case ast.KindEmDash:
		r.write("—")
	case ast.KindEnDash:
		r.write("–")
	}
}

func (r *semanticHTMLRenderer) collectText(i int) string {
	var out strings.Builder
	r.appendText(&out, i)
	return out.String()
}

func (r *semanticHTMLRenderer) appendText(out *strings.Builder, i int) {
	switch r.kind(i) {
	case ast.KindText:
		out.WriteString(r.tape.text(r.tape.records[i].payload))
		return
	case ast.KindSoftBreak, ast.KindHardBreak, ast.KindNonBreakingSpace:
		out.WriteByte(' ')
		return
	}
	r.children(i, func(child int) { r.appendText(out, child) })
}

func (r *semanticHTMLRenderer) renderFootnotes() {
	if len(r.footnoteOrder) == 0 {
		return
	}
	r.write("<section role=\"doc-endnotes\">\n<hr>\n<ol>\n")
	for _, footnote := range r.footnoteOrder {
		r.write(`<li id="`)
		r.write(escapeAttr(r.footnoteID(footnote.num)))
		r.write(`">` + "\n")
		if footnote.node >= 0 && int(r.tape.records[footnote.node].subtreeEnd) > footnote.node+1 {
			lastParagraph := -1
			r.children(footnote.node, func(child int) {
				if r.kind(child) == ast.KindParagraph {
					lastParagraph = child
				}
			})
			r.children(footnote.node, func(child int) {
				if child == lastParagraph {
					previousParagraph, previousInfo := r.footnoteParagraph, r.footnoteParagraphInfo
					r.footnoteParagraph, r.footnoteParagraphInfo = child, &footnote
					r.renderNode(child)
					r.footnoteParagraph, r.footnoteParagraphInfo = previousParagraph, previousInfo
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
	total := 1
	if r.multiBacklinks && r.fnrefTotal[footnote.label] > 1 {
		total = r.fnrefTotal[footnote.label]
	}
	for k := 1; k <= total; k++ {
		if k > 1 {
			r.writeByte(' ')
		}
		r.write(`<a href="`)
		r.write(escapeAttr("#" + r.footnoteRefID(footnote.num, k)))
		r.write(`" role="doc-backlink">`)
		r.write(escapeHTML(r.footnoteBacklinkLabel(footnote.num, k, total)))
		r.write(`</a>`)
	}
}
