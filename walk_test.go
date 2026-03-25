package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestWalkContinue(t *testing.T) {
	doc := djot.Parse("Hello *world* and _more_")
	var kinds []djot.NodeKind
	djot.Walk(doc.Root, func(n *djot.Node) any {
		kinds = append(kinds, n.Kind)
		return djot.Continue
	})
	// Should visit all nodes: Section, Paragraph, Text, Strong, Text, Text, Emphasis, Text
	if len(kinds) == 0 {
		t.Fatal("Walk visited no nodes")
	}
	// First child of root should be Section (from heading wrapping) or Paragraph
	found := false
	for _, k := range kinds {
		if k == djot.Paragraph {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Paragraph in visited nodes, got %v", kinds)
	}
}

func TestWalkSkipChildren(t *testing.T) {
	doc := djot.Parse("Hello *world*")
	var visited []djot.NodeKind
	djot.Walk(doc.Root, func(n *djot.Node) any {
		visited = append(visited, n.Kind)
		if n.Kind == djot.Strong {
			return djot.SkipChildren
		}
		return djot.Continue
	})
	// Should visit Strong but NOT its Text child
	hasStrong := false
	for _, k := range visited {
		if k == djot.Strong {
			hasStrong = true
		}
	}
	if !hasStrong {
		t.Fatal("expected Strong to be visited")
	}
	// Count Text nodes — should only see the "Hello " text, not "world" inside Strong
	textCount := 0
	for _, k := range visited {
		if k == djot.Text {
			textCount++
		}
	}
	if textCount != 1 {
		t.Errorf("expected 1 Text node (skipped Strong's child), got %d", textCount)
	}
}

func TestWalkRemove(t *testing.T) {
	doc := djot.Parse("Hello *world* goodbye")
	djot.Walk(doc.Root, func(n *djot.Node) any {
		if n.Kind == djot.Strong {
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
	djot.Walk(doc.Root, func(n *djot.Node) any {
		if n.Kind == djot.Strong {
			return djot.Replace(&djot.Node{
				Kind: djot.Emphasis,
				Children: []*djot.Node{
					{Kind: djot.Text, Text: "replaced"},
				},
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
	var visitedAfterReplace []djot.NodeKind
	replaced := false
	djot.Walk(doc.Root, func(n *djot.Node) any {
		if n.Kind == djot.Strong && !replaced {
			replaced = true
			return djot.Replace(&djot.Node{
				Kind: djot.Emphasis,
				Children: []*djot.Node{
					{Kind: djot.Text, Text: "inner"},
				},
			})
		}
		if replaced {
			visitedAfterReplace = append(visitedAfterReplace, n.Kind)
		}
		return djot.Continue
	})
	// Walker should visit the replacement's children (Text "inner")
	found := false
	for _, k := range visitedAfterReplace {
		if k == djot.Text {
			found = true
		}
	}
	if !found {
		t.Error("expected walker to visit replacement's Text child")
	}
}

func TestWalkDoesNotVisitRoot(t *testing.T) {
	doc := djot.Parse("Hello")
	visitedRoot := false
	djot.Walk(doc.Root, func(n *djot.Node) any {
		if n.Kind == djot.Document {
			visitedRoot = true
		}
		return djot.Continue
	})
	if visitedRoot {
		t.Error("Walk should not visit the root node itself")
	}
}

func TestWalkRemoveMultiple(t *testing.T) {
	doc := djot.Parse("*a* *b* *c*")
	djot.Walk(doc.Root, func(n *djot.Node) any {
		if n.Kind == djot.Strong {
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
	var kinds []djot.NodeKind
	djot.WalkBottomUp(doc.Root, func(n *djot.Node) {
		kinds = append(kinds, n.Kind)
	})
	if len(kinds) == 0 {
		t.Fatal("WalkBottomUp visited no nodes")
	}
	// Bottom-up: leaf nodes should appear before their parents.
	// Last node visited should be Document (root).
	if kinds[len(kinds)-1] != djot.Document {
		t.Errorf("expected Document visited last, got %s", kinds[len(kinds)-1])
	}
	// Text nodes should appear before Paragraph.
	textIdx, paraIdx := -1, -1
	for i, k := range kinds {
		if k == djot.Text && textIdx == -1 {
			textIdx = i
		}
		if k == djot.Paragraph {
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
	djot.WalkBottomUp(doc.Root, func(n *djot.Node) {
		if n.Kind == djot.Document {
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
