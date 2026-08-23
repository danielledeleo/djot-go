package djot

var kindNames = [...]string{
	KindDocument:          "document",
	KindSection:           "section",
	KindParagraph:         "paragraph",
	KindHeading:           "heading",
	KindThematicBreak:     "thematic_break",
	KindCodeBlock:         "code_block",
	KindRawBlock:          "raw_block",
	KindBlockQuote:        "block_quote",
	KindDiv:               "div",
	KindBulletList:        "bullet_list",
	KindOrderedList:       "ordered_list",
	KindTaskList:          "task_list",
	KindListItem:          "list_item",
	KindTaskListItem:      "task_list_item",
	KindDefinitionList:    "definition_list",
	KindTerm:              "term",
	KindDefinition:        "definition",
	KindTable:             "table",
	KindTableRow:          "table_row",
	KindTableCell:         "table_cell",
	KindCaption:           "caption",
	KindFootnote:          "footnote",
	KindText:              "text",
	KindSoftBreak:         "soft_break",
	KindHardBreak:         "hard_break",
	KindNonBreakingSpace:  "non_breaking_space",
	KindEmphasis:          "emphasis",
	KindStrong:            "strong",
	KindSuperscript:       "superscript",
	KindSubscript:         "subscript",
	KindInsert:            "insert",
	KindDelete:            "delete",
	KindMark:              "mark",
	KindLink:              "link",
	KindImage:             "image",
	KindSpan:              "span",
	KindVerbatim:          "verbatim",
	KindInlineMath:        "inline_math",
	KindDisplayMath:       "display_math",
	KindRawInline:         "raw_inline",
	KindSymbol:            "symbol",
	KindFootnoteReference: "footnote_reference",
	KindDoubleQuoted:      "double_quoted",
	KindSingleQuoted:      "single_quoted",
	KindEllipsis:          "ellipsis",
	KindEmDash:            "em_dash",
	KindEnDash:            "en_dash",
}

// String returns the snake_case name of the node kind (e.g. "heading", "bullet_list").
func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// IsBlock reports whether this node kind is a block-level element.
func (k Kind) IsBlock() bool {
	return k >= KindSection && k <= KindFootnote
}

// IsInline reports whether this node kind is an inline element.
func (k Kind) IsInline() bool {
	return k >= KindText && k <= KindEnDash
}
