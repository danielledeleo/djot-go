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
	// Should visit all nodes: KindSection, KindParagraph, KindText, KindStrong, KindText, KindText, KindEmphasis, KindText
	if len(kinds) == 0 {
		t.Fatal("Walk visited no nodes")
	}
	// First child of root should be KindSection (from heading wrapping) or KindParagraph
	found := false
	for _, k := range kinds {
		if k == djot.KindParagraph {
			found = true
		}
	}
	if !found {
		t.Errorf("expected KindParagraph in visited nodes, got %v", kinds)
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
	// Should visit KindStrong but NOT its KindText child
	hasStrong := false
	for _, k := range visited {
		if k == djot.KindStrong {
			hasStrong = true
		}
	}
	if !hasStrong {
		t.Fatal("expected KindStrong to be visited")
	}
	// Count KindText nodes — should only see the "Hello " text, not "world" inside KindStrong
	textCount := 0
	for _, k := range visited {
		if k == djot.KindText {
			textCount++
		}
	}
	if textCount != 1 {
		t.Errorf("expected 1 KindText node (skipped KindStrong's child), got %d", textCount)
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
		t.Errorf("KindStrong should have been removed, got: %s", got)
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
		t.Errorf("KindStrong should have been replaced, got: %s", html)
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
	// Walker should visit the replacement's children (KindText "inner")
	found := false
	for _, k := range visitedAfterReplace {
		if k == djot.KindText {
			found = true
		}
	}
	if !found {
		t.Error("expected walker to visit replacement's KindText child")
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
		t.Errorf("all KindStrong nodes should have been removed, got: %s", html)
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
	// Last node visited should be KindDocument (root).
	if kinds[len(kinds)-1] != djot.KindDocument {
		t.Errorf("expected KindDocument visited last, got %s", kinds[len(kinds)-1])
	}
	// KindText nodes should appear before KindParagraph.
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
		t.Errorf("expected KindText before KindParagraph in bottom-up order, text=%d para=%d", textIdx, paraIdx)
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
