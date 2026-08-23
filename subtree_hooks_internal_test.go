package djot

import (
	"io"
	"strings"
	"testing"
)

func TestSubtreeRendererMaterializationBoundary(t *testing.T) {
	t.Run("semantic tape remains compact", func(t *testing.T) {
		doc := Parse("::: example\n```\ncode\n```\n:::\n")
		calls := 0
		err := RenderHTMLTo(io.Discard, doc, WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, r ElementRenderer) {
			calls++
			if !subtree.Contains(KindCodeBlock) {
				t.Fatal("subtree missed code block")
			}
			r.Default()
		}))
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("subtree callback count = %d, want 1", calls)
		}
		if doc.root != nil || doc.rootRequested {
			t.Fatal("subtree inspection materialized the typed AST")
		}
	})

	t.Run("mutated tree remains authoritative", func(t *testing.T) {
		doc := Parse("::: example\ntext\n:::\n")
		div := findTypedNode[*Div](doc.Root())
		div.Children = append(div.Children, &CodeBlock{Text: "added"})
		containsCode := false
		RenderHTML(doc, WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, r ElementRenderer) {
			containsCode = subtree.Contains(KindCodeBlock)
			r.Default()
		}))
		if !containsCode {
			t.Fatal("tree fallback did not expose added code block")
		}
	})

	t.Run("externally constructed tree", func(t *testing.T) {
		doc := NewDoc(&Document{Children: []Block{
			&Div{Children: []Block{&CodeBlock{Text: "external"}}},
		}})
		got := RenderHTML(doc, WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, r ElementRenderer) {
			if !subtree.Contains(KindCodeBlock) {
				t.Fatal("external subtree missed code block")
			}
			r.Children()
		}))
		if got != "<pre><code>external</code></pre>\n" {
			t.Fatalf("external subtree output = %q", got)
		}
	})
}

func TestSubtreeRendererAllocationsAreFixed(t *testing.T) {
	measureCallbacks := func(divs int) float64 {
		var input strings.Builder
		for i := 0; i < divs; i++ {
			input.WriteString("::: example\n```\ncode\n```\n:::\n")
		}
		doc := Parse(input.String())
		option := WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, _ ElementRenderer) {
			if !subtree.Contains(KindCodeBlock) {
				panic("missing code block")
			}
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	one := measureCallbacks(1)
	many := measureCallbacks(1000)
	if one > 12 {
		t.Fatalf("fixed subtree hook allocations = %.0f, want at most 12", one)
	}
	if many > one+1 {
		t.Fatalf("allocations scale with subtree callbacks: one=%.0f many=%.0f", one, many)
	}

	measureScan := func(paragraphs int) float64 {
		var input strings.Builder
		input.WriteString("::: prose\n")
		for i := 0; i < paragraphs; i++ {
			input.WriteString("paragraph\n\n")
		}
		input.WriteString(":::\n")
		doc := Parse(input.String())
		option := WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, _ ElementRenderer) {
			if subtree.Contains(KindCodeBlock) {
				panic("unexpected code block")
			}
		})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	short := measureScan(1)
	long := measureScan(1000)
	if long > short+1 {
		t.Fatalf("allocations scale with scanned records: short=%.0f long=%.0f", short, long)
	}
}

func BenchmarkSubtreeExtensionDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 1024; i++ {
		source.WriteString("::: example\n```\ncode\n```\n:::\n")
	}
	input := source.String()

	b.Run("NodeCrawl", func(b *testing.B) {
		doc := Parse(input)
		doc.Root()
		option := WithNodeRenderer(KindDiv, func(node Node, r NodeRenderer) {
			found := false
			first := true
			Preorder(node, func(descendant Node) bool {
				if first {
					first = false
					return true
				}
				found = descendant.Kind() == KindCodeBlock
				return !found
			})
			if found {
				r.Children()
				return
			}
			r.Default()
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TapeSubtree", func(b *testing.B) {
		doc := Parse(input)
		option := WithSubtreeRenderer(KindDiv, func(subtree SubtreeView, r ElementRenderer) {
			if subtree.Contains(KindCodeBlock) {
				r.Children()
				return
			}
			r.Default()
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})
}
