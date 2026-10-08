package djot

import (
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

// collectDocumentText returns the presentation text of the tape record at
// index, such as image alt text: smart punctuation and quotes as rendered,
// symbols as :name:, and code and math contents.
func collectDocumentText(tape *semanticTape, index int) string {
	var text strings.Builder
	appendDocumentText(&text, tape, index)
	return text.String()
}

func appendDocumentText(text *strings.Builder, tape *semanticTape, index int) {
	record := tape.records[index]
	prefix, suffix := "", ""
	switch ast.Kind(record.kind) {
	case ast.KindText, ast.KindVerbatim, ast.KindInlineMath, ast.KindDisplayMath:
		text.WriteString(tape.text(record.payload))
		return
	case ast.KindSoftBreak, ast.KindHardBreak, ast.KindNonBreakingSpace:
		text.WriteByte(' ')
		return
	case ast.KindSymbol:
		text.WriteByte(':')
		text.WriteString(tape.text(record.payload))
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
	for child, end := index+1, int(record.subtreeEnd); child < end; child = int(tape.records[child].subtreeEnd) {
		appendDocumentText(text, tape, child)
	}
	text.WriteString(suffix)
}
