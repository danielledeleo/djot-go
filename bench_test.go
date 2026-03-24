package djot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danielledeleo/homedoc/djot"
)

// ---------------------------------------------------------------------------
// Test document generators
// ---------------------------------------------------------------------------

func smallDoc() string {
	return `# Hello World

A short paragraph with *emphasis* and **strong** text.

Another paragraph with a [link](https://example.com).
`
}

func mediumDoc() string {
	var b strings.Builder
	b.WriteString("# Main Heading\n\n")
	b.WriteString("An introductory paragraph with *emphasis* and **strong** text.\n\n")
	b.WriteString("## Section One\n\n")
	b.WriteString("Some text with a [link](https://example.com) and `inline code`.\n\n")
	b.WriteString("- First item\n- Second item with *emphasis*\n- Third item\n\n")
	b.WriteString("## Section Two\n\n")
	b.WriteString("| Header 1 | Header 2 |\n|----------|----------|\n| Cell 1   | Cell 2   |\n| Cell 3   | Cell 4   |\n\n")
	b.WriteString("> A blockquote with some text\n> and a second line.\n\n")
	b.WriteString("```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```\n\n")
	b.WriteString("::: note\nAn admonition div with _formatted_ content.\n:::\n\n")
	b.WriteString("Text with :symbol: and 'smart quotes' and an em---dash.\n\n")
	return b.String()
}

func largeDoc() string {
	med := mediumDoc()
	var b strings.Builder
	for i := 0; i < 10; i++ {
		// Rewrite the top-level heading so each chunk is distinct.
		chunk := strings.Replace(med, "# Main Heading", fmt.Sprintf("# Part %d", i+1), 1)
		b.WriteString(chunk)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// BenchmarkParse — parse only (no rendering)
// ---------------------------------------------------------------------------

func BenchmarkParse(b *testing.B) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = djot.Parse(tc.doc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkRenderHTML — render a pre-parsed document
// ---------------------------------------------------------------------------

func BenchmarkRenderHTML(b *testing.B) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			parsed := djot.Parse(tc.doc)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = djot.RenderHTML(parsed)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkParseAndRender — end-to-end parse + render
// ---------------------------------------------------------------------------

func BenchmarkParseAndRender(b *testing.B) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d := djot.Parse(tc.doc)
				_ = djot.RenderHTML(d)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkParseAttrs — attribute parsing
// ---------------------------------------------------------------------------

func BenchmarkParseAttrs(b *testing.B) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"Simple", `.class`},
		{"Complex", `.class1 .class2 #id key1="val1" key2="val2"`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = djot.ParseAttrs(tc.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BenchmarkWalk — walk a parsed AST
// ---------------------------------------------------------------------------

func BenchmarkWalk(b *testing.B) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
	} {
		b.Run(tc.name, func(b *testing.B) {
			parsed := djot.Parse(tc.doc)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				djot.Walk(parsed.Root, func(n *djot.Node) any {
					return djot.Continue
				})
			}
		})
	}
}
