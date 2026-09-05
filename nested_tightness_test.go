package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// A blank line between blocks of a nested item made every enclosing list
// loose. The spec counts blank lines "between items, or between blocks
// inside an item"; djot.js charges a blank to the innermost list only.
func TestNestedItemBlankDoesNotLoosenOuterList(t *testing.T) {
	cases := []struct {
		in         string
		outerTight bool
		innerTight bool
	}{
		{"- a\n\n  - b\n\n    c\n", true, false},
		{"* *\n\n  * a\n\n    0\n", true, false},
		{"- a\n\n  - b\n\n    c\n- d\n", true, false},
		{"- a\n\n  - b\n\n  c\n", false, true},
	}
	for _, tc := range cases {
		root := djot.Parse(tc.in).Root()
		outer := root.Children[0].(*ast.BulletList)
		item := outer.Items[0]
		var inner *ast.BulletList
		for _, b := range item.Children {
			if l, ok := b.(*ast.BulletList); ok {
				inner = l
			}
		}
		if inner == nil || outer.Tight != tc.outerTight || inner.Tight != tc.innerTight {
			t.Errorf("%q: outer tight=%v inner=%v, want %v %v", tc.in, outer.Tight, inner != nil && inner.Tight, tc.outerTight, tc.innerTight)
		}
	}
}
