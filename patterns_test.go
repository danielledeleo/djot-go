package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// ---------------------------------------------------------------------------
// Symbol shortcodes via WithRenderFunc
// ---------------------------------------------------------------------------

func TestPatternSymbolIcons(t *testing.T) {
	doc := djot.Parse("Click :arrow-right: to continue")
	html := djot.RenderHTML(doc, djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
		return `<i class="lucide lucide-` + n.(*ast.Symbol).Name + `"></i>`
	}))
	if !strings.Contains(html, `<i class="lucide lucide-arrow-right"></i>`) {
		t.Errorf("expected icon markup, got:\n%s", html)
	}
}

func TestPatternSymbolEmoji(t *testing.T) {
	emoji := map[string]string{
		"star":   "⭐",
		"check":  "✅",
		"rocket": "🚀",
	}
	doc := djot.Parse("Launch :rocket: and :star: it")
	html := djot.RenderHTML(doc, djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
		return emoji[n.(*ast.Symbol).Name] // unknown returns "" and falls through
	}))
	if !strings.Contains(html, "🚀") || !strings.Contains(html, "⭐") {
		t.Errorf("expected emoji, got:\n%s", html)
	}
}

func TestPatternSymbolFallthrough(t *testing.T) {
	doc := djot.Parse(":known: and :unknown:")
	html := djot.RenderHTML(doc, djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
		if n.(*ast.Symbol).Name == "known" {
			return "REPLACED"
		}
		return "" // fall through to default
	}))
	if !strings.Contains(html, "REPLACED") {
		t.Errorf("expected REPLACED, got:\n%s", html)
	}
	if !strings.Contains(html, ":unknown:") {
		t.Errorf("expected default :unknown:, got:\n%s", html)
	}
}

func TestPatternSymbolYouTubeEmbed(t *testing.T) {
	doc := djot.Parse(`:youtube:{id="dQw4w9WgXcQ"}`)
	html := djot.RenderHTML(doc, djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
		if n.(*ast.Symbol).Name == "youtube" {
			id := n.Attributes().Get("id")
			if id != "" {
				return `<iframe src="https://www.youtube.com/embed/` + id + `" allowfullscreen></iframe>`
			}
		}
		return ""
	}))
	if !strings.Contains(html, `src="https://www.youtube.com/embed/dQw4w9WgXcQ"`) {
		t.Errorf("expected youtube embed, got:\n%s", html)
	}
}

func TestPatternSymbolDataWidget(t *testing.T) {
	doc := djot.Parse(`:chart:{type="bar" data="1,2,3,4"}`)
	html := djot.RenderHTML(doc, djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
		if n.(*ast.Symbol).Name == "chart" {
			return `<div class="chart" data-type="` + n.Attributes().Get("type") + `" data-values="` + n.Attributes().Get("data") + `"></div>`
		}
		return ""
	}))
	if !strings.Contains(html, `data-type="bar"`) || !strings.Contains(html, `data-values="1,2,3,4"`) {
		t.Errorf("expected chart widget, got:\n%s", html)
	}
}

// ---------------------------------------------------------------------------
// Div shortcodes via WithNodeRenderer (needs Children())
// ---------------------------------------------------------------------------

func TestPatternDivAdmonition(t *testing.T) {
	doc := djot.Parse("::: warning\nDo not do this!\n:::")
	html := djot.RenderHTML(doc, djot.WithNodeRenderer(ast.KindDiv, func(n ast.Node, r djot.NodeRenderer) {
		class := n.Attributes().Get("class")
		if class == "warning" || class == "note" || class == "tip" {
			r.Write(`<aside class="admonition ` + class + `">`)
			r.Children()
			r.Write("</aside>")
			return
		}
		r.Default()
	}))
	if !strings.Contains(html, `<aside class="admonition warning">`) {
		t.Errorf("expected admonition aside, got:\n%s", html)
	}
	if !strings.Contains(html, "Do not do this!") {
		t.Errorf("expected content preserved, got:\n%s", html)
	}
	if strings.Contains(html, "<div") {
		t.Errorf("should not contain <div>, got:\n%s", html)
	}
}

// ---------------------------------------------------------------------------
// Walk patterns (AST transforms)
// ---------------------------------------------------------------------------

func TestPatternWalkCollectHeadings(t *testing.T) {
	doc := djot.Parse("# Intro\n\ntext\n\n## Methods\n\ntext\n\n## Results\n\ntext")
	var headings []string
	ast.Walk(doc.Root(), func(n ast.Node) ast.Action {
		if heading, ok := n.(*ast.Heading); ok {
			var text string
			for _, inline := range heading.Children {
				if textNode, ok := inline.(*ast.Text); ok {
					text += textNode.Value
				}
			}
			headings = append(headings, text)
			return ast.SkipChildren
		}
		return ast.Continue
	})
	if len(headings) != 3 {
		t.Fatalf("expected 3 headings, got %d: %v", len(headings), headings)
	}
	if headings[0] != "Intro" || headings[1] != "Methods" || headings[2] != "Results" {
		t.Errorf("unexpected headings: %v", headings)
	}
}

func TestPatternWalkRewriteImageURLs(t *testing.T) {
	doc := djot.Parse("![photo](images/cat.png)")
	base := "https://cdn.example.com/"
	ast.Walk(doc.Root(), func(n ast.Node) ast.Action {
		if image, ok := n.(*ast.Image); ok && !strings.HasPrefix(image.Destination, "http") {
			image.Destination = base + image.Destination
		}
		return ast.Continue
	})
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `src="https://cdn.example.com/images/cat.png"`) {
		t.Errorf("expected rewritten URL, got:\n%s", html)
	}
}

// ---------------------------------------------------------------------------
// AST include/splice with footnote merging
// ---------------------------------------------------------------------------

func TestPatternASTInclude(t *testing.T) {
	parent := djot.Parse("Main text[^a].\n\n[^a]: Parent footnote.")
	child := djot.Parse("Included text[^b].\n\n[^b]: Child footnote.")

	// Splice the child's content into the parent AST.
	// Wrap in a Div to replace a placeholder, simulating :include:.
	parentRoot := parent.Root()
	wrapper := &ast.Div{Children: child.Root().Children}
	parentRoot.Children = append(parentRoot.Children, wrapper)

	// Render — footnotes should be derived from the combined AST.
	html := djot.RenderHTML(parent)

	if !strings.Contains(html, "Parent footnote") {
		t.Errorf("expected parent footnote, got:\n%s", html)
	}
	if !strings.Contains(html, "Child footnote") {
		t.Errorf("expected child footnote, got:\n%s", html)
	}
	// Both footnote references should have rendered as links.
	if strings.Count(html, `role="doc-noteref"`) != 2 {
		t.Errorf("expected 2 footnote references, got:\n%s", html)
	}
}

func TestPatternASTIncludeViaWalk(t *testing.T) {
	// Replace the block paragraph containing a standalone include symbol.
	input := "Before.\n\n:include:\n\nAfter."
	doc := djot.Parse(input)

	included := djot.Parse("Included paragraph[^x].\n\n[^x]: Included note.")

	ast.Walk(doc.Root(), func(n ast.Node) ast.Action {
		if paragraph, ok := n.(*ast.Paragraph); ok && len(paragraph.Children) == 1 {
			if symbol, ok := paragraph.Children[0].(*ast.Symbol); ok && symbol.Name == "include" {
				return ast.Replace(&ast.Div{Children: included.Root().Children})
			}
		}
		return ast.Continue
	})

	html := djot.RenderHTML(doc)

	if !strings.Contains(html, "Before.") {
		t.Errorf("expected 'Before.', got:\n%s", html)
	}
	if !strings.Contains(html, "Included paragraph") {
		t.Errorf("expected included content, got:\n%s", html)
	}
	if !strings.Contains(html, "Included note") {
		t.Errorf("expected included footnote, got:\n%s", html)
	}
	if !strings.Contains(html, "After.") {
		t.Errorf("expected 'After.', got:\n%s", html)
	}
}

// ---------------------------------------------------------------------------
// Composition: multiple hooks together
// ---------------------------------------------------------------------------

func TestPatternComposition(t *testing.T) {
	input := `::: note
Check :star: and :arrow-right: below.
:::

Regular :star: outside.`

	doc := djot.Parse(input)

	emoji := map[string]string{"star": "⭐"}

	html := djot.RenderHTML(doc,
		// Admonition divs
		djot.WithNodeRenderer(ast.KindDiv, func(n ast.Node, r djot.NodeRenderer) {
			if class := n.Attributes().Get("class"); class == "note" || class == "warning" {
				r.Write(`<aside class="` + class + `">`)
				r.Children()
				r.Write("</aside>")
				return
			}
			r.Default()
		}),
		// Icons
		djot.WithRenderFunc(ast.KindSymbol, func(n ast.Node) string {
			name := n.(*ast.Symbol).Name
			if e, ok := emoji[name]; ok {
				return e
			}
			if strings.HasPrefix(name, "arrow-") {
				return `<i class="icon-` + name + `"></i>`
			}
			return ""
		}),
	)

	if !strings.Contains(html, `<aside class="note">`) {
		t.Errorf("expected admonition, got:\n%s", html)
	}
	if !strings.Contains(html, "⭐") {
		t.Errorf("expected star emoji, got:\n%s", html)
	}
	if !strings.Contains(html, `<i class="icon-arrow-right"></i>`) {
		t.Errorf("expected arrow icon, got:\n%s", html)
	}
	// Star outside the div should also render as emoji.
	if strings.Count(html, "⭐") != 2 {
		t.Errorf("expected 2 star emojis (inside and outside div), got:\n%s", html)
	}
}
