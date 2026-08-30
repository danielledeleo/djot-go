# djot-go

[![Go Reference](https://pkg.go.dev/badge/github.com/danielledeleo/djot-go.svg)](https://pkg.go.dev/github.com/danielledeleo/djot-go)
[![CI](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fdanielledeleo%2Fdjot-go%2Fbadges%2Fcoverage.json)](https://github.com/danielledeleo/djot-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/danielledeleo/djot-go)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-0-brightgreen)](https://github.com/danielledeleo/djot-go/blob/main/go.mod)

A zero-dependency Go parser and HTML renderer for
[djot](https://djot.net), John MacFarlane's light markup language.

djot-go provides a compact direct-to-HTML path for ordinary rendering and a
typed, mutable [`ast`](https://pkg.go.dev/github.com/danielledeleo/djot-go/ast)
package for inspection and transformation.

- Compatible with djot.js and the official syntax and rendering tests
- Streaming HTML, text-AST, and JSON output
- Source positions and ordered attributes
- Compact render hooks for elements, subtrees, and document indexes
- Full mutable-tree hooks when arbitrary inspection is needed
- A `djot` command-line tool

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
| [Getting started](docs/getting-started.md) | Parsing, writers, output formats, the CLI, and common workflows |
| [Working with the AST](docs/ast.md) | Traversal, mutation, construction, attributes, and source positions |
| [Rendering and extensions](docs/rendering.md) | Choosing hooks, compact rendering, custom HTML, document indexes, and footnotes |
| [Compatibility](docs/compatibility.md) | djot.js policy, supported syntax, conformance tests, and security |

API references: [`djot`](https://pkg.go.dev/github.com/danielledeleo/djot-go)
and [`djot/ast`](https://pkg.go.dev/github.com/danielledeleo/djot-go/ast).

## Compatibility

djot-go targets djot.js, the reference implementation. Where djot.js and the
prose syntax reference differ, djot-go follows djot.js unless documented
otherwise. See the [compatibility guide](docs/compatibility.md) for details.

## Security

HTML output is not sanitized. Pass rendered output through an HTML sanitizer
when processing untrusted input; see [Security](docs/compatibility.md#security).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for tests, fuzzing, benchmarks, and the
djot.js differential test workflow.

## License

MIT
