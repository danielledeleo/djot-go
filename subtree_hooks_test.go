package djot_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func nodeContainsDescendant(root djot.Node, kind djot.Kind) bool {
	first := true
	found := false
	djot.Preorder(root, func(node djot.Node) bool {
		if first {
			first = false
			return true
		}
		found = node.Kind() == kind
		return !found
	})
	return found
}

func TestSubtreeRendererInspectsBeforeWriting(t *testing.T) {
	input := "::: prose\nplain\n:::\n\n::: example\n``` go\ncode\n```\n:::\n"
	nodeDoc := djot.Parse(input)
	want := djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(djot.KindDiv, func(node djot.Node, r djot.NodeRenderer) {
		if nodeContainsDescendant(node, djot.KindCodeBlock) {
			r.Write(`<section class="contains-code">`)
			r.Children()
			r.Write("</section>\n")
			return
		}
		r.Default()
	}))

	tapeDoc := djot.Parse(input)
	got := djot.RenderHTML(tapeDoc, djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
		if subtree.Contains(djot.KindCodeBlock) {
			r.Write(`<section class="contains-code">`)
			r.Children()
			r.Write("</section>\n")
			return
		}
		r.Default()
	}))
	if got != want {
		t.Fatalf("subtree renderer differs from Node crawl\nwant: %q\n got: %q", want, got)
	}
}

func TestSubtreeViewTraversalMatchesNodePreorder(t *testing.T) {
	type snapshot struct {
		kind       djot.Kind
		span       djot.SourceSpan
		attributes []djot.Attribute
		symbol     string
	}
	fromNode := func(node djot.Node) snapshot {
		result := snapshot{kind: node.Kind(), span: node.Span(), attributes: node.Attributes().Entries()}
		if symbol, ok := node.(*djot.Symbol); ok {
			result.symbol = symbol.Name
		}
		return result
	}
	fromView := func(element djot.ElementView) snapshot {
		result := snapshot{kind: element.Kind(), span: element.Span()}
		attributes := element.Attributes()
		result.attributes = make([]djot.Attribute, 0, attributes.Len())
		attributes.Range(func(attribute djot.Attribute) bool {
			result.attributes = append(result.attributes, attribute)
			return true
		})
		if symbol, ok := element.Symbol(); ok {
			result.symbol = symbol.Name
		}
		return result
	}
	equal := func(left, right snapshot) bool {
		if left.kind != right.kind || left.span != right.span || left.symbol != right.symbol || len(left.attributes) != len(right.attributes) {
			return false
		}
		for i := range left.attributes {
			if left.attributes[i] != right.attributes[i] {
				return false
			}
		}
		return true
	}

	input := "{#outer key=value}\n::: note\nText :star:.\n\n- one\n- *two*\n:::\n"
	var want []snapshot
	nodeDoc := djot.Parse(input)
	djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(djot.KindDiv, func(node djot.Node, r djot.NodeRenderer) {
		djot.Preorder(node, func(descendant djot.Node) bool {
			want = append(want, fromNode(descendant))
			return true
		})
		r.Default()
	}))

	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			var got []snapshot
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
				subtree.Preorder(func(element djot.ElementView) bool {
					got = append(got, fromView(element))
					return true
				})
				r.Default()
			}))
			if len(got) != len(want) {
				t.Fatalf("preorder length = %d, want %d", len(got), len(want))
			}
			for i := range got {
				if !equal(got[i], want[i]) {
					t.Fatalf("preorder element %d differs\nwant: %#v\n got: %#v", i, want[i], got[i])
				}
			}
		})
	}
	if len(want) == 0 {
		t.Fatal("Node oracle visited no elements")
	}
	for _, element := range want {
		if element.kind == djot.KindSymbol && element.symbol != "star" {
			t.Fatalf("symbol payload = %q, want star", element.symbol)
		}
	}
}

func TestSubtreeViewTraversalBoundaries(t *testing.T) {
	doc := djot.Parse(":::: outer\n::: inner\n:star:\n:::\n::::\n")
	outerCalls := 0
	djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
		outerCalls++
		root := subtree.Root()
		if root.Kind() != djot.KindDiv {
			t.Fatalf("root kind = %v", root.Kind())
		}
		preorder, descendants := 0, 0
		subtree.Preorder(func(djot.ElementView) bool {
			preorder++
			return preorder < 2
		})
		subtree.Descendants(func(djot.ElementView) bool {
			descendants++
			return descendants < 1
		})
		if preorder != 2 || descendants != 1 {
			t.Fatalf("early traversal counts = preorder %d, descendants %d", preorder, descendants)
		}
		if subtree.Contains(djot.KindDocument) {
			t.Fatal("bounded subtree leaked its document ancestor")
		}
		if !subtree.Contains(djot.KindSymbol) {
			t.Fatal("bounded subtree missed symbol descendant")
		}
		r.Default()
	}))
	if outerCalls != 2 {
		t.Fatalf("nested Div callback count = %d, want 2", outerCalls)
	}
}

func TestSubtreeRendererWorksForNonDivKinds(t *testing.T) {
	input := "- plain\n- item with *emphasis*\n"
	nodeDoc := djot.Parse(input)
	want := djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(djot.KindListItem, func(node djot.Node, r djot.NodeRenderer) {
		if nodeContainsDescendant(node, djot.KindEmphasis) {
			r.Write("<li class=\"emphasis\">")
			r.Children()
			r.Write("</li>\n")
			return
		}
		r.Default()
	}))

	tapeDoc := djot.Parse(input)
	got := djot.RenderHTML(tapeDoc, djot.WithSubtreeRenderer(djot.KindListItem, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
		if subtree.Contains(djot.KindEmphasis) {
			r.Write("<li class=\"emphasis\">")
			r.Children()
			r.Write("</li>\n")
			return
		}
		r.Default()
	}))
	if got != want {
		t.Fatalf("list-item subtree renderer differs\nwant: %q\n got: %q", want, got)
	}
}

func TestSubtreeRendererCompositionAndPrecedence(t *testing.T) {
	doc := djot.Parse("::: note\n:star:\n:::\n")
	got := djot.RenderHTML(doc,
		djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
			if !subtree.Contains(djot.KindSymbol) {
				t.Fatal("Div subtree did not see symbol")
			}
			r.Write("<aside>")
			r.Children()
			r.Write("</aside>")
		}),
		djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
			r.Write("[" + symbol.Name + "]")
		}),
	)
	if got != "<aside><p>[star]</p>\n</aside>" {
		t.Fatalf("composed output = %q", got)
	}

	last := djot.RenderHTML(doc,
		djot.WithNodeRenderer(djot.KindDiv, func(djot.Node, djot.NodeRenderer) {
			t.Fatal("replaced Node hook ran")
		}),
		djot.WithSubtreeRenderer(djot.KindDiv, func(djot.SubtreeView, djot.ElementRenderer) {
			t.Fatal("replaced subtree hook ran")
		}),
		djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) { r.Children() }),
	)
	if last != "<p>:star:</p>\n" {
		t.Fatalf("last specialized hook output = %q", last)
	}

	subtreeLast := djot.RenderHTML(doc,
		djot.WithDivRenderer(func(djot.DivView, djot.ElementRenderer) {
			t.Fatal("replaced Div hook ran")
		}),
		djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
			r.Write("<subtree>")
			r.Children()
			r.Write("</subtree>")
		}),
	)
	if subtreeLast != "<subtree><p>:star:</p>\n</subtree>" {
		t.Fatalf("last subtree hook output = %q", subtreeLast)
	}

	nodeLast := djot.RenderHTML(doc,
		djot.WithSubtreeRenderer(djot.KindDiv, func(djot.SubtreeView, djot.ElementRenderer) {
			t.Fatal("replaced subtree hook ran")
		}),
		djot.WithNodeRenderer(djot.KindDiv, func(_ djot.Node, r djot.NodeRenderer) {
			r.Children()
		}),
	)
	if nodeLast != "<p>:star:</p>\n" {
		t.Fatalf("last Node hook output = %q", nodeLast)
	}

	symbolSubtreeLast := djot.RenderHTML(doc,
		djot.WithSymbolRenderer(func(djot.SymbolView, djot.ElementRenderer) {
			t.Fatal("replaced symbol hook ran")
		}),
		djot.WithSubtreeRenderer(djot.KindSymbol, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
			if symbol, ok := subtree.Root().Symbol(); !ok || symbol.Name != "star" {
				t.Fatalf("symbol subtree root = %#v, %v", symbol, ok)
			}
			r.Write("[subtree-symbol]")
		}),
	)
	if symbolSubtreeLast != "<div class=\"note\">\n<p>[subtree-symbol]</p>\n</div>\n" {
		t.Fatalf("symbol subtree output = %q", symbolSubtreeLast)
	}
}

func TestSubtreeRendererDefaultReplayAndSuppression(t *testing.T) {
	doc := djot.Parse("::: note\ntext\n:::\n")
	defaultHTML := djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
		r.Default()
	}))
	if defaultHTML != djot.RenderHTML(doc) {
		t.Fatalf("Default differs\nwant: %q\n got: %q", djot.RenderHTML(doc), defaultHTML)
	}

	replayed := djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
		r.Children()
		r.Children()
	}))
	if replayed != "<p>text</p>\n<p>text</p>\n" {
		t.Fatalf("replayed children output = %q", replayed)
	}

	suppressed := djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(djot.SubtreeView, djot.ElementRenderer) {}))
	if suppressed != "" {
		t.Fatalf("suppressed subtree output = %q", suppressed)
	}
}

func TestSubtreeRendererConcurrentRendering(t *testing.T) {
	doc := djot.Parse("::: note\n```\ncode\n```\n:::\n")
	option := djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
		if subtree.Contains(djot.KindCodeBlock) {
			r.Write("<code-example>")
			r.Children()
			r.Write("</code-example>")
			return
		}
		r.Default()
	})
	want := djot.RenderHTML(doc, option)

	const workers = 32
	var wait sync.WaitGroup
	errors := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if got := djot.RenderHTML(doc, option); got != want {
				errors <- got
			}
		}()
	}
	wait.Wait()
	close(errors)
	for got := range errors {
		t.Fatalf("concurrent output differs\nwant: %q\n got: %q", want, got)
	}
}

func TestSubtreeRendererHTMLToParity(t *testing.T) {
	doc := djot.Parse("::: note\ntext\n:::\n")
	option := djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
		r.Children()
	})
	want := djot.RenderHTML(doc, option)
	var output strings.Builder
	if err := djot.RenderHTMLTo(&output, doc, option); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != want {
		t.Fatalf("RenderHTMLTo differs\nwant: %q\n got: %q", want, got)
	}
}

func TestSubtreeRendererWriterErrorStopsCallbacks(t *testing.T) {
	const input = ":::: outer\n::: inner\ntext\n:::\n::::\n"
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			writer := &errWriter{limit: 0}
			calls := 0
			err := djot.RenderHTMLTo(writer, doc, djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
				calls++
				r.Write("<aside>")
				r.Children()
			}))
			if err == nil {
				t.Fatal("expected writer error")
			}
			if calls != 1 {
				t.Fatalf("callbacks after writer error = %d, want 1 total callback", calls)
			}
		})
	}
}

func TestSubtreeRendererPanicPropagates(t *testing.T) {
	const marker = "subtree hook panic"
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != marker {
					t.Fatalf("recovered %v, want %q", recovered, marker)
				}
			}()
			doc := djot.Parse("::: note\ntext\n:::\n")
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			djot.RenderHTML(doc, djot.WithSubtreeRenderer(djot.KindDiv, func(djot.SubtreeView, djot.ElementRenderer) {
				panic(marker)
			}))
		})
	}
}

func FuzzSubtreeRendererMatchesNodeCrawl(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"::: note\ntext\n:::\n",
		":::: outer\n::: inner\n```\ncode\n```\n:::\n::::\n",
		"::: list\n- one\n- *two*\n:::\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		nodeDoc := djot.Parse(input)
		want := djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(djot.KindDiv, func(node djot.Node, r djot.NodeRenderer) {
			if nodeContainsDescendant(node, djot.KindCodeBlock) {
				r.Write("<code-div>")
				r.Children()
				r.Write("</code-div>")
				return
			}
			r.Default()
		}))

		tapeDoc := djot.Parse(input)
		got := djot.RenderHTML(tapeDoc, djot.WithSubtreeRenderer(djot.KindDiv, func(subtree djot.SubtreeView, r djot.ElementRenderer) {
			if subtree.Contains(djot.KindCodeBlock) {
				r.Write("<code-div>")
				r.Children()
				r.Write("</code-div>")
				return
			}
			r.Default()
		}))
		if got != want {
			t.Fatalf("subtree hook differs from Node crawl\ninput: %q\nwant: %q\n got: %q", input, want, got)
		}
	})
}
