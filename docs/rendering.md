# Rendering and extensions

The default path is intentionally small:

```go
doc := djot.Parse(source)
html := djot.RenderHTML(doc)
```

Use `RenderHTMLTo` for an `io.Writer` and write-error propagation:

```go
if err := djot.RenderHTMLTo(w, doc); err != nil {
    return err
}
```

## Choose the narrowest hook

The hook APIs differ mainly in how much context they expose. Narrow hooks can
read the compact parsed representation without constructing the mutable AST.

| Need | API | View | Untouched document stays compact? |
| --- | --- | --- | --- |
| Customize a symbol | `WithSymbolRenderer` | Symbol name | Yes |
| Customize a div from its own metadata | `WithDivRenderer` | Div attributes and span | Yes |
| Inspect one bounded subtree | `WithSubtreeRenderer` | Read-only element traversal | Yes |
| Inspect document-wide indexes | `WithDocumentRenderer` | Headings, kinds, footnotes, references, anchors | Yes |
| Customize footnote IDs or backlinks | Footnote options | Numbering callbacks | Yes |
| Inspect arbitrary concrete nodes | `WithNodeRenderer`, `WithRenderer`, `WithRenderFunc` | Mutable `ast.Node` | No; uses the typed tree |

Use the narrowest hook that provides the required context. This keeps intent
clear and avoids materializing a tree solely for presentation logic.

## Renderer controls

Element, subtree, and node callbacks receive a renderer with three controls:

- `Default()` emits the built-in rendering for the current element.
- `Children()` emits existing children without the current wrapper.
- `Write(string)` writes raw HTML.

Returning without calling any of them suppresses the element and its children.
Values passed to `Write` are not escaped automatically.

## Symbols

Symbols such as `:rocket:` render literally by default and make convenient
shortcode extension points:

```go
html := djot.RenderHTML(doc,
    djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
        if symbol.Name == "rocket" {
            r.Write("🚀")
            return
        }
        r.Default()
    }),
)
```

The same callback runs for an untouched compact document and a materialized or
mutated tree.

## Divs

Use `WithDivRenderer` when the decision depends only on the div itself:

```go
html := djot.RenderHTML(doc,
    djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
        if div.Attributes().Get("class") != "warning" {
            r.Default()
            return
        }

        r.Write(`<aside class="warning">`)
        r.Children()
        r.Write(`</aside>`)
    }),
)
```

`DivView` exposes ordered attributes and the source span. It deliberately does
not inspect descendants before rendering.

## Bounded subtree inspection

When a wrapper depends on descendant content, use `WithSubtreeRenderer`:

```go
html := djot.RenderHTML(doc,
    djot.WithSubtreeRenderer(ast.KindDiv,
        func(subtree djot.SubtreeView, r djot.ElementRenderer) {
            if subtree.Contains(ast.KindCodeBlock) {
                r.Write(`<section class="contains-code">`)
                r.Children()
                r.Write(`</section>`)
                return
            }
            r.Default()
        },
    ),
)
```

`Contains(kind)` is convenient for feature checks. `Preorder` includes the
subtree root; `Descendants` excludes it:

```go
subtree.Preorder(func(element djot.ElementView) bool {
    fmt.Println(element.Kind(), element.Span())
    return true
})
```

Each inspection scans that subtree's contiguous range. Avoid repeatedly
scanning many heavily overlapping subtrees; use the AST for unrestricted
whole-document analysis.

## Document-wide indexes

`WithDocumentRenderer` runs before normal output and supplies lazily built,
reusable indexes. This table of contents wraps the default document rendering:

```go
html := djot.RenderHTML(doc,
    djot.WithDocumentRenderer(
        func(document djot.DocumentView, r djot.DocumentRenderer) {
            r.Write(`<nav><ol>`)
            for _, heading := range document.Headings() {
                r.Write(`<li><a href="#` + html.EscapeString(heading.ID()) + `">`)
                r.Write(html.EscapeString(heading.Plaintext()))
                r.Write(`</a></li>`)
            }
            r.Write(`</ol></nav>`)
            r.Default()
        },
    ),
)
```

`r.Default()` emits the complete ordinary document, including endnotes and
other registered hooks. Output before and after it can wrap that document.

Available document indexes are:

- `Headings()` — level, plain text, id, attributes, and span
- `Contains(kind)` and `Count(kind)` — a shared lazy kind index
- `Footnotes()` — number, reference count, definition state, attributes, span
- `References()` — normalized labels, destinations, and ordered attributes
- `Anchors()` — elements with non-empty `id` attributes in document order

Duplicates remain in `Anchors()` so callers can validate them. Renderer-created
footnote IDs are not document anchors.

## Full node hooks

Use a node hook when rendering depends on concrete mutable node data:

```go
html := djot.RenderHTML(doc,
    djot.WithNodeRenderer(ast.KindCodeBlock,
        func(node ast.Node, r djot.NodeRenderer) {
            code := node.(*ast.CodeBlock)
            r.Write(`<pre class="highlight"><code>`)
            r.Write(html.EscapeString(code.Text))
            r.Write(`</code></pre>`)
        },
    ),
)
```

`WithRenderer` infers the kind from a concrete pointer type:

```go
option := djot.WithRenderer(
    func(symbol *ast.Symbol, r djot.NodeRenderer) {
        r.Write(html.EscapeString(symbol.Name))
    },
)
```

`WithRenderFunc` is concise for leaf nodes whose callback either returns a
replacement string or falls through to the default:

```go
option := djot.WithRenderFunc(ast.KindSymbol, func(node ast.Node) string {
    if node.(*ast.Symbol).Name == "star" {
        return "⭐"
    }
    return ""
})
```

Registering any node hook selects the typed-tree renderer for that render, even
when the requested kind does not occur.

## Composition and precedence

Options are applied in order. For the same kind, the last specialized element,
subtree, or node hook wins. Document rendering composes with element and subtree
hooks: calling `DocumentRenderer.Default` replays the normal document using the
other active options.

Callbacks may panic, and those panics are not recovered. Writer errors stop
further rendering and are returned by `RenderHTMLTo`.

## Footnote backlinks

By default, repeated references follow djot.js behavior: only the first
reference carries an `id`, and the footnote emits one `↩︎` backlink.

Enable one backlink per reference with unique IDs:

```go
html := djot.RenderHTML(doc, djot.WithMultiBacklinks())
```

Namespace all generated footnote IDs when embedding output in a larger page:

```go
html := djot.RenderHTML(doc,
    djot.WithFootnotePrefix("post42-"),
    djot.WithFootnoteBacklinkLabel(func(number, occurrence, total int) string {
        return strconv.Itoa(occurrence)
    }),
)
```

For individual control, use `WithFootnoteID`, `WithFootnoteRefID`, and
`WithFootnoteBacklinkLabel`. Callback results are escaped for their HTML context.

Footnote numbering describes the logical document and is computed before render
hooks run. Suppressing a rendered subtree does not remove its logical footnotes.
Use the mutable AST when an extension must alter footnote structure rather than
presentation.

## Security

Every `Write` method writes raw HTML. Escape derived text with
`html.EscapeString`, validate URLs as appropriate for the application, and
sanitize the complete result when the source is untrusted.

---

[README](../README.md) · [Getting started](getting-started.md) ·
[AST](ast.md) · [Compatibility](compatibility.md)
