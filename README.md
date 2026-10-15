# djot-go

[![Go Reference](https://pkg.go.dev/badge/github.com/danielledeleo/djot-go.svg)](https://pkg.go.dev/github.com/danielledeleo/djot-go)
[![CI](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fdanielledeleo%2Fdjot-go%2Fbadges%2Fcoverage.json)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danielledeleo/djot-go)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)

A zero-dependency Go parser and HTML renderer for
[Djot](https://djot.net), John MacFarlane's light markup language.

`djot-go` parses and renders Djot markup directly to HTML, and hands you a
typed [AST](https://pkg.go.dev/github.com/danielledeleo/djot-go/ast) whenever
you want to inspect or transform it.

- Compatible with djot.js and the official syntax and rendering tests
- Source positions and ordered attributes
- Custom HTML for any standard element: render `:memo:` as 📝
- Edit the AST and render the result with `NewDoc`
- A `djot` command-line tool with HTML, AST, and JSON output

## Install

Requires Go 1.22 or later.

```sh
go get github.com/danielledeleo/djot-go
```

For the command-line tool:

```sh
go install github.com/danielledeleo/djot-go/cmd/djot@latest
```

## Quick start

```go
package main

import (
    "fmt"

    "github.com/danielledeleo/djot-go"
)

func main() {
    doc := djot.Parse("Hello *world*!")
    fmt.Println(djot.RenderHTML(doc))
}
```

Output:

```html
<p>Hello <strong>world</strong>!</p>
```

Render directly to an `io.Writer` when building a response or file:

```go
if err := djot.RenderHTMLTo(w, doc); err != nil {
    return err
}
```

The CLI reads files or standard input and emits HTML by default:

```console
$ echo 'Hello *world*' | djot
<p>Hello <strong>world</strong></p>
```

## Documentation

| Guide | What it covers |
| --- | --- |
| [Getting started](docs/getting-started.md) | Parsing, rendering, editing a document, and the CLI |
| [Working with the AST](docs/ast.md) | Traversal, transforms, construction, attributes, source positions, and references |
| [Rendering](docs/rendering.md) | Custom HTML, whole-document context, and footnotes |
| [Compatibility notes](docs/compatibility.md) | djot.js policy, supported syntax, differences from djot.js, and security |
| [Design](docs/design.md) | Why the API is shaped the way it is |

API references: [`djot`](https://pkg.go.dev/github.com/danielledeleo/djot-go)
and [`djot/ast`](https://pkg.go.dev/github.com/danielledeleo/djot-go/ast).

## Compatibility

`djot-go` aims to implement the Djot spec published at [djot.net](https://djot.net),
but does not guarantee byte-for-byte parity with any other implementation.
`djot-go` passes the whole djot.js test suite, excluding Lua filters. There are
some places where `djot-go` and other implementations differ; see the
[compatibility notes](docs/compatibility.md) for details.

## Stability

From v1.0.0, `djot-go` follows semantic versioning. Within a major version:

- Exported names in the `djot` and `ast` packages keep their meaning, types,
  and fields.
- Minor versions may add node kinds, node fields, options, and functions. Refer
  to kinds by their constants, not their numeric values, and give type switches
  over nodes a default case.
- HTML output may change in a minor version, to follow djot.js or to fix a bug.
- The `djot` command's flags and output formats may change in any release.

## Security

HTML output is not sanitized, and Djot's
[raw HTML](https://github.com/jgm/djot/blob/main/doc/quickstart-for-markdown-users.md#raw-html)
syntax passes through unchanged. Sanitize rendered output, for example with
[bluemonday](https://github.com/microcosm-cc/bluemonday), when processing
untrusted input; see [Security](docs/compatibility.md#security).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for tests, fuzzing, benchmarks, and the
djot.js differential test workflow.

## License

MIT
