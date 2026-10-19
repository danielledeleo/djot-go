# Compatibility notes

`djot-go` follows [djot.js](https://github.com/jgm/djot.js), the reference
implementation of [Djot](https://djot.net). Where the prose [syntax reference]
and djot.js differ, `djot-go` follows djot.js. Where djot.js's behaviour is
ambiguous or apparently unintentional, `djot-go` may differ in subtle ways;
known differences are listed below.

## Conformance testing

The djot.js test suite is vendored under `testdata/official` and runs as part
of `go test ./...`. `testdata/README` records the djot.js commit it was taken
from. Every test passes except the filter tests, which run Lua filters;
`djot-go` has no filter runtime.

A separate differential test, run in Docker, compares `djot-go` with djot.js at
the same commit on hand-written and generated edge cases. See
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
spans and other supported inline elements. Attributes keep their order in the
typed AST and in HTML output.

## Documented differences

### Ordered list enumerator bound

`djot-go` accepts decimal ordered-list enumerators up to 2147483647. A line whose
enumerator exceeds that is not a list; it stays a paragraph:

```
2147483647. item    ->  <ol start="2147483647">
2147483648. item    ->  <p>2147483648. item</p>
```

The bound is the widest value an HTML `ol` `start` attribute can carry, so a
larger start would not survive rendering in any case. Fixing it to that value
rather than to the platform `int` maximum also keeps parsing identical on 32-
and 64-bit builds.

djot.js reads enumerators with `parseInt`, so it has no explicit bound and
accepts larger values, exactly up to 2^53 and with rounding above that. Alpha
and Roman enumerators are far below any of these limits and are unaffected.

### Nested parentheses in link destinations

For `[link](url_(with_(nested)_parens))`, `djot-go` keeps the whole
destination. djot.js ends the destination at the first unbalanced `)` and
emits the rest as text.

### Block quotes in tight lists

In a tight list, a list item's own paragraphs are not wrapped in `<p>`.
`djot-go` applies that only to the item's own paragraphs. djot.js also drops
the `<p>` from paragraphs inside a block quote in the item:

```
- > quote    ->  djot-go: <blockquote>\n<p>quote</p>\n</blockquote>
                 djot.js: <blockquote>\nquote\n</blockquote>
```

## Known divergences

The differential test in [CONTRIBUTING.md](../CONTRIBUTING.md#differential-tests)
finds eight cases where `djot-go` and djot.js disagree. Two are the differences
listed above. The rest need unbalanced or malformed delimiters; the equivalent
well-formed inputs agree.

- **An unpaired opening `"` inside emphasis or strong.** `*"x*` renders the
  quote literally rather than as `“`. A paired quote (`_"x" y_`), an unpaired
  quote outside a span (`"x`), and a closing quote after a digit (`*a 5" pipe*`)
  all agree.
- **Smart punctuation inside an unterminated attribute block.** `a{#id---`
  converts the dashes; djot.js leaves them literal. Closed attribute blocks
  agree.
- **An escaped `$` before a verbatim span**, in one generated case that does not
  reduce to a smaller input.
- **A `^` right before a footnote reference.** In `a^[^1]`, djot.js reads the
  `^` inside `[^1]` as closing a superscript and renders `a<sup>[</sup>1]`;
  `djot-go` renders a footnote reference.

## Output formats

The library renders HTML. The `djot` command can also print:

- The text AST format used by the djot.js tests, optionally with source
  positions
- JSON with the same tags and fields, optionally with positions

The JSON follows `djot-go`'s AST. It does not match djot.js's JSON output.

## Reporting a difference

When reporting parser or renderer behavior, include:

- The smallest Djot input that demonstrates it
- `djot-go`'s output
- djot.js's output
- The expected behavior and relevant syntax-reference section, if known

If djot.js and the prose reference disagree, identify both results. That makes
it possible to distinguish a compatibility regression from an intentional
reference-implementation choice.

## Security

`djot-go` does not sanitize rendered HTML. Raw HTML and render hooks can emit
any HTML, and ordinary URLs may still need application-level
scheme validation.

When processing untrusted input, sanitize the complete rendered result with a
library such as [bluemonday](https://github.com/microcosm-cc/bluemonday). This
matches the integration model used by other Go markup libraries: parsing and
rendering are separate from the application's trust policy.

---

[README](../README.md) · [Getting started](getting-started.md) ·
[AST](ast.md) · [Rendering](rendering.md)

[syntax reference]: https://htmlpreview.github.io/?https://github.com/jgm/djot/blob/main/doc/syntax.html
