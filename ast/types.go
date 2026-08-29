package ast

// Kind identifies the type of a syntax-tree node.
type Kind int

const (
	// Block-level node kinds.

	KindDocument      Kind = iota // KindDocument is the root node of a parsed djot AST.
	KindSection                   // KindSection wraps a heading and all content under it until the next heading of equal or higher level.
	KindParagraph                 // KindParagraph is a block of inline content separated by blank lines.
	KindHeading                   // KindHeading is a section heading (level 1-6).
	KindThematicBreak             // KindThematicBreak is a horizontal rule (* * * or similar).
	KindCodeBlock                 // KindCodeBlock is a fenced or indented code block.
	KindRawBlock                  // KindRawBlock is a raw block passed through in a specific output format.
	KindBlockQuote                // KindBlockQuote is a quoted block (> prefix).
	KindDiv                       // KindDiv is a generic block container (fenced with :::).

	KindBulletList   // KindBulletList is an unordered list.
	KindOrderedList  // KindOrderedList is a numbered list.
	KindTaskList     // KindTaskList is a list of checkbox items.
	KindListItem     // KindListItem is an item in a KindBulletList or KindOrderedList.
	KindTaskListItem // KindTaskListItem is an item in a KindTaskList with a checkbox.

	KindDefinitionList // KindDefinitionList is a list of term/definition pairs.
	KindTerm           // KindTerm is the term in a KindDefinitionList entry.
	KindDefinition     // KindDefinition is the definition body in a KindDefinitionList entry.

	KindTable     // KindTable is a pipe table.
	KindTableRow  // KindTableRow is a row in a KindTable.
	KindTableCell // KindTableCell is a cell in a KindTableRow.
	KindCaption   // KindCaption is a table caption.

	KindFootnote // KindFootnote is a footnote definition block.

	// Inline-level node kinds.

	KindText              // KindText is a run of literal text.
	KindSoftBreak         // KindSoftBreak is a newline within a paragraph (typically rendered as a space).
	KindHardBreak         // KindHardBreak is an explicit line break (backslash at end of line).
	KindNonBreakingSpace  // KindNonBreakingSpace is a non-breaking space (\ followed by a space).
	KindEmphasis          // KindEmphasis is emphasized (italic) text (_..._).
	KindStrong            // KindStrong is strongly emphasized (bold) text (*...*).
	KindSuperscript       // KindSuperscript is superscripted text (^...^).
	KindSubscript         // KindSubscript is subscripted text (~...~).
	KindInsert            // KindInsert marks inserted text ({+...+}).
	KindDelete            // KindDelete marks deleted text ({-...-}).
	KindMark              // KindMark is highlighted text ({=...=}).
	KindLink              // KindLink is a hyperlink.
	KindImage             // KindImage is an inline image.
	KindSpan              // KindSpan is a generic inline container ([content]{attrs}).
	KindVerbatim          // KindVerbatim is inline code (`...`).
	KindInlineMath        // KindInlineMath is inline LaTeX math ($...$).
	KindDisplayMath       // KindDisplayMath is display-mode LaTeX math ($$...$$).
	KindRawInline         // KindRawInline is raw inline content in a specific output format.
	KindSymbol            // KindSymbol is a symbolic name (:name:).
	KindFootnoteReference // KindFootnoteReference is an inline reference to a footnote (^[label]).
	KindDoubleQuoted      // KindDoubleQuoted is smart double-quoted text ("...").
	KindSingleQuoted      // KindSingleQuoted is smart single-quoted text ('...').

	// Smart punctuation node kinds.

	KindEllipsis // KindEllipsis represents a smart ellipsis (...).
	KindEmDash   // KindEmDash represents a smart em-dash (---).
	KindEnDash   // KindEnDash represents a smart en-dash (--).
)

// Pos identifies a byte position in a source file. Use djot.Doc.Position to
// resolve it to a human-readable filename, line, and column.
type Pos struct {
	File   FileID
	Offset int
}

// FileID identifies a source file in the document's file table.
type FileID uint16

// FileInfo describes a source file used during parsing.
type FileInfo struct {
	Path       string
	Source     []byte
	lineStarts []int // lazily computed
}

// Position resolves a byte offset to a 1-based line and column.
func (fi *FileInfo) Position(offset int) (line, col int) {
	fi.ensureLineStarts()
	// Binary search for the line containing offset.
	lo, hi := 0, len(fi.lineStarts)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if fi.lineStarts[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	line = lo // 1-based: lineStarts[0] is line 1
	col = offset - fi.lineStarts[line-1] + 1
	return line, col
}

func (fi *FileInfo) ensureLineStarts() {
	if fi.lineStarts != nil {
		return
	}
	fi.lineStarts = []int{0}
	for i, b := range fi.Source {
		if b == '\n' {
			fi.lineStarts = append(fi.lineStarts, i+1)
		}
	}
}

// ListStyle describes the marker type for an ordered list.
type ListStyle int

const (
	ListDecimal    ListStyle = iota // ListDecimal uses decimal numbering (1. 2. 3.).
	ListAlphaLower                  // ListAlphaLower uses lowercase letters (a. b. c.).
	ListAlphaUpper                  // ListAlphaUpper uses uppercase letters (A. B. C.).
	ListRomanLower                  // ListRomanLower uses lowercase Roman numerals (i. ii. iii.).
	ListRomanUpper                  // ListRomanUpper uses uppercase Roman numerals (I. II. III.).
)

// CellAlign describes horizontal alignment in a table cell.
type CellAlign int

const (
	AlignDefault CellAlign = iota // AlignDefault indicates no explicit alignment.
	AlignLeft                     // AlignLeft aligns cell content to the left.
	AlignRight                    // AlignRight aligns cell content to the right.
	AlignCenter                   // AlignCenter centers cell content.
)
