# Rendering

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

Output is written in many small pieces, so wrap files and network connections
in a `bufio.Writer`.

## Custom HTML

`WithNodeRenderer` replaces the HTML for one kind of node. The callback
receives the node and a `NodeRenderer` with three controls:

- `Write(string)` writes raw HTML. It is not escaped.
- `Children()` renders the node's children without its own wrapper.
- `Default()` renders the node as usual.

Returning without calling any of them drops the node and its children.

```go
html := djot.RenderHTML(doc,
    djot.WithNodeRenderer(ast.KindSymbol, func(n ast.Node, r djot.NodeRenderer) {
        if n.(*ast.Symbol).Name == "memo" {
            r.Write("📝")
            return
        }
        r.Default()
    }),
    djot.WithNodeRenderer(ast.KindDiv, func(n ast.Node, r djot.NodeRenderer) {
        if n.Attributes().Get("class") != "warning" {
            r.Default()
            return
        }
        r.Write(`<aside class="warning">`)
        r.Children()
        r.Write(`</aside>`)
    }),
)
```

Hooks on small or rare nodes, like symbols, cost almost nothing. Hooks on nodes
that contain most of the document, like sections, are slower.

## Whole-document context

Hooks see one node at a time. For anything that needs the whole document, such
as a table of contents, read `doc.AST()` first, then render as usual:

```go
var toc strings.Builder
toc.WriteString("<nav><ul>\n")
ast.Preorder(doc.AST(), func(n ast.Node) bool {
    section, ok := n.(*ast.Section)
    if !ok {
        return true
    }
    heading := section.Children[0].(*ast.Heading)
    var text strings.Builder
    ast.Preorder(heading, func(n ast.Node) bool {
        if t, ok := n.(*ast.Text); ok {
            text.WriteString(t.Value)
        }
        return true
    })
    fmt.Fprintf(&toc, "<li><a href=\"#%s\">%s</a></li>\n",
        html.EscapeString(section.Attributes().Get("id")),
        html.EscapeString(text.String()))
    return true
})
toc.WriteString("</ul></nav>\n")

page := toc.String() + djot.RenderHTML(doc)
```

## Composition and precedence

Options are applied in order. If two hooks target the same kind, the last one
wins. Inside a hook, `Children()` and `Default()` render the node's contents
with every registered hook applied: a symbol inside a hooked div still goes
through the symbol hook, and a div nested inside it goes through the div hook
again.

Callbacks may panic, and those panics are not recovered. Writer errors stop
further rendering and are returned by `RenderHTMLTo`.

## Footnotes

By default, repeated references follow djot.js behavior: only the first
reference carries an `id`, and the footnote emits one `↩︎` backlink.

Enable one backlink per reference with unique IDs:

```go
html := djot.RenderHTML(doc, djot.WithMultiBacklinks())
```

Namespace all generated footnote IDs when embedding output in a larger page:

```go
html := djot.RenderHTML(doc, djot.WithFootnotePrefix("post42-"))
```

Change the backlink text with `WithFootnoteBacklinkLabel`. It receives the
footnote number, which backlink this is, and how many there are:

```go
html := djot.RenderHTML(doc,
    djot.WithFootnoteBacklinkLabel(func(number, occurrence, total int) string {
        return strconv.Itoa(occurrence)
    }),
)
```

The prefix and the backlink labels are escaped for their HTML context.

Footnotes are numbered in the order their references appear in the output.
Content dropped by a hook takes its footnotes with it.

## Security

`NodeRenderer.Write` writes raw HTML. Escape derived text with
`html.EscapeString`, validate URLs as appropriate for the application, and
sanitize the complete result when the source is untrusted.

---

[README](../README.md) · [Getting started](getting-started.md) ·
[AST](ast.md) · [Compatibility notes](compatibility.md)
