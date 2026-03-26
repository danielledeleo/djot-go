# djot-go

[![Go Reference](https://pkg.go.dev/badge/github.com/danielledeleo/djot-go.svg)](https://pkg.go.dev/github.com/danielledeleo/djot-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/danielledeleo/djot-go)](https://goreportcard.com/report/github.com/danielledeleo/djot-go)
[![CI](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/danielledeleo/djot-go/branch/main/graph/badge.svg)](https://codecov.io/gh/danielledeleo/djot-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danielledeleo/djot-go)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)

A Go parser and HTML renderer for [djot](https://djot.net), a light markup
language designed by John MacFarlane as a successor to Markdown.

- Passes all 288 official spec tests
- Zero dependencies
- Typed AST with source positions
- Custom rendering via hooks
- ~24 MB/s parse throughput on realistic documents

## Install

```
go get github.com/danielledeleo/djot-go
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
djot.Walk(doc.Root, func(n *djot.Node) any {
    if n.Kind == djot.Link {
        fmt.Println(n.Target)
    }
    return djot.Continue
})
```

The walker supports `Continue`, `SkipChildren`, `Remove`, and `Replace(node)`
actions. `WalkBottomUp` visits children before parents.

### Custom rendering

Override the HTML output for specific node kinds:

```go
html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.CodeBlock, func(n *djot.Node, r djot.NodeRenderer) {
    r.Write(`<pre class="highlight"><code>`)
    r.Write(n.Text)
    r.Write("</code></pre>")
}))
```

Inside a hook, `r.Default()` emits the built-in rendering and `r.Children()`
renders child nodes without the wrapper element.

For simple cases that just return an HTML string, use `WithRenderFunc`:

```go
html := djot.RenderHTML(doc, djot.WithRenderFunc(djot.Symbol, func(n *djot.Node) string {
    if n.Name == "youtube" {
        return `<iframe src="https://www.youtube.com/embed/` + n.Attr("id") + `"></iframe>`
    }
    return "" // empty string falls through to default rendering
}))
```

Symbols (`:name:{attrs}`) are parsed into typed AST nodes and render as
`:name:` by default — making them natural extension points for icons, embeds,
and shortcodes with arbitrary attributes.

### Inspect the AST

```go
fmt.Println(djot.RenderAST(doc, false)) // set true for source positions
```

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
