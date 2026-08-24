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
			if len(headings) != 2 || headings[0].Plaintext() != "First" || headings[1].Plaintext() != "Second" {
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

	t.Run("footnote index remains compact", func(t *testing.T) {
		doc := Parse("reference[^a]\n\n[^a]: note\n")
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			footnotes := document.Footnotes()
			if len(footnotes) != 1 || footnotes[0].Label() != "a" || footnotes[0].Number() != 1 {
				t.Fatalf("footnote index = %#v", footnotes)
			}
			if document.state.footnotes == nil || !document.state.footnotesReady {
				t.Fatal("footnote index was not cached")
			}
		}))
		if doc.root != nil || doc.rootRequested {
			t.Fatal("footnote index materialized the typed AST")
		}
	})

	t.Run("mutated footnote metadata remains authoritative", func(t *testing.T) {
		doc := Parse("reference[^a]\n\n[^a]: note\n")
		footnote := findTypedNode[*Footnote](doc.Root())
		footnote.Label = "changed"
		footnote.Attributes().Set("class", "aside")
		var got []FootnoteView
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			got = document.Footnotes()
		}))
		if len(got) != 2 || got[0].Label() != "a" || got[0].HasDefinition() ||
			got[1].Label() != "changed" || !got[1].HasDefinition() || got[1].Attributes().Get("class") != "aside" {
			t.Fatalf("mutated footnote index = %#v", got)
		}
	})

	t.Run("reference index remains compact", func(t *testing.T) {
		doc := Parse("[label]: /target\n\n[label][]\n")
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			references := document.References()
			if len(references) != 1 || references[0].Label() != "label" || references[0].Destination() != "/target" {
				t.Fatalf("reference index = %#v", references)
			}
			if document.state.references == nil || !document.state.referencesReady {
				t.Fatal("reference index was not cached")
			}
		}))
		if doc.root != nil || doc.rootRequested {
			t.Fatal("reference index materialized the typed AST")
		}
	})

	t.Run("mutated references remain authoritative", func(t *testing.T) {
		doc := Parse("[label]: /target\n\n[label][]\n")
		reference := doc.References()["label"]
		reference.Destination = "/changed"
		reference.Attributes.Set("class", "external")
		var got []ReferenceView
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			got = document.References()
		}))
		if len(got) != 1 || got[0].Destination() != "/changed" || got[0].Attributes().Get("class") != "external" {
			t.Fatalf("mutated reference index = %#v", got)
		}
		if doc.root != nil || doc.rootRequested {
			t.Fatal("reference mutation materialized the typed AST")
		}
	})

	t.Run("externally supplied references", func(t *testing.T) {
		doc := NewDoc(&Document{})
		doc.References()["custom"] = &Reference{Destination: "/custom", DestinationSet: true}
		var got []ReferenceView
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			got = document.References()
		}))
		if len(got) != 1 || got[0].Label() != "custom" || got[0].Destination() != "/custom" {
			t.Fatalf("external reference index = %#v", got)
		}
	})

	t.Run("anchor index remains compact", func(t *testing.T) {
		doc := Parse("{#one}\n# Heading\n")
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			anchors := document.Anchors()
			if len(anchors) != 1 || anchors[0].ID() != "one" || anchors[0].Kind() != KindSection {
				t.Fatalf("anchor index = %#v", anchors)
			}
			if document.state.anchors == nil || !document.state.anchorsReady {
				t.Fatal("anchor index was not cached")
			}
		}))
		if doc.root != nil || doc.rootRequested {
			t.Fatal("anchor index materialized the typed AST")
		}
	})

	t.Run("mutated anchors remain authoritative", func(t *testing.T) {
		doc := Parse("# Heading\n\nparagraph\n")
		section := findTypedNode[*Section](doc.Root())
		section.Attributes().Set("id", "changed")
		paragraph := findTypedNode[*Paragraph](doc.Root())
		paragraph.Attributes().Set("id", "paragraph")
		var got []AnchorView
		RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			got = document.Anchors()
		}))
		if len(got) != 2 || got[0].ID() != "changed" || got[1].ID() != "paragraph" || got[1].Kind() != KindParagraph {
			t.Fatalf("mutated anchor index = %#v", got)
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
		if got.Level() != 3 || got.Plaintext() != "Changed" {
			t.Fatalf("tree-backed heading = level %d text %q", got.Level(), got.Plaintext())
		}
	})

	t.Run("externally constructed tree", func(t *testing.T) {
		doc := NewDoc(&Document{Children: []Block{
			&Heading{Level: 2, Children: []Inline{&Text{Value: "External"}}},
		}})
		got := RenderHTML(doc, WithDocumentRenderer(func(document DocumentView, r DocumentRenderer) {
			headings := document.Headings()
			if len(headings) != 1 || headings[0].Plaintext() != "External" {
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

	measureFootnotes := func(footnotes int, inspect bool) float64 {
		var input strings.Builder
		for i := 0; i < footnotes; i++ {
			input.WriteString("reference[^f")
			input.WriteString(itoa(i))
			input.WriteString("]\n\n[^f")
			input.WriteString(itoa(i))
			input.WriteString("]: note\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if inspect {
				_ = document.Footnotes()
			}
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	oneFootnoteDelta := measureFootnotes(1, true) - measureFootnotes(1, false)
	manyFootnoteDelta := measureFootnotes(1000, true) - measureFootnotes(1000, false)
	const maxIndexAllocationsPerAdditionalFootnote = 4
	if slope := (manyFootnoteDelta - oneFootnoteDelta) / 999; slope > maxIndexAllocationsPerAdditionalFootnote {
		t.Fatalf("footnote index allocations grew by %.2f per footnote, want at most %d", slope, maxIndexAllocationsPerAdditionalFootnote)
	}

	measureReferences := func(references int) float64 {
		var input strings.Builder
		for i := 0; i < references; i++ {
			input.WriteString("[r")
			input.WriteString(itoa(i))
			input.WriteString("]: /target\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			_ = document.References()
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	oneReference := measureReferences(1)
	manyReferences := measureReferences(1000)
	const maxAllocationsPerAdditionalReference = 1
	if slope := (manyReferences - oneReference) / 999; slope > maxAllocationsPerAdditionalReference {
		t.Fatalf("reference index allocations grew by %.2f per reference, want at most %d", slope, maxAllocationsPerAdditionalReference)
	}

	measureAnchors := func(anchors int) float64 {
		var input strings.Builder
		for i := 0; i < anchors; i++ {
			input.WriteString("{#a")
			input.WriteString(itoa(i))
			input.WriteString("}\nparagraph\n\n")
		}
		doc := Parse(input.String())
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			_ = document.Anchors()
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	oneAnchor := measureAnchors(1)
	manyAnchors := measureAnchors(1000)
	const maxAllocationsPerAdditionalAnchor = 1
	if slope := (manyAnchors - oneAnchor) / 999; slope > maxAllocationsPerAdditionalAnchor {
		t.Fatalf("anchor index allocations grew by %.2f per anchor, want at most %d", slope, maxAllocationsPerAdditionalAnchor)
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

func BenchmarkDocumentFootnotesDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 1024; i++ {
		source.WriteString("reference[^f")
		source.WriteString(itoa(i))
		source.WriteString("]\n\n[^f")
		source.WriteString(itoa(i))
		source.WriteString("]: note\n\n")
	}
	input := source.String()

	b.Run("RawNodeCrawl", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			definitions := 0
			Preorder(doc.Root(), func(node Node) bool {
				if node.Kind() == KindFootnote {
					definitions++
				}
				return true
			})
			if definitions != 1024 {
				b.Fatal(definitions)
			}
		}
	})

	b.Run("TapeFootnoteIndex", func(b *testing.B) {
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if footnotes := document.Footnotes(); len(footnotes) != 1024 {
				b.Fatal(len(footnotes))
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

func BenchmarkDocumentReferencesDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 1024; i++ {
		source.WriteString("[r")
		source.WriteString(itoa(i))
		source.WriteString("]: /target\n\n")
	}
	input := source.String()

	b.Run("TypedReferenceIndex", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			if references := doc.References(); len(references) != 1024 {
				b.Fatal(len(references))
			}
		}
	})

	b.Run("CompactReferenceIndex", func(b *testing.B) {
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if references := document.References(); len(references) != 1024 {
				b.Fatal(len(references))
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

func BenchmarkDocumentAnchorsDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 4096; i++ {
		source.WriteString("{#a")
		source.WriteString(itoa(i))
		source.WriteString("}\nparagraph\n\n")
	}
	input := source.String()

	b.Run("RawNodeCrawl", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			anchors := 0
			Preorder(doc.Root(), func(node Node) bool {
				if id, ok := node.Attributes().Lookup("id"); ok && id != "" {
					anchors++
				}
				return true
			})
			if anchors != 4096 {
				b.Fatal(anchors)
			}
		}
	})

	b.Run("TapeAnchorIndex", func(b *testing.B) {
		option := WithDocumentRenderer(func(document DocumentView, _ DocumentRenderer) {
			if anchors := document.Anchors(); len(anchors) != 4096 {
				b.Fatal(len(anchors))
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
