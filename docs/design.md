# Design

This page explains why the API is shaped the way it is. You don't need it to
use `djot-go`.

## One document type, one renderer

`Parse` produces a `Doc` holding a compact, read-only form of the document: a
flat array of node records plus side tables for text and attributes. The HTML
renderer reads that form directly. It never builds a tree of Go objects, which
is most of why rendering is fast.

There is one renderer. Everything that produces HTML, including render hooks
and documents built from edited trees, goes through it, so output can't drift
between code paths.

## The AST is a copy

`Doc.AST()` builds a tree of ordinary Go structs from the compact form and
returns it. Each call builds a new one.

A shared, editable tree would need rules about who may change it, when
rendering sees the changes, and what is safe across goroutines. Returning a
copy removes those questions: a `Doc` never changes, and a tree is only ever
changed by whoever asked for it. `string` and `[]byte` divide the work the same
way.

`NewDoc(tree)` goes the other direction. It compiles a tree into the compact
form, and the result renders exactly like a parsed document.

## Hooks by node kind

`WithNodeRenderer(kind, fn)` is the only render hook. When the renderer reaches
a node of a hooked kind, it builds that node, with its children, as an
`ast.Node` and calls `fn`. Nodes of other kinds never become Go objects, so a
hook costs roughly in proportion to how much of the document it covers.

Anything that needs the whole document at once, such as a table of contents,
reads `doc.AST()` before rendering instead of asking the renderer for more
context.

## Footnotes follow the output

Footnotes are numbered when their first reference is rendered, and only
referenced footnotes are listed. A hook that drops content therefore drops its
footnotes too, and numbering always matches what is on the page. djot.js does
the same.

## djot.js decides

Where the prose syntax reference and djot.js disagree, `djot-go` follows djot.js
and runs the djot.js test suite. Each difference that remains is listed in the
[compatibility notes](compatibility.md).

## A small API

Adding to an API later breaks no one; removing from it does. So the API leaves
out anything that duplicates something else, or that hasn't been asked for:
there is one renderer, one hook, and one way to get a tree.

---

[README](../README.md) · [Getting started](getting-started.md) ·
[AST](ast.md) · [Rendering](rendering.md) ·
[Compatibility notes](compatibility.md)
