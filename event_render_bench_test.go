package djot_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

type semanticFootnotePrototype struct {
	num   int
	label string
	node  int
}

// semanticHTMLPrototype renders the compact tape without rebuilding Nodes.
// It intentionally implements only the default renderer configuration; hook
// planning is a separate prototype. Its output is checked byte-for-byte against
// the production AST renderer before its benchmark is meaningful.
type semanticHTMLPrototype struct {
	tape           *semanticTapePrototype
	kindPayloads   *kindPayloadsPrototype
	sourcePayloads *kindSourcePayloadsPrototype
	out            strings.Builder

	tight         bool
	footnotes     map[string]int
	footnoteNums  map[string]int
	footnoteOrder []semanticFootnotePrototype
	fnrefTotal    map[string]int
	fnrefSeen     map[string]int
}

func renderSemanticHTMLPrototype(tape *semanticTapePrototype) string {
	return renderSemanticHTMLWithPayloadsPrototype(tape, nil)
}

func renderSemanticHTMLWithPayloadsPrototype(tape *semanticTapePrototype, kindPayloads *kindPayloadsPrototype) string {
	r := &semanticHTMLPrototype{
		tape:         tape,
		kindPayloads: kindPayloads,
		footnotes:    make(map[string]int),
		footnoteNums: make(map[string]int),
		fnrefTotal:   make(map[string]int),
		fnrefSeen:    make(map[string]int),
	}
	for i := 0; i+1 < len(tape.records); i++ {
		if r.kind(i) == djot.Footnote {
			r.footnotes[r.payload(i).extra] = i
		}
	}
	r.walkRefs(0)
	for i := 0; i < len(r.footnoteOrder); i++ {
		fi := r.footnoteOrder[i]
		if fi.node >= 0 {
			r.walkChildRefs(fi.node)
		}
	}
	r.renderChildren(0)
	r.renderFootnotes()
	return r.out.String()
}

func renderSemanticHTMLWithSourcePayloadsPrototype(tape *semanticTapePrototype, sourcePayloads *kindSourcePayloadsPrototype) string {
	r := &semanticHTMLPrototype{
		tape:           tape,
		sourcePayloads: sourcePayloads,
		footnotes:      make(map[string]int),
		footnoteNums:   make(map[string]int),
		fnrefTotal:     make(map[string]int),
		fnrefSeen:      make(map[string]int),
	}
	for i := 0; i+1 < len(tape.records); i++ {
		if r.kind(i) == djot.Footnote {
			r.footnotes[r.payload(i).extra] = i
		}
	}
	r.walkRefs(0)
	for i := 0; i < len(r.footnoteOrder); i++ {
		fi := r.footnoteOrder[i]
		if fi.node >= 0 {
			r.walkChildRefs(fi.node)
		}
	}
	r.renderChildren(0)
	r.renderFootnotes()
	return r.out.String()
}

func (r *semanticHTMLPrototype) kind(i int) djot.NodeKind {
	return djot.NodeKind(r.tape.records[i].kind)
}

func (r *semanticHTMLPrototype) payload(i int) semanticPayloadPrototype {
	if r.sourcePayloads != nil {
		id := r.sourcePayloads.payloadIDs[i]
		if id == 0 {
			return semanticPayloadPrototype{}
		}
		switch r.kind(i) {
		case djot.Text, djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol:
			if id&sourceTextValueBitPrototype != 0 {
				return semanticPayloadPrototype{text: r.sourcePayloads.textValues[id&^sourceTextValueBitPrototype]}
			}
			span := r.sourcePayloads.textSpans[id]
			return semanticPayloadPrototype{text: r.sourcePayloads.source[span.start:span.end]}
		case djot.Link, djot.Image:
			return semanticPayloadPrototype{target: r.sourcePayloads.targets[id]}
		case djot.CodeBlock, djot.RawBlock, djot.RawInline:
			payload := r.sourcePayloads.textExtras[id]
			return semanticPayloadPrototype{text: payload.text, extra: payload.extra}
		case djot.Footnote, djot.FootnoteReference:
			return semanticPayloadPrototype{extra: r.sourcePayloads.labels[id]}
		case djot.OrderedList:
			return semanticPayloadPrototype{number: int32(id)}
		}
	}
	if r.kindPayloads != nil {
		id := r.kindPayloads.payloadIDs[i]
		if id == 0 {
			return semanticPayloadPrototype{}
		}
		switch r.kind(i) {
		case djot.Text, djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol:
			return semanticPayloadPrototype{text: r.kindPayloads.texts[id]}
		case djot.Link, djot.Image:
			return semanticPayloadPrototype{target: r.kindPayloads.targets[id]}
		case djot.CodeBlock, djot.RawBlock, djot.RawInline:
			payload := r.kindPayloads.textExtras[id]
			return semanticPayloadPrototype{text: payload.text, extra: payload.extra}
		case djot.Footnote, djot.FootnoteReference:
			return semanticPayloadPrototype{extra: r.kindPayloads.labels[id]}
		case djot.OrderedList:
			return semanticPayloadPrototype{number: int32(id)}
		}
	}
	index := r.tape.records[i].payload
	if index == 0 {
		return semanticPayloadPrototype{}
	}
	return r.tape.payloads[index]
}

func (r *semanticHTMLPrototype) children(i int, fn func(int)) {
	for child, end := i+1, int(r.tape.records[i].subtreeEnd); child < end; {
		fn(child)
		child = int(r.tape.records[child].subtreeEnd)
	}
}

func (r *semanticHTMLPrototype) walkRefs(i int) {
	if r.kind(i) == djot.Footnote {
		return
	}
	if r.kind(i) == djot.FootnoteReference {
		label := r.payload(i).extra
		r.footnoteNum(label)
		r.fnrefTotal[label]++
	}
	r.children(i, r.walkRefs)
}

func (r *semanticHTMLPrototype) walkChildRefs(i int) {
	r.children(i, r.walkRefs)
}

func (r *semanticHTMLPrototype) footnoteNum(label string) int {
	if num, ok := r.footnoteNums[label]; ok {
		return num
	}
	num := len(r.footnoteOrder) + 1
	r.footnoteNums[label] = num
	node := -1
	if i, ok := r.footnotes[label]; ok {
		node = i
	}
	r.footnoteOrder = append(r.footnoteOrder, semanticFootnotePrototype{
		num: num, label: label, node: node,
	})
	return num
}

func (r *semanticHTMLPrototype) renderAttrs(i int) {
	start := r.tape.records[i].attrStart
	end := r.tape.records[i+1].attrStart
	for j := start; j < end; j++ {
		attr := r.tape.attributes[j]
		r.out.WriteByte(' ')
		r.out.WriteString(attr.key)
		r.out.WriteString(`="`)
		r.out.WriteString(escapeSemanticAttrPrototype(attr.value))
		r.out.WriteByte('"')
	}
}

func (r *semanticHTMLPrototype) renderChildren(i int) {
	r.children(i, r.renderNode)
}

func (r *semanticHTMLPrototype) renderInlineChildren(i int) {
	r.renderChildren(i)
}

func (r *semanticHTMLPrototype) withTight(tight bool, fn func()) {
	previous := r.tight
	r.tight = tight
	fn()
	r.tight = previous
}

func (r *semanticHTMLPrototype) renderListItemChildren(i int) {
	if !r.tight {
		r.renderChildren(i)
		return
	}
	r.children(i, func(child int) {
		if r.kind(child) == djot.Paragraph {
			r.renderInlineChildren(child)
			r.out.WriteByte('\n')
		} else {
			r.renderNode(child)
		}
	})
}

func (r *semanticHTMLPrototype) renderNode(i int) {
	record := r.tape.records[i]
	payload := r.payload(i)
	switch r.kind(i) {
	case djot.Document:
		r.renderChildren(i)
	case djot.Section:
		r.out.WriteString("<section")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderChildren(i)
		r.out.WriteString("</section>\n")
	case djot.Paragraph:
		r.out.WriteString("<p")
		r.renderAttrs(i)
		r.out.WriteByte('>')
		r.renderInlineChildren(i)
		r.out.WriteString("</p>\n")
	case djot.Heading:
		level := int(record.small)
		if level < 1 {
			level = 1
		} else if level > 6 {
			level = 6
		}
		tag := "h" + strconv.Itoa(level)
		r.out.WriteByte('<')
		r.out.WriteString(tag)
		r.renderAttrs(i)
		r.out.WriteByte('>')
		r.renderInlineChildren(i)
		r.out.WriteString("</" + tag + ">\n")
	case djot.ThematicBreak:
		r.out.WriteString("<hr")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
	case djot.CodeBlock:
		r.out.WriteString("<pre")
		r.renderAttrs(i)
		r.out.WriteString("><code")
		if payload.extra != "" {
			r.out.WriteString(` class="language-` + escapeSemanticAttrPrototype(payload.extra) + `"`)
		}
		r.out.WriteByte('>')
		r.out.WriteString(escapeSemanticHTMLPrototype(payload.text))
		r.out.WriteString("</code></pre>\n")
	case djot.RawBlock:
		if payload.extra == "html" {
			r.out.WriteString(payload.text)
		}
	case djot.BlockQuote:
		r.out.WriteString("<blockquote")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderChildren(i)
		r.out.WriteString("</blockquote>\n")
	case djot.Div:
		r.out.WriteString("<div")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderChildren(i)
		r.out.WriteString("</div>\n")
	case djot.BulletList, djot.TaskList, djot.DefinitionList:
		tag := "ul"
		if r.kind(i) == djot.DefinitionList {
			tag = "dl"
		}
		r.out.WriteByte('<')
		r.out.WriteString(tag)
		if r.kind(i) == djot.TaskList {
			r.out.WriteString(` class="task-list"`)
		}
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.withTight(record.flags&semanticFlagTight != 0, func() { r.renderChildren(i) })
		r.out.WriteString("</" + tag + ">\n")
	case djot.OrderedList:
		r.out.WriteString("<ol")
		if payload.number != 1 {
			r.out.WriteString(` start="` + strconv.Itoa(int(payload.number)) + `"`)
		}
		switch djot.ListStyle(record.small) {
		case djot.ListAlphaLower:
			r.out.WriteString(` type="a"`)
		case djot.ListAlphaUpper:
			r.out.WriteString(` type="A"`)
		case djot.ListRomanLower:
			r.out.WriteString(` type="i"`)
		case djot.ListRomanUpper:
			r.out.WriteString(` type="I"`)
		}
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.withTight(record.flags&semanticFlagTight != 0, func() { r.renderChildren(i) })
		r.out.WriteString("</ol>\n")
	case djot.Table:
		r.out.WriteString("<table")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderChildren(i)
		r.out.WriteString("</table>\n")
	case djot.Caption:
		r.out.WriteString("<caption>")
		r.renderInlineChildren(i)
		r.out.WriteString("</caption>\n")
	case djot.TableRow:
		r.out.WriteString("<tr")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderChildren(i)
		r.out.WriteString("</tr>\n")
	case djot.TableCell:
		tag := "td"
		if record.flags&semanticFlagHeader != 0 {
			tag = "th"
		}
		r.out.WriteByte('<')
		r.out.WriteString(tag)
		if alignment := djot.CellAlign(record.small); alignment != djot.AlignDefault {
			name := "left"
			if alignment == djot.AlignRight {
				name = "right"
			} else if alignment == djot.AlignCenter {
				name = "center"
			}
			r.out.WriteString(` style="text-align: ` + name + `;"`)
		}
		r.renderAttrs(i)
		r.out.WriteByte('>')
		r.renderInlineChildren(i)
		r.out.WriteString("</" + tag + ">\n")
	case djot.Term:
		r.out.WriteString("<dt>")
		r.renderInlineChildren(i)
		r.out.WriteString("</dt>\n")
	case djot.Definition:
		r.out.WriteString("<dd>\n")
		r.renderListItemChildren(i)
		r.out.WriteString("</dd>\n")
	case djot.ListItem:
		r.out.WriteString("<li")
		r.renderAttrs(i)
		r.out.WriteString(">\n")
		r.renderListItemChildren(i)
		r.out.WriteString("</li>\n")
	case djot.TaskListItem:
		r.out.WriteString("<li>\n")
		if record.flags&semanticFlagChecked != 0 {
			r.out.WriteString(`<input disabled="" type="checkbox" checked=""/>`)
		} else {
			r.out.WriteString(`<input disabled="" type="checkbox"/>`)
		}
		r.out.WriteByte('\n')
		r.renderListItemChildren(i)
		r.out.WriteString("</li>\n")
	case djot.Text:
		r.out.WriteString(escapeSemanticHTMLPrototype(payload.text))
	case djot.SoftBreak:
		r.out.WriteByte('\n')
	case djot.HardBreak:
		r.out.WriteString("<br>\n")
	case djot.NonBreakingSpace:
		r.out.WriteString("&nbsp;")
	case djot.Emphasis, djot.Strong, djot.Superscript, djot.Subscript,
		djot.Insert, djot.Delete, djot.Mark, djot.Span:
		tags := map[djot.NodeKind]string{
			djot.Emphasis: "em", djot.Strong: "strong", djot.Superscript: "sup",
			djot.Subscript: "sub", djot.Insert: "ins", djot.Delete: "del",
			djot.Mark: "mark", djot.Span: "span",
		}
		tag := tags[r.kind(i)]
		r.out.WriteByte('<')
		r.out.WriteString(tag)
		r.renderAttrs(i)
		r.out.WriteByte('>')
		r.renderInlineChildren(i)
		r.out.WriteString("</" + tag + ">")
	case djot.Link:
		r.out.WriteString("<a")
		if payload.target != "" || record.flags&semanticFlagHasTarget != 0 {
			r.out.WriteString(` href="` + escapeSemanticAttrPrototype(payload.target) + `"`)
		}
		r.renderAttrs(i)
		r.out.WriteByte('>')
		r.renderInlineChildren(i)
		r.out.WriteString("</a>")
	case djot.Image:
		r.out.WriteString("<img")
		if alt := r.collectText(i); alt != "" {
			r.out.WriteString(` alt="` + escapeSemanticAttrPrototype(alt) + `"`)
		}
		if payload.target != "" || record.flags&semanticFlagHasTarget != 0 {
			r.out.WriteString(` src="` + escapeSemanticAttrPrototype(payload.target) + `"`)
		}
		r.renderAttrs(i)
		r.out.WriteByte('>')
	case djot.Verbatim:
		r.out.WriteString("<code>" + escapeSemanticHTMLPrototype(payload.text) + "</code>")
	case djot.InlineMath:
		r.out.WriteString(`<span class="math inline">\(` + escapeSemanticHTMLPrototype(payload.text) + `\)</span>`)
	case djot.DisplayMath:
		r.out.WriteString(`<span class="math display">\[` + escapeSemanticHTMLPrototype(payload.text) + `\]</span>`)
	case djot.RawInline:
		if payload.extra == "html" {
			r.out.WriteString(payload.text)
		}
	case djot.Symbol:
		r.out.WriteByte(':')
		r.out.WriteString(escapeSemanticHTMLPrototype(payload.text))
		r.out.WriteByte(':')
	case djot.Footnote:
		return
	case djot.FootnoteReference:
		label := payload.extra
		num := r.footnoteNums[label]
		r.fnrefSeen[label]++
		id := ""
		if r.fnrefSeen[label] == 1 {
			id = ` id="fnref` + strconv.Itoa(num) + `"`
		}
		r.out.WriteString(`<a` + id + ` href="#fn` + strconv.Itoa(num) + `" role="doc-noteref"><sup>` + strconv.Itoa(num) + `</sup></a>`)
	case djot.DoubleQuoted:
		r.out.WriteString("“")
		r.renderInlineChildren(i)
		r.out.WriteString("”")
	case djot.SingleQuoted:
		r.out.WriteString("‘")
		r.renderInlineChildren(i)
		r.out.WriteString("’")
	case djot.Ellipsis:
		r.out.WriteString("…")
	case djot.EmDash:
		r.out.WriteString("—")
	case djot.EnDash:
		r.out.WriteString("–")
	}
}

func (r *semanticHTMLPrototype) collectText(i int) string {
	switch r.kind(i) {
	case djot.Text:
		return r.payload(i).text
	case djot.SoftBreak, djot.HardBreak, djot.NonBreakingSpace:
		return " "
	}
	var out strings.Builder
	r.children(i, func(child int) { out.WriteString(r.collectText(child)) })
	if out.Len() == 0 {
		return r.payload(i).text
	}
	return out.String()
}

func (r *semanticHTMLPrototype) renderFootnotes() {
	if len(r.footnoteOrder) == 0 {
		return
	}
	r.out.WriteString("<section role=\"doc-endnotes\">\n<hr>\n<ol>\n")
	for _, fi := range r.footnoteOrder {
		r.out.WriteString(`<li id="fn` + strconv.Itoa(fi.num) + `">` + "\n")
		if fi.node >= 0 && int(r.tape.records[fi.node].subtreeEnd) > fi.node+1 {
			lastParagraph := -1
			r.children(fi.node, func(child int) {
				if r.kind(child) == djot.Paragraph {
					lastParagraph = child
				}
			})
			r.children(fi.node, func(child int) {
				if child == lastParagraph {
					r.out.WriteString("<p")
					r.renderAttrs(child)
					r.out.WriteByte('>')
					r.renderInlineChildren(child)
					r.out.WriteString(r.backref(fi))
					r.out.WriteString("</p>\n")
				} else {
					r.renderNode(child)
				}
			})
			if lastParagraph < 0 {
				r.out.WriteString("<p>" + r.backref(fi) + "</p>\n")
			}
		} else {
			r.out.WriteString("<p>" + r.backref(fi) + "</p>\n")
		}
		r.out.WriteString("</li>\n")
	}
	r.out.WriteString("</ol>\n</section>\n")
}

func (r *semanticHTMLPrototype) backref(fi semanticFootnotePrototype) string {
	return `<a href="#fnref` + strconv.Itoa(fi.num) + `" role="doc-backlink">↩︎</a>`
}

func escapeSemanticHTMLPrototype(s string) string {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>':
			var out strings.Builder
			out.Grow(len(s) + 10)
			out.WriteString(s[:i])
			for ; i < len(s); i++ {
				switch s[i] {
				case '&':
					out.WriteString("&amp;")
				case '<':
					out.WriteString("&lt;")
				case '>':
					out.WriteString("&gt;")
				default:
					out.WriteByte(s[i])
				}
			}
			return out.String()
		}
	}
	return s
}

func escapeSemanticAttrPrototype(s string) string {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"':
			var out strings.Builder
			out.Grow(len(s) + 10)
			out.WriteString(s[:i])
			for ; i < len(s); i++ {
				switch s[i] {
				case '&':
					out.WriteString("&amp;")
				case '<':
					out.WriteString("&lt;")
				case '>':
					out.WriteString("&gt;")
				case '"':
					out.WriteString("&quot;")
				default:
					out.WriteByte(s[i])
				}
			}
			return out.String()
		}
	}
	return s
}

func TestSemanticTapePrototypeHTML(t *testing.T) {
	for _, input := range []string{
		smallDoc(), mediumDoc(), hugeDoc(),
		"A reference[^a] and another[^a].\n\n[^a]: *note*.\n",
	} {
		doc := djot.Parse(input)
		want := djot.RenderHTML(doc)
		got := renderSemanticHTMLPrototype(buildHintedSemanticTapePrototype(doc))
		if got != want {
			t.Fatalf("semantic renderer mismatch\nwant: %q\n got: %q", want, got)
		}
	}
}

func TestSemanticTapePrototypeOfficialHTML(t *testing.T) {
	for file, cases := range loadOfficialTests(t) {
		for _, tc := range cases {
			if tc.IsAST {
				continue
			}
			t.Run(file+"/"+tc.Name, func(t *testing.T) {
				doc := djot.Parse(tc.Input)
				want := djot.RenderHTML(doc)
				got := renderSemanticHTMLPrototype(buildHintedSemanticTapePrototype(doc))
				if got != want {
					t.Fatalf("semantic renderer mismatch\nwant: %q\n got: %q", want, got)
				}
			})
		}
	}
}

func BenchmarkSemanticHTMLPrototype(b *testing.B) {
	doc := djot.Parse(hugeDoc())
	tape := buildHintedSemanticTapePrototype(doc)
	kindPayloads := kindPayloadsFromTapePrototype(tape)
	kindSourcePayloads := kindSourcePayloadsFromASTPrototype(doc, tape)
	for _, strategy := range []struct {
		name           string
		payloads       *kindPayloadsPrototype
		sourcePayloads *kindSourcePayloadsPrototype
	}{
		{"Fat", nil, nil},
		{"KindSpecific", &kindPayloads, nil},
		{"KindSource", nil, &kindSourcePayloads},
	} {
		b.Run(strategy.name, func(b *testing.B) {
			b.SetBytes(int64(len(doc.Files[0].Source)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if strategy.sourcePayloads != nil {
					_ = renderSemanticHTMLWithSourcePayloadsPrototype(tape, strategy.sourcePayloads)
				} else {
					_ = renderSemanticHTMLWithPayloadsPrototype(tape, strategy.payloads)
				}
			}
		})
	}
}
