package djot

import (
	"io"
	"strings"
	"testing"
)

func TestSymbolRendererMaterializationBoundary(t *testing.T) {
	t.Run("semantic tape remains compact", func(t *testing.T) {
		doc := Parse(":star: :moon:")
		calls := 0
		err := RenderHTMLTo(io.Discard, doc, WithSymbolRenderer(func(symbol SymbolView, r ElementRenderer) {
			calls++
			r.Default()
		}))
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 {
			t.Fatalf("symbol callback count = %d, want 2", calls)
		}
		if doc.root != nil || doc.rootRequested {
			t.Fatal("tape-backed symbol hook materialized the typed AST")
		}
	})

	t.Run("mutated tree remains authoritative", func(t *testing.T) {
		doc := Parse(":star:")
		symbol := findTypedNode[*Symbol](doc.Root())
		symbol.Name = "changed"
		var seen string
		got := RenderHTML(doc, WithSymbolRenderer(func(symbol SymbolView, r ElementRenderer) {
			seen = symbol.Name
			r.Default()
		}))
		if seen != "changed" || got != "<p>:changed:</p>\n" {
			t.Fatalf("tree fallback saw %q and rendered %q", seen, got)
		}
	})

	t.Run("unrelated tree option preserves symbol hook", func(t *testing.T) {
		doc := Parse(":star:")
		got := RenderHTML(doc,
			WithNodeRenderer(KindParagraph, func(_ Node, r NodeRenderer) {
				r.Children()
			}),
			WithSymbolRenderer(func(symbol SymbolView, r ElementRenderer) {
				r.Write("[" + symbol.Name + "]")
			}),
		)
		if got != "[star]" {
			t.Fatalf("mixed tree/tape hooks rendered %q", got)
		}
		if doc.root == nil {
			t.Fatal("Node hook did not select tree execution")
		}
	})
}

func TestSymbolRendererAllocationsDoNotScalePerCallback(t *testing.T) {
	measure := func(symbols int) float64 {
		input := ""
		for i := 0; i < symbols; i++ {
			input += ":star: "
		}
		doc := Parse(input)
		option := WithSymbolRenderer(func(SymbolView, ElementRenderer) {})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	one := measure(1)
	many := measure(1000)
	if one > 10 {
		t.Fatalf("fixed symbol hook allocations = %.0f, want at most 10", one)
	}
	if many > one+1 {
		t.Fatalf("allocations scale with callbacks: one=%.0f many=%.0f", one, many)
	}
}

func BenchmarkSymbolExtensionDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 4096; i++ {
		source.WriteString("text :star: ")
	}
	input := source.String()

	b.Run("RawNodeCrawlAndRender", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			Preorder(doc.Root(), func(node Node) bool {
				if symbol, ok := node.(*Symbol); ok && symbol.Name == "star" {
					symbol.Name = "sparkle"
				}
				return true
			})
			if err := RenderHTMLTo(io.Discard, doc); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TapeSymbolHook", func(b *testing.B) {
		b.ReportAllocs()
		option := WithSymbolRenderer(func(symbol SymbolView, r ElementRenderer) {
			if symbol.Name == "star" {
				r.Write(":sparkle:")
				return
			}
			r.Default()
		})
		for i := 0; i < b.N; i++ {
			doc := Parse(input)
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RenderOnlyRawNodeCrawl", func(b *testing.B) {
		doc := Parse(input)
		doc.Root()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Preorder(doc.Root(), func(node Node) bool {
				if symbol, ok := node.(*Symbol); ok {
					symbol.Name = "sparkle"
				}
				return true
			})
			if err := RenderHTMLTo(io.Discard, doc); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RenderOnlyTapeSymbolHook", func(b *testing.B) {
		doc := Parse(input)
		option := WithSymbolRenderer(func(symbol SymbolView, r ElementRenderer) {
			if symbol.Name == "star" {
				r.Write(":sparkle:")
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

func TestDivRendererMaterializationBoundary(t *testing.T) {
	t.Run("semantic tape remains compact", func(t *testing.T) {
		doc := Parse("::: warning\ntext\n:::\n")
		got := RenderHTML(doc, WithDivRenderer(func(div DivView, r ElementRenderer) {
			if div.Attributes().Get("class") != "warning" {
				t.Fatalf("div class = %q", div.Attributes().Get("class"))
			}
			r.Children()
		}))
		if got != "<p>text</p>\n" {
			t.Fatalf("children output = %q", got)
		}
		if doc.root != nil || doc.rootRequested {
			t.Fatal("tape-backed Div hook materialized the typed AST")
		}
	})

	t.Run("mutated tree remains authoritative", func(t *testing.T) {
		doc := Parse("::: note\ntext\n:::\n")
		div := findTypedNode[*Div](doc.Root())
		div.Attributes().Set("class", "changed")
		var seen string
		RenderHTML(doc, WithDivRenderer(func(div DivView, r ElementRenderer) {
			seen = div.Attributes().Get("class")
			r.Default()
		}))
		if seen != "changed" {
			t.Fatalf("tree fallback saw class %q", seen)
		}
	})

	t.Run("externally constructed tree", func(t *testing.T) {
		doc := NewDoc(&Document{Children: []Block{
			&Div{Children: []Block{
				&Paragraph{Children: []Inline{&Text{Value: "external"}}},
			}},
		}})
		got := RenderHTML(doc, WithDivRenderer(func(_ DivView, r ElementRenderer) {
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		}))
		if got != "<aside><p>external</p>\n</aside>" {
			t.Fatalf("external tree output = %q", got)
		}
	})
}

func TestDivRendererAllocationsDoNotScalePerCallback(t *testing.T) {
	measure := func(divs int) float64 {
		var input strings.Builder
		for i := 0; i < divs; i++ {
			input.WriteString("::: note\ntext\n:::\n")
		}
		doc := Parse(input.String())
		option := WithDivRenderer(func(DivView, ElementRenderer) {})
		return testing.AllocsPerRun(20, func() {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				t.Fatal(err)
			}
		})
	}

	one := measure(1)
	many := measure(1000)
	if one > 10 {
		t.Fatalf("fixed Div hook allocations = %.0f, want at most 10", one)
	}
	if many > one+1 {
		t.Fatalf("allocations scale with Div callbacks: one=%.0f many=%.0f", one, many)
	}
}

func BenchmarkDivExtensionDelta(b *testing.B) {
	var source strings.Builder
	for i := 0; i < 1024; i++ {
		source.WriteString("::: warning\ntext\n:::\n")
	}
	input := source.String()

	b.Run("NodeRenderer", func(b *testing.B) {
		doc := Parse(input)
		option := WithNodeRenderer(KindDiv, func(_ Node, r NodeRenderer) {
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("TapeDivHook", func(b *testing.B) {
		doc := Parse(input)
		option := WithDivRenderer(func(_ DivView, r ElementRenderer) {
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc, option); err != nil {
				b.Fatal(err)
			}
		}
	})
}
