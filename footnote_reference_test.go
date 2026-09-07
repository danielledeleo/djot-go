package djot_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
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

// One inlineParser serves every block of a document, so the closing-bracket
// cache must not survive from one block's input into the next: a stale
// offset past the end panics, and one still in bounds takes the wrong label.
// Expected HTML is djot.js 0.3.2's; the labels are asserted directly.
func TestFootnoteReferencesAcrossBlocks(t *testing.T) {
	cases := []struct {
		in, want string
		labels   []string
	}{
		{"[^longlabel]\n\n[^a]", "<p><a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n<p><a id=\"fnref2\" href=\"#fn2\" role=\"doc-noteref\"><sup>2</sup></a></p>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p><a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n<li id=\"fn2\">\n<p><a href=\"#fnref2\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n", []string{"longlabel", "a"}},
		{"[^a]\n\n[^longlabel]", "<p><a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n<p><a id=\"fnref2\" href=\"#fn2\" role=\"doc-noteref\"><sup>2</sup></a></p>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p><a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n<li id=\"fn2\">\n<p><a href=\"#fnref2\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n", []string{"a", "longlabel"}},
		{"[^abc] x\n\n[^de] y\n\n[^longer] z", "<p><a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a> x</p>\n<p><a id=\"fnref2\" href=\"#fn2\" role=\"doc-noteref\"><sup>2</sup></a> y</p>\n<p><a id=\"fnref3\" href=\"#fn3\" role=\"doc-noteref\"><sup>3</sup></a> z</p>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p><a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n<li id=\"fn2\">\n<p><a href=\"#fnref2\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n<li id=\"fn3\">\n<p><a href=\"#fnref3\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n", []string{"abc", "de", "longer"}},
		{"|[^a]|[^bb]|\n", "<table>\n<tr>\n<td><a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></td>\n<td><a id=\"fnref2\" href=\"#fn2\" role=\"doc-noteref\"><sup>2</sup></a></td>\n</tr>\n</table>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p><a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n<li id=\"fn2\">\n<p><a href=\"#fnref2\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n", []string{"a", "bb"}},
	}
	for _, tc := range cases {
		doc := djot.Parse(tc.in)
		if got := djot.RenderHTML(doc); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
		var labels []string
		ast.Preorder(doc.Root(), func(n ast.Node) bool {
			if ref, ok := n.(*ast.FootnoteReference); ok {
				labels = append(labels, ref.Label)
			}
			return true
		})
		if !reflect.DeepEqual(labels, tc.labels) {
			t.Errorf("%q: labels = %q, want %q", tc.in, labels, tc.labels)
		}
	}
}
