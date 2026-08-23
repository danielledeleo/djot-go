package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestNodeKindRanges guards the iota ordering that IsBlock/IsInline depend on.
// If you add a new Kind, add it to the correct group here. If this test
// breaks, you've probably inserted a kind in the wrong position.
func TestNodeKindRanges(t *testing.T) {
	blocks := []djot.Kind{
		djot.KindSection, djot.KindParagraph, djot.KindHeading,
		djot.KindThematicBreak, djot.KindCodeBlock, djot.KindRawBlock,
		djot.KindBlockQuote, djot.KindDiv,
		djot.KindBulletList, djot.KindOrderedList, djot.KindTaskList,
		djot.KindListItem, djot.KindTaskListItem,
		djot.KindDefinitionList, djot.KindTerm, djot.KindDefinition,
		djot.KindTable, djot.KindTableRow, djot.KindTableCell, djot.KindCaption,
		djot.KindFootnote,
	}
	if djot.KindDocument.IsBlock() || djot.KindDocument.IsInline() {
		t.Error("document kind must be neither Block nor Inline")
	}
	inlines := []djot.Kind{
		djot.KindText, djot.KindSoftBreak, djot.KindHardBreak, djot.KindNonBreakingSpace,
		djot.KindEmphasis, djot.KindStrong, djot.KindSuperscript, djot.KindSubscript,
		djot.KindInsert, djot.KindDelete, djot.KindMark,
		djot.KindLink, djot.KindImage, djot.KindSpan,
		djot.KindVerbatim, djot.KindInlineMath, djot.KindDisplayMath, djot.KindRawInline,
		djot.KindSymbol, djot.KindFootnoteReference,
		djot.KindDoubleQuoted, djot.KindSingleQuoted,
		djot.KindEllipsis, djot.KindEmDash, djot.KindEnDash,
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
