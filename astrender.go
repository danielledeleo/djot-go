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
	// The only error source is the io.Writer; strings.Builder never fails.
	_ = RenderASTTo(&b, doc, positions)
	return b.String()
}

// RenderASTTo renders the AST text format (see [RenderAST]) to w. It returns
// the first write error encountered, if any.
func RenderASTTo(w io.Writer, doc *Doc, positions bool) error {
	r := &astRenderer{w: w, doc: doc, positions: positions}
	r.renderNode(doc.Root, 0)
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

func (r *astRenderer) renderNode(n *Node, indent int) {
	r.write(strings.Repeat(" ", indent))
	r.write(astTagName(n))

	if r.positions && n.Kind != Document {
		r.renderPos(n)
	}

	r.renderFields(n)
	r.renderAttrs(n)
	r.write("\n")

	for _, child := range n.Children {
		r.renderNode(child, indent+2)
	}
}

func (r *astRenderer) renderPos(n *Node) {
	if r.doc == nil || len(r.doc.Files) == 0 {
		return
	}
	fi := &r.doc.Files[n.Start.File]
	sLine, sCol := fi.Position(n.Start.Offset)
	eLine, eCol := r.endPosition(fi, n.End.Offset)
	r.write(fmt.Sprintf(" (%d:%d:%d-%d:%d:%d)",
		sLine, sCol, n.Start.Offset,
		eLine, eCol, n.End.Offset))
}

// endPosition resolves an end offset to line:col using the djot.js convention:
// newline characters and end-of-input map to the next line at column 0.
func (r *astRenderer) endPosition(fi *FileInfo, offset int) (line, col int) {
	return astEndPosition(fi, offset)
}

// astEndPosition resolves an end offset to line:col using the djot.js
// convention: newline characters and end-of-input map to the next line at
// column 0.
func astEndPosition(fi *FileInfo, offset int) (line, col int) {
	src := fi.Source
	if offset >= len(src) || (offset < len(src) && src[offset] == '\n') {
		// Position is at a newline or past the end — report as next line, col 0.
		line, _ = fi.Position(offset)
		return line + 1, 0
	}
	return fi.Position(offset)
}

func (r *astRenderer) renderFields(n *Node) {
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

func astFields(n *Node) []astField {
	switch n.Kind {
	case Heading:
		return []astField{{"level", n.Level}}

	case CodeBlock:
		var fs []astField
		if n.Lang != "" {
			fs = append(fs, astField{"lang", n.Lang})
		}
		return append(fs, astField{"text", n.Text})

	case RawBlock:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case BulletList:
		var fs []astField
		if n.tight {
			fs = append(fs, astField{"tight", true})
		}
		if n.Marker != 0 {
			fs = append(fs, astField{"style", string(n.Marker)})
		}
		return fs

	case OrderedList:
		var fs []astField
		if n.tight {
			fs = append(fs, astField{"tight", true})
		}
		fs = append(fs, astField{"style", astListStyle(n.ListStyle)})
		if n.ListStart != 1 {
			fs = append(fs, astField{"start", n.ListStart})
		}
		return fs

	case TaskList:
		if n.tight {
			return []astField{{"tight", true}}
		}
		return nil

	case TaskListItem:
		if n.Checked {
			return []astField{{"checkbox", "checked"}}
		}
		return []astField{{"checkbox", "unchecked"}}

	case TableRow:
		return []astField{{"head", n.IsHeader}}

	case TableCell:
		return []astField{{"head", n.IsHeader}, {"align", astCellAlign(n.CellAlign)}}

	case Footnote:
		return []astField{{"label", n.Label}}

	case Text:
		return []astField{{"text", n.Text}}

	case Symbol:
		return []astField{{"alias", n.Name}}

	case Verbatim:
		return []astField{{"text", n.Text}}

	case InlineMath, DisplayMath:
		return []astField{{"text", n.Text}}

	case RawInline:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case Link:
		if n.Target != "" || n.HasTarget {
			return []astField{{"destination", n.Target}}
		}
		return nil

	case Image:
		if n.Target != "" || n.HasTarget {
			return []astField{{"destination", n.Target}}
		}
		return nil

	case FootnoteReference:
		return []astField{{"text", n.Label}}

	case Ellipsis:
		return []astField{{"type", "ellipsis"}, {"text", "..."}}

	case EmDash:
		return []astField{{"type", "em_dash"}, {"text", "---"}}

	case EnDash:
		return []astField{{"type", "en_dash"}, {"text", "--"}}
	}
	return nil
}

func (r *astRenderer) renderAttrs(n *Node) {
	if n.Attrs == nil {
		return
	}
	for _, k := range n.attrOrder {
		v := n.Attrs[k]
		r.write(fmt.Sprintf(" %s=%s", k, astStringify(v)))
	}
}

func astTagName(n *Node) string {
	switch n.Kind {
	case Document:
		return "doc"
	case Section:
		return "section"
	case Paragraph:
		return "para"
	case Heading:
		return "heading"
	case ThematicBreak:
		return "thematic_break"
	case CodeBlock:
		return "code_block"
	case RawBlock:
		return "raw_block"
	case BlockQuote:
		return "block_quote"
	case Div:
		return "div"
	case BulletList:
		return "bullet_list"
	case OrderedList:
		return "ordered_list"
	case TaskList:
		return "task_list"
	case ListItem:
		return "list_item"
	case TaskListItem:
		return "task_list_item"
	case DefinitionList:
		return "definition_list"
	case Term:
		return "term"
	case Definition:
		return "definition"
	case Table:
		return "table"
	case TableRow:
		return "row"
	case TableCell:
		return "cell"
	case Caption:
		return "caption"
	case Footnote:
		return "footnote"
	case Text:
		return "str"
	case SoftBreak:
		return "soft_break"
	case HardBreak:
		return "hard_break"
	case NonBreakingSpace:
		return "non_breaking_space"
	case Emphasis:
		return "emph"
	case Strong:
		return "strong"
	case Superscript:
		return "superscript"
	case Subscript:
		return "subscript"
	case Insert:
		return "insert"
	case Delete:
		return "delete"
	case Mark:
		return "mark"
	case Link:
		return "link"
	case Image:
		return "image"
	case Span:
		return "span"
	case Verbatim:
		return "verbatim"
	case InlineMath:
		return "inline_math"
	case DisplayMath:
		return "display_math"
	case RawInline:
		return "raw_inline"
	case Symbol:
		return "symb"
	case FootnoteReference:
		return "footnote_reference"
	case DoubleQuoted:
		return "double_quoted"
	case SingleQuoted:
		return "single_quoted"
	case Ellipsis, EmDash, EnDash:
		return "smart_punctuation"
	default:
		return n.Kind.String()
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
