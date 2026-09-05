package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Expectations come from djot.js 0.3.2 (npm @djot/djot) run over the same
// input.

// A footnote definition's paragraph could not continue lazily on an
// unindented line; the spec allows it "as with block quotes and list items".
// Only a paragraph continues lazily: a fence or table closes the footnote,
// while a line that merely looks like a block start ("{" without a valid
// attribute list, ">" without a space) is text. All checked against djot.js.
func TestFootnoteLazyContinuationNeedsOpenParagraph(t *testing.T) {
	ref := "<p>x<a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n"
	note := func(body string) string {
		return "<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n" + body +
			"<a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n"
	}
	cases := []struct{ in, want string }{
		{"x[^a]\n\n[^a]: ```\n  c\nlazy\n", ref + "<p>lazy</p>\n" + note("<pre><code>c\n</code></pre>\n<p>")},
		{"x[^a]\n\n[^a]: |t|\nlazy\n", ref + "<p>lazy</p>\n" + note("<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n<p>")},
		{"x[^a]\n\n[^a]: one\n{not attr\n", ref + note("<p>one\n{not attr")},
		{"x[^a]\n\n[^a]: one\n>q\n", ref + note("<p>one\n&gt;q")},
	}
	for _, tc := range cases {
		if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
}

func TestFootnoteParagraphContinuesLazily(t *testing.T) {
	in := "x[^a]\n\n[^a]: This is a note\nwith two paragraphs.\n\n  Second.\n"
	want := "<p>x<a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n" +
		"<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n" +
		"<p>This is a note\nwith two paragraphs.</p>\n<p>Second.<a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n" +
		"</li>\n</ol>\n</section>\n"
	if got := djot.RenderHTML(djot.Parse(in)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
