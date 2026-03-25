package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestRenderHooks(t *testing.T) {
	t.Run("replace CodeBlock rendering", func(t *testing.T) {
		doc := djot.Parse("```go\nfmt.Println()\n```")
		html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.CodeBlock, func(n *djot.Node, r djot.NodeRenderer) {
			r.Write("<custom-code>" + n.Text + "</custom-code>")
		}))
		if !strings.Contains(html, "<custom-code>") {
			t.Errorf("expected <custom-code>, got:\n%s", html)
		}
		if strings.Contains(html, "<pre><code") {
			t.Errorf("should not contain default <pre><code>, got:\n%s", html)
		}
	})

	t.Run("augment Heading with Default", func(t *testing.T) {
		doc := djot.Parse("# Hello")
		html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.Heading, func(n *djot.Node, r djot.NodeRenderer) {
			r.Default()
			r.Write(`<a href="#">¶</a>`)
		}))
		if !strings.Contains(html, "</h1>\n<a href=\"#\">\u00b6</a>") {
			t.Errorf("expected heading followed by permalink, got:\n%s", html)
		}
	})

	t.Run("Div with Children for admonition", func(t *testing.T) {
		doc := djot.Parse("::: warning\nBe careful!\n:::")
		html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.Div, func(n *djot.Node, r djot.NodeRenderer) {
			if n.Attr("class") == "warning" {
				r.Write(`<aside class="warning">`)
				r.Children()
				r.Write("</aside>")
				return
			}
			r.Default()
		}))
		if !strings.Contains(html, `<aside class="warning">`) {
			t.Errorf("expected <aside>, got:\n%s", html)
		}
		if strings.Contains(html, "<div") {
			t.Errorf("should not contain <div>, got:\n%s", html)
		}
		if !strings.Contains(html, "Be careful!") {
			t.Errorf("expected children content, got:\n%s", html)
		}
	})

	t.Run("hooks compose", func(t *testing.T) {
		doc := djot.Parse("::: note\n:star:\n:::")
		html := djot.RenderHTML(doc,
			djot.WithNodeRenderer(djot.Div, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("<aside>")
				r.Children()
				r.Write("</aside>")
			}),
			djot.WithNodeRenderer(djot.Symbol, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("<svg>" + n.Name + "</svg>")
			}),
		)
		if !strings.Contains(html, "<aside>") {
			t.Errorf("expected <aside>, got:\n%s", html)
		}
		if !strings.Contains(html, "<svg>star</svg>") {
			t.Errorf("expected symbol hook to fire inside div, got:\n%s", html)
		}
	})

	t.Run("noop hook matches default behavior", func(t *testing.T) {
		doc := djot.Parse("Hello *world*")
		withHook := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.Strong, func(n *djot.Node, r djot.NodeRenderer) {
			r.Default()
		}))
		without := djot.RenderHTML(doc)
		if withHook != without {
			t.Errorf("no-op hook output differs:\n%s\nvs\n%s", withHook, without)
		}
	})

	t.Run("multiple hooks for different kinds", func(t *testing.T) {
		doc := djot.Parse("# Title\n\n```\ncode\n```")
		html := djot.RenderHTML(doc,
			djot.WithNodeRenderer(djot.Heading, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("<h1 class=\"custom\">")
				r.Children()
				r.Write("</h1>")
			}),
			djot.WithNodeRenderer(djot.CodeBlock, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("<pre class=\"custom\">" + n.Text + "</pre>")
			}),
		)
		if !strings.Contains(html, `<h1 class="custom">`) {
			t.Errorf("expected custom heading, got:\n%s", html)
		}
		if !strings.Contains(html, `<pre class="custom">`) {
			t.Errorf("expected custom code block, got:\n%s", html)
		}
	})

	t.Run("last hook wins for same kind", func(t *testing.T) {
		doc := djot.Parse(":star:")
		html := djot.RenderHTML(doc,
			djot.WithNodeRenderer(djot.Symbol, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("FIRST")
			}),
			djot.WithNodeRenderer(djot.Symbol, func(n *djot.Node, r djot.NodeRenderer) {
				r.Write("SECOND")
			}),
		)
		if !strings.Contains(html, "SECOND") {
			t.Errorf("expected last hook to win, got:\n%s", html)
		}
		if strings.Contains(html, "FIRST") {
			t.Errorf("first hook should not fire, got:\n%s", html)
		}
	})

	t.Run("Default does not re-trigger hook", func(t *testing.T) {
		callCount := 0
		doc := djot.Parse("# Hello")
		djot.RenderHTML(doc, djot.WithNodeRenderer(djot.Heading, func(n *djot.Node, r djot.NodeRenderer) {
			callCount++
			r.Default()
		}))
		if callCount != 1 {
			t.Errorf("hook called %d times, expected 1 (Default should not re-trigger)", callCount)
		}
	})
}
