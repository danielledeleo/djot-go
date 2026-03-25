//go:build difftest

package djot_test

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// renderWithJS calls the djot.js renderer via node.
func renderWithJS(input string) (string, error) {
	cmd := exec.Command("node", "/app/render.mjs")
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("djot.js: %v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func TestDifferential(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	if _, err := os.Stat("/app/render.mjs"); err != nil {
		t.Skip("render.mjs not found (run inside difftest container)")
	}

	// Hand-crafted edge cases targeting known parser subtleties.
	edgeCases := []string{
		// Nested emphasis edge cases
		"*_nested_*",
		"_*nested*_",
		"***triple***",
		"**_mixed **bold_ end**",
		"*not closed\n\nnew para*",

		// Bracket/link edge cases
		"[link](url_(with_(nested)_parens))",
		"[link](url with spaces)",
		"[link]()",
		"[](empty text)",
		"[![image](img.png)](link)",
		"[foo][]\n\n[foo]: /url",
		"[foo][bar]\n\n[bar]: /url",
		"[link](url\nspanning\nlines)",

		// Attribute edge cases
		"{.class #id key=\"val\"}\nparagraph",
		"text{.class}",
		"*emph*{.highlight}",
		"{key='single quoted'}",
		"{data-x=\"a\\\"b\"}",
		"{.a .b .c}\n# heading",

		// List edge cases
		"- item 1\n- item 2\n\n- item 3",
		"1. first\n2. second\n10. tenth",
		"- [ ] unchecked\n- [x] checked",
		"- a\n  - b\n    - c\n      - d",
		"- tight\n- list",
		"- loose\n\n- list",

		// Definition lists
		": term\n\n  definition",
		": term 1\n  def 1\n: term 2\n  def 2",

		// Block nesting
		"> > nested blockquote",
		"> - list in blockquote",
		"- > blockquote in list",
		"::: outer\n::: inner\ncontent\n:::\n:::",
		"> [ref]: /url\n\n[text][ref]",

		// Code and raw
		"`` `code` ``",
		"```\ncode\n```",
		"````\n```\ncode\n```\n````",
		"`raw`{=html}",

		// Math
		"$x^2$",
		"$$\nE = mc^2\n$$",
		"$\\$escaped$",

		// Tables
		"| a | b |\n|---|---|\n| 1 | 2 |",
		"| left | center | right |\n|:-----|:------:|------:|\n| a    | b      | c     |",
		"| a |\n|---|\n| b |\n\n^ caption",

		// Footnotes
		"text[^1]\n\n[^1]: footnote",
		"text[^a]\n\n[^a]: footnote with\n  multiple lines",
		"[^1][^2]\n\n[^1]: first\n\n[^2]: second",

		// Smart punctuation
		`"hello" 'world'`,
		"it's a test",
		"a -- b --- c",
		"...",

		// Thematic breaks
		"* * *",
		"- - -",
		"*-*-*",

		// Symbols
		":smile:",
		":+1:",

		// Escapes
		"\\*not emphasis\\*",
		"\\[not a link\\]",
		"hard break\\\ntext",

		// Spans
		"[text]{.class}",
		"[text]{key=\"value\"}",

		// Edge: empty document
		"",

		// Edge: only whitespace
		"   \n\n   ",

		// Edge: very deep nesting
		strings.Repeat("> ", 20) + "deep",

		// Edge: adjacent special chars
		"***___^^^~~~",
		"[](){}",
		"[[[nested brackets]]]",

		// Edge: interleaved formatting
		"*a _b* c_",
		"_a *b_ c*",

		// Edge: line-ending variations
		"line1\nline2",
		"line1\r\nline2",

		// Edge: unicode
		"# Ünïcödé héading",
		"*émphasis* and **ströng**",

		// Edge: heading levels
		"# h1\n## h2\n### h3\n#### h4\n##### h5\n###### h6",

		// Edge: reference before definition
		"[click][ref]\n\n[ref]: /url",

		// Edge: raw blocks
		"```=html\n<div class=\"foo\">\n```",

		// Edge: insert/delete/mark
		"{+insert+}",
		"{-delete-}",
		"{=mark=}",

		// Edge: superscript/subscript
		"H~2~O",
		"x^2^",
	}

	// Generate random inputs from fragments.
	fragments := []string{
		"*", "_", "**", "__", "[", "]", "(", ")", "{", "}", "`", "``",
		"#", "##", "###", ">", "- ", "1. ", "| ", "---", "***",
		"\n", "\n\n", " ", "  ", "\\", "^", "~", "$", "$$",
		"text", "word", "hello", "http://x", ".class", "#id",
		":", "::", ":::", "[^1]", "=", "+", "-",
		`"`, `'`, "...", "<", ">",
	}

	rng := rand.New(rand.NewPCG(42, 0))
	for i := 0; i < 200; i++ {
		var b strings.Builder
		nFrags := rng.IntN(15) + 1
		for j := 0; j < nFrags; j++ {
			b.WriteString(fragments[rng.IntN(len(fragments))])
		}
		edgeCases = append(edgeCases, b.String())
	}

	var failures []string
	for i, input := range edgeCases {
		goHTML := djot.RenderHTML(djot.Parse(input))
		jsHTML, err := renderWithJS(input)
		if err != nil {
			t.Logf("case %d: djot.js error: %v (input: %q)", i, err, input)
			continue
		}
		if goHTML != jsHTML {
			failures = append(failures, fmt.Sprintf(
				"case %d:\n  input: %q\n  go:   %q\n  js:   %q", i, input, goHTML, jsHTML))
		}
	}

	if len(failures) > 0 {
		t.Errorf("%d/%d divergences:\n%s", len(failures), len(edgeCases), strings.Join(failures, "\n\n"))
	} else {
		t.Logf("all %d cases match", len(edgeCases))
	}
}
