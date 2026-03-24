package djot_test

import (
	"testing"

	"github.com/danielledeleo/homedoc/djot"
)

func FuzzParse(f *testing.F) {
	seeds := []string{
		"# Hello\n\nworld",
		"*emphasis* and **strong**",
		"[link](url)",
		"![image](src)",
		"> blockquote",
		"- item 1\n- item 2",
		"1. first\n2. second",
		"```go\ncode\n```",
		"::: div\ncontent\n:::",
		"| a | b |\n|---|---|\n| c | d |",
		": term\n  definition",
		"- [ ] task\n- [x] done",
		"[^fn]: footnote\n\ntext[^fn]",
		"{.class #id key=val}\n# heading",
		"_({_foo_})_",
		"'smart' \"quotes\"",
		"---",
		"...",
		"$`math`",
		"$$`display`",
		"`code`{=html}",
		":symbol:",
		"[text]{.class}",
		"\\*escaped\\*",
		"\\ \n",
		"<http://example.com>",
		"",
		"\n\n\n",
		string([]byte{0, 1, 2, 3, 255}),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		doc := djot.Parse(input)
		_ = djot.RenderHTML(doc)
	})
}

func FuzzParseAttrs(f *testing.F) {
	seeds := []string{
		".class",
		"#id",
		"key=value",
		`key="quoted value"`,
		".class #id key=val",
		"",
		"%%%",
		`key="unclosed`,
		".a .b .c #d #e",
		"key=bare",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_ = djot.ParseAttrs(input)
	})
}

func FuzzWalk(f *testing.F) {
	f.Add("# Hello\n\n*world*\n\n- a\n- b")
	f.Fuzz(func(t *testing.T, input string) {
		doc := djot.Parse(input)
		djot.Walk(doc.Root, func(n *djot.Node) any {
			return djot.Continue
		})
	})
}
