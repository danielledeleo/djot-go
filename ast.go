package djot

// NodeKind identifies the type of an AST node.
type NodeKind int

const (
	// Block nodes

	Document       NodeKind = iota
	Section                 // wraps heading + content under it
	Paragraph
	Heading
	ThematicBreak
	CodeBlock
	RawBlock
	BlockQuote
	Div

	BulletList
	OrderedList
	TaskList
	ListItem
	TaskListItem

	DefinitionList
	Term
	Definition

	Table
	TableRow
	TableCell
	Caption

	Footnote

	// Inline nodes

	Text
	SoftBreak
	HardBreak
	NonBreakingSpace
	Emphasis
	Strong
	Superscript
	Subscript
	Insert
	Delete
	Mark
	Link
	Image
	Span
	Verbatim
	InlineMath
	DisplayMath
	RawInline
	Symbol
	FootnoteReference
	DoubleQuoted
	SingleQuoted

	// Smart punctuation
	Ellipsis
	EmDash
	EnDash
)

// Pos identifies a position in a source file.
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
	ListDecimal    ListStyle = iota // 1. 2. 3.
	ListAlphaLower                  // a. b. c.
	ListAlphaUpper                  // A. B. C.
	ListRomanLower                  // i. ii. iii.
	ListRomanUpper                  // I. II. III.
)

// CellAlign describes horizontal alignment in a table cell.
type CellAlign int

const (
	AlignDefault CellAlign = iota
	AlignLeft
	AlignRight
	AlignCenter
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

// Doc is the top-level parse result.
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
