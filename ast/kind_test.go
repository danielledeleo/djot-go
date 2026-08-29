package ast_test

import (
	"testing"

	"github.com/danielledeleo/djot-go/ast"
)

// TestNodeKindRanges guards the iota ordering that IsBlock/IsInline depend on.
// If you add a new Kind, add it to the correct group here. If this test
// breaks, you've probably inserted a kind in the wrong position.
func TestNodeKindRanges(t *testing.T) {
	blocks := []ast.Kind{
		ast.KindSection, ast.KindParagraph, ast.KindHeading,
		ast.KindThematicBreak, ast.KindCodeBlock, ast.KindRawBlock,
		ast.KindBlockQuote, ast.KindDiv,
		ast.KindBulletList, ast.KindOrderedList, ast.KindTaskList,
		ast.KindListItem, ast.KindTaskListItem,
		ast.KindDefinitionList, ast.KindTerm, ast.KindDefinition,
		ast.KindTable, ast.KindTableRow, ast.KindTableCell, ast.KindCaption,
		ast.KindFootnote,
	}
	if ast.KindDocument.IsBlock() || ast.KindDocument.IsInline() {
		t.Error("document kind must be neither Block nor Inline")
	}
	inlines := []ast.Kind{
		ast.KindText, ast.KindSoftBreak, ast.KindHardBreak, ast.KindNonBreakingSpace,
		ast.KindEmphasis, ast.KindStrong, ast.KindSuperscript, ast.KindSubscript,
		ast.KindInsert, ast.KindDelete, ast.KindMark,
		ast.KindLink, ast.KindImage, ast.KindSpan,
		ast.KindVerbatim, ast.KindInlineMath, ast.KindDisplayMath, ast.KindRawInline,
		ast.KindSymbol, ast.KindFootnoteReference,
		ast.KindDoubleQuoted, ast.KindSingleQuoted,
		ast.KindEllipsis, ast.KindEmDash, ast.KindEnDash,
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
