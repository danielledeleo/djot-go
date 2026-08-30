# Compatibility

djot-go targets compatibility with
[djot.js](https://github.com/jgm/djot.js), the reference implementation of
[djot](https://djot.net).

Where the prose [syntax reference] and djot.js differ, djot-go follows djot.js
unless a difference is explicitly documented. This policy keeps behavior
predictable for users moving documents between implementations.

## Conformance testing

The repository includes the official syntax and rendering fixtures under
`testdata/official`. They run as part of `go test ./...`.

The Lua-only filter fixtures do not apply because djot-go does not embed the Lua
filter runtime. This exception concerns filter execution, not parsing or HTML
rendering.

A separate Docker differential test compares generated output with djot.js on
hand-written and generated edge cases. See
[CONTRIBUTING.md](../CONTRIBUTING.md#differential-tests) to run it.

## Supported syntax

Block elements:

- Paragraphs and headings, with generated sections and heading IDs
- Fenced code blocks, including language info strings
- Raw blocks with `=format` info strings
- Block quotes and fenced divs
- Bullet, ordered, task, and definition lists
- Tables with alignment and captions
- Footnote and reference-link definitions
- Thematic breaks

Inline elements:

- Emphasis, strong, superscript, subscript, insert, delete, and mark
- Links, images, reference links, and autolinks
- Spans, verbatim text, inline math, and display math
- Raw inline content
- Symbols, footnote references, and hard, soft, and non-breaking spaces
- Smart quotes, ellipses, em dashes, and en dashes

Block attributes can be attached to block elements. Inline attributes apply to
spans and other supported inline elements. Ordered attributes are retained in
the typed AST and renderer views.

## Output formats

The library and CLI produce:

- HTML
- The official text AST representation, optionally with source positions
- JSON mirroring the text AST's tags and fields, optionally with positions

The JSON format is djot-go's structural output rather than a promise to match a
separate djot.js JSON schema.

## Reporting a difference

When reporting parser or renderer behavior, include:

- The smallest djot input that demonstrates it
- djot-go's output
- djot.js's output
- The expected behavior and relevant syntax-reference section, if known

If djot.js and the prose reference disagree, identify both results. That makes
it possible to distinguish a compatibility regression from an intentional
reference-implementation choice.

## Security

djot-go does not sanitize rendered HTML. Raw HTML constructs and renderer hooks
may intentionally emit HTML, and ordinary URLs may still need application-level
scheme validation.

When processing untrusted input, sanitize the complete rendered result with a
library such as [bluemonday](https://github.com/microcosm-cc/bluemonday). This
matches the integration model used by other Go markup libraries: parsing and
rendering are separate from the application's trust policy.

---

[README](../README.md) · [Getting started](getting-started.md) ·
[AST](ast.md) · [Rendering](rendering.md)

[syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
