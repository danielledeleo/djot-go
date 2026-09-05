package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// A footnote reference was recognized only at its closing bracket, after
// delimiters inside the label had already been paired; djot.js matches
// "[^label]" as soon as it sees the opening bracket.
func TestFootnoteReferenceBeatsDelimiters(t *testing.T) {
	note := "<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p>%s<a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n"
	ref := "<a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a>"
	cases := []struct{ in, want string }{
		{"*[^*]*\n\n[^*]: n", "<p><strong>" + ref + "</strong></p>\n" + strings.Replace(note, "%s", "n", 1)},
		{"*[^ a*]*", "<p><strong>" + ref + "</strong></p>\n" + strings.Replace(note, "%s", "", 1)},
		{"^x [^1]", "<p>^x " + ref + "</p>\n" + strings.Replace(note, "%s", "", 1)},
	}
	for _, tc := range cases {
		if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
}
