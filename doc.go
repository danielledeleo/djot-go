// Package djot parses and renders [djot] markup, a light markup language
// designed by John MacFarlane as a successor to Markdown.
//
// The package is designed for Go applications embedding Djot in publishing
// systems, documentation services, wikis, and developer tools. Ordinary HTML
// rendering stays compact, while a typed mutable AST supports transformations
// and open-ended analysis.
//
// The parser targets compatibility with djot.js, the reference implementation,
// and passes the official syntax and rendering tests; the Lua-only filter tests
// do not apply to this Go implementation. Where the prose syntax reference and
// djot.js differ, this package follows djot.js unless documented otherwise.
// Parsed documents retain a compact semantic representation for rendering and
// materialize a mutable AST only when [Doc.Root] is requested.
//
// # Quick start
//
//	doc := djot.Parse(input)
//	html := djot.RenderHTML(doc)
//
// # Traversing the AST
//
// Package ast provides the mutable tree and traversal API. ast.Walk visits
// nodes top-down and supports continue, skip, remove, and replace actions;
// ast.WalkBottomUp visits children before parents.
//
//	ast.Walk(doc.Root(), func(n ast.Node) ast.Action {
//	    if strong, ok := n.(*ast.Strong); ok {
//	        replacement := &ast.Emphasis{Children: strong.Children}
//	        ast.CopyMetadata(replacement, strong)
//	        return ast.Replace(replacement)
//	    }
//	    return ast.Continue
//	})
//
// # Custom rendering
//
// Override rendering for specific node kinds with [WithNodeRenderer]:
//
//	html := djot.RenderHTML(doc, djot.WithNodeRenderer(ast.KindImage, func(n ast.Node, r djot.NodeRenderer) {
//	    r.Write("<figure>")
//	    r.Default()
//	    r.Write("</figure>")
//	}))
//
// Compact render views cover common streaming and structural decisions without
// materializing the AST. They intentionally expose only the data required by
// each hook. Use ast.Node values when an extension needs arbitrary typed
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
// [bluemonday]: https://github.com/microcosm-cc/bluemonday
//
// [djot syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
package djot
