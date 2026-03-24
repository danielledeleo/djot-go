package djot_test

import (
	"testing"

	"github.com/danielledeleo/homedoc/djot"
)

func TestPositionTracking(t *testing.T) {
	input := "# Hello\n\nworld"
	doc := djot.Parse(input)

	// The AST should be: Document > Section > [Heading, Paragraph]
	root := doc.Root
	if root.Kind != djot.Document {
		t.Fatalf("expected Document, got %v", root.Kind)
	}
	if root.Start.Offset != 0 {
		t.Errorf("document Start.Offset = %d, want 0", root.Start.Offset)
	}

	// Find the section (headings get wrapped).
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}
	section := root.Children[0]
	if section.Kind != djot.Section {
		t.Fatalf("expected Section, got %v", section.Kind)
	}

	if len(section.Children) < 2 {
		t.Fatalf("section has %d children, want >= 2", len(section.Children))
	}

	heading := section.Children[0]
	para := section.Children[1]

	if heading.Kind != djot.Heading {
		t.Fatalf("expected Heading, got %v", heading.Kind)
	}
	if para.Kind != djot.Paragraph {
		t.Fatalf("expected Paragraph, got %v", para.Kind)
	}

	// Heading "# Hello" starts at offset 0.
	if heading.Start.Offset != 0 {
		t.Errorf("heading Start.Offset = %d, want 0", heading.Start.Offset)
	}
	// Heading ends at offset 7 (exclusive end of "# Hello" line).
	if heading.End.Offset != 7 {
		t.Errorf("heading End.Offset = %d, want 7", heading.End.Offset)
	}

	// Paragraph "world" starts at offset 9 (after "# Hello\n\n").
	if para.Start.Offset != 9 {
		t.Errorf("paragraph Start.Offset = %d, want 9", para.Start.Offset)
	}
	// Paragraph ends at offset 14 (exclusive end of "world").
	if para.End.Offset != 14 {
		t.Errorf("paragraph End.Offset = %d, want 14", para.End.Offset)
	}

	// Section should inherit heading start and paragraph end.
	if section.Start.Offset != heading.Start.Offset {
		t.Errorf("section Start.Offset = %d, want %d (heading start)",
			section.Start.Offset, heading.Start.Offset)
	}
	if section.End.Offset != para.End.Offset {
		t.Errorf("section End.Offset = %d, want %d (paragraph end)",
			section.End.Offset, para.End.Offset)
	}

	// Verify that inline nodes within the heading also have positions.
	if len(heading.Children) == 0 {
		t.Fatal("heading has no inline children")
	}
	textNode := heading.Children[0]
	if textNode.Kind != djot.Text {
		t.Fatalf("expected Text child, got %v", textNode.Kind)
	}
	if textNode.Start.Offset == 0 && textNode.End.Offset == 0 {
		t.Error("heading text node should have non-zero positions")
	}
}

func TestPositionCodeBlock(t *testing.T) {
	input := "```go\nfmt.Println()\n```"
	doc := djot.Parse(input)

	root := doc.Root
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	cb := root.Children[0]
	if cb.Kind != djot.CodeBlock {
		t.Fatalf("expected CodeBlock, got %v", cb.Kind)
	}

	if cb.Start.Offset != 0 {
		t.Errorf("code block Start.Offset = %d, want 0", cb.Start.Offset)
	}
	// Code block ends at the closing fence "```" which ends at offset 21.
	if cb.End.Offset == 0 {
		t.Error("code block End.Offset should be non-zero")
	}
}

func TestPositionThematicBreak(t *testing.T) {
	input := "hello\n\n* * *\n\nworld"
	doc := djot.Parse(input)

	root := doc.Root
	// Should have: Paragraph("hello"), ThematicBreak, Paragraph("world")
	if len(root.Children) < 3 {
		t.Fatalf("expected >= 3 children, got %d", len(root.Children))
	}

	tb := root.Children[1]
	if tb.Kind != djot.ThematicBreak {
		t.Fatalf("expected ThematicBreak, got %v", tb.Kind)
	}

	// "* * *" starts at offset 7 (after "hello\n\n").
	if tb.Start.Offset != 7 {
		t.Errorf("thematic break Start.Offset = %d, want 7", tb.Start.Offset)
	}
	if tb.End.Offset == 0 {
		t.Error("thematic break End.Offset should be non-zero")
	}
}

func TestPositionList(t *testing.T) {
	input := "- one\n- two"
	doc := djot.Parse(input)

	root := doc.Root
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	list := root.Children[0]
	if list.Kind != djot.BulletList {
		t.Fatalf("expected BulletList, got %v", list.Kind)
	}

	if list.Start.Offset != 0 {
		t.Errorf("list Start.Offset = %d, want 0", list.Start.Offset)
	}
	if list.End.Offset == 0 {
		t.Error("list End.Offset should be non-zero")
	}

	// Check items.
	if len(list.Children) < 2 {
		t.Fatalf("expected >= 2 items, got %d", len(list.Children))
	}

	item1 := list.Children[0]
	item2 := list.Children[1]

	if item1.Start.Offset != 0 {
		t.Errorf("item1 Start.Offset = %d, want 0", item1.Start.Offset)
	}
	if item2.Start.Offset != 6 {
		t.Errorf("item2 Start.Offset = %d, want 6", item2.Start.Offset)
	}
}

func TestPositionBlockQuote(t *testing.T) {
	input := "> hello"
	doc := djot.Parse(input)

	root := doc.Root
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	bq := root.Children[0]
	if bq.Kind != djot.BlockQuote {
		t.Fatalf("expected BlockQuote, got %v", bq.Kind)
	}

	if bq.Start.Offset != 0 {
		t.Errorf("block quote Start.Offset = %d, want 0", bq.Start.Offset)
	}
	if bq.End.Offset == 0 {
		t.Error("block quote End.Offset should be non-zero")
	}
}

func TestPositionDiv(t *testing.T) {
	input := "::: warning\nhello\n:::"
	doc := djot.Parse(input)

	root := doc.Root
	if len(root.Children) == 0 {
		t.Fatal("document has no children")
	}

	div := root.Children[0]
	if div.Kind != djot.Div {
		t.Fatalf("expected Div, got %v", div.Kind)
	}

	if div.Start.Offset != 0 {
		t.Errorf("div Start.Offset = %d, want 0", div.Start.Offset)
	}
	// Div ends at closing ":::" which ends at offset 20.
	if div.End.Offset == 0 {
		t.Error("div End.Offset should be non-zero")
	}
}
