package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func checkBothHTML(t *testing.T, input, want string) {
	t.Helper()
	doc := djot.Parse(input)
	var compact string
	for _, tree := range []bool{false, true} {
		if tree {
			doc = djot.NewDoc(doc.Root())
		}
		got := djot.RenderHTML(doc)
		if tree && got != compact {
			t.Errorf("renderers disagree:\ncompact: %s\ntree: %s", compact, got)
		}
		compact = got
		if !strings.Contains(got, want) {
			t.Errorf("tree=%v: want %q in:\n%s", tree, want, got)
		}
	}
}

func TestTightListNestedContainers(t *testing.T) {
	for _, input := range []string{
		"- > first\n  >\n  > second\n- last\n",
		"- :::\n  first\n\n  second\n  :::\n- last\n",
	} {
		checkBothHTML(t, input, "<p>first</p>\n<p>second</p>")
	}
	checkBothHTML(t, "- > first\n  >\n  > second\n- last\n", "<li>\nlast\n</li>")
}

func TestImageAltPreservesPresentationText(t *testing.T) {
	checkBothHTML(t, "![`code` $`x` $$`y` :smile: ... -- ---](x)", `alt="code x y :smile: … – —"`)
	checkBothHTML(t, `!["quoted" *bold* & <](x)`, `alt="“quoted” bold &amp; &lt;"`)
	checkBothHTML(t, "![a[^n] `hidden`{=html}](x)\n\n[^n]: note", `alt="a "`)
}

func TestTightListContainerHooks(t *testing.T) {
	const input = "- > first\n  >\n  > second\n- last\n"
	options := []djot.RenderOption{
		djot.WithSubtreeRenderer(ast.KindBlockQuote, func(_ djot.SubtreeView, r djot.ElementRenderer) { r.Children() }),
		djot.WithNodeRenderer(ast.KindBlockQuote, func(_ ast.Node, r djot.NodeRenderer) { r.Children() }),
	}
	for _, option := range options {
		got := djot.RenderHTML(djot.Parse(input), option)
		const want = "<ul>\n<li>\n<p>first</p>\n<p>second</p>\n</li>\n<li>\nlast\n</li>\n</ul>\n"
		if got != want {
			t.Errorf("hook output:\n%s\nwant:\n%s", got, want)
		}
	}
}
