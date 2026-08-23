package djot

import (
	"io"
	"strings"
	"testing"
)

func TestDocumentViewMaterializationBoundary(t *testing.T) {
	t.Run("semantic tape remains compact", func(t *testing.T) {
		doc := Parse("# First\n\n## Second\n")
		err := RenderHTMLTo(io.Discard, doc, WithDocumentRenderer(func(document DocumentView, r DocumentRenderer) {
			headings := document.Headings()
			if len(headings) != 2 || headings[0].Text() != "First" || headings[1].Text() != "Second" {
				t.Fatalf("heading index = %#v", headings)
			}
			r.Default()
		}))
		if err != nil {
			t.Fatal(err)
		}
		if doc.root != nil || doc.rootRequested {
			t.Fatal("document inspection materialized the typed AST")
		}
	})

	t.Run("kind summary remains compact", func(t *testing.T) {
		doc := Parse("# Heading\n\nparagraph\n")
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if document.Count(KindHeading) != 1 || !document.Contains(KindParagraph) {
				t.Fatal("kind summary did not inspect the semantic tape")
			}
			counts := document.state.kindCounts
			_ = document.Count(KindText)
			if document.state.kindCounts != counts {
				t.Fatal("kind summary was rebuilt within one callback")
			}
		}))
		if doc.root != nil || doc.rootRequested {
			t.Fatal("kind summary materialized the typed AST")
		}
	})

	t.Run("mutated tree remains authoritative", func(t *testing.T) {
		doc := Parse("# Original\n")
		heading := findTypedNode[*Heading](doc.Root())
		heading.Level = 3
		heading.Children = []Inline{&Text{Value: "Changed"}}
		var got HeadingView
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, r DocumentRenderer) {
			got = document.Headings()[0]
			r.Default()
		}))
		if got.Level() != 3 || got.Text() != "Changed" {
			t.Fatalf("tree-backed heading = level %d text %q", got.Level(), got.Text())
		}
	})

	t.Run("externally constructed tree", func(t *testing.T) {
		doc := NewDoc(&Document{Children: []Block{
			&Heading{Level: 2, Children: []Inline{&Text{Value: "External"}}},
		}})
		got := RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, r DocumentRenderer) {
			headings := document.Headings()
			if len(headings) != 1 || headings[0].Text() != "External" {
				t.Fatalf("external heading index = %#v", headings)
			}
			r.Default()
		}))
		if got != "<h2>External</h2>\n" {
			t.Fatalf("external document output = %q", got)
		}
	})

	t.Run("heading anchor survives an unlabelled section", func(t *testing.T) {
		heading := &Heading{Level: 2, Children: []Inline{&Text{Value: "External"}}}
		heading.Attributes().Set("id", "external-heading")
		doc := NewDoc(&Document{Children: []Block{
			&Section{Children: []Block{heading}},
		}})
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			headings := document.Headings()
			if len(headings) != 1 || headings[0].ID() != "external-heading" {
				t.Fatalf("external heading anchor = %#v", headings)
			}
		}))
	})
}

func TestDocumentViewAllocations(t *testing.T) {
	measureHook := func(paragraphs int) float64 {
		var input strings.Builder
		for i := 0; i < paragraphs; i++ {
			input.WriteString("paragraph\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(DocumentView, DocumentRenderer) {})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	short := measureHook(1)
	long := measureHook(1000)
	if short > 12 {
		t.Fatalf("fixed document hook allocations = %.0f, want at most 12", short)
	}
	if long > short+1 {
		t.Fatalf("document hook allocations scale with records: short=%.0f long=%.0f", short, long)
	}

	measureKindSummary := func(paragraphs int) float64 {
		var input strings.Builder
		for i := 0; i < paragraphs; i++ {
			input.WriteString("paragraph\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			_ = document.Count(KindParagraph)
			_ = document.Contains(KindText)
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	shortSummary := measureKindSummary(1)
	longSummary := measureKindSummary(1000)
	if longSummary > shortSummary+1 {
		t.Fatalf("kind summary allocations scale with records: short=%.0f long=%.0f", shortSummary, longSummary)
	}

	measureHeadings := func(headings int) float64 {
		var input strings.Builder
		for i := 0; i < headings; i++ {
			input.WriteString("# Heading\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			_ = document.Headings()
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	one := measureHeadings(1)
	many := measureHeadings(1000)
	const maxAllocationsPerAdditionalHeading = 8
	if slope := (many - one) / 999; slope > maxAllocationsPerAdditionalHeading {
		t.Fatalf("heading index allocations grew by %.2f per heading, want at most %d (one=%.0f many=%.0f)",
			slope, maxAllocationsPerAdditionalHeading, one, many)
	}
}

func BenchmarkDocumentKindCountsDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 4096; i++ {
		source.WriteString("paragraph :star:\n\n")
	}
	input := source.String()

	b.Run("RawNodeCrawl", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			count := 0
			Preorder(doc.Root(), func(node Node) bool {
				if node.Kind() == KindSymbol {
					count++
				}
				return true
			})
			if count != 4096 {
				b.Fatal(count)
			}
		}
	})

	b.Run("TapeKindIndex", func(b *testing.B) {
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if count := document.Count(KindSymbol); count != 4096 {
				b.Fatal(count)
			}
		})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkDocumentViewDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 1024; i++ {
		source.WriteString("# Heading :star:\n\nparagraph\n\n")
	}
	input := source.String()

	b.Run("RawNodeCrawlAndRender", func(b *testing.B) {
		forceTree := WithNodeRenderer(KindHeading, func(_ Node, r NodeRenderer) { r.Default() })
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			var headings []*Heading
			Preorder(doc.Root(), func(node Node) bool {
				if heading, ok := node.(*Heading); ok {
					headings = append(headings, heading)
				}
				return true
			})
			if len(headings) != 1024 {
				b.Fatal(len(headings))
			}
			if err := RenderHTMLTo(io.Discard, doc, forceTree); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TapeDocumentView", func(b *testing.B) {
		option := WithDocumentRenderer(func(document DocumentView, r DocumentRenderer) {
			if len(document.Headings()) != 1024 {
				b.Fatal(len(document.Headings()))
			}
			r.Default()
		})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})
}
