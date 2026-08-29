package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func TestPositionTracking(t *testing.T) {
	input := "# Hello\n\nworld"
	doc := djot.Parse(input)

	// The AST should be: Document > Section > [Heading, Paragraph].
	root := doc.Root()
	if root.Kind() != ast.KindDocument {
		t.Fatalf("expected Document, got %v", root.Kind())
	}
	if root.Span().Start.Offset != 0 {
		t.Errorf("document Start.Offset = %d, want 0", root.Span().Start.Offset)
	}

	// Find the section (headings get wrapped).
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}
	section, ok := root.Children[0].(*ast.Section)
	if !ok {
		t.Fatalf("expected *Section, got %T", root.Children[0])
	}

	if len(section.Children) < 2 {
		t.Fatalf("section has %d children, want >= 2", len(section.Children))
	}

	heading, ok := section.Children[0].(*ast.Heading)
	if !ok {
		t.Fatalf("expected *Heading, got %T", section.Children[0])
	}
	para, ok := section.Children[1].(*ast.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", section.Children[1])
	}

	// The heading "# Hello" starts at offset 0.
	if heading.Span().Start.Offset != 0 {
		t.Errorf("heading Start.Offset = %d, want 0", heading.Span().Start.Offset)
	}
	// The heading ends at offset 7 (exclusive end of the "# Hello" line).
	if heading.Span().End.Offset != 7 {
		t.Errorf("heading End.Offset = %d, want 7", heading.Span().End.Offset)
	}

	// The paragraph "world" starts at offset 9 (after "# Hello\n\n").
	if para.Span().Start.Offset != 9 {
		t.Errorf("paragraph Start.Offset = %d, want 9", para.Span().Start.Offset)
	}
	// The paragraph ends at offset 14 (exclusive end of "world").
	if para.Span().End.Offset != 14 {
		t.Errorf("paragraph End.Offset = %d, want 14", para.Span().End.Offset)
	}

	// The section should inherit the heading start and paragraph end.
	if section.Span().Start.Offset != heading.Span().Start.Offset {
		t.Errorf("section Start.Offset = %d, want %d (heading start)",
			section.Span().Start.Offset, heading.Span().Start.Offset)
	}
	if section.Span().End.Offset != para.Span().End.Offset {
		t.Errorf("section End.Offset = %d, want %d (paragraph end)",
			section.Span().End.Offset, para.Span().End.Offset)
	}

	// Verify that inline nodes within the heading also have positions.
	if len(heading.Children) == 0 {
		t.Fatal("heading has no inline children")
	}
	textNode := heading.Children[0]
	if textNode.Kind() != ast.KindText {
		t.Fatalf("expected Text child, got %v", textNode.Kind())
	}
	if textNode.Span().Start.Offset == 0 && textNode.Span().End.Offset == 0 {
		t.Error("heading text node should have non-zero positions")
	}
}

func TestPositionCodeBlock(t *testing.T) {
	input := "```go\nfmt.Println()\n```"
	doc := djot.Parse(input)

	root := doc.Root()
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	cb := root.Children[0]
	if cb.Kind() != ast.KindCodeBlock {
		t.Fatalf("expected CodeBlock, got %v", cb.Kind())
	}

	if cb.Span().Start.Offset != 0 {
		t.Errorf("code block Start.Offset = %d, want 0", cb.Span().Start.Offset)
	}
	// Code block: "```go\nfmt.Println()\n```" = 5+1+13+1+3 = 23 bytes.
	if cb.Span().End.Offset != 23 {
		t.Errorf("code block End.Offset = %d, want 23", cb.Span().End.Offset)
	}
}

func TestPositionThematicBreak(t *testing.T) {
	input := "hello\n\n* * *\n\nworld"
	doc := djot.Parse(input)

	root := doc.Root()
	// Should have: Paragraph("hello"), ThematicBreak, Paragraph("world").
	if len(root.Children) < 3 {
		t.Fatalf("expected >= 3 children, got %d", len(root.Children))
	}

	tb := root.Children[1]
	if tb.Kind() != ast.KindThematicBreak {
		t.Fatalf("expected ThematicBreak, got %v", tb.Kind())
	}

	// "* * *" starts at offset 7 (after "hello\n\n").
	if tb.Span().Start.Offset != 7 {
		t.Errorf("thematic break Start.Offset = %d, want 7", tb.Span().Start.Offset)
	}
	// "* * *" ends at offset 12.
	if tb.Span().End.Offset != 12 {
		t.Errorf("thematic break End.Offset = %d, want 12", tb.Span().End.Offset)
	}
}

func TestPositionList(t *testing.T) {
	input := "- one\n- two"
	doc := djot.Parse(input)

	root := doc.Root()
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	list, ok := root.Children[0].(*ast.BulletList)
	if !ok {
		t.Fatalf("expected *BulletList, got %T", root.Children[0])
	}

	if list.Span().Start.Offset != 0 {
		t.Errorf("list Start.Offset = %d, want 0", list.Span().Start.Offset)
	}
	// "- one\n- two" ends at offset 11.
	if list.Span().End.Offset != 11 {
		t.Errorf("list End.Offset = %d, want 11", list.Span().End.Offset)
	}

	// Check items.
	if len(list.Items) < 2 {
		t.Fatalf("expected >= 2 items, got %d", len(list.Items))
	}

	item1 := list.Items[0]
	item2 := list.Items[1]

	if item1.Span().Start.Offset != 0 {
		t.Errorf("item1 Start.Offset = %d, want 0", item1.Span().Start.Offset)
	}
	if item2.Span().Start.Offset != 6 {
		t.Errorf("item2 Start.Offset = %d, want 6", item2.Span().Start.Offset)
	}
}

func TestPositionBlockQuote(t *testing.T) {
	input := "> hello"
	doc := djot.Parse(input)

	root := doc.Root()
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	bq := root.Children[0]
	if bq.Kind() != ast.KindBlockQuote {
		t.Fatalf("expected BlockQuote, got %v", bq.Kind())
	}

	if bq.Span().Start.Offset != 0 {
		t.Errorf("block quote Start.Offset = %d, want 0", bq.Span().Start.Offset)
	}
	// "> hello" ends at offset 7.
	if bq.Span().End.Offset != 7 {
		t.Errorf("block quote End.Offset = %d, want 7", bq.Span().End.Offset)
	}
}

func TestPositionDiv(t *testing.T) {
	input := "::: warning\nhello\n:::"
	doc := djot.Parse(input)

	root := doc.Root()
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	div := root.Children[0]
	if div.Kind() != ast.KindDiv {
		t.Fatalf("expected Div, got %v", div.Kind())
	}

	if div.Span().Start.Offset != 0 {
		t.Errorf("div Start.Offset = %d, want 0", div.Span().Start.Offset)
	}
	// The Div ends at the closing ":::" at offset 21.
	if div.Span().End.Offset != 21 {
		t.Errorf("div End.Offset = %d, want 21", div.Span().End.Offset)
	}
}

// TestPositionSubParsers verifies that content inside containers (blockquotes,
// divs, list items, etc.) has positions that refer back to the original source,
// not the reconstructed sub-parser input.
func TestPositionSubParsers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "blockquote content positions",
			input: "> hello\n> world",
			expected: `doc
  block_quote (1:1:0-3:0:15)
    para (1:3:2-3:0:15)
      str (1:3:2-1:7:6) text="hello"
      soft_break (1:8:7-2:0:7)
      str (2:1:8-2:5:12) text="world"`,
		},
		{
			name:  "div content positions",
			input: "::: note\nfirst\n\nsecond\n:::",
			expected: `doc
  div (1:1:0-6:0:26) class="note"
    para (2:1:9-3:0:14)
      str (2:1:9-2:5:13) text="first"
    para (4:1:16-5:0:22)
      str (4:1:16-4:6:21) text="second"`,
		},
		{
			name:  "bullet list item content positions",
			input: " - alpha\n - beta",
			expected: `doc
  bullet_list (1:2:1-3:0:16) tight=true style="-"
    list_item (1:2:1-2:0:8)
      para (1:4:3-2:0:8)
        str (1:4:3-1:8:7) text="alpha"
    list_item (2:2:10-3:0:16)
      para (2:4:12-3:0:16)
        str (2:4:12-2:7:15) text="beta"`,
		},
		{
			name:  "ordered list item content positions",
			input: "1. first\n2. second",
			expected: `doc
  ordered_list (1:1:0-3:0:18) tight=true style="1."
    list_item (1:1:0-2:0:8)
      para (1:4:3-2:0:8)
        str (1:4:3-1:8:7) text="first"
    list_item (2:1:9-3:0:18)
      para (2:4:12-3:0:18)
        str (2:4:12-2:9:17) text="second"`,
		},
		{
			name:  "nested blockquote in list",
			input: "- > quoted",
			expected: `doc
  bullet_list (1:1:0-2:0:10) tight=true style="-"
    list_item (1:1:0-2:0:10)
      block_quote (1:3:2-2:0:10)
        para (1:6:5-2:0:10)
          str (1:6:5-2:0:10) text="quoted"`,
		},
		{
			name:  "long line column numbers",
			input: "This has *emphasis* at column ten and a [link](https://example.com) further along.",
			expected: `doc
  para (1:1:0-2:0:82)
    str (1:1:0-1:9:8) text="This has "
    strong (1:10:9-1:19:18)
      str (1:11:10-1:18:17) text="emphasis"
    str (1:20:19-1:40:39) text=" at column ten and a "
    link (1:41:40-1:67:66) destination="https://example.com"
      str (1:42:41-1:45:44) text="link"
    str (1:68:67-1:81:80) text=" further along."`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.input)
			got := trimTrailingNewline(djot.RenderAST(doc, true))
			expected := trimTrailingNewline(tc.expected)
			if got != expected {
				t.Errorf("input: %q\n\nexpected:\n%s\n\ngot:\n%s", tc.input, expected, got)
			}
		})
	}
}
