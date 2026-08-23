package djot

import "strings"

// HeadingView is a read-only heading entry produced by [DocumentView.Headings].
type HeadingView struct {
	element ElementView
	level   int
	text    string
	id      string
}

// Level returns the heading level.
func (h HeadingView) Level() int { return h.level }

// Text returns the heading's unescaped plain-text content with inline markup
// removed and rendered punctuation and symbols preserved. Escape it before
// including it in HTML written with [DocumentRenderer.Write].
func (h HeadingView) Text() string { return h.text }

// ID returns the unescaped anchor id targeted by links to this heading. Escape
// it before including it in HTML written with [DocumentRenderer.Write].
func (h HeadingView) ID() string { return h.id }

// Span returns the heading's half-open source range.
func (h HeadingView) Span() SourceSpan { return h.element.Span() }

// Attributes returns the heading's read-only ordered attributes. Use
// [HeadingView.ID] for its generated or explicit anchor id.
func (h HeadingView) Attributes() AttributeView { return h.element.Attributes() }

type documentViewState struct {
	root          ElementView
	headings      []HeadingView
	headingsReady bool
}

// DocumentView provides focused, read-only indexes over a complete parsed
// document. Queries use the compact semantic tape when possible and never
// materialize Nodes solely for inspection. The view and slices returned by it
// are valid only during the document render callback.
type DocumentView struct {
	state *documentViewState
}

// Headings returns document headings in source order. The index and its plain
// text are built lazily and reused for the duration of the callback. Building
// the index allocates in proportion to the number and text size of headings.
func (d DocumentView) Headings() []HeadingView {
	if d.state == nil {
		return nil
	}
	if !d.state.headingsReady {
		d.state.headings = buildHeadingViews(d.state.root)
		d.state.headingsReady = true
	}
	return d.state.headings
}

// DocumentRenderFunc controls rendering after inspecting the complete
// document. Call [DocumentRenderer.Default] to render the normal document,
// including its endnotes.
type DocumentRenderFunc func(document DocumentView, renderer DocumentRenderer)

// DocumentRenderer controls output from a document rendering hook. It is valid
// only during the callback. Its zero value emits no output.
type DocumentRenderer struct {
	semantic *semanticHTMLRenderer
	tree     *htmlRenderer
}

// Write emits raw HTML.
func (r DocumentRenderer) Write(s string) {
	if r.semantic != nil {
		r.semantic.write(s)
	} else if r.tree != nil {
		r.tree.write(s)
	}
}

// Default renders the normal document, including endnotes, through the
// remaining element and subtree hooks. It does not invoke the document hook
// again. Calling Default repeatedly replays the complete document.
func (r DocumentRenderer) Default() {
	if r.semantic != nil {
		r.semantic.renderDocumentDefault()
	} else if r.tree != nil {
		r.tree.renderDocumentDefault()
	}
}

func buildHeadingViews(root ElementView) []HeadingView {
	if root.tape != nil {
		var headings []HeadingView
		end := int(root.tape.records[root.record].subtreeEnd)
		for i := root.record + 1; i < end; i++ {
			if Kind(root.tape.records[i].kind) == KindHeading {
				element := ElementView{tape: root.tape, record: i}
				id := element.Attributes().Get("id")
				if i > root.record && Kind(root.tape.records[i-1].kind) == KindSection &&
					int(root.tape.records[i-1].subtreeEnd) > i {
					if sectionID := (ElementView{tape: root.tape, record: i - 1}).Attributes().Get("id"); sectionID != "" {
						id = sectionID
					}
				}
				headings = append(headings, HeadingView{
					element: element,
					level:   int(root.tape.records[i].small),
					text:    collectDocumentText(element),
					id:      id,
				})
			}
		}
		return headings
	}
	if root.node == nil {
		return nil
	}
	var headings []HeadingView
	var walk func(Node)
	walk = func(node Node) {
		if section, ok := node.(*Section); ok {
			for i, child := range section.Children {
				if i == 0 {
					if heading, ok := child.(*Heading); ok {
						element := ElementView{node: heading}
						id := heading.Attributes().Get("id")
						if sectionID := section.Attributes().Get("id"); sectionID != "" {
							id = sectionID
						}
						headings = append(headings, HeadingView{
							element: element,
							level:   heading.Level,
							text:    collectDocumentText(element),
							id:      id,
						})
						forEachChild(heading, walk)
						continue
					}
				}
				walk(child)
			}
			return
		}
		if heading, ok := node.(*Heading); ok {
			element := ElementView{node: heading}
			headings = append(headings, HeadingView{
				element: element,
				level:   heading.Level,
				text:    collectDocumentText(element),
				id:      heading.Attributes().Get("id"),
			})
		}
		forEachChild(node, walk)
	}
	walk(root.node)
	return headings
}

func collectDocumentText(element ElementView) string {
	prefix, suffix := "", ""
	switch element.Kind() {
	case KindText:
		if element.tape != nil {
			return element.tape.text(element.tape.records[element.record].payload)
		}
		return element.node.(*Text).Value
	case KindSoftBreak, KindHardBreak, KindNonBreakingSpace:
		return " "
	case KindVerbatim, KindInlineMath, KindDisplayMath:
		if element.tape != nil {
			return element.tape.text(element.tape.records[element.record].payload)
		}
		switch node := element.node.(type) {
		case *Verbatim:
			return node.Text
		case *InlineMath:
			return node.Text
		case *DisplayMath:
			return node.Text
		}
	case KindSymbol:
		symbol, _ := element.Symbol()
		return ":" + symbol.Name + ":"
	case KindEllipsis:
		return "…"
	case KindEmDash:
		return "—"
	case KindEnDash:
		return "–"
	case KindDoubleQuoted:
		prefix, suffix = "“", "”"
	case KindSingleQuoted:
		prefix, suffix = "‘", "’"
	}

	var text strings.Builder
	if element.tape != nil {
		tape := element.tape
		for child, end := element.record+1, int(tape.records[element.record].subtreeEnd); child < end; {
			text.WriteString(collectDocumentText(ElementView{tape: tape, record: child}))
			child = int(tape.records[child].subtreeEnd)
		}
	} else if element.node != nil {
		forEachChild(element.node, func(child Node) {
			text.WriteString(collectDocumentText(ElementView{node: child}))
		})
	}
	return prefix + text.String() + suffix
}
