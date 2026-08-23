package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestSymbolRenderer(t *testing.T) {
	t.Run("replace and default", func(t *testing.T) {
		doc := djot.Parse("A :star: and a :moon:.")
		got := djot.RenderHTML(doc, djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
			if symbol.Name == "star" {
				r.Write("⭐")
				return
			}
			r.Default()
		}))
		want := "<p>A ⭐ and a :moon:.</p>\n"
		if got != want {
			t.Fatalf("symbol rendering differs\nwant: %q\n got: %q", want, got)
		}
	})

	t.Run("last symbol hook wins", func(t *testing.T) {
		doc := djot.Parse(":star:")
		got := djot.RenderHTML(doc,
			djot.WithSymbolRenderer(func(djot.SymbolView, djot.ElementRenderer) {
				t.Fatal("replaced hook ran")
			}),
			djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
				r.Write("[" + symbol.Name + "]")
			}),
		)
		if got != "<p>[star]</p>\n" {
			t.Fatalf("last hook output = %q", got)
		}
	})

	t.Run("last hook wins across execution models", func(t *testing.T) {
		doc := djot.Parse(":star:")
		tapeLast := djot.RenderHTML(doc,
			djot.WithNodeRenderer(djot.KindSymbol, func(djot.Node, djot.NodeRenderer) {
				t.Fatal("replaced Node hook ran")
			}),
			djot.WithSymbolRenderer(func(djot.SymbolView, djot.ElementRenderer) {}),
		)
		if tapeLast != "<p></p>\n" {
			t.Fatalf("tape hook output = %q", tapeLast)
		}

		nodeLast := djot.RenderHTML(doc,
			djot.WithSymbolRenderer(func(djot.SymbolView, djot.ElementRenderer) {
				t.Fatal("replaced tape hook ran")
			}),
			djot.WithNodeRenderer(djot.KindSymbol, func(n djot.Node, r djot.NodeRenderer) {
				r.Write("[" + n.(*djot.Symbol).Name + "]")
			}),
		)
		if nodeLast != "<p>[star]</p>\n" {
			t.Fatalf("Node hook output = %q", nodeLast)
		}
	})

	t.Run("writer error stops later callbacks", func(t *testing.T) {
		doc := djot.Parse(":one: :two:")
		writer := &errWriter{limit: 3}
		calls := 0
		err := djot.RenderHTMLTo(writer, doc, djot.WithSymbolRenderer(func(_ djot.SymbolView, r djot.ElementRenderer) {
			calls++
			r.Write("replacement")
		}))
		if err == nil {
			t.Fatal("expected writer error")
		}
		if calls != 1 {
			t.Fatalf("callbacks after writer error = %d, want 1 total callback", calls)
		}
	})
}

func TestSymbolRendererMatchesRawNodeCrawl(t *testing.T) {
	const input = "A :star:, a :moon:, and another :star:."

	nodeDoc := djot.Parse(input)
	djot.Preorder(nodeDoc.Root(), func(node djot.Node) bool {
		if symbol, ok := node.(*djot.Symbol); ok && symbol.Name == "star" {
			symbol.Name = "sparkle"
		}
		return true
	})
	want := djot.RenderHTML(nodeDoc)

	tapeDoc := djot.Parse(input)
	got := djot.RenderHTML(tapeDoc, djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
		if symbol.Name == "star" {
			r.Write(":sparkle:")
			return
		}
		r.Default()
	}))

	if got != want {
		t.Fatalf("tape hook differs from raw Node crawl\nwant: %q\n got: %q", want, got)
	}
	if strings.Count(got, ":sparkle:") != 2 || strings.Contains(got, ":star:") {
		t.Fatalf("replacement policy did not run as expected: %q", got)
	}
}

func FuzzSymbolRendererMatchesRawNodeCrawl(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		":star:",
		"A :star:, a :moon:, and another :star:.",
		"# :heading:\n\n[^note]: :footnote:\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		nodeDoc := djot.Parse(input)
		djot.Preorder(nodeDoc.Root(), func(node djot.Node) bool {
			if symbol, ok := node.(*djot.Symbol); ok {
				symbol.Name = "x_" + symbol.Name
			}
			return true
		})
		want := djot.RenderHTML(nodeDoc)

		tapeDoc := djot.Parse(input)
		got := djot.RenderHTML(tapeDoc, djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
			r.Write(":x_" + symbol.Name + ":")
		}))
		if got != want {
			t.Fatalf("tape hook differs from raw Node crawl\ninput: %q\nwant: %q\n got: %q", input, want, got)
		}
	})
}

func TestDivRenderer(t *testing.T) {
	t.Run("replace wrapper and stream hooked children", func(t *testing.T) {
		doc := djot.Parse("::: warning\n:star:\n:::\n")
		got := djot.RenderHTML(doc,
			djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
				if div.Attributes().Get("class") != "warning" {
					r.Default()
					return
				}
				r.Write(`<aside class="warning">`)
				r.Children()
				r.Write("</aside>\n")
			}),
			djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
				r.Write("[" + symbol.Name + "]")
			}),
		)
		want := "<aside class=\"warning\"><p>[star]</p>\n</aside>\n"
		if got != want {
			t.Fatalf("div rendering differs\nwant: %q\n got: %q", want, got)
		}
	})

	t.Run("default and repeated children", func(t *testing.T) {
		doc := djot.Parse("::: note\ntext\n:::\n")
		defaultHTML := djot.RenderHTML(doc, djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) {
			r.Default()
		}))
		if defaultHTML != djot.RenderHTML(doc) {
			t.Fatalf("Default differs\nwant: %q\n got: %q", djot.RenderHTML(doc), defaultHTML)
		}

		repeated := djot.RenderHTML(doc, djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) {
			r.Children()
			r.Children()
		}))
		if repeated != "<p>text</p>\n<p>text</p>\n" {
			t.Fatalf("repeated Children output = %q", repeated)
		}
	})

	t.Run("attribute iteration and span", func(t *testing.T) {
		doc := djot.Parse("{#alert .extra key=value}\n::: note\ntext\n:::\n")
		var wantSpan djot.SourceSpan
		var wantAttributes []djot.Attribute
		djot.Preorder(doc.Root(), func(node djot.Node) bool {
			if div, ok := node.(*djot.Div); ok {
				wantSpan = div.Span()
				wantAttributes = div.Attributes().Entries()
				return false
			}
			return true
		})
		var attributes []djot.Attribute
		var span djot.SourceSpan
		djot.RenderHTML(doc, djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
			span = div.Span()
			if div.Attributes().Len() != len(wantAttributes) {
				t.Fatalf("attribute count = %d, want %d", div.Attributes().Len(), len(wantAttributes))
			}
			if value, ok := div.Attributes().Lookup("key"); !ok || value != "value" {
				t.Fatalf("key lookup = %q, %v", value, ok)
			}
			div.Attributes().Range(func(attribute djot.Attribute) bool {
				attributes = append(attributes, attribute)
				return true
			})
			r.Default()
		}))
		if span != wantSpan {
			t.Fatalf("Div view span = %+v, Node span = %+v", span, wantSpan)
		}
		if len(attributes) != len(wantAttributes) {
			t.Fatalf("attribute order = %#v, want %#v", attributes, wantAttributes)
		}
		for i := range attributes {
			if attributes[i] != wantAttributes[i] {
				t.Fatalf("attribute %d = %#v, want %#v", i, attributes[i], wantAttributes[i])
			}
		}
	})
}

func FuzzDivRendererMatchesNodeRenderer(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"::: note\ntext\n:::\n",
		"::: outer\n::: inner\n:star:\n:::\n:::\n",
		"{.a #id key=value}\n::: list\n- one\n- two\n:::\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		nodeDoc := djot.Parse(input)
		want := djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(djot.KindDiv, func(_ djot.Node, r djot.NodeRenderer) {
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		}))

		tapeDoc := djot.Parse(input)
		got := djot.RenderHTML(tapeDoc, djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) {
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		}))
		if got != want {
			t.Fatalf("tape Div hook differs from Node renderer\ninput: %q\nwant: %q\n got: %q", input, want, got)
		}
	})
}
