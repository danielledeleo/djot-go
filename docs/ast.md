# Working with the AST

The `ast` package defines djot-go's typed, mutable syntax tree. Parsing and
rendering remain in the root `djot` package.

```go
import (
    "github.com/danielledeleo/djot-go"
    "github.com/danielledeleo/djot-go/ast"
)
```

## Materialization and ownership

`djot.Parse` initially keeps a compact semantic representation. `Doc.Root`
materializes and returns its shared typed tree:

```go
doc := djot.Parse("Hello *world*!")
root := doc.Root()
```

Repeated calls return the same root. Mutations are therefore authoritative for
later rendering. Concurrent read-only first access is safe; callers must
synchronize subsequent mutation with other tree users.

## Read-only traversal

Use `ast.Preorder` when no mutation is needed. Returning `false` stops the
walk:

```go
var links []string
ast.Preorder(doc.Root(), func(node ast.Node) bool {
    if link, ok := node.(*ast.Link); ok {
        links = append(links, link.Destination)
    }
    return true
})
```

`ast.ForEachChild` visits only direct children. `ast.WalkBottomUp` visits all
descendants before their parents.

Every node exposes:

```go
kind := node.Kind()
span := node.Span()
attrs := node.Attributes()
```

Kinds can be classified without a type switch:

```go
switch {
case kind.IsBlock():
    // Block-level node.
case kind.IsInline():
    // Inline-level node.
}
```

## Transform a tree

`ast.Walk` performs top-down transformations. Its callback returns one of:

- `ast.Continue` — retain the node and visit its children
- `ast.SkipChildren` — retain the node without descending
- `ast.Remove` — remove the node from its occupied slot
- `ast.Replace(node)` — replace it and visit the replacement's children

This changes strong emphasis to regular emphasis:

```go
ast.Walk(doc.Root(), func(node ast.Node) ast.Action {
    strong, ok := node.(*ast.Strong)
    if !ok {
        return ast.Continue
    }

    replacement := &ast.Emphasis{Children: strong.Children}
    ast.CopyMetadata(replacement, strong)
    return ast.Replace(replacement)
})
```

`CopyMetadata` copies the source span and an independent copy of the ordered
attributes. `SetSpan` changes a node's source span directly.

Replacements must preserve the grammar category of their occupied slot:
documents replace documents, blocks replace blocks, and inlines replace
inlines. Specialized slots are stricter: list children must remain list items,
task-list children must remain task-list items, and table-row children must
remain cells. Invalid replacements panic instead of creating a malformed tree.

When replacing the root itself, use the value returned by `Walk`:

```go
next := ast.Walk(doc.Root(), transform)
root, ok := next.(*ast.Document)
if !ok {
    return errors.New("transform removed the document root")
}
doc.SetRoot(root)
```

## Construct a tree

Concrete child fields are typed as `[]ast.Block`, `[]ast.Inline`, or a more
specific child type, so ordinary composite literals catch many malformed trees
at compile time:

```go
root := &ast.Document{
    Children: []ast.Block{
        &ast.Heading{
            Level: 1,
            Children: []ast.Inline{
                &ast.Text{Value: "Created in Go"},
            },
        },
        &ast.Paragraph{
            Children: []ast.Inline{
                &ast.Text{Value: "No parser required."},
            },
        },
    },
}

html := djot.RenderHTML(djot.NewDoc(root))
```

`ast.AppendChild(parent, child)` is useful for generic builders. It enforces
the same category and specialized-slot rules at runtime.

## Attributes

Attributes preserve insertion order:

```go
heading := &ast.Heading{Level: 2}
heading.Attributes().Set("id", "installation")
heading.Attributes().AddClass("numbered")
heading.Attributes().Set("data-level", "2")
```

Available operations include `Get`, `Lookup`, `Set`, `Delete`, `AddClass`,
`Len`, `Range`, `Entries`, and `Clone`. `Set` rejects keys outside the djot
attribute-name grammar and leaves the collection unchanged.

Use `djot.ParseAttrs` to parse djot attribute syntax without parsing a complete
document:

```go
values := djot.ParseAttrs(`.warning #notice role="note"`)
```

## Source positions

Node spans are half-open byte ranges. Resolve a position through the document's
source-file table:

```go
doc := djot.Parse("# Heading")
section := doc.Root().Children[0].(*ast.Section)
heading := section.Children[0].(*ast.Heading)
file, line, column := doc.Position(heading.Span().Start)
fmt.Printf("%s:%d:%d\n", file, line, column)
```

Line and column are one-based. `ast.Pos.Offset` is a byte offset, which matters
for UTF-8 input.

## References and footnotes

`Doc.References` returns the mutable resolved-reference metadata, including
implicit heading references. `Doc.Footnotes` rebuilds an index of footnote
definitions from the current tree:

```go
for label, reference := range doc.References() {
    fmt.Println(label, reference.Destination)
}

for label, footnote := range doc.Footnotes() {
    fmt.Println(label, len(footnote.Children))
}
```

For read-only document summaries that avoid materializing the AST, use
`djot.WithDocumentRenderer`; see [Rendering and extensions](rendering.md).

---

[README](../README.md) · [Getting started](getting-started.md) ·
[Rendering](rendering.md) · [Compatibility](compatibility.md)
