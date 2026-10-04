package djot

import "github.com/danielledeleo/djot-go/ast"

// Doc is a parsed document. It is immutable: rendering reads its compact form
// directly, and AST returns a copy of its syntax tree for inspection or
// editing. To render an edited tree, pass it to NewDoc.
type Doc struct {
	source   *sourceFile
	semantic *semanticTape

	// Parser working state, released before Parse returns.
	parseRoot       *parseNode
	parseReferences map[string]*parseNode
}

// NewDoc returns a document compiled from tree, including tree.References.
// Later changes to tree do not affect the document.
func NewDoc(tree *ast.Document) *Doc {
	if tree == nil {
		tree = &ast.Document{}
	}
	return &Doc{semantic: compileSemanticTape(tree, tree.References)}
}

// AST returns a new syntax tree for the document on each call. The caller owns
// the tree; changing it does not affect the document.
func (d *Doc) AST() *ast.Document {
	if d.semantic == nil {
		return &ast.Document{}
	}
	tree := d.semantic.materializeAST()
	tree.References = d.semantic.materializeReferences()
	return tree
}

// tape returns the document's compact form, or an empty one for a zero Doc.
func (d *Doc) tape() *semanticTape {
	if d.semantic == nil {
		d = NewDoc(nil)
	}
	return d.semantic
}

// Position resolves a source position to a line and column. It returns the
// zero Position when the document has no source text, as for one built with
// NewDoc, or when the offset lies outside the source.
func (d *Doc) Position(position ast.Pos) ast.Position {
	if d.source == nil || position.Offset < 0 || position.Offset > len(d.source.text) {
		return ast.Position{}
	}
	line, col := d.source.position(position.Offset)
	return ast.Position{Line: line, Column: col}
}
