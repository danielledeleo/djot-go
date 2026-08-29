package djot

import (
	"testing"

	. "github.com/danielledeleo/djot-go/ast"
)

func TestDeepTextCollectorsDoNotAllocatePerLevel(t *testing.T) {
	const depth = 2048

	parseRoot := &parseNode{Kind: KindText, Text: "x"}
	for range depth {
		parseRoot = &parseNode{Kind: KindEmphasis, Children: []*parseNode{parseRoot}}
	}
	if got := collectParseText(parseRoot); got != "x" {
		t.Fatalf("parse text = %q, want x", got)
	}
	if allocations := testing.AllocsPerRun(10, func() { collectParseText(parseRoot) }); allocations > 20 {
		t.Fatalf("parse text allocations = %.0f, want at most 20", allocations)
	}

	var node Inline = &Text{Value: "x"}
	for range depth {
		node = &Emphasis{Children: []Inline{node}}
	}
	if got := collectText(node); got != "x" {
		t.Fatalf("node text = %q, want x", got)
	}
	if allocations := testing.AllocsPerRun(10, func() { collectText(node) }); allocations > 20 {
		t.Fatalf("node text allocations = %.0f, want at most 20", allocations)
	}
	if got := collectDocumentText(ElementView{node: node}); got != "x" {
		t.Fatalf("document text = %q, want x", got)
	}
	if allocations := testing.AllocsPerRun(10, func() {
		collectDocumentText(ElementView{node: node})
	}); allocations > 20 {
		t.Fatalf("document text allocations = %.0f, want at most 20", allocations)
	}
}
