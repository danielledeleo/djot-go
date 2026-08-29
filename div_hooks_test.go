package djot_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func TestDivRendererCorpusParity(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"empty", "::: empty\n:::\n"},
		{"paragraphs", "::: prose\nfirst\n\nsecond with *emphasis*\n:::\n"},
		{"nested divs", ":::: outer\n::: inner\ntext\n:::\n::::\n"},
		{"block quote", "::: quote\n> quoted\n> text\n:::\n"},
		{"tight list", "::: list\n- one\n- two\n:::\n"},
		{"loose list", "::: list\n- one\n\n- two\n:::\n"},
		{"ordered list", "::: list\n3. three\n4. four\n:::\n"},
		{"task list", "::: list\n- [x] done\n- [ ] later\n:::\n"},
		{"table", "::: table\n| a | b |\n|---|:--:|\n| c | d |\n:::\n"},
		{"code and raw", "::: code\n``` go\nfmt.Println()\n```\n\n`<b>raw</b>`{=html}\n:::\n"},
		{"heading and section", "::: section\n## Heading\n\nbody\n:::\n"},
		{"footnote", "::: notes\nreference[^a]\n\n[^a]: note\n:::\n"},
		{"attributes", "{.extra #alert key=\"<&>\" empty=\"\"}\n::: note\ntext\n:::\n"},
	}

	type action struct {
		name string
		node djot.NodeRenderFunc
		div  djot.DivRenderFunc
	}
	actions := []action{
		{
			name: "default",
			node: func(_ ast.Node, r djot.NodeRenderer) { r.Default() },
			div:  func(_ djot.DivView, r djot.ElementRenderer) { r.Default() },
		},
		{
			name: "stream children",
			node: func(_ ast.Node, r djot.NodeRenderer) { r.Children() },
			div:  func(_ djot.DivView, r djot.ElementRenderer) { r.Children() },
		},
		{
			name: "replace wrapper",
			node: func(node ast.Node, r djot.NodeRenderer) {
				r.Write(`<aside data-class="` + node.Attributes().Get("class") + `">`)
				r.Children()
				r.Write("</aside>")
			},
			div: func(div djot.DivView, r djot.ElementRenderer) {
				r.Write(`<aside data-class="` + div.Attributes().Get("class") + `">`)
				r.Children()
				r.Write("</aside>")
			},
		},
		{
			name: "suppress",
			node: func(ast.Node, djot.NodeRenderer) {},
			div:  func(djot.DivView, djot.ElementRenderer) {},
		},
	}

	for _, input := range inputs {
		for _, action := range actions {
			t.Run(input.name+"/"+action.name, func(t *testing.T) {
				nodeDoc := djot.Parse(input.input)
				want := djot.RenderHTML(nodeDoc, djot.WithNodeRenderer(ast.KindDiv, action.node))

				tapeDoc := djot.Parse(input.input)
				got := djot.RenderHTML(tapeDoc, djot.WithDivRenderer(action.div))
				if got != want {
					t.Fatalf("Div hook differs from Node hook\ninput: %q\nwant: %q\n got: %q", input.input, want, got)
				}
			})
		}
	}
}

func TestDivRendererDeepNesting(t *testing.T) {
	const depth = 64
	var input strings.Builder
	for fence := depth + 2; fence >= 3; fence-- {
		input.WriteString(strings.Repeat(":", fence))
		input.WriteString(" level\n")
	}
	input.WriteString("bottom\n")
	for fence := 3; fence <= depth+2; fence++ {
		input.WriteString(strings.Repeat(":", fence))
		input.WriteByte('\n')
	}

	doc := djot.Parse(input.String())
	calls := 0
	got := djot.RenderHTML(doc, djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) {
		calls++
		r.Children()
	}))
	if calls != depth {
		t.Fatalf("Div callback count = %d, want %d", calls, depth)
	}
	if got != "<p>bottom</p>\n" {
		t.Fatalf("deep child output = %q", got)
	}
}

func TestDivRendererOptionOrder(t *testing.T) {
	doc := djot.Parse("::: note\ntext\n:::\n")
	tapeLast := djot.RenderHTML(doc,
		djot.WithNodeRenderer(ast.KindDiv, func(ast.Node, djot.NodeRenderer) {
			t.Fatal("replaced Node hook ran")
		}),
		djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) { r.Children() }),
	)
	if tapeLast != "<p>text</p>\n" {
		t.Fatalf("Div-last output = %q", tapeLast)
	}

	nodeLast := djot.RenderHTML(doc,
		djot.WithDivRenderer(func(djot.DivView, djot.ElementRenderer) {
			t.Fatal("replaced Div hook ran")
		}),
		djot.WithNodeRenderer(ast.KindDiv, func(_ ast.Node, r djot.NodeRenderer) { r.Children() }),
	)
	if nodeLast != "<p>text</p>\n" {
		t.Fatalf("Node-last output = %q", nodeLast)
	}
}

func TestDivRendererWriterErrorStopsNestedCallbacks(t *testing.T) {
	const input = ":::: outer\n::: inner\ntext\n:::\n::::\n"
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			writer := &errWriter{limit: 0}
			calls := 0
			err := djot.RenderHTMLTo(writer, doc, djot.WithDivRenderer(func(_ djot.DivView, r djot.ElementRenderer) {
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

func TestDivRendererPanicPropagates(t *testing.T) {
	const marker = "div hook panic"
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
			djot.RenderHTML(doc, djot.WithDivRenderer(func(djot.DivView, djot.ElementRenderer) {
				panic(marker)
			}))
		})
	}
}

func TestDivRendererConcurrentRendering(t *testing.T) {
	doc := djot.Parse("::: note\ntext :star:\n:::\n")
	option := djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
		r.Write(`<aside class="` + div.Attributes().Get("class") + `">`)
		r.Children()
		r.Write("</aside>")
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

func TestDivRendererHTMLToParityAndAttributeEdges(t *testing.T) {
	doc := djot.Parse("{present=\"\" first=1 second=2}\n::: note\ntext\n:::\n")
	option := djot.WithDivRenderer(func(div djot.DivView, r djot.ElementRenderer) {
		if value, ok := div.Attributes().Lookup("present"); !ok || value != "" {
			t.Fatalf("present empty attribute = %q, %v", value, ok)
		}
		if _, ok := div.Attributes().Lookup("missing"); ok {
			t.Fatal("missing attribute reported present")
		}
		visited := 0
		div.Attributes().Range(func(ast.Attribute) bool {
			visited++
			return false
		})
		if visited != 1 {
			t.Fatalf("early Range visited %d attributes", visited)
		}
		r.Default()
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
