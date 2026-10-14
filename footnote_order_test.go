package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// Footnotes are numbered in the order their references are rendered, as in
// djot.js. A reference that never reaches the output numbers nothing, and a
// footnote with no rendered reference is not listed.

func dropKind(kind ast.Kind) djot.RenderOption {
	return djot.WithNodeRenderer(kind, func(ast.Node, djot.NodeRenderer) {})
}

func TestFootnotesDroppedWithTheirReferences(t *testing.T) {
	doc := djot.Parse("::: x\nhidden[^a]\n:::\n\nshown[^b]\n\n[^a]: A\n\n[^b]: B\n")
	got := djot.RenderHTML(doc, dropKind(ast.KindDiv))
	want := `<p>shown<a id="fnref1" href="#fn1" role="doc-noteref"><sup>1</sup></a></p>
<section role="doc-endnotes">
<hr>
<ol>
<li id="fn1">
<p>B<a href="#fnref1" role="doc-backlink">↩︎</a></p>
</li>
</ol>
</section>
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFootnoteReferenceRenderedByHook(t *testing.T) {
	doc := djot.Parse("x[^a]\n\n[^a]: A\n")
	got := djot.RenderHTML(doc, djot.WithNodeRenderer(ast.KindFootnoteReference,
		func(_ ast.Node, r djot.NodeRenderer) { r.Write("*") }))
	if want := "<p>x*</p>\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFootnoteReferencedOnlyFromDroppedNoteContent(t *testing.T) {
	doc := djot.Parse("x[^a]\n\n[^a]: A\n\n    ::: d\n    see[^b]\n    :::\n\n[^b]: B\n")
	got := djot.RenderHTML(doc, dropKind(ast.KindDiv))
	if strings.Contains(got, `id="fn2"`) || strings.Contains(got, "<p>B") {
		t.Errorf("footnote b should not be listed:\n%s", got)
	}
	if !strings.Contains(got, `<li id="fn1">`) {
		t.Errorf("footnote a should be listed:\n%s", got)
	}
}

func TestFootnoteReferencedOnlyFromNoteIsListed(t *testing.T) {
	doc := djot.Parse("x[^a]\n\n[^a]: A[^b]\n\n[^b]: B\n")
	got := djot.RenderHTML(doc)
	if !strings.Contains(got, `<li id="fn2">`) || !strings.Contains(got, "<p>B") {
		t.Errorf("footnote b should be listed second:\n%s", got)
	}
	assertAnchorIntegrity(t, got, false)
}

func TestMultiBacklinksCountRenderedReferences(t *testing.T) {
	doc := djot.Parse("x[^a]\n\n::: d\ny[^a]\n:::\n\n[^a]: A\n")
	got := djot.RenderHTML(doc, djot.WithMultiBacklinks(), dropKind(ast.KindDiv))
	if strings.Contains(got, "fnref1-2") {
		t.Errorf("the dropped reference should not be counted:\n%s", got)
	}
	if !strings.Contains(got, `<a href="#fnref1" role="doc-backlink">↩︎</a>`) {
		t.Errorf("expected a single backlink:\n%s", got)
	}
	assertAnchorIntegrity(t, got, true)
}

func TestFootnoteReferenceRenderedTwice(t *testing.T) {
	doc := djot.Parse("::: d\nx[^a]\n:::\n\n[^a]: A\n")
	twice := djot.WithNodeRenderer(ast.KindDiv, func(_ ast.Node, r djot.NodeRenderer) {
		r.Default()
		r.Default()
	})
	for _, multi := range []bool{false, true} {
		opts := []djot.RenderOption{twice}
		if multi {
			opts = append(opts, djot.WithMultiBacklinks())
		}
		got := djot.RenderHTML(doc, opts...)
		if strings.Count(got, `<li id="fn1">`) != 1 {
			t.Errorf("multi=%v: footnote should be listed once:\n%s", multi, got)
		}
		assertAnchorIntegrity(t, got, multi)
	}
}
