package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestWalkContinue(t *testing.T) {
	doc := djot.Parse("Hello *world* and _more_")
	var kinds []djot.Kind
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		kinds = append(kinds, n.Kind())
		return djot.Continue
	})
	// The walk should visit every node.
	if len(kinds) == 0 {
		t.Fatal("Walk visited no nodes")
	}
	// The root should contain a paragraph, possibly inside a section.
	found := false
	for _, k := range kinds {
		if k == djot.KindParagraph {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a paragraph in visited nodes, got %v", kinds)
	}
}

func TestWalkSkipChildren(t *testing.T) {
	doc := djot.Parse("Hello *world*")
	var visited []djot.Kind
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		visited = append(visited, n.Kind())
		if n.Kind() == djot.KindStrong {
			return djot.SkipChildren
		}
		return djot.Continue
	})
	// The Strong node should be visited, but not its Text child.
	hasStrong := false
	for _, k := range visited {
		if k == djot.KindStrong {
			hasStrong = true
		}
	}
	if !hasStrong {
		t.Fatal("expected Strong to be visited")
	}
	// Only the "Hello " text should be seen, not "world" inside Strong.
	textCount := 0
	for _, k := range visited {
		if k == djot.KindText {
			textCount++
		}
	}
	if textCount != 1 {
		t.Errorf("expected 1 Text node after skipping Strong's child, got %d", textCount)
	}
}

func TestWalkRemove(t *testing.T) {
	doc := djot.Parse("Hello *world* goodbye")
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if n.Kind() == djot.KindStrong {
			return djot.Remove
		}
		return djot.Continue
	})
	html := djot.RenderHTML(doc)
	if got := html; contains(got, "<strong>") {
		t.Errorf("Strong should have been removed, got: %s", got)
	}
	if !contains(html, "Hello") || !contains(html, "goodbye") {
		t.Errorf("non-removed text should remain, got: %s", html)
	}
}

func TestWalkReplace(t *testing.T) {
	doc := djot.Parse("Hello *world*")
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if n.Kind() == djot.KindStrong {
			return djot.Replace(&djot.Emphasis{
				Children: []djot.Inline{&djot.Text{Value: "replaced"}},
			})
		}
		return djot.Continue
	})
	html := djot.RenderHTML(doc)
	if !contains(html, "<em>replaced</em>") {
		t.Errorf("expected replaced emphasis, got: %s", html)
	}
	if contains(html, "<strong>") {
		t.Errorf("Strong should have been replaced, got: %s", html)
	}
}

func TestWalkReplaceVisitsReplacementChildren(t *testing.T) {
	doc := djot.Parse("*bold*")
	var visitedAfterReplace []djot.Kind
	replaced := false
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if n.Kind() == djot.KindStrong && !replaced {
			replaced = true
			return djot.Replace(&djot.Emphasis{
				Children: []djot.Inline{&djot.Text{Value: "inner"}},
			})
		}
		if replaced {
			visitedAfterReplace = append(visitedAfterReplace, n.Kind())
		}
		return djot.Continue
	})
	// The walker should visit the replacement's Text child.
	found := false
	for _, k := range visitedAfterReplace {
		if k == djot.KindText {
			found = true
		}
	}
	if !found {
		t.Error("expected walker to visit the replacement's Text child")
	}
}

func TestWalkVisitsRoot(t *testing.T) {
	doc := djot.Parse("Hello")
	visitedRoot := false
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if n.Kind() == djot.KindDocument {
			visitedRoot = true
		}
		return djot.Continue
	})
	if !visitedRoot {
		t.Error("Walk should visit the root node itself")
	}
}

func TestWalkRemoveMultiple(t *testing.T) {
	doc := djot.Parse("*a* *b* *c*")
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if n.Kind() == djot.KindStrong {
			return djot.Remove
		}
		return djot.Continue
	})
	html := djot.RenderHTML(doc)
	if contains(html, "<strong>") {
		t.Errorf("all Strong nodes should have been removed, got: %s", html)
	}
}

func TestWalkBottomUp(t *testing.T) {
	doc := djot.Parse("Hello *world*")
	var kinds []djot.Kind
	djot.WalkBottomUp(doc.Root(), func(n djot.Node) {
		kinds = append(kinds, n.Kind())
	})
	if len(kinds) == 0 {
		t.Fatal("WalkBottomUp visited no nodes")
	}
	// Bottom-up: leaf nodes should appear before their parents.
	// The root Document should be visited last.
	if kinds[len(kinds)-1] != djot.KindDocument {
		t.Errorf("expected Document to be visited last, got %s", kinds[len(kinds)-1])
	}
	// Text nodes should appear before their Paragraph.
	textIdx, paraIdx := -1, -1
	for i, k := range kinds {
		if k == djot.KindText && textIdx == -1 {
			textIdx = i
		}
		if k == djot.KindParagraph {
			paraIdx = i
		}
	}
	if textIdx >= paraIdx {
		t.Errorf("expected Text before Paragraph in bottom-up order, text=%d para=%d", textIdx, paraIdx)
	}
}

func TestWalkBottomUpVisitsRoot(t *testing.T) {
	doc := djot.Parse("Hello")
	visitedRoot := false
	djot.WalkBottomUp(doc.Root(), func(n djot.Node) {
		if n.Kind() == djot.KindDocument {
			visitedRoot = true
		}
	})
	if !visitedRoot {
		t.Error("WalkBottomUp should visit the root node")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
