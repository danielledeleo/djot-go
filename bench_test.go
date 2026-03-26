package djot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
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

// hugeDoc generates a ~1 MB document with diverse block and inline content.
func hugeDoc() string {
	var b strings.Builder

	// A realistic mix of djot features repeated to reach ~1 MB.
	// Each "chapter" is ~10 KB, so ~100 chapters ≈ 1 MB.
	for ch := 0; ch < 100; ch++ {
		b.WriteString(fmt.Sprintf("# Chapter %d: The Art of Parsing\n\n", ch+1))

		// Prose paragraphs with inline formatting
		b.WriteString("The quick brown fox *jumps* over the **lazy dog**. ")
		b.WriteString("This sentence has _emphasis_, **strong**, and `inline code`. ")
		b.WriteString("Here is a [link](https://example.com/page) and an ")
		b.WriteString("![image](https://example.com/img.png \"title\"). ")
		b.WriteString("Smart quotes: \"hello\" and 'world'. ")
		b.WriteString("Dashes: em---dash and en--dash. Ellipsis...\n\n")

		b.WriteString(fmt.Sprintf("## Section %d.1: Lists\n\n", ch+1))

		// Bullet list
		for i := 0; i < 10; i++ {
			b.WriteString(fmt.Sprintf("- Item %d with *emphasis* and a [link](https://example.com/%d)\n", i+1, i))
		}
		b.WriteString("\n")

		// Ordered list
		for i := 0; i < 5; i++ {
			b.WriteString(fmt.Sprintf("%d. Ordered item with `code` and **bold** text\n", i+1))
		}
		b.WriteString("\n")

		// Task list
		b.WriteString("- [ ] Unchecked task\n- [x] Checked task\n- [ ] Another task\n\n")

		b.WriteString(fmt.Sprintf("## Section %d.2: Blocks\n\n", ch+1))

		// Blockquote
		b.WriteString("> This is a blockquote with *formatting*.\n")
		b.WriteString("> It spans multiple lines and has **bold** words.\n")
		b.WriteString(">\n")
		b.WriteString("> > Nested blockquote with a [link](https://example.com).\n\n")

		// Code block
		b.WriteString("```go\n")
		b.WriteString("func process(items []string) error {\n")
		b.WriteString("    for _, item := range items {\n")
		b.WriteString("        if err := handle(item); err != nil {\n")
		b.WriteString("            return fmt.Errorf(\"process %s: %w\", item, err)\n")
		b.WriteString("        }\n")
		b.WriteString("    }\n")
		b.WriteString("    return nil\n")
		b.WriteString("}\n")
		b.WriteString("```\n\n")

		// Table
		b.WriteString("| Name   | Type    | Description                          |\n")
		b.WriteString("|--------|---------|--------------------------------------|\n")
		for i := 0; i < 5; i++ {
			b.WriteString(fmt.Sprintf("| field%d | string  | A field with *formatted* description |\n", i))
		}
		b.WriteString("\n")

		// Div
		b.WriteString("::: warning\n")
		b.WriteString("This is a warning admonition with _emphasis_ and a :symbol:.\n")
		b.WriteString(":::\n\n")

		// Definition list
		b.WriteString(": Term one\n\n")
		b.WriteString("  Definition of term one with **bold** text.\n\n")
		b.WriteString(": Term two\n\n")
		b.WriteString("  Definition of term two with `code`.\n\n")

		// Footnotes
		b.WriteString(fmt.Sprintf("A paragraph with footnotes[^%da] and more[^%db].\n\n", ch, ch))
		b.WriteString(fmt.Sprintf("[^%da]: First footnote for chapter %d.\n\n", ch, ch+1))
		b.WriteString(fmt.Sprintf("[^%db]: Second footnote with *emphasis*.\n\n", ch))

		// Thematic break
		b.WriteString("* * *\n\n")
	}

	return b.String()
}

// pathologicalInline generates a document heavy on inline parsing (many
// delimiters, links, attributes) to stress the inline parser.
func pathologicalInline(size int) string {
	var b strings.Builder
	// Repeat a dense inline paragraph up to the target size.
	line := "The *quick* _brown_ **fox** [jumps](url) over `the` {+lazy+} {-dog-} {=mark=} H~2~O x^2^ :star: \"quotes\" 'single' a--b c---d ...\n"
	for b.Len() < size {
		b.WriteString(line)
	}
	return b.String()
}

// pathologicalNesting generates deeply nested blockquotes and lists.
func pathologicalNesting(depth int) string {
	var b strings.Builder
	// Nested blockquotes
	for i := 0; i < depth; i++ {
		b.WriteString(strings.Repeat("> ", i+1))
		b.WriteString(fmt.Sprintf("Level %d\n", i+1))
	}
	b.WriteString("\n")
	// Nested lists
	for i := 0; i < depth; i++ {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString(fmt.Sprintf("- Nest %d\n", i+1))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// BenchmarkParse — parse only (no rendering)
// ---------------------------------------------------------------------------

func BenchmarkParse(b *testing.B) {
	huge := hugeDoc()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
		{"Huge_1MB", huge},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.doc)))
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
	huge := hugeDoc()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
		{"Huge_1MB", huge},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.doc)))
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
	huge := hugeDoc()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
		{"Huge_1MB", huge},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.doc)))
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
	huge := hugeDoc()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"Small", smallDoc()},
		{"Medium", mediumDoc()},
		{"Large", largeDoc()},
		{"Huge_1MB", huge},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.doc)))
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

// ---------------------------------------------------------------------------
// BenchmarkPathological — stress tests for worst-case inputs
// ---------------------------------------------------------------------------

func BenchmarkPathologicalInline(b *testing.B) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	} {
		doc := pathologicalInline(tc.size)
		b.Run(tc.name+"/Parse", func(b *testing.B) {
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = djot.Parse(doc)
			}
		})
		b.Run(tc.name+"/ParseAndRender", func(b *testing.B) {
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d := djot.Parse(doc)
				_ = djot.RenderHTML(d)
			}
		})
	}
}

func BenchmarkPathologicalNesting(b *testing.B) {
	for _, depth := range []int{50, 200} {
		doc := pathologicalNesting(depth)
		b.Run(fmt.Sprintf("Depth%d/Parse", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = djot.Parse(doc)
			}
		})
		b.Run(fmt.Sprintf("Depth%d/ParseAndRender", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d := djot.Parse(doc)
				_ = djot.RenderHTML(d)
			}
		})
	}
}
