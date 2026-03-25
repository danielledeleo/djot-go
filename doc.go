// Package djot parses and renders [djot] markup, a light markup language
// designed by John MacFarlane as a successor to Markdown.
//
// The parser is spec-compliant with the [djot syntax reference] and passes the
// full official test suite. It produces a typed AST that can be inspected,
// transformed, and rendered to HTML.
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
//	djot.Walk(doc.Root, func(n *djot.Node) any {
//	    if n.Kind == djot.Strong {
//	        return djot.Replace(&djot.Node{Kind: djot.Emphasis, Children: n.Children})
//	    }
//	    return djot.Continue
//	})
//
// # Custom rendering
//
// Override rendering for specific node kinds with [WithNodeRenderer]:
//
//	html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.Image, func(n *djot.Node, r djot.NodeRenderer) {
//	    r.Write("<figure>")
//	    r.Default()
//	    r.Write("</figure>")
//	}))
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
