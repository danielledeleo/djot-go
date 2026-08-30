# Getting started

This guide covers the common path from djot source to rendered output. For
tree transformations, continue to [Working with the AST](ast.md). For custom
HTML, see [Rendering and extensions](rendering.md).

## Install

djot-go requires Go 1.22 or later and has no third-party dependencies.

```sh
go get github.com/danielledeleo/djot-go
```

Import the root package for parsing and rendering:

```go
import "github.com/danielledeleo/djot-go"
```

Import the `ast` subpackage only when inspecting or changing the typed tree:

```go
import "github.com/danielledeleo/djot-go/ast"
```

## Parse and render HTML

`Parse` accepts a string and returns a reusable document:

```go
source := "# Welcome\n\nHello *world*!"
doc := djot.Parse(source)
html := djot.RenderHTML(doc)
```

`RenderHTML` returns a string. `RenderHTMLTo` streams to an `io.Writer` and
reports write errors:

```go
var output strings.Builder
if err := djot.RenderHTMLTo(&output, doc); err != nil {
    return err
}
fmt.Print(output.String())
```

Documents can be rendered repeatedly. An untouched parsed document uses the
compact representation directly. `Root` materializes the typed tree; unchanged
trees can still use compact rendering, while mutations become authoritative.

## Inspect parser output

The text AST is useful while developing syntax or diagnosing a document:

```go
fmt.Print(djot.RenderAST(doc, false))
```

Pass `true` to include source positions:

```go
fmt.Print(djot.RenderAST(doc, true))
```

JSON output mirrors the same tags and fields:

```go
if err := djot.RenderASTJSONTo(os.Stdout, doc, true); err != nil {
    return err
}
```

The string-returning equivalents are `RenderAST` and `RenderASTJSON`; the
writer variants are `RenderASTTo` and `RenderASTJSONTo`.

## Use the command-line tool

Install the CLI:

```sh
go install github.com/danielledeleo/djot-go/cmd/djot@latest
```

It reads named files, or standard input when no files are supplied:

```console
$ echo 'Hello *world*' | djot
<p>Hello <strong>world</strong></p>

$ djot article.dj
$ djot article.dj appendix.dj
```

Choose text-AST or JSON output with `--to`:

```console
$ djot --to ast article.dj
$ djot -t json --sourcepos article.dj
$ djot -t html -o article.html article.dj
```

| Option | Description |
| --- | --- |
| `-t`, `--to FORMAT` | `html`, `ast`, or `json`; defaults to `html` |
| `-o`, `--output FILE` | Write to a file instead of standard output |
| `-p`, `--sourcepos` | Include source positions in AST or JSON output |
| `--version` | Print the installed version |
| `-h`, `--help` | Show command help |

Multiple input files are concatenated in argument order. Diagnostics go to
standard error and invalid options return a nonzero exit status.

## Reuse or replace a document tree

Calling `doc.Root()` materializes the typed tree on demand. Mutations to that
shared tree are reflected by later renders:

```go
root := doc.Root()
// Inspect or mutate root.
fmt.Print(djot.RenderHTML(doc))
```

Use `SetRoot` when replacing the complete document tree:

```go
root := &ast.Document{
    Children: []ast.Block{
        &ast.Paragraph{Children: []ast.Inline{
            &ast.Text{Value: "constructed in Go"},
        }},
    },
}
doc.SetRoot(root)
```

Use `NewDoc` when starting with an independently constructed tree:

```go
doc := djot.NewDoc(root)
html := djot.RenderHTML(doc)
```

## Next steps

- [Traverse and transform the typed AST](ast.md)
- [Choose an HTML rendering hook](rendering.md)
- [Review syntax compatibility and security](compatibility.md)

---

[README](../README.md) · [AST](ast.md) · [Rendering](rendering.md) ·
[Compatibility](compatibility.md)
