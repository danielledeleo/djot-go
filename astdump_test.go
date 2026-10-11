package djot_test

import (
	"strings"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/internal/astdump"
)

// renderAST returns doc's AST in the djot.js text format.
func renderAST(doc *djot.Doc, positions bool) string {
	var b strings.Builder
	_ = astdump.Text(&b, doc, positions)
	return b.String()
}

// renderASTJSON returns doc's AST as JSON.
func renderASTJSON(doc *djot.Doc, positions bool) string {
	var b strings.Builder
	_ = astdump.JSON(&b, doc, positions)
	return b.String()
}
