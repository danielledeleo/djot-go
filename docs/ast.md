# Working with the AST

The `ast` package defines `djot-go`'s typed, mutable syntax tree. Parsing and
rendering remain in the root `djot` package.

```go
import (
    "github.com/danielledeleo/djot-go"
    "github.com/danielledeleo/djot-go/ast"
)
```

## Get a tree

```go
doc := djot.Parse("Hello *world*!")
tree := doc.AST()
```

`AST` returns a new copy of the tree on each call. Changes to it don't affect
`doc`; render the changed tree with `NewDoc`:

```go
tree.Children = append(tree.Children, &ast.ThematicBreak{})
html := djot.RenderHTML(djot.NewDoc(tree))
```

`doc` itself never changes, so it can be rendered or read from many goroutines
at once.

## Read-only traversal

Use `ast.Preorder` when no mutation is needed. Returning `false` stops the
walk:

```go
var links []string
ast.Preorder(tree, func(node ast.Node) bool {
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
ast.Walk(tree, func(node ast.Node) ast.Action {
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
next := ast.Walk(tree, transform)
root, ok := next.(*ast.Document)
if !ok {
    return errors.New("transform removed the document root")
}
html := djot.RenderHTML(djot.NewDoc(root))
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
`Len`, `Range`, `Entries`, and `Clone`. `Set` rejects keys outside the Djot
attribute-name grammar and leaves the collection unchanged.

## Source positions

Node spans are half-open byte ranges. Resolve a position to a line and column
through the document:

```go
doc := djot.Parse("# Heading")
section := doc.AST().Children[0].(*ast.Section)
heading := section.Children[0].(*ast.Heading)
pos := doc.Position(heading.Span().Start)
fmt.Printf("%d:%d\n", pos.Line, pos.Column)
```

Line and column are one-based. `ast.Pos.Offset` is a byte offset, which matters
for UTF-8 input. The `djot` command's `ast` and `json` output keep the djot.js
convention of inclusive inline end offsets; use `Span` when slicing source
text.

`Position` returns the zero `ast.Position` when the offset is outside the
source, or when the document has no source text, as for one built with
`djot.NewDoc`.

## References and footnotes

`tree.References` holds the document's reference definitions as parsed,
including the implicit references headings create. Reference links are
resolved during parsing, so each link already carries its destination:

```go
for label, reference := range tree.References {
    fmt.Println(label, reference.Destination)
}
```

Footnote definitions are ordinary `ast.Footnote` nodes, found where they
appear in the source:

```go
ast.Preorder(tree, func(n ast.Node) bool {
    if footnote, ok := n.(*ast.Footnote); ok {
        fmt.Println(footnote.Label, len(footnote.Children))
    }
    return true
})
```

---

[README](../README.md) · [Getting started](getting-started.md) ·
[Rendering](rendering.md) · [Compatibility notes](compatibility.md)
