package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func TestInlineSpansAreHalfOpen(t *testing.T) {
	for _, input := range []string{
		"hello", "x", "é", "*bold*", "_emphasis_", "`code`",
		"[link](url)", "![alt](url)", "^sup^", "~sub~", `"quote"`,
		"a!", `a\!`, "a$$", "$`x`", "$$`x`", "{+added+}",
		"[text][ref]", "![alt][ref]", "[text]{.class}", "[^note]",
	} {
		t.Run(input, func(t *testing.T) {
			doc := djot.Parse(input)
			check := func(span ast.SourceSpan) {
				t.Helper()
				if span.Start.Offset != 0 || span.End.Offset != len(input) {
					t.Fatalf("span = [%d:%d], want [0:%d]", span.Start.Offset, span.End.Offset, len(input))
				}
				if got := input[span.Start.Offset:span.End.Offset]; got != input {
					t.Fatalf("source slice = %q, want %q", got, input)
				}
			}
			// Inspect the compact backend before requesting the mutable tree.
			djot.RenderHTML(doc, djot.WithSubtreeRenderer(ast.KindParagraph, func(view djot.SubtreeView, r djot.ElementRenderer) {
				view.Descendants(func(element djot.ElementView) bool { check(element.Span()); return false })
				r.Default()
			}))
			paragraph := doc.Root().Children[0].(*ast.Paragraph)
			check(paragraph.Children[0].Span())
		})
	}
}

func TestInlineSpanSerializationKeepsInclusiveEnds(t *testing.T) {
	doc := djot.Parse("hello")
	if got := djot.RenderAST(doc, true); !strings.Contains(got, "str (1:1:0-1:5:4)") {
		t.Fatalf("unexpected text AST positions:\n%s", got)
	}
	got := compactJSON(t, djot.RenderASTJSON(doc, true))
	if !strings.Contains(got, `"end":{"line":1,"col":5,"offset":4}`) {
		t.Fatalf("unexpected JSON positions: %s", got)
	}
}

func TestReferenceSpanWithDefinition(t *testing.T) {
	const input = "before [text][ref]\n\n[ref]: /url"
	doc := djot.Parse(input)
	link := doc.Root().Children[0].(*ast.Paragraph).Children[1].(*ast.Link)
	span := link.Span()
	if got := input[span.Start.Offset:span.End.Offset]; got != "[text][ref]" {
		t.Fatalf("reference source = %q", got)
	}
	child := link.Children[0].Span()
	if got := input[child.Start.Offset:child.End.Offset]; got != "text" {
		t.Fatalf("reference child source = %q", got)
	}
}
