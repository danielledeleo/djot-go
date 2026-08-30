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

## Documented differences

### Ordered list enumerator bound

djot-go accepts decimal ordered-list enumerators up to 2147483647. A line whose
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

### Generated heading identifiers

Headings without an explicit `#id` get one derived from their text. djot-go and
djot.js derive it differently, and djot-go keeps its own rule:

| Heading | djot-go | djot.js |
|------------------|---------------|---------------|
| `# foo: bar`     | `foo-bar`     | `foo:-bar`    |
| `# Section 1.2`  | `Section-12`  | `Section-1-2` |
| `# a(b)c`        | `abc`         | `a-b-c`       |
| `# 50% off & more` | `50-off--more` | `50-off-more` |
| `# a---b`        | `ab`          | `a---b`       |

djot-go keeps letters, digits, `-`, and `_`, turns each run of whitespace into a
single `-`, and drops everything else. djot.js instead replaces runs of
``][~!@#$%^&*(){}`,.<>\|=+/?`` and whitespace with a separator, preserves the
punctuation outside that set (`:`, `;`, `'`, `"`), and reads the heading before
smart punctuation is applied, so `---` survives as three hyphens.

Headings whose text is entirely letters, digits, and spaces get the same
identifier from both. The identifiers are otherwise not interchangeable, so
anchors written against one implementation may not resolve under the other.

## Known divergences

The differential suite described in [CONTRIBUTING.md](../CONTRIBUTING.md#differential-tests)
compares generated edge cases against djot.js. Ten still diverge, in five
groups. All but the heading identifiers need unbalanced or invalid delimiters;
the equivalent well-formed inputs agree.

- **Generated heading identifiers**, as above. A deliberate difference.
- **An unpaired opening `"` inside emphasis or strong.** `*"x*` renders the
  quote literally rather than as `“`. A paired quote (`_"x" y_`), an unpaired
  quote outside a span (`"x`), and a closing quote after a digit (`*a 5" pipe*`)
  all agree.
- **A superscript that swallows a footnote reference.** In `^x [^1]` djot-go
  pairs the opening `^` with the one inside `[^1]`, where djot.js gives the
  footnote reference precedence. Requires an unclosed `^`; `x^2^ and a note[^1]`
  agrees.
- **Smart punctuation inside an unterminated attribute block.** `a{#id---`
  converts the dashes; djot.js leaves them literal. Closed specifiers agree.
- **An escaped `$` before a verbatim span**, in one generated case that does not
  reduce to a smaller input.

One further difference runs the other way and is not treated as a divergence to
fix: for `[link](url_(with_(nested)_parens))` djot-go keeps the whole
destination, while djot.js truncates it at the first unbalanced `)` and emits
the remainder as text.

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
