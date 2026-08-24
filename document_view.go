package djot

import (
	"sort"
	"strings"
)

// HeadingView is a read-only heading entry produced by [DocumentView.Headings].
type HeadingView struct {
	element ElementView
	level   int
	text    string
	id      string
}

// FootnoteView is compact, read-only metadata for one logical footnote.
// Inspecting a footnote's block contents requires the Node API.
type FootnoteView struct {
	element        ElementView
	label          string
	number         int
	referenceCount int
	defined        bool
}

// ReferenceView is compact, read-only metadata for one resolved reference
// definition. Reference definitions are document metadata rather than Nodes.
type ReferenceView struct {
	label          string
	destination    string
	destinationSet bool
	attributes     AttributeView
}

// AnchorView is compact, read-only metadata for an element with a non-empty id
// attribute. Renderer-generated footnote IDs are not document anchors.
type AnchorView struct {
	element ElementView
	id      string
}

// ID returns the unescaped anchor id.
func (a AnchorView) ID() string { return a.id }

// Kind returns the kind of element carrying the id.
func (a AnchorView) Kind() Kind { return a.element.Kind() }

// Span returns the element's half-open source range.
func (a AnchorView) Span() SourceSpan { return a.element.Span() }

// Attributes returns the element's read-only ordered attributes.
func (a AnchorView) Attributes() AttributeView { return a.element.Attributes() }

// Label returns the normalized reference label.
func (r ReferenceView) Label() string { return r.label }

// Destination returns the resolved reference destination.
func (r ReferenceView) Destination() string { return r.destination }

// DestinationSet reports whether the reference explicitly has a destination.
func (r ReferenceView) DestinationSet() bool { return r.destinationSet }

// Attributes returns the reference definition's read-only ordered attributes.
func (r ReferenceView) Attributes() AttributeView { return r.attributes }

// Label returns the normalized footnote label.
func (f FootnoteView) Label() string { return f.label }

// Number returns the footnote's one-based render number, or zero when the
// footnote is never referenced.
func (f FootnoteView) Number() int { return f.number }

// ReferenceCount returns the number of references to the footnote, including
// references encountered inside other footnote definitions.
func (f FootnoteView) ReferenceCount() int { return f.referenceCount }

// Defined reports whether the document contains a definition for the label.
func (f FootnoteView) Defined() bool { return f.defined }

// Span returns the definition's half-open source range. It is zero when the
// footnote is referenced but undefined.
func (f FootnoteView) Span() SourceSpan { return f.element.Span() }

// Attributes returns the definition's read-only ordered attributes. It is
// empty when the footnote is referenced but undefined.
func (f FootnoteView) Attributes() AttributeView { return f.element.Attributes() }

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
	root            ElementView
	headings        []HeadingView
	headingsReady   bool
	kindCounts      *[int(KindEnDash) + 1]int
	footnotes       []FootnoteView
	footnotesReady  bool
	references      []ReferenceView
	referencesReady bool
	anchors         []AnchorView
	anchorsReady    bool
	semantic        *semanticHTMLRenderer
	tree            *htmlRenderer
	doc             *Doc
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

// Contains reports whether the document contains an element of kind. It shares
// a lazily built kind index with [DocumentView.Count].
func (d DocumentView) Contains(kind Kind) bool {
	return d.Count(kind) != 0
}

// Count returns the number of elements of kind in the document. The document
// root is included, so Count(KindDocument) returns one for a valid document.
// The kind index is built lazily and reused for the duration of the callback.
func (d DocumentView) Count(kind Kind) int {
	if d.state == nil || kind < 0 || kind > KindEnDash {
		return 0
	}
	if d.state.kindCounts == nil {
		d.state.kindCounts = buildDocumentKindCounts(d.state.root)
	}
	return d.state.kindCounts[kind]
}

// Footnotes returns logical footnotes in render order, followed by unreferenced
// definitions in source order. Referenced but undefined labels are included.
// The index is built lazily and reused for the duration of the callback.
func (d DocumentView) Footnotes() []FootnoteView {
	if d.state == nil {
		return nil
	}
	if !d.state.footnotesReady {
		d.state.footnotes = buildDocumentFootnotes(d.state)
		d.state.footnotesReady = true
	}
	return d.state.footnotes
}

// References returns resolved reference definitions in normalized-label order,
// including implicit heading references. The index is built lazily and reused
// for the duration of the callback.
func (d DocumentView) References() []ReferenceView {
	if d.state == nil {
		return nil
	}
	if !d.state.referencesReady {
		d.state.references = buildDocumentReferences(d.state)
		d.state.referencesReady = true
	}
	return d.state.references
}

// Anchors returns elements with non-empty id attributes in document order.
// Duplicate ids are preserved so callers can detect them. Renderer-generated
// footnote ids are excluded. The index is built lazily and reused for the
// duration of the callback.
func (d DocumentView) Anchors() []AnchorView {
	if d.state == nil {
		return nil
	}
	if !d.state.anchorsReady {
		d.state.anchors = buildDocumentAnchors(d.state.root)
		d.state.anchorsReady = true
	}
	return d.state.anchors
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

func buildDocumentKindCounts(root ElementView) *[int(KindEnDash) + 1]int {
	counts := new([int(KindEnDash) + 1]int)
	if root.tape != nil {
		end := int(root.tape.records[root.record].subtreeEnd)
		for i := root.record; i < end; i++ {
			kind := Kind(root.tape.records[i].kind)
			if kind >= 0 && kind <= KindEnDash {
				counts[kind]++
			}
		}
	} else if root.node != nil {
		Preorder(root.node, func(node Node) bool {
			kind := node.Kind()
			if kind >= 0 && kind <= KindEnDash {
				counts[kind]++
			}
			return true
		})
	}
	return counts
}

func buildDocumentFootnotes(state *documentViewState) []FootnoteView {
	if state.semantic != nil {
		renderer := state.semantic
		footnotes := make([]FootnoteView, 0, len(renderer.footnoteOrder)+len(renderer.footnotes))
		seen := make(map[string]struct{}, len(renderer.footnotes))
		for _, item := range renderer.footnoteOrder {
			view := FootnoteView{
				label: item.label, number: item.num,
				referenceCount: renderer.fnrefTotal[item.label], defined: item.node >= 0,
			}
			if item.node >= 0 {
				view.element = ElementView{tape: renderer.tape, record: item.node}
			}
			footnotes = append(footnotes, view)
			seen[item.label] = struct{}{}
		}
		end := int(renderer.tape.records[0].subtreeEnd)
		for i := 1; i < end; i++ {
			if Kind(renderer.tape.records[i].kind) != KindFootnote {
				continue
			}
			label := renderer.label(i)
			if _, ok := seen[label]; ok || renderer.footnotes[label] != i {
				continue
			}
			footnotes = append(footnotes, FootnoteView{
				element: ElementView{tape: renderer.tape, record: i}, label: label, defined: true,
			})
			seen[label] = struct{}{}
		}
		return footnotes
	}
	if state.tree != nil {
		renderer := state.tree
		footnotes := make([]FootnoteView, 0, len(renderer.footnoteOrder)+len(renderer.footnotes))
		seen := make(map[string]struct{}, len(renderer.footnotes))
		for _, item := range renderer.footnoteOrder {
			view := FootnoteView{
				label: item.label, number: item.num,
				referenceCount: renderer.fnrefTotal[item.label], defined: item.node != nil,
			}
			if item.node != nil {
				view.element = ElementView{node: item.node}
			}
			footnotes = append(footnotes, view)
			seen[item.label] = struct{}{}
		}
		Preorder(state.root.node, func(node Node) bool {
			definition, ok := node.(*Footnote)
			if !ok {
				return true
			}
			if _, ok := seen[definition.Label]; ok || renderer.footnotes[definition.Label] != definition {
				return true
			}
			footnotes = append(footnotes, FootnoteView{
				element: ElementView{node: definition}, label: definition.Label, defined: true,
			})
			seen[definition.Label] = struct{}{}
			return true
		})
		return footnotes
	}
	return nil
}

func buildDocumentReferences(state *documentViewState) []ReferenceView {
	if state.doc == nil {
		return nil
	}
	state.doc.mu.RLock()
	defer state.doc.mu.RUnlock()

	if state.doc.references != nil {
		labels := make([]string, 0, len(state.doc.references))
		for label := range state.doc.references {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		references := make([]ReferenceView, 0, len(labels))
		for _, label := range labels {
			reference := state.doc.references[label]
			if reference == nil {
				continue
			}
			references = append(references, ReferenceView{
				label: label, destination: reference.Destination,
				destinationSet: reference.DestinationSet,
				attributes:     AttributeView{attributes: &reference.Attributes},
			})
		}
		return references
	}

	tape := state.doc.semantic
	if tape == nil || len(tape.references) == 0 {
		return nil
	}
	references := make([]ReferenceView, 0, len(tape.references))
	for _, named := range tape.references {
		reference := named.semanticReference
		references = append(references, ReferenceView{
			label: named.name, destination: reference.target,
			destinationSet: reference.hasTarget,
			attributes:     AttributeView{referenceAttributes: reference.attrs},
		})
	}
	return references
}

func buildDocumentAnchors(root ElementView) []AnchorView {
	var anchors []AnchorView
	if root.tape != nil {
		end := int(root.tape.records[root.record].subtreeEnd)
		for i := root.record; i < end; i++ {
			element := ElementView{tape: root.tape, record: i}
			if id, ok := element.Attributes().Lookup("id"); ok && id != "" {
				anchors = append(anchors, AnchorView{element: element, id: id})
			}
		}
	} else if root.node != nil {
		Preorder(root.node, func(node Node) bool {
			if id, ok := node.Attributes().Lookup("id"); ok && id != "" {
				anchors = append(anchors, AnchorView{element: ElementView{node: node}, id: id})
			}
			return true
		})
	}
	return anchors
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
