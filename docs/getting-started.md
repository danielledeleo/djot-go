# Getting started

This guide covers the common path from Djot source to rendered output. For
tree transformations, continue to [Working with the AST](ast.md). For custom
HTML, see [Rendering](rendering.md).

## Install

`djot-go` requires Go 1.22 or later and has no third-party dependencies.

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

`RenderHTML` returns a string. `RenderHTMLTo` writes to an `io.Writer` and
returns the first write error:

```go
if err := djot.RenderHTMLTo(os.Stdout, doc); err != nil {
    return err
}
```

`Parse` uses the default configuration. To configure parsing, create a
`Parser` with `NewParser` and call its `Parse` method. A `Parser` cannot be
changed after it is created, so one can be shared across goroutines:

```go
parser := djot.NewParser()
doc := parser.Parse(source)
```

A document never changes after parsing, so it can be rendered any number of
times, from any goroutine.

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

Print the parsed AST, as text or JSON, with `--to`. This is useful when
checking how a document was parsed:

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

## Edit and render again

`doc.AST()` returns a copy of the parsed tree. Edit the copy, then render it
with `NewDoc`:

```go
tree := doc.AST()
tree.Children = append(tree.Children, &ast.Paragraph{
    Children: []ast.Inline{&ast.Text{Value: "Added in Go."}},
})
html := djot.RenderHTML(djot.NewDoc(tree))
```

`NewDoc` also renders trees built from scratch; see [Working with the
AST](ast.md).

## Next steps

- [Traverse and transform the typed AST](ast.md)
- [Customize HTML output](rendering.md)
- [Read the compatibility notes](compatibility.md)

---

[README](../README.md) · [AST](ast.md) · [Rendering](rendering.md) ·
[Compatibility notes](compatibility.md)
