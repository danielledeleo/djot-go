package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// One space was stripped from a verbatim's ends only when both ends had one
// and the content touched a backtick; djot.js strips each end on its own
// merits, as the spec's "a single space is removed between the opening or
// closing backticks and the content" describes.
func TestVerbatimStripsEachEndIndependently(t *testing.T) {
	cases := []struct{ in, want string }{
		{"` \n`` `", `verbatim text=" \n` + "``" + `"`},
		{"`\n`` `", `verbatim text="\n` + "``" + `"`},
		{"`` `foo` ``", `verbatim text="` + "`foo`" + `"`},
		{"` a `", `verbatim text=" a "`},
	}
	for _, tc := range cases {
		got := djot.RenderAST(djot.Parse(tc.in), false)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%q: want %s in\n%s", tc.in, tc.want, got)
		}
	}
}
