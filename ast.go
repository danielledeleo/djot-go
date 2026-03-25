package djot

// NodeKind identifies the type of an AST node.
type NodeKind int

const (
	// Block-level node kinds.

	Document       NodeKind = iota // Document is the root node of a parsed djot AST.
	Section                        // Section wraps a heading and all content under it until the next heading of equal or higher level.
	Paragraph                      // Paragraph is a block of inline content separated by blank lines.
	Heading                        // Heading is a section heading (level 1-6).
	ThematicBreak                  // ThematicBreak is a horizontal rule (* * * or similar).
	CodeBlock                      // CodeBlock is a fenced or indented code block.
	RawBlock                       // RawBlock is a raw block passed through in a specific output format.
	BlockQuote                     // BlockQuote is a quoted block (> prefix).
	Div                            // Div is a generic block container (fenced with :::).

	BulletList                     // BulletList is an unordered list.
	OrderedList                    // OrderedList is a numbered list.
	TaskList                       // TaskList is a list of checkbox items.
	ListItem                       // ListItem is an item in a BulletList or OrderedList.
	TaskListItem                   // TaskListItem is an item in a TaskList with a checkbox.

	DefinitionList                 // DefinitionList is a list of term/definition pairs.
	Term                           // Term is the term in a DefinitionList entry.
	Definition                     // Definition is the definition body in a DefinitionList entry.

	Table                          // Table is a pipe table.
	TableRow                       // TableRow is a row in a Table.
	TableCell                      // TableCell is a cell in a TableRow.
	Caption                        // Caption is a table caption.

	Footnote                       // Footnote is a footnote definition block.

	// Inline-level node kinds.

	Text                           // Text is a run of literal text.
	SoftBreak                      // SoftBreak is a newline within a paragraph (typically rendered as a space).
	HardBreak                      // HardBreak is an explicit line break (backslash at end of line).
	NonBreakingSpace               // NonBreakingSpace is a non-breaking space (\ followed by a space).
	Emphasis                       // Emphasis is emphasized (italic) text (_..._).
	Strong                         // Strong is strongly emphasized (bold) text (*...*).
	Superscript                    // Superscript is superscripted text (^...^).
	Subscript                      // Subscript is subscripted text (~...~).
	Insert                         // Insert marks inserted text ({+...+}).
	Delete                         // Delete marks deleted text ({-...-}).
	Mark                           // Mark is highlighted text ({=...=}).
	Link                           // Link is a hyperlink.
	Image                          // Image is an inline image.
	Span                           // Span is a generic inline container ([content]{attrs}).
	Verbatim                       // Verbatim is inline code (`...`).
	InlineMath                     // InlineMath is inline LaTeX math ($...$).
	DisplayMath                    // DisplayMath is display-mode LaTeX math ($$...$$).
	RawInline                      // RawInline is raw inline content in a specific output format.
	Symbol                         // Symbol is a symbolic name (:name:).
	FootnoteReference              // FootnoteReference is an inline reference to a footnote (^[label]).
	DoubleQuoted                   // DoubleQuoted is smart double-quoted text ("...").
	SingleQuoted                   // SingleQuoted is smart single-quoted text ('...').

	// Smart punctuation node kinds.

	Ellipsis                       // Ellipsis represents a smart ellipsis (...).
	EmDash                         // EmDash represents a smart em-dash (---).
	EnDash                         // EnDash represents a smart en-dash (--).
)

// Pos identifies a byte position in a source file. Use [Doc.Position] to
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

// Node is the single AST node type for all djot elements.
// Kind-specific fields are zero-valued when not applicable.
type Node struct {
	Kind      NodeKind
	Children  []*Node
	Attrs     map[string]string
	AttrOrder []string // tracks insertion order of attribute keys
	Start     Pos
	End       Pos

	// Text content (Text, Verbatim, RawInline, RawBlock, CodeBlock)
	Text string

	// Heading
	Level int

	// Link, Image
	Target    string
	HasTarget bool // true when target was explicitly set (even if empty)

	// Symbol
	Name string

	// CodeBlock
	Lang string

	// RawBlock, RawInline
	Format string

	// BulletList
	Marker byte // list marker character (e.g., '-', '*', '+')

	// OrderedList
	ListStyle ListStyle
	ListStart int // starting number

	// TaskListItem
	Checked bool

	// TableCell
	CellAlign CellAlign
	IsHeader  bool

	// FootnoteReference
	Label string
}

// Attr returns the value of an attribute, or empty string if absent.
func (n *Node) Attr(key string) string {
	if n.Attrs == nil {
		return ""
	}
	return n.Attrs[key]
}

// SetAttr sets an attribute on the node, allocating the map if needed.
// Tracks insertion order for deterministic rendering.
func (n *Node) SetAttr(key, value string) {
	if n.Attrs == nil {
		n.Attrs = make(map[string]string)
	}
	if _, exists := n.Attrs[key]; !exists {
		n.AttrOrder = append(n.AttrOrder, key)
	}
	n.Attrs[key] = value
}

// AddClass appends a class to the node's class attribute.
func (n *Node) AddClass(class string) {
	if existing := n.Attr("class"); existing != "" {
		n.SetAttr("class", existing+" "+class)
	} else {
		n.SetAttr("class", class)
	}
}

// Doc is the top-level result of parsing a djot document, containing
// the AST root, source file information, and collected footnotes and references.
type Doc struct {
	Root       *Node
	Files      []FileInfo
	Footnotes  map[string]*Node // label → Footnote node
	References map[string]*Node // label → reference definition
}

// Position resolves a Pos to a filename, line, and column.
func (d *Doc) Position(p Pos) (file string, line, col int) {
	fi := &d.Files[p.File]
	line, col = fi.Position(p.Offset)
	return fi.Path, line, col
}
