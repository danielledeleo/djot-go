package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// Sibling items had to share the marker's column; djot.js groups items of
// the same type regardless of how far each marker is indented.
// A block start at or left of the marker ends the item instead of being
// absorbed as lazy text; djot.js closes the item and opens the block.
func TestListItemEndsAtBlockStart(t *testing.T) {
	cases := []struct{ in, want string }{
		{"* :\n:", "<ul>\n<li>\n<dl>\n<dt></dt>\n<dd>\n</dd>\n</dl>\n</li>\n</ul>\n<dl>\n<dt></dt>\n<dd>\n</dd>\n</dl>\n"},
		{"- a\n> q\n", "<ul>\n<li>\na\n</li>\n</ul>\n<blockquote>\n<p>q</p>\n</blockquote>\n"},
		{"- a\n|t|\n", "<ul>\n<li>\na\n</li>\n</ul>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n"},
	}
	for _, tc := range cases {
		if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
}

func TestListItemsJoinAcrossMarkerIndents(t *testing.T) {
	in := "* *\n\n *"
	want := "<ul>\n<li>\n<ul>\n<li>\n</li>\n<li>\n</li>\n</ul>\n</li>\n</ul>\n"
	if got := djot.RenderHTML(djot.Parse(in)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
