package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// The header flag was set on cells only; the row above a separator line is
// itself a header row in djot.js's AST.
func TestTableHeaderRowIsMarked(t *testing.T) {
	root := djot.Parse("|a|\n|-|\n|b|\n").Root()
	table := root.Children[0].(*ast.Table)
	var rows []*ast.TableRow
	for _, child := range table.Children {
		if row, ok := child.(*ast.TableRow); ok {
			rows = append(rows, row)
		}
	}
	if len(rows) != 2 || !rows[0].Header || rows[1].Header {
		t.Fatalf("row headers = %v, want [true false]", []bool{rows[0].Header, rows[1].Header})
	}
}
