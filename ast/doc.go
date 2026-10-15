// Package ast defines the syntax tree for djot-go.
//
// djot.Doc.AST returns a new copy of a parsed document's tree on each call.
// Nodes may be inspected directly or transformed with Walk. Edited or
// constructed trees are rendered by passing their Document to djot.NewDoc.
package ast
