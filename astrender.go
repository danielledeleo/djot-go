package djot

import (
	"fmt"
	"io"
	"strings"
)

// RenderAST renders a parsed document to the djot AST text format,
// compatible with the official djot.js test suite. If positions is true,
// source positions are included in the output.
func RenderAST(doc *Doc, positions bool) string {
	var b strings.Builder
	_ = RenderASTTo(&b, doc, positions)
	return b.String()
}

// RenderASTTo renders the AST text format (see [RenderAST]) to w. It returns
// the first write error encountered, if any.
func RenderASTTo(w io.Writer, doc *Doc, positions bool) error {
	r := &astRenderer{w: w, doc: doc, positions: positions}
	r.renderNode(doc.Root(), 0)
	return r.err
}

type astRenderer struct {
	w         io.Writer
	doc       *Doc
	positions bool
	err       error
}

func (r *astRenderer) write(s string) {
	if r.err != nil {
		return
	}
	_, r.err = io.WriteString(r.w, s)
}

func (r *astRenderer) renderNode(n Node, indent int) {
	r.write(strings.Repeat(" ", indent))
	r.write(astTagName(n))

	if r.positions && n.Kind() != KindDocument {
		r.renderPos(n)
	}

	r.renderFields(n)
	r.renderAttrs(n)
	r.write("\n")

	forEachChild(n, func(child Node) { r.renderNode(child, indent+2) })
}

func (r *astRenderer) renderPos(n Node) {
	if r.doc == nil || len(r.doc.Files) == 0 {
		return
	}
	span := n.Span()
	fi := &r.doc.Files[span.Start.File]
	sLine, sCol := fi.Position(span.Start.Offset)
	eLine, eCol := astEndPosition(fi, span.End.Offset)
	r.write(fmt.Sprintf(" (%d:%d:%d-%d:%d:%d)",
		sLine, sCol, span.Start.Offset,
		eLine, eCol, span.End.Offset))
}

// astEndPosition resolves an end offset to line:col using the djot.js
// convention: newline characters and end-of-input map to the next line at
// column 0.
func astEndPosition(fi *FileInfo, offset int) (line, col int) {
	src := fi.Source
	if offset >= len(src) || (offset < len(src) && src[offset] == '\n') {
		line, _ = fi.Position(offset)
		return line + 1, 0
	}
	return fi.Position(offset)
}

func (r *astRenderer) renderFields(n Node) {
	for _, f := range astFields(n) {
		switch v := f.val.(type) {
		case string:
			r.write(fmt.Sprintf(" %s=%s", f.key, astStringify(v)))
		case int:
			r.write(fmt.Sprintf(" %s=%d", f.key, v))
		case bool:
			r.write(fmt.Sprintf(" %s=%v", f.key, v))
		}
	}
}

// astField is one kind-specific field of an AST node, paired with its value
// (always a string, int, or bool). astFields returns these in the canonical
// order shared by RenderAST (text format) and RenderASTJSON (JSON format), so
// the two renderers never drift.
type astField struct {
	key string
	val any
}

func astFields(n Node) []astField {
	switch n := n.(type) {
	case *Heading:
		return []astField{{"level", n.Level}}

	case *CodeBlock:
		var fs []astField
		if n.Language != "" {
			fs = append(fs, astField{"lang", n.Language})
		}
		return append(fs, astField{"text", n.Text})

	case *RawBlock:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case *BulletList:
		var fs []astField
		if n.Tight {
			fs = append(fs, astField{"tight", true})
		}
		if n.Marker != 0 {
			fs = append(fs, astField{"style", string(n.Marker)})
		}
		return fs

	case *OrderedList:
		var fs []astField
		if n.Tight {
			fs = append(fs, astField{"tight", true})
		}
		fs = append(fs, astField{"style", astListStyle(n.Style)})
		if n.Start != 1 {
			fs = append(fs, astField{"start", n.Start})
		}
		return fs

	case *TaskList:
		if n.Tight {
			return []astField{{"tight", true}}
		}
		return nil

	case *TaskListItem:
		if n.Checked {
			return []astField{{"checkbox", "checked"}}
		}
		return []astField{{"checkbox", "unchecked"}}

	case *TableRow:
		return []astField{{"head", n.Header}}

	case *TableCell:
		return []astField{{"head", n.Header}, {"align", astCellAlign(n.Alignment)}}

	case *Footnote:
		return []astField{{"label", n.Label}}

	case *Text:
		return []astField{{"text", n.Value}}

	case *Symbol:
		return []astField{{"alias", n.Name}}

	case *Verbatim:
		return []astField{{"text", n.Text}}

	case *InlineMath:
		return []astField{{"text", n.Text}}

	case *DisplayMath:
		return []astField{{"text", n.Text}}

	case *RawInline:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case *Link:
		if n.Destination != "" || n.DestinationSet {
			return []astField{{"destination", n.Destination}}
		}
		return nil

	case *Image:
		if n.Destination != "" || n.DestinationSet {
			return []astField{{"destination", n.Destination}}
		}
		return nil

	case *FootnoteReference:
		return []astField{{"text", n.Label}}

	case *Ellipsis:
		return []astField{{"type", "ellipsis"}, {"text", "..."}}

	case *EmDash:
		return []astField{{"type", "em_dash"}, {"text", "---"}}

	case *EnDash:
		return []astField{{"type", "en_dash"}, {"text", "--"}}
	}
	return nil
}

func (r *astRenderer) renderAttrs(n Node) {
	for _, attribute := range n.Attributes().items {
		r.write(fmt.Sprintf(" %s=%s", attribute.Key, astStringify(attribute.Value)))
	}
}

func astTagName(n Node) string {
	switch n.Kind() {
	case KindDocument:
		return "doc"
	case KindSection:
		return "section"
	case KindParagraph:
		return "para"
	case KindHeading:
		return "heading"
	case KindThematicBreak:
		return "thematic_break"
	case KindCodeBlock:
		return "code_block"
	case KindRawBlock:
		return "raw_block"
	case KindBlockQuote:
		return "block_quote"
	case KindDiv:
		return "div"
	case KindBulletList:
		return "bullet_list"
	case KindOrderedList:
		return "ordered_list"
	case KindTaskList:
		return "task_list"
	case KindListItem:
		return "list_item"
	case KindTaskListItem:
		return "task_list_item"
	case KindDefinitionList:
		return "definition_list"
	case KindTerm:
		return "term"
	case KindDefinition:
		return "definition"
	case KindTable:
		return "table"
	case KindTableRow:
		return "row"
	case KindTableCell:
		return "cell"
	case KindCaption:
		return "caption"
	case KindFootnote:
		return "footnote"
	case KindText:
		return "str"
	case KindSoftBreak:
		return "soft_break"
	case KindHardBreak:
		return "hard_break"
	case KindNonBreakingSpace:
		return "non_breaking_space"
	case KindEmphasis:
		return "emph"
	case KindStrong:
		return "strong"
	case KindSuperscript:
		return "superscript"
	case KindSubscript:
		return "subscript"
	case KindInsert:
		return "insert"
	case KindDelete:
		return "delete"
	case KindMark:
		return "mark"
	case KindLink:
		return "link"
	case KindImage:
		return "image"
	case KindSpan:
		return "span"
	case KindVerbatim:
		return "verbatim"
	case KindInlineMath:
		return "inline_math"
	case KindDisplayMath:
		return "display_math"
	case KindRawInline:
		return "raw_inline"
	case KindSymbol:
		return "symb"
	case KindFootnoteReference:
		return "footnote_reference"
	case KindDoubleQuoted:
		return "double_quoted"
	case KindSingleQuoted:
		return "single_quoted"
	case KindEllipsis, KindEmDash, KindEnDash:
		return "smart_punctuation"
	default:
		return n.Kind().String()
	}
}

func astListStyle(s ListStyle) string {
	switch s {
	case ListDecimal:
		return "1."
	case ListAlphaLower:
		return "a."
	case ListAlphaUpper:
		return "A."
	case ListRomanLower:
		return "i."
	case ListRomanUpper:
		return "I."
	default:
		return "1."
	}
}

func astCellAlign(a CellAlign) string {
	switch a {
	case AlignLeft:
		return "left"
	case AlignRight:
		return "right"
	case AlignCenter:
		return "center"
	default:
		return "default"
	}
}

// astStringify formats a value like JSON.stringify for the AST text format.
func astStringify(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}
