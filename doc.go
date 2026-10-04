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
// A [Doc] is immutable and renders from a compact representation. [Doc.AST]
// returns a syntax tree the caller owns; [NewDoc] turns an edited tree back
// into a Doc.
//
// # Quick start
//
//	doc := djot.Parse(input)
//	html := djot.RenderHTML(doc)
//
// [Parse] uses the default configuration. [NewParser] returns a [Parser]
// configured by [ParseOption] values; a Parser is immutable and safe for
// concurrent use.
//
// # Traversing the AST
//
// Package ast provides the mutable tree and traversal API. ast.Walk visits
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
// This package does not sanitize HTML output. When processing untrusted input,
// pass the output through an HTML sanitizer such as [bluemonday].
//
// [djot]: https://djot.net
// [bluemonday]: https://github.com/microcosm-cc/bluemonday
//
// [djot syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
package djot
