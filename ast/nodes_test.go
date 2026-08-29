package ast_test

import (
	"reflect"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func TestTypedNodeConstruction(t *testing.T) {
	strong := &ast.Strong{Children: []ast.Inline{&ast.Text{Value: "typed"}}}
	if !strong.Attributes().Set("class", "important") {
		t.Fatal("failed to set valid attribute")
	}
	doc := djot.NewDoc(&ast.Document{Children: []ast.Block{
		&ast.Paragraph{Children: []ast.Inline{strong}},
	}})
	if got, want := djot.RenderHTML(doc), "<p><strong class=\"important\">typed</strong></p>\n"; got != want {
		t.Fatalf("typed tree render = %q, want %q", got, want)
	}
}

func TestAttributesOrderedValue(t *testing.T) {
	var attributes ast.Attributes
	attributes.Set("class", "one")
	attributes.Set("id", "target")
	attributes.Set("class", "two")
	if got, want := attributes.Entries(), []ast.Attribute{
		{Key: "class", Value: "two"},
		{Key: "id", Value: "target"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Entries() = %#v, want %#v", got, want)
	}
	clone := attributes.Clone()
	clone.Delete("class")
	if attributes.Get("class") != "two" || clone.Get("class") != "" {
		t.Fatal("Clone or Delete shared attribute storage")
	}
	entries := attributes.Entries()
	entries[0].Value = "mutated copy"
	if attributes.Get("class") != "two" {
		t.Fatal("Entries exposed internal storage")
	}
	if attributes.Set("invalid key", "value") {
		t.Fatal("Set accepted an invalid key")
	}
}

func TestAttributesRangeAndClasses(t *testing.T) {
	var attributes ast.Attributes
	if !attributes.AddClass("one") || !attributes.AddClass("two") {
		t.Fatal("AddClass rejected valid classes")
	}
	attributes.Set("id", "target")

	var visited []ast.Attribute
	attributes.Range(func(attribute ast.Attribute) bool {
		visited = append(visited, attribute)
		return false
	})
	if got, want := visited, []ast.Attribute{{Key: "class", Value: "one two"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Range() = %#v, want %#v", got, want)
	}

	var nilAttributes *ast.Attributes
	called := false
	nilAttributes.Range(func(ast.Attribute) bool {
		called = true
		return true
	})
	if called || nilAttributes.Len() != 0 || nilAttributes.Entries() != nil {
		t.Fatal("nil Attributes did not behave as an empty collection")
	}
}

func TestKindString(t *testing.T) {
	for _, test := range []struct {
		kind ast.Kind
		want string
	}{
		{ast.KindDocument, "document"},
		{ast.KindEnDash, "en_dash"},
		{ast.Kind(-1), "unknown"},
		{ast.KindEnDash + 1, "unknown"},
	} {
		if got := test.kind.String(); got != test.want {
			t.Errorf("Kind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestPreorderVisitsRootAndStops(t *testing.T) {
	doc := djot.Parse("one *two* three")
	var kinds []ast.Kind
	ast.Preorder(doc.Root(), func(node ast.Node) bool {
		kinds = append(kinds, node.Kind())
		return len(kinds) < 3
	})
	if len(kinds) != 3 || kinds[0] != ast.KindDocument {
		t.Fatalf("Preorder kinds = %v", kinds)
	}
}

func TestNilAndRemovedRootTraversal(t *testing.T) {
	visited := false
	ast.Preorder(nil, func(ast.Node) bool {
		visited = true
		return true
	})
	ast.WalkBottomUp(nil, func(ast.Node) { visited = true })
	if got := ast.Walk(nil, nil); got != nil || visited {
		t.Fatal("nil traversal invoked a callback or returned a node")
	}
	if got := ast.Walk(&ast.Text{}, func(ast.Node) ast.Action { return ast.Remove }); got != nil {
		t.Fatalf("removed root = %#v, want nil", got)
	}
}

func TestWalkRejectsCategoryMismatch(t *testing.T) {
	root := &ast.Document{Children: []ast.Block{
		&ast.Paragraph{Children: []ast.Inline{&ast.Text{Value: "text"}}},
	}}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a block replacement in an inline slot")
		}
	}()
	ast.Walk(root, func(node ast.Node) ast.Action {
		if _, ok := node.(*ast.Text); ok {
			return ast.Replace(&ast.Paragraph{})
		}
		return ast.Continue
	})
}

func TestWalkRejectsSpecializedSlotMismatch(t *testing.T) {
	root := &ast.Document{Children: []ast.Block{
		&ast.BulletList{Items: []*ast.ListItem{{}}},
	}}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a paragraph replacement in a list-item slot")
		}
	}()
	ast.Walk(root, func(node ast.Node) ast.Action {
		if _, ok := node.(*ast.ListItem); ok {
			return ast.Replace(&ast.Paragraph{})
		}
		return ast.Continue
	})
}

func TestWalkRejectsTypedNil(t *testing.T) {
	var replacement *ast.Text
	root := &ast.Text{Value: "before"}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a typed-nil replacement")
		}
	}()
	ast.Walk(root, func(ast.Node) ast.Action {
		return ast.Replace(replacement)
	})
}

func TestWalkReturnsReplacedRoot(t *testing.T) {
	root := &ast.Text{Value: "before"}
	result := ast.Walk(root, func(node ast.Node) ast.Action {
		return ast.Replace(&ast.Text{Value: "after"})
	})
	if got := result.(*ast.Text).Value; got != "after" {
		t.Fatalf("replaced root value = %q", got)
	}
}

func TestSetSpan(t *testing.T) {
	text := &ast.Text{Value: "positioned"}
	want := ast.SourceSpan{Start: ast.Pos{Offset: 2}, End: ast.Pos{Offset: 11}}
	ast.SetSpan(text, want)
	if got := text.Span(); got != want {
		t.Fatalf("Span() = %#v, want %#v", got, want)
	}
}

func TestCopyMetadata(t *testing.T) {
	source := &ast.Strong{}
	ast.SetSpan(source, ast.SourceSpan{Start: ast.Pos{Offset: 2}, End: ast.Pos{Offset: 8}})
	source.Attributes().Set("class", "source")
	destination := &ast.Emphasis{}
	ast.CopyMetadata(destination, source)

	if destination.Span() != source.Span() || destination.Attributes().Get("class") != "source" {
		t.Fatal("CopyMetadata did not copy the span and attributes")
	}
	destination.Attributes().Set("class", "destination")
	if source.Attributes().Get("class") != "source" {
		t.Fatal("CopyMetadata shared attribute storage")
	}
}
