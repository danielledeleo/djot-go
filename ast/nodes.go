package ast

// SourceSpan identifies the half-open source range occupied by a node: Start is
// inclusive and End is exclusive.
type SourceSpan struct {
	Start Pos
	End   Pos
}

// Attribute is one ordered node attribute.
type Attribute struct {
	Key   string
	Value string
}

// Attributes stores node attributes in insertion order. Djot nodes generally
// have very few attributes, so a compact slice avoids a map and a second order
// index while keeping deterministic serialization.
type Attributes struct {
	items []Attribute
}

// Len returns the number of attributes.
func (a *Attributes) Len() int {
	if a == nil {
		return 0
	}
	return len(a.items)
}

// Get returns the value associated with key, or an empty string when absent.
func (a *Attributes) Get(key string) string {
	value, _ := a.Lookup(key)
	return value
}

// Lookup returns the value associated with key and whether the key is present.
func (a *Attributes) Lookup(key string) (string, bool) {
	if a == nil {
		return "", false
	}
	for i := range a.items {
		if a.items[i].Key == key {
			return a.items[i].Value, true
		}
	}
	return "", false
}

// Set inserts or replaces an attribute. It returns false without modifying the
// collection when key is not a valid Djot/HTML attribute name.
func (a *Attributes) Set(key, value string) bool {
	if a == nil || !isValidAttrKey(key) {
		return false
	}
	for i := range a.items {
		if a.items[i].Key == key {
			a.items[i].Value = value
			return true
		}
	}
	a.items = append(a.items, Attribute{Key: key, Value: value})
	return true
}

// Delete removes key and reports whether it was present.
func (a *Attributes) Delete(key string) bool {
	if a == nil {
		return false
	}
	for i := range a.items {
		if a.items[i].Key == key {
			copy(a.items[i:], a.items[i+1:])
			a.items = a.items[:len(a.items)-1]
			return true
		}
	}
	return false
}

// AddClass appends class to the space-separated class attribute.
func (a *Attributes) AddClass(class string) bool {
	if current := a.Get("class"); current != "" {
		return a.Set("class", current+" "+class)
	}
	return a.Set("class", class)
}

// Range calls fn for each attribute in insertion order. Iteration stops when
// fn returns false.
func (a *Attributes) Range(fn func(Attribute) bool) {
	if a == nil {
		return
	}
	for _, attribute := range a.items {
		if !fn(attribute) {
			return
		}
	}
}

// Entries returns an ordered copy of the attributes.
func (a *Attributes) Entries() []Attribute {
	if a == nil || len(a.items) == 0 {
		return nil
	}
	return append([]Attribute(nil), a.items...)
}

// Clone returns an independent copy of the attributes.
func (a *Attributes) Clone() Attributes {
	return Attributes{items: a.Entries()}
}

func isValidAttrKey(key string) bool {
	if key == "" || !isAttrKeyStart(key[0]) {
		return false
	}
	for i := 1; i < len(key); i++ {
		if !isAttrKeyChar(key[i]) {
			return false
		}
	}
	return true
}

func isAttrKeyStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == ':'
}

func isAttrKeyChar(c byte) bool {
	return isAttrKeyStart(c) || c >= '0' && c <= '9' || c == '-'
}

// Node is the closed interface implemented by every Djot syntax-tree node.
// All implementations are pointers to the concrete types in this package.
type Node interface {
	Kind() Kind
	Span() SourceSpan
	Attributes() *Attributes
	node()
	base() *nodeBase
}

// Block is a node valid in block-level child positions.
type Block interface {
	Node
	block()
}

// Inline is a node valid in inline child positions.
type Inline interface {
	Node
	inline()
}

type nodeBase struct {
	span  SourceSpan
	attrs Attributes
}

func (*nodeBase) node()                     {}
func (n *nodeBase) base() *nodeBase         { return n }
func (n *nodeBase) Span() SourceSpan        { return n.span }
func (n *nodeBase) Attributes() *Attributes { return &n.attrs }

type blockBase struct{ nodeBase }

func (*blockBase) block() {}

type inlineBase struct{ nodeBase }

func (*inlineBase) inline() {}

// SetSpan replaces node's source span. It panics when node is nil.
func SetSpan(node Node, span SourceSpan) {
	requireNode(node)
	node.base().span = span
}

// CopyMetadata copies the source span and an independent attribute value from
// src to dst. It is useful when a tree transformation replaces one node with
// another concrete type.
func CopyMetadata(dst, src Node) {
	requireNode(dst)
	requireNode(src)
	dst.base().span = src.base().span
	dst.base().attrs = src.base().attrs.Clone()
}

// Document is the root of a Djot syntax tree.
type Document struct {
	nodeBase
	Children []Block
}

func (*Document) Kind() Kind { return KindDocument }

// Section groups a heading and the blocks governed by it.
type Section struct {
	blockBase
	Children []Block
}

func (*Section) Kind() Kind { return KindSection }

// Paragraph contains inline content.
type Paragraph struct {
	blockBase
	Children []Inline
}

func (*Paragraph) Kind() Kind { return KindParagraph }

// Heading is a level-one through level-six section heading.
type Heading struct {
	blockBase
	Level    int
	Children []Inline
}

func (*Heading) Kind() Kind { return KindHeading }

// ThematicBreak is a horizontal thematic separator.
type ThematicBreak struct{ blockBase }

func (*ThematicBreak) Kind() Kind { return KindThematicBreak }

// CodeBlock contains fenced or indented code and an optional language.
type CodeBlock struct {
	blockBase
	Text     string
	Language string
}

func (*CodeBlock) Kind() Kind { return KindCodeBlock }

// RawBlock contains output-format-specific block content.
type RawBlock struct {
	blockBase
	Text   string
	Format string
}

func (*RawBlock) Kind() Kind { return KindRawBlock }

// BlockQuote contains quoted blocks.
type BlockQuote struct {
	blockBase
	Children []Block
}

func (*BlockQuote) Kind() Kind { return KindBlockQuote }

// Div is a generic block container.
type Div struct {
	blockBase
	Children []Block
}

func (*Div) Kind() Kind { return KindDiv }

// BulletList contains unordered list items.
type BulletList struct {
	blockBase
	Items  []*ListItem
	Tight  bool
	Marker byte
}

func (*BulletList) Kind() Kind { return KindBulletList }

// OrderedList contains numbered list items.
type OrderedList struct {
	blockBase
	Items []*ListItem
	Tight bool
	Style ListStyle
	Start int
}

func (*OrderedList) Kind() Kind { return KindOrderedList }

// TaskList contains checkbox list items.
type TaskList struct {
	blockBase
	Items []*TaskListItem
	Tight bool
}

func (*TaskList) Kind() Kind { return KindTaskList }

// ListItem contains the blocks in a bullet or ordered list item.
type ListItem struct {
	blockBase
	Children []Block
}

func (*ListItem) Kind() Kind { return KindListItem }

// TaskListItem contains the blocks and state of a checkbox item.
type TaskListItem struct {
	blockBase
	Children []Block
	Checked  bool
}

func (*TaskListItem) Kind() Kind { return KindTaskListItem }

// DefinitionList contains alternating terms and definitions.
type DefinitionList struct {
	blockBase
	Children []Block
	Tight    bool
}

func (*DefinitionList) Kind() Kind { return KindDefinitionList }

// Term contains the inline term of a definition-list entry.
type Term struct {
	blockBase
	Children []Inline
}

func (*Term) Kind() Kind { return KindTerm }

// Definition contains the block body of a definition-list entry.
type Definition struct {
	blockBase
	Children []Block
}

func (*Definition) Kind() Kind { return KindDefinition }

// Table contains an optional caption and table rows.
type Table struct {
	blockBase
	Children []Block
}

func (*Table) Kind() Kind { return KindTable }

// TableRow contains table cells.
type TableRow struct {
	blockBase
	Cells  []*TableCell
	Header bool
}

func (*TableRow) Kind() Kind { return KindTableRow }

// TableCell contains inline cell content and alignment metadata.
type TableCell struct {
	blockBase
	Children  []Inline
	Alignment CellAlign
	Header    bool
}

func (*TableCell) Kind() Kind { return KindTableCell }

// Caption contains inline table-caption content.
type Caption struct {
	blockBase
	Children []Inline
}

func (*Caption) Kind() Kind { return KindCaption }

// Footnote is a labeled block definition rendered as an endnote.
type Footnote struct {
	blockBase
	Label    string
	Children []Block
}

func (*Footnote) Kind() Kind { return KindFootnote }

// Text is a literal text run.
type Text struct {
	inlineBase
	Value string
}

func (*Text) Kind() Kind { return KindText }

// SoftBreak is a source newline within inline content.
type SoftBreak struct{ inlineBase }

func (*SoftBreak) Kind() Kind { return KindSoftBreak }

// HardBreak is an explicit line break.
type HardBreak struct{ inlineBase }

func (*HardBreak) Kind() Kind { return KindHardBreak }

// NonBreakingSpace is an explicit non-breaking space.
type NonBreakingSpace struct{ inlineBase }

func (*NonBreakingSpace) Kind() Kind { return KindNonBreakingSpace }

// Emphasis contains emphasized inline content.
type Emphasis struct {
	inlineBase
	Children []Inline
}

func (*Emphasis) Kind() Kind { return KindEmphasis }

// Strong contains strongly emphasized inline content.
type Strong struct {
	inlineBase
	Children []Inline
}

func (*Strong) Kind() Kind { return KindStrong }

// Superscript contains superscripted inline content.
type Superscript struct {
	inlineBase
	Children []Inline
}

func (*Superscript) Kind() Kind { return KindSuperscript }

// Subscript contains subscripted inline content.
type Subscript struct {
	inlineBase
	Children []Inline
}

func (*Subscript) Kind() Kind { return KindSubscript }

// Insert marks inserted inline content.
type Insert struct {
	inlineBase
	Children []Inline
}

func (*Insert) Kind() Kind { return KindInsert }

// Delete marks deleted inline content.
type Delete struct {
	inlineBase
	Children []Inline
}

func (*Delete) Kind() Kind { return KindDelete }

// Mark contains highlighted inline content.
type Mark struct {
	inlineBase
	Children []Inline
}

func (*Mark) Kind() Kind { return KindMark }

// Link contains linked inline content and an optional destination.
type Link struct {
	inlineBase
	Children       []Inline
	Destination    string
	DestinationSet bool
}

func (*Link) Kind() Kind { return KindLink }

// Image contains alternate inline content and an optional destination.
type Image struct {
	inlineBase
	Children       []Inline
	Destination    string
	DestinationSet bool
}

func (*Image) Kind() Kind { return KindImage }

// Span is a generic inline container.
type Span struct {
	inlineBase
	Children []Inline
}

func (*Span) Kind() Kind { return KindSpan }

// Verbatim is inline code.
type Verbatim struct {
	inlineBase
	Text string
}

func (*Verbatim) Kind() Kind { return KindVerbatim }

// InlineMath contains inline mathematical source.
type InlineMath struct {
	inlineBase
	Text string
}

func (*InlineMath) Kind() Kind { return KindInlineMath }

// DisplayMath contains display-mode mathematical source.
type DisplayMath struct {
	inlineBase
	Text string
}

func (*DisplayMath) Kind() Kind { return KindDisplayMath }

// RawInline contains output-format-specific inline content.
type RawInline struct {
	inlineBase
	Text   string
	Format string
}

func (*RawInline) Kind() Kind { return KindRawInline }

// Symbol is a symbolic shortcode such as :name:.
type Symbol struct {
	inlineBase
	Name string
}

func (*Symbol) Kind() Kind { return KindSymbol }

// FootnoteReference refers to a labeled footnote definition.
type FootnoteReference struct {
	inlineBase
	Label string
}

func (*FootnoteReference) Kind() Kind { return KindFootnoteReference }

// DoubleQuoted contains smart double-quoted inline content.
type DoubleQuoted struct {
	inlineBase
	Children []Inline
}

func (*DoubleQuoted) Kind() Kind { return KindDoubleQuoted }

// SingleQuoted contains smart single-quoted inline content.
type SingleQuoted struct {
	inlineBase
	Children []Inline
}

func (*SingleQuoted) Kind() Kind { return KindSingleQuoted }

// Ellipsis is smart ellipsis punctuation.
type Ellipsis struct{ inlineBase }

func (*Ellipsis) Kind() Kind { return KindEllipsis }

// EmDash is smart em-dash punctuation.
type EmDash struct{ inlineBase }

func (*EmDash) Kind() Kind { return KindEmDash }

// EnDash is smart en-dash punctuation.
type EnDash struct{ inlineBase }

func (*EnDash) Kind() Kind { return KindEnDash }

// Reference is a resolved link reference definition. It is document metadata,
// not a node in the syntax tree.
type Reference struct {
	Destination    string
	DestinationSet bool
	Attributes     Attributes
}

var (
	_ Node   = (*Document)(nil)
	_ Block  = (*Section)(nil)
	_ Block  = (*Paragraph)(nil)
	_ Block  = (*Heading)(nil)
	_ Block  = (*ThematicBreak)(nil)
	_ Block  = (*CodeBlock)(nil)
	_ Block  = (*RawBlock)(nil)
	_ Block  = (*BlockQuote)(nil)
	_ Block  = (*Div)(nil)
	_ Block  = (*BulletList)(nil)
	_ Block  = (*OrderedList)(nil)
	_ Block  = (*TaskList)(nil)
	_ Block  = (*ListItem)(nil)
	_ Block  = (*TaskListItem)(nil)
	_ Block  = (*DefinitionList)(nil)
	_ Block  = (*Term)(nil)
	_ Block  = (*Definition)(nil)
	_ Block  = (*Table)(nil)
	_ Block  = (*TableRow)(nil)
	_ Block  = (*TableCell)(nil)
	_ Block  = (*Caption)(nil)
	_ Block  = (*Footnote)(nil)
	_ Inline = (*Text)(nil)
	_ Inline = (*SoftBreak)(nil)
	_ Inline = (*HardBreak)(nil)
	_ Inline = (*NonBreakingSpace)(nil)
	_ Inline = (*Emphasis)(nil)
	_ Inline = (*Strong)(nil)
	_ Inline = (*Superscript)(nil)
	_ Inline = (*Subscript)(nil)
	_ Inline = (*Insert)(nil)
	_ Inline = (*Delete)(nil)
	_ Inline = (*Mark)(nil)
	_ Inline = (*Link)(nil)
	_ Inline = (*Image)(nil)
	_ Inline = (*Span)(nil)
	_ Inline = (*Verbatim)(nil)
	_ Inline = (*InlineMath)(nil)
	_ Inline = (*DisplayMath)(nil)
	_ Inline = (*RawInline)(nil)
	_ Inline = (*Symbol)(nil)
	_ Inline = (*FootnoteReference)(nil)
	_ Inline = (*DoubleQuoted)(nil)
	_ Inline = (*SingleQuoted)(nil)
	_ Inline = (*Ellipsis)(nil)
	_ Inline = (*EmDash)(nil)
	_ Inline = (*EnDash)(nil)
)
