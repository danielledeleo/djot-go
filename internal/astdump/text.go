// Package astdump writes a document's AST in the text and JSON formats used by
// the djot.js test suite and the djot command's --to ast and --to json.
package astdump

import (
	"fmt"
	"io"
	"strings"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// Text writes doc's AST in the djot.js text format to w. If positions is true,
// source positions are included. Inline end offsets are inclusive in this
// format, unlike the half-open ranges returned by ast.Node.Span. It returns the
// first write error, if any.
func Text(w io.Writer, doc *djot.Doc, positions bool) error {
	r := &astRenderer{w: w, doc: doc, positions: positions}
	r.renderNode(doc.AST(), 0)
	return r.err
}

type astRenderer struct {
	w         io.Writer
	doc       *djot.Doc
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
	start, end, ok := astSpan(r.doc, n)
	if !ok {
		return
	}
	r.write(fmt.Sprintf(" (%d:%d:%d-%d:%d:%d)",
		start.Line, start.Column, n.Span().Start.Offset,
		end.Line, end.Column, astOutputEnd(n)))
}

// astSpan returns the start and end positions of n in the AST formats. It
// reports false when doc has no source text (a Doc built by NewDoc).
func astSpan(doc *djot.Doc, n ast.Node) (start, end ast.Position, ok bool) {
	if doc.Position(ast.Pos{}).Line == 0 {
		return start, end, false
	}
	return doc.Position(n.Span().Start), astEndPosition(doc, astOutputEnd(n)), true
}

// astOutputEnd preserves the text/JSON AST format's inclusive inline ends.
// Public source spans remain half-open for both block and inline nodes.
func astOutputEnd(n ast.Node) int {
	span := n.Span()
	if _, inline := n.(ast.Inline); inline && span.End.Offset > span.Start.Offset {
		return span.End.Offset - 1
	}
	return span.End.Offset
}

// astEndPosition resolves an end offset to line:col using the djot.js
// convention: newline characters and end-of-input map to the next line at
// column 0. The offset is at a newline or the end of input exactly when the
// following offset starts a line or is out of range.
func astEndPosition(doc *djot.Doc, offset int) ast.Position {
	if next := doc.Position(ast.Pos{Offset: offset + 1}); next.Line == 0 || next.Column == 1 {
		return ast.Position{Line: doc.Position(ast.Pos{Offset: offset}).Line + 1}
	}
	return doc.Position(ast.Pos{Offset: offset})
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
// order shared by Text and JSON, so
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
