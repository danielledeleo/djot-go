// Package ast defines djot-go's typed, mutable abstract syntax tree.
//
// Parsed documents expose their tree through djot.Doc.Root. Nodes may be
// inspected directly or transformed with Walk. Constructed trees can be
// rendered by wrapping their Document in djot.NewDoc.
package ast
