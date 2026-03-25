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
	r := &astRenderer{w: &b, doc: doc, positions: positions}
	r.renderNode(doc.Root, 0)
	return b.String()
}

type astRenderer struct {
	w         io.Writer
	doc       *Doc
	positions bool
}

func (r *astRenderer) write(s string) {
	io.WriteString(r.w, s)
}

func (r *astRenderer) renderNode(n *Node, indent int) {
	r.write(strings.Repeat(" ", indent))
	r.write(astTagName(n))

	if r.positions {
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
	eLine, eCol := fi.Position(n.End.Offset)
	r.write(fmt.Sprintf(" (%d:%d:%d-%d:%d:%d)",
		sLine, sCol, n.Start.Offset,
		eLine, eCol, n.End.Offset))
}

func (r *astRenderer) renderFields(n *Node) {
	switch n.Kind {
	case Heading:
		r.write(fmt.Sprintf(" level=%d", n.Level))

	case CodeBlock:
		if n.Lang != "" {
			r.write(fmt.Sprintf(" lang=%s", astStringify(n.Lang)))
		}
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case RawBlock:
		r.write(fmt.Sprintf(" format=%s", astStringify(n.Format)))
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case BulletList:
		if n.Attr("tight") == "true" {
			r.write(" tight=true")
		}
		if n.Marker != 0 {
			r.write(fmt.Sprintf(" style=%s", astStringify(string(n.Marker))))
		}

	case OrderedList:
		if n.Attr("tight") == "true" {
			r.write(" tight=true")
		}
		r.write(fmt.Sprintf(" style=%s", astStringify(astListStyle(n.ListStyle))))
		if n.ListStart != 1 {
			r.write(fmt.Sprintf(" start=%d", n.ListStart))
		}

	case TaskList:
		if n.Attr("tight") == "true" {
			r.write(" tight=true")
		}

	case TaskListItem:
		if n.Checked {
			r.write(` checkbox="checked"`)
		} else {
			r.write(` checkbox="unchecked"`)
		}

	case TableRow:
		if n.IsHeader {
			r.write(" head=true")
		} else {
			r.write(" head=false")
		}

	case TableCell:
		if n.IsHeader {
			r.write(" head=true")
		} else {
			r.write(" head=false")
		}
		r.write(fmt.Sprintf(" align=%s", astStringify(astCellAlign(n.CellAlign))))

	case Footnote:
		r.write(fmt.Sprintf(" label=%s", astStringify(n.Label)))

	case Text:
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case Symbol:
		r.write(fmt.Sprintf(" alias=%s", astStringify(n.Name)))

	case Verbatim:
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case InlineMath, DisplayMath:
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case RawInline:
		r.write(fmt.Sprintf(" format=%s", astStringify(n.Format)))
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Text)))

	case Link:
		if n.Target != "" || n.HasTarget {
			r.write(fmt.Sprintf(" destination=%s", astStringify(n.Target)))
		}

	case Image:
		if n.Target != "" || n.HasTarget {
			r.write(fmt.Sprintf(" destination=%s", astStringify(n.Target)))
		}

	case FootnoteReference:
		r.write(fmt.Sprintf(" text=%s", astStringify(n.Label)))

	case Ellipsis:
		r.write(` type="ellipsis" text="..."`)

	case EmDash:
		r.write(` type="em_dash" text="---"`)

	case EnDash:
		r.write(` type="en_dash" text="--"`)
	}
}

func (r *astRenderer) renderAttrs(n *Node) {
	if n.Attrs == nil {
		return
	}
	for _, k := range n.AttrOrder {
		// Skip internal attributes already rendered as dedicated fields.
		if k == "tight" {
			continue
		}
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
