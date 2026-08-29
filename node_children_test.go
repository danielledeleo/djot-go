package djot

import (
	"testing"

	. "github.com/danielledeleo/djot-go/ast"
)

func TestAllocateTypedNodeCoversEveryKind(t *testing.T) {
	var arena typedNodeArena
	for kind := KindDocument; kind <= KindEnDash; kind++ {
		node := allocateTypedNode(kind, &arena)
		if node.Kind() != kind {
			t.Fatalf("allocateTypedNode(%v) returned %T with kind %v", kind, node, node.Kind())
		}
	}

	defer func() {
		if recover() == nil {
			t.Fatal("allocateTypedNode accepted an unknown kind")
		}
	}()
	allocateTypedNode(KindEnDash+1, &arena)
}

func TestTypedChildDispatch(t *testing.T) {
	cases := []struct {
		parent Node
		child  Node
	}{
		{&Document{}, &Paragraph{}},
		{&Section{}, &Paragraph{}},
		{&Paragraph{}, &Text{}},
		{&Heading{}, &Text{}},
		{&BlockQuote{}, &Paragraph{}},
		{&Div{}, &Paragraph{}},
		{&BulletList{}, &ListItem{}},
		{&OrderedList{}, &ListItem{}},
		{&TaskList{}, &TaskListItem{}},
		{&ListItem{}, &Paragraph{}},
		{&TaskListItem{}, &Paragraph{}},
		{&DefinitionList{}, &Term{}},
		{&Term{}, &Text{}},
		{&Definition{}, &Paragraph{}},
		{&Table{}, &TableRow{}},
		{&TableRow{}, &TableCell{}},
		{&TableCell{}, &Text{}},
		{&Caption{}, &Text{}},
		{&Footnote{}, &Paragraph{}},
		{&Emphasis{}, &Text{}},
		{&Strong{}, &Text{}},
		{&Superscript{}, &Text{}},
		{&Subscript{}, &Text{}},
		{&Insert{}, &Text{}},
		{&Delete{}, &Text{}},
		{&Mark{}, &Text{}},
		{&Link{}, &Text{}},
		{&Image{}, &Text{}},
		{&Span{}, &Text{}},
		{&DoubleQuoted{}, &Text{}},
		{&SingleQuoted{}, &Text{}},
	}

	for _, test := range cases {
		AppendChild(test.parent, test.child)
		var children []Node
		ForEachChild(test.parent, func(child Node) { children = append(children, child) })
		if len(children) != 1 || children[0] != test.child {
			t.Errorf("%T children = %#v, want %#v", test.parent, children, test.child)
		}
	}
}
