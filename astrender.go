package djot

import (
	"fmt"
	"io"
	"strings"

	"github.com/danielledeleo/djot-go/ast"
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

func (r *astRenderer) renderNode(n ast.Node, indent int) {
	r.write(strings.Repeat(" ", indent))
	r.write(astTagName(n))

	if r.positions && n.Kind() != ast.KindDocument {
		r.renderPos(n)
	}

	r.renderFields(n)
	r.renderAttrs(n)
	r.write("\n")

	ast.ForEachChild(n, func(child ast.Node) { r.renderNode(child, indent+2) })
}

func (r *astRenderer) renderPos(n ast.Node) {
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
func astEndPosition(fi *ast.FileInfo, offset int) (line, col int) {
	src := fi.Source
	if offset >= len(src) || (offset < len(src) && src[offset] == '\n') {
		line, _ = fi.Position(offset)
		return line + 1, 0
	}
	return fi.Position(offset)
}

func (r *astRenderer) renderFields(n ast.Node) {
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

func astFields(n ast.Node) []astField {
	switch n := n.(type) {
	case *ast.Heading:
		return []astField{{"level", n.Level}}

	case *ast.CodeBlock:
		var fs []astField
		if n.Language != "" {
			fs = append(fs, astField{"lang", n.Language})
		}
		return append(fs, astField{"text", n.Text})

	case *ast.RawBlock:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case *ast.BulletList:
		var fs []astField
		if n.Tight {
			fs = append(fs, astField{"tight", true})
		}
		if n.Marker != 0 {
			fs = append(fs, astField{"style", string(n.Marker)})
		}
		return fs

	case *ast.OrderedList:
		var fs []astField
		if n.Tight {
			fs = append(fs, astField{"tight", true})
		}
		fs = append(fs, astField{"style", astListStyle(n.Style)})
		if n.Start != 1 {
			fs = append(fs, astField{"start", n.Start})
		}
		return fs

	case *ast.TaskList:
		if n.Tight {
			return []astField{{"tight", true}}
		}
		return nil

	case *ast.TaskListItem:
		if n.Checked {
			return []astField{{"checkbox", "checked"}}
		}
		return []astField{{"checkbox", "unchecked"}}

	case *ast.TableRow:
		return []astField{{"head", n.Header}}

	case *ast.TableCell:
		return []astField{{"head", n.Header}, {"align", astCellAlign(n.Alignment)}}

	case *ast.Footnote:
		return []astField{{"label", n.Label}}

	case *ast.Text:
		return []astField{{"text", n.Value}}

	case *ast.Symbol:
		return []astField{{"alias", n.Name}}

	case *ast.Verbatim:
		return []astField{{"text", n.Text}}

	case *ast.InlineMath:
		return []astField{{"text", n.Text}}

	case *ast.DisplayMath:
		return []astField{{"text", n.Text}}

	case *ast.RawInline:
		return []astField{{"format", n.Format}, {"text", n.Text}}

	case *ast.Link:
		if n.Destination != "" || n.DestinationSet {
			return []astField{{"destination", n.Destination}}
		}
		return nil

	case *ast.Image:
		if n.Destination != "" || n.DestinationSet {
			return []astField{{"destination", n.Destination}}
		}
		return nil

	case *ast.FootnoteReference:
		return []astField{{"text", n.Label}}

	case *ast.Ellipsis:
		return []astField{{"type", "ellipsis"}, {"text", "..."}}

	case *ast.EmDash:
		return []astField{{"type", "em_dash"}, {"text", "---"}}

	case *ast.EnDash:
		return []astField{{"type", "en_dash"}, {"text", "--"}}
	}
	return nil
}

func (r *astRenderer) renderAttrs(n ast.Node) {
	for _, attribute := range n.Attributes().Entries() {
		r.write(fmt.Sprintf(" %s=%s", attribute.Key, astStringify(attribute.Value)))
	}
}

func astTagName(n ast.Node) string {
	switch n.Kind() {
	case ast.KindDocument:
		return "doc"
	case ast.KindSection:
		return "section"
	case ast.KindParagraph:
		return "para"
	case ast.KindHeading:
		return "heading"
	case ast.KindThematicBreak:
		return "thematic_break"
	case ast.KindCodeBlock:
		return "code_block"
	case ast.KindRawBlock:
		return "raw_block"
	case ast.KindBlockQuote:
		return "block_quote"
	case ast.KindDiv:
		return "div"
	case ast.KindBulletList:
		return "bullet_list"
	case ast.KindOrderedList:
		return "ordered_list"
	case ast.KindTaskList:
		return "task_list"
	case ast.KindListItem:
		return "list_item"
	case ast.KindTaskListItem:
		return "task_list_item"
	case ast.KindDefinitionList:
		return "definition_list"
	case ast.KindTerm:
		return "term"
	case ast.KindDefinition:
		return "definition"
	case ast.KindTable:
		return "table"
	case ast.KindTableRow:
		return "row"
	case ast.KindTableCell:
		return "cell"
	case ast.KindCaption:
		return "caption"
	case ast.KindFootnote:
		return "footnote"
	case ast.KindText:
		return "str"
	case ast.KindSoftBreak:
		return "soft_break"
	case ast.KindHardBreak:
		return "hard_break"
	case ast.KindNonBreakingSpace:
		return "non_breaking_space"
	case ast.KindEmphasis:
		return "emph"
	case ast.KindStrong:
		return "strong"
	case ast.KindSuperscript:
		return "superscript"
	case ast.KindSubscript:
		return "subscript"
	case ast.KindInsert:
		return "insert"
	case ast.KindDelete:
		return "delete"
	case ast.KindMark:
		return "mark"
	case ast.KindLink:
		return "link"
	case ast.KindImage:
		return "image"
	case ast.KindSpan:
		return "span"
	case ast.KindVerbatim:
		return "verbatim"
	case ast.KindInlineMath:
		return "inline_math"
	case ast.KindDisplayMath:
		return "display_math"
	case ast.KindRawInline:
		return "raw_inline"
	case ast.KindSymbol:
		return "symb"
	case ast.KindFootnoteReference:
		return "footnote_reference"
	case ast.KindDoubleQuoted:
		return "double_quoted"
	case ast.KindSingleQuoted:
		return "single_quoted"
	case ast.KindEllipsis, ast.KindEmDash, ast.KindEnDash:
		return "smart_punctuation"
	default:
		return n.Kind().String()
	}
}

func astListStyle(s ast.ListStyle) string {
	switch s {
	case ast.ListDecimal:
		return "1."
	case ast.ListAlphaLower:
		return "a."
	case ast.ListAlphaUpper:
		return "A."
	case ast.ListRomanLower:
		return "i."
	case ast.ListRomanUpper:
		return "I."
	default:
		return "1."
	}
}

func astCellAlign(a ast.CellAlign) string {
	switch a {
	case ast.AlignLeft:
		return "left"
	case ast.AlignRight:
		return "right"
	case ast.AlignCenter:
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
