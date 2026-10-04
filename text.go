package djot

import (
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

// textElement is one element of either the compact tape or a typed tree, for
// collecting presentation text such as image alt text.
type textElement struct {
	tape   *semanticTape
	node   ast.Node
	record int
}

func (e textElement) kind() ast.Kind {
	if e.tape != nil {
		return ast.Kind(e.tape.records[e.record].kind)
	}
	return e.node.Kind()
}

// collectDocumentText returns an element's presentation text: smart
// punctuation and quotes as rendered, symbols as :name:, and code and math
// contents.
func collectDocumentText(element textElement) string {
	var text strings.Builder
	appendDocumentText(&text, element)
	return text.String()
}

func appendDocumentText(text *strings.Builder, element textElement) {
	prefix, suffix := "", ""
	switch element.kind() {
	case ast.KindText:
		if element.tape != nil {
			text.WriteString(element.tape.text(element.tape.records[element.record].payload))
			return
		}
		text.WriteString(element.node.(*ast.Text).Value)
		return
	case ast.KindSoftBreak, ast.KindHardBreak, ast.KindNonBreakingSpace:
		text.WriteByte(' ')
		return
	case ast.KindVerbatim, ast.KindInlineMath, ast.KindDisplayMath:
		if element.tape != nil {
			text.WriteString(element.tape.text(element.tape.records[element.record].payload))
			return
		}
		switch node := element.node.(type) {
		case *ast.Verbatim:
			text.WriteString(node.Text)
		case *ast.InlineMath:
			text.WriteString(node.Text)
		case *ast.DisplayMath:
			text.WriteString(node.Text)
		}
		return
	case ast.KindSymbol:
		text.WriteByte(':')
		if element.tape != nil {
			text.WriteString(element.tape.text(element.tape.records[element.record].payload))
		} else {
			text.WriteString(element.node.(*ast.Symbol).Name)
		}
		text.WriteByte(':')
		return
	case ast.KindEllipsis:
		text.WriteString("…")
		return
	case ast.KindEmDash:
		text.WriteString("—")
		return
	case ast.KindEnDash:
		text.WriteString("–")
		return
	case ast.KindDoubleQuoted:
		prefix, suffix = "“", "”"
	case ast.KindSingleQuoted:
		prefix, suffix = "‘", "’"
	}

	text.WriteString(prefix)
	if element.tape != nil {
		tape := element.tape
		for child, end := element.record+1, int(tape.records[element.record].subtreeEnd); child < end; {
			appendDocumentText(text, textElement{tape: tape, record: child})
			child = int(tape.records[child].subtreeEnd)
		}
	} else if element.node != nil {
		ast.ForEachChild(element.node, func(child ast.Node) {
			appendDocumentText(text, textElement{node: child})
		})
	}
	text.WriteString(suffix)
}
