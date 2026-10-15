// Package djot parses and renders [Djot] markup, a light markup language
// designed by John MacFarlane as a successor to Markdown.
//
//	doc := djot.Parse(input)
//	html := djot.RenderHTML(doc)
//
// A [Doc] never changes after parsing, so it can be rendered any number of
// times, from any goroutine. [Doc.AST] returns a new copy of its syntax tree
// on each call; [NewDoc] turns an edited tree back into a Doc.
//
// [Parse] uses the default configuration. [NewParser] returns a [Parser]
// configured by [ParseOption] values; a Parser cannot be changed after it is
// created and is safe for concurrent use.
//
// # Compatibility
//
// The parser follows djot.js, the reference implementation, and passes the
// djot.js test suite, excluding Lua filters. Where the prose [Djot syntax
// reference] and djot.js differ, this package follows djot.js unless
// documented otherwise.
//
// # Traversing the AST
//
// Package ast provides the tree and traversal API. ast.Walk visits
// nodes top-down and supports continue, skip, remove, and replace actions;
// ast.WalkBottomUp visits children before parents.
//
//	tree := doc.AST()
//	ast.Walk(tree, func(n ast.Node) ast.Action {
//	    if strong, ok := n.(*ast.Strong); ok {
//	        replacement := &ast.Emphasis{Children: strong.Children}
//	        ast.CopyMetadata(replacement, strong)
//	        return ast.Replace(replacement)
//	    }
//	    return ast.Continue
//	})
//	html := djot.RenderHTML(djot.NewDoc(tree))
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
// # Security
//
// This package does not sanitize HTML output, and Djot's raw HTML syntax passes
// through unchanged. When processing untrusted input, pass the output through
// an HTML sanitizer such as [bluemonday].
//
// [Djot]: https://djot.net
// [bluemonday]: https://github.com/microcosm-cc/bluemonday
// [Djot syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
package djot
