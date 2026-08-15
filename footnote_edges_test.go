package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestFootnoteEdges covers footnote-reference behavior that the official corpus
// in testdata/official leaves untested and that djot-go once got wrong.
// Expectations come from the reference implementation (jgm/djot.js) at f0191eb,
// captured by running its renderer over the same input.
func TestFootnoteEdges(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			// A bare "[^]" is a valid reference with an empty label. Its own
			// "[^]:" line is a reference definition labelled "^", not a
			// footnote definition, so nothing binds and the note stays empty.
			name:  "empty footnote label",
			input: "[^] x\n\n[^]: note",
			want: `<p><a id="fnref1" href="#fn1" role="doc-noteref"><sup>1</sup></a> x</p>
<section role="doc-endnotes">
<hr>
<ol>
<li id="fn1">
<p><a href="#fnref1" role="doc-backlink">↩︎</a></p>
</li>
</ol>
</section>
`,
		},
		{
			// A "!" before a footnote reference is just text: "![" opens an
			// image only when a footnote reference doesn't follow.
			name:  "bang before footnote reference",
			input: "![^a] x\n\n[^a]: note",
			want: `<p>!<a id="fnref1" href="#fn1" role="doc-noteref"><sup>1</sup></a> x</p>
<section role="doc-endnotes">
<hr>
<ol>
<li id="fn1">
<p>note<a href="#fnref1" role="doc-backlink">↩︎</a></p>
</li>
</ol>
</section>
`,
		},
		{
			// A label containing "[" could never match a definition, whose
			// label ends at its first "]", so the run stays literal rather
			// than becoming a reference nothing can resolve.
			name:  "brackets inside footnote label",
			input: "[^a[b]c]",
			want:  "<p>[^a[b]c]</p>\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := djot.RenderHTML(djot.Parse(tt.input))
			if got != tt.want {
				t.Errorf("input:\n%s\n\nwant:\n%s\ngot:\n%s", tt.input, tt.want, got)
			}
		})
	}
}
