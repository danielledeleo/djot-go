# djot-go

[![Go Reference](https://pkg.go.dev/badge/github.com/danielledeleo/djot-go.svg)](https://pkg.go.dev/github.com/danielledeleo/djot-go)
[![CI](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fdanielledeleo%2Fdjot-go%2Fbadges%2Fcoverage.json)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danielledeleo/djot-go)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)

A Go parser and HTML renderer for [djot](https://djot.net), a light markup
language designed by John MacFarlane as a successor to Markdown.

- Passes the official djot spec test suite (the Lua-only `filters` tests excepted)
- Zero dependencies
- Typed AST with source positions
- Custom rendering via hooks
- `djot` command-line tool (HTML, AST, and JSON output)

## Install

As a library:

```
go get github.com/danielledeleo/djot-go
```

As a command-line tool:

```
go install github.com/danielledeleo/djot-go/cmd/djot@latest
```

Requires Go 1.22+.

## Usage

### Parse and render

```go
doc := djot.Parse(input)
html := djot.RenderHTML(doc)
```

### Write to an existing buffer

```go
var buf strings.Builder
djot.RenderHTMLTo(&buf, doc)
```

### Walk the AST

```go
djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
    if link, ok := n.(*djot.Link); ok {
        fmt.Println(link.Destination)
    }
    return djot.Continue
})
```

The walker supports `Continue`, `SkipChildren`, `Remove`, and `Replace(node)`
actions. `WalkBottomUp` visits children before parents.

Use `djot.NewDoc(root)` to render an externally constructed AST. To replace a
document's root, call `doc.SetRoot(root)`; parsed documents otherwise materialize
their mutable AST only when `doc.Root()` is requested.

### Custom rendering

Override the HTML output for specific node kinds:

```go
html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.KindCodeBlock, func(n djot.Node, r djot.NodeRenderer) {
    code := n.(*djot.CodeBlock)
    r.Write(`<pre class="highlight"><code>`)
    r.Write(code.Text)
    r.Write("</code></pre>")
}))
```

Inside a hook, `r.Default()` emits the built-in rendering and `r.Children()`
renders child nodes without the wrapper element.

For a common container customization, `WithDivRenderer` reads the Div directly
from the compact representation. It exposes read-only attributes and streams
children without materializing the AST:

```go
html := djot.RenderHTML(doc, djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
    if div.Attributes().Get("class") != "warning" {
        r.Default()
        return
    }
    r.Write(`<aside class="warning">`)
    r.Children()
    r.Write(`</aside>`)
}))
```

`r.Default()` retains the built-in `<div>` wrapper. `r.Children()` renders the
existing children without that wrapper, while returning without writing or
rendering children suppresses the Div and its contents. Parsed, unmodified
documents stay on the compact rendering path; mutated and externally built
trees receive the same callback through the tree renderer.

`WithDivRenderer` can inspect the Div itself but deliberately cannot inspect
its children before rendering. When the wrapper depends on descendant content,
use a bounded, read-only subtree view:

```go
html := djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv,
    func(subtree djot.SubtreeView, r djot.ElementRenderer) {
        if subtree.Contains(djot.KindCodeBlock) {
            r.Write(`<section class="contains-code">`)
            r.Children()
            r.Write(`</section>`)
            return
        }
        r.Default()
    },
))
```

Subtree inspection scans only that element's contiguous tape range. It remains
read-only and does not construct Nodes. `Preorder` and `Descendants` provide
full traversal when `Contains` is not sufficient. Since every inspection is a
scan, avoid repeatedly scanning heavily nested overlapping subtrees. Use Nodes
when an extension requires a general whole-document traversal.

For decisions that require a focused index over the entire document,
`WithDocumentRenderer` can inspect headings before output begins. This example
builds a table of contents before emitting the normal document:

```go
html := djot.RenderHTML(doc, djot.WithDocumentRenderer(
    func(document djot.DocumentView, r djot.DocumentRenderer) {
        r.Write(`<nav><ol>`)
        for _, heading := range document.Headings() {
            r.Write(`<li><a href="#` + html.EscapeString(heading.ID()) + `">`)
            r.Write(html.EscapeString(heading.Text()))
            r.Write(`</a></li>`)
        }
        r.Write(`</ol></nav>`)
        r.Default()
    },
))
```

`Headings()` is built lazily and reused within that render. `r.Default()` emits
the complete normal document, including endnotes and other registered hooks, so
output written before and after it properly wraps everything. Heading inspection
remains tape-backed and does not materialize the AST unless another option or
prior mutation requires the tree.

`DocumentRenderer.Write` writes raw HTML, so derived text and attribute values
must be escaped as shown above.

Symbols have a compact rendering hook that does not materialize the AST for an
ordinary parsed document:

```go
html := djot.RenderHTML(doc, djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
    if symbol.Name == "youtube" {
        r.Write(`<span class="youtube-icon"></span>`)
        return
    }
    r.Default()
}))
```

Symbols (`:name:`) render literally by default, making them natural extension
points for icons and shortcodes. `r.Default()` retains that built-in rendering;
returning without writing suppresses the symbol. If the typed AST has been
modified, the same hook runs through the tree renderer and sees those changes.

`WithRenderFunc` remains available as a concise Node-based hook for other kinds.

### Footnote backlinks

A footnote referenced more than once carries its `id` on the first reference
only (matching djot.js), with a single `↩︎` linking back to it.
`WithMultiBacklinks` switches to MediaWiki-style backlinks — every reference
gets a unique `id` and the footnote links back to each with lettered labels
(`a`, `b`, `c`, …):

```go
html := djot.RenderHTML(doc, djot.WithMultiBacklinks())
```

The footnote id and label scheme is overridable — for example to namespace ids
when embedding output in a larger page, or to use numeric backlinks:

```go
html := djot.RenderHTML(doc,
    djot.WithFootnotePrefix("post42-"), // post42-fn1, post42-fnref1, …
    djot.WithFootnoteBacklinkLabel(func(num, k, total int) string {
        return strconv.Itoa(k)
    }),
)
```

`WithFootnoteID`, `WithFootnoteRefID`, and `WithFootnoteBacklinkLabel` set the
pieces individually; `WithFootnotePrefix` is shorthand for namespacing the ids.

### Inspect the AST

```go
fmt.Println(djot.RenderAST(doc, false)) // set true for source positions
```

## Command-line tool

The `djot` command converts djot to HTML (default), the text AST, or JSON. It
reads from the given files, or from stdin when none are given:

```
$ echo 'Hello *world*' | djot
<p>Hello <strong>world</strong></p>

$ djot -t json doc.dj
$ djot --to ast --sourcepos doc.dj
$ djot -o out.html doc.dj
```

| Option | Description |
| --- | --- |
| `-t`, `--to FORMAT` | Output format: `html`, `ast`, or `json` (default `html`). |
| `-o`, `--output FILE` | Write to `FILE` instead of stdout. |
| `-p`, `--sourcepos` | Include source positions (`ast` and `json` formats). |
| `--version` | Print version and exit. |
| `-h`, `--help` | Show help and exit. |

The `json` format mirrors djot-go's own AST (the same tag and field names as the
text AST), making it convenient for tooling and diffing.

## Supported features

Blocks: paragraphs, headings (1-6), code blocks (with language), raw blocks,
block quotes, divs, bullet/ordered/task/definition lists, tables with
alignments and captions, footnotes, thematic breaks, reference link
definitions.

Inline: emphasis, strong, superscript, subscript, insert, delete, mark,
links, images, autolinks, spans, verbatim, inline/display math, raw inline,
smart quotes, em/en dashes, ellipses, symbols, hard/soft breaks,
non-breaking spaces, footnote references.

Block attributes (`{.class #id key="value"}`) can be attached to any block
element. Inline attributes work on spans and other inline elements.

Sections are automatically generated from headings with auto-ID slugification.

## Security

This package does not sanitize HTML output. If you render untrusted input,
pass the output through an HTML sanitizer such as
[bluemonday](https://github.com/microcosm-cc/bluemonday). This is the same
approach taken by [goldmark](https://github.com/yuin/goldmark),
[blackfriday](https://github.com/russross/blackfriday), and other Go markup
libraries.

## License

MIT
