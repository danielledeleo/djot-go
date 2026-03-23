package djot

var kindNames = [...]string{
	Document:           "document",
	Section:            "section",
	Paragraph:          "paragraph",
	Heading:            "heading",
	ThematicBreak:      "thematic_break",
	CodeBlock:          "code_block",
	RawBlock:           "raw_block",
	BlockQuote:         "block_quote",
	Div:                "div",
	BulletList:         "bullet_list",
	OrderedList:        "ordered_list",
	TaskList:           "task_list",
	ListItem:           "list_item",
	TaskListItem:       "task_list_item",
	DefinitionList:     "definition_list",
	Term:               "term",
	Definition:         "definition",
	Table:              "table",
	TableRow:           "table_row",
	TableCell:          "table_cell",
	Footnote:           "footnote",
	Text:               "text",
	SoftBreak:          "soft_break",
	HardBreak:          "hard_break",
	NonBreakingSpace:   "non_breaking_space",
	Emphasis:           "emphasis",
	Strong:             "strong",
	Superscript:        "superscript",
	Subscript:          "subscript",
	Insert:             "insert",
	Delete:             "delete",
	Mark:               "mark",
	Link:               "link",
	Image:              "image",
	Span:               "span",
	Verbatim:           "verbatim",
	InlineMath:         "inline_math",
	DisplayMath:        "display_math",
	RawInline:          "raw_inline",
	Symbol:             "symbol",
	FootnoteReference:  "footnote_reference",
	DoubleQuoted:       "double_quoted",
	SingleQuoted:       "single_quoted",
	Ellipsis:           "ellipsis",
	EmDash:             "em_dash",
	EnDash:             "en_dash",
}

func (k NodeKind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// IsBlock reports whether this node kind is a block-level element.
func (k NodeKind) IsBlock() bool {
	return k >= Document && k <= Footnote
}

// IsInline reports whether this node kind is an inline element.
func (k NodeKind) IsInline() bool {
	return k >= Text && k <= EnDash
}
