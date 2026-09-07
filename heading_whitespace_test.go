package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// Heading text and a footnote definition's first line were trimmed with
// TrimSpace, so non-ASCII whitespace at their edges vanished. djot.js strips
// only spaces and tabs after the marker and trailing spaces at the line end.
func TestHeadingAndFootnoteKeepUnicodeWhitespace(t *testing.T) {
	cases := []struct{ in, want string }{
		{"# \u00a0a\u00a0", "str text=\"\u00a0a\u00a0\""},
		{"# \u0085a", "str text=\"\u0085a\""},
		{"# \u2003a\u2003", "str text=\"\u2003a\u2003\""},
		{"# \fa\f", "str text=\"\fa\f\""},
		{"#\ta \n", "str text=\"a\""},
		{"# a\t\n", "str text=\"a\\t\""},
		{"# a\n# b\t\n", "str text=\"b\\t\""},
		{"x[^1]\n\n[^1]: \u00a0a", "str text=\"\u00a0a\""},
		{"x[^1]\n\n[^1]:\ta\t\n", "str text=\"a\\t\""},
	}
	for _, tc := range cases {
		got := djot.RenderAST(djot.Parse(tc.in), false)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%q: want %s in\n%s", tc.in, tc.want, got)
		}
	}
}
