package djot_test

import (
	"reflect"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestTypedNodeConstruction(t *testing.T) {
	strong := &djot.Strong{Children: []djot.Inline{&djot.Text{Value: "typed"}}}
	if !strong.Attributes().Set("class", "important") {
		t.Fatal("failed to set valid attribute")
	}
	doc := djot.NewDoc(&djot.Document{Children: []djot.Block{
		&djot.Paragraph{Children: []djot.Inline{strong}},
	}})
	if got, want := djot.RenderHTML(doc), "<p><strong class=\"important\">typed</strong></p>\n"; got != want {
		t.Fatalf("typed tree render = %q, want %q", got, want)
	}
}

func TestAttributesOrderedValue(t *testing.T) {
	var attributes djot.Attributes
	attributes.Set("class", "one")
	attributes.Set("id", "target")
	attributes.Set("class", "two")
	if got, want := attributes.Entries(), []djot.Attribute{
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

func TestPreorderVisitsRootAndStops(t *testing.T) {
	doc := djot.Parse("one *two* three")
	var kinds []djot.Kind
	djot.Preorder(doc.Root(), func(node djot.Node) bool {
		kinds = append(kinds, node.Kind())
		return len(kinds) < 3
	})
	if len(kinds) != 3 || kinds[0] != djot.KindDocument {
		t.Fatalf("Preorder kinds = %v", kinds)
	}
}

func TestWalkRejectsCategoryMismatch(t *testing.T) {
	root := &djot.Document{Children: []djot.Block{
		&djot.Paragraph{Children: []djot.Inline{&djot.Text{Value: "text"}}},
	}}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a block replacement in an inline slot")
		}
	}()
	djot.Walk(root, func(node djot.Node) djot.Action {
		if _, ok := node.(*djot.Text); ok {
			return djot.Replace(&djot.Paragraph{})
		}
		return djot.Continue
	})
}

func TestWalkRejectsSpecializedSlotMismatch(t *testing.T) {
	root := &djot.Document{Children: []djot.Block{
		&djot.BulletList{Items: []*djot.ListItem{{}}},
	}}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a paragraph replacement in a list-item slot")
		}
	}()
	djot.Walk(root, func(node djot.Node) djot.Action {
		if _, ok := node.(*djot.ListItem); ok {
			return djot.Replace(&djot.Paragraph{})
		}
		return djot.Continue
	})
}

func TestWalkRejectsTypedNil(t *testing.T) {
	var replacement *djot.Text
	root := &djot.Text{Value: "before"}
	defer func() {
		if recover() == nil {
			t.Fatal("Walk accepted a typed-nil replacement")
		}
	}()
	djot.Walk(root, func(djot.Node) djot.Action {
		return djot.Replace(replacement)
	})
}

func TestWalkReturnsReplacedRoot(t *testing.T) {
	root := &djot.Text{Value: "before"}
	result := djot.Walk(root, func(node djot.Node) djot.Action {
		return djot.Replace(&djot.Text{Value: "after"})
	})
	if got := result.(*djot.Text).Value; got != "after" {
		t.Fatalf("replaced root value = %q", got)
	}
}

func TestSetSpan(t *testing.T) {
	text := &djot.Text{Value: "positioned"}
	want := djot.SourceSpan{Start: djot.Pos{Offset: 2}, End: djot.Pos{Offset: 11}}
	djot.SetSpan(text, want)
	if got := text.Span(); got != want {
		t.Fatalf("Span() = %#v, want %#v", got, want)
	}
}

func TestCopyMetadata(t *testing.T) {
	source := &djot.Strong{}
	djot.SetSpan(source, djot.SourceSpan{Start: djot.Pos{Offset: 2}, End: djot.Pos{Offset: 8}})
	source.Attributes().Set("class", "source")
	destination := &djot.Emphasis{}
	djot.CopyMetadata(destination, source)

	if destination.Span() != source.Span() || destination.Attributes().Get("class") != "source" {
		t.Fatal("CopyMetadata did not copy the span and attributes")
	}
	destination.Attributes().Set("class", "destination")
	if source.Attributes().Get("class") != "source" {
		t.Fatal("CopyMetadata shared attribute storage")
	}
}
