// Package djot parses and renders [djot] markup, a light markup language
// designed by John MacFarlane as a successor to Markdown.
//
// The parser is spec-compliant with the [djot syntax reference] and passes the
// full official test suite. Parsed documents retain a compact semantic
// representation for rendering and materialize a mutable AST only when
// [Doc.Root] is requested.
//
// # Quick start
//
//	doc := djot.Parse(input)
//	html := djot.RenderHTML(doc)
//
// # Traversing the AST
//
// [Walk] visits nodes top-down and supports [Continue], [SkipChildren], [Remove],
// and [Replace] actions. [WalkBottomUp] visits children before parents.
//
//	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
//	    if strong, ok := n.(*djot.Strong); ok {
//	        replacement := &djot.Emphasis{Children: strong.Children}
//	        djot.CopyMetadata(replacement, strong)
//	        return djot.Replace(replacement)
//	    }
//	    return djot.Continue
//	})
//
// # Custom rendering
//
// Override rendering for specific node kinds with [WithNodeRenderer]:
//
//	html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.KindImage, func(n djot.Node, r djot.NodeRenderer) {
//	    r.Write("<figure>")
//	    r.Default()
//	    r.Write("</figure>")
//	}))
//
// Compact render views cover common streaming and structural decisions without
// materializing the AST. They intentionally expose only the data required by
// each hook. Use [Node] values when an extension needs arbitrary typed
// inspection or mutation; doing so may materialize the AST.
//
// Lightweight symbol and Div customizations can use [WithSymbolRenderer] and
// [WithDivRenderer]. [WithSubtreeRenderer] adds bounded, read-only structural
// inspection when a rendering decision depends on an element's descendants.
// [WithDocumentRenderer] provides focused indexes and summaries such as
// [DocumentView.Headings], [DocumentView.Footnotes],
// [DocumentView.References], [DocumentView.Anchors], [DocumentView.Contains],
// and [DocumentView.Count].
//
// # Security
//
// This package does not sanitize HTML output. When processing untrusted input,
// pass the output through an HTML sanitizer such as [bluemonday].
//
// [djot]: https://djot.net
// [djot syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
// [bluemonday]: https://github.com/microcosm-cc/bluemonday
package djot
