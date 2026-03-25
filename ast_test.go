package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestNodeKindRanges guards the iota ordering that IsBlock/IsInline depend on.
// If you add a new NodeKind, add it to the correct group here. If this test
// breaks, you've probably inserted a kind in the wrong position.
func TestNodeKindRanges(t *testing.T) {
	blocks := []djot.NodeKind{
		djot.Document, djot.Section, djot.Paragraph, djot.Heading,
		djot.ThematicBreak, djot.CodeBlock, djot.RawBlock,
		djot.BlockQuote, djot.Div,
		djot.BulletList, djot.OrderedList, djot.TaskList,
		djot.ListItem, djot.TaskListItem,
		djot.DefinitionList, djot.Term, djot.Definition,
		djot.Table, djot.TableRow, djot.TableCell, djot.Caption,
		djot.Footnote,
	}
	inlines := []djot.NodeKind{
		djot.Text, djot.SoftBreak, djot.HardBreak, djot.NonBreakingSpace,
		djot.Emphasis, djot.Strong, djot.Superscript, djot.Subscript,
		djot.Insert, djot.Delete, djot.Mark,
		djot.Link, djot.Image, djot.Span,
		djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.RawInline,
		djot.Symbol, djot.FootnoteReference,
		djot.DoubleQuoted, djot.SingleQuoted,
		djot.Ellipsis, djot.EmDash, djot.EnDash,
	}

	for _, k := range blocks {
		if !k.IsBlock() {
			t.Errorf("%s (value %d) should be a block kind", k, k)
		}
		if k.IsInline() {
			t.Errorf("%s (value %d) should not be an inline kind", k, k)
		}
	}
	for _, k := range inlines {
		if !k.IsInline() {
			t.Errorf("%s (value %d) should be an inline kind", k, k)
		}
		if k.IsBlock() {
			t.Errorf("%s (value %d) should not be a block kind", k, k)
		}
	}

	// Verify the lists are exhaustive: last block + 1 == first inline.
	lastBlock := blocks[len(blocks)-1]
	firstInline := inlines[0]
	if int(lastBlock)+1 != int(firstInline) {
		t.Errorf("gap between block and inline kinds: %s (%d) and %s (%d)",
			lastBlock, lastBlock, firstInline, firstInline)
	}
}
