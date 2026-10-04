package djot

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go/ast"
)

var compileSeeds = []string{
	"# Title\n\n*strong* _em_ [link](u) ![img](i.png) :sym: `code` $`m`\n",
	"- a\n- b\n\n1. x\n2. y\n\n- [ ] t\n- [x] u\n",
	"| a | b |\n|:--|--:|\n| c | d |\n^ caption\n",
	"ref[^a] again[^a]\n\n[^a]: note with [link][r]\n\n[r]: /dest\n",
	"::: note\n> quote\n\n``` go\ncode\n```\n:::\n\nterm\n: def\n",
	"{#id .c}\n## Heading\n\ntext\\\nbreak \"quote\" 'single' -- --- ...\n\n``` =html\n<b>\n```\n",
}

func renderCompiledForTest(tree *ast.Document) string {
	var b strings.Builder
	if err := renderSemanticHTMLTo(&b, compileSemanticTape(tree, nil)); err != nil {
		panic(err)
	}
	return b.String()
}

// mutateForTest applies random edits through the public AST API.
func mutateForTest(root *ast.Document, seed int64) *ast.Document {
	rng := rand.New(rand.NewSource(seed))
	out := ast.Walk(root, func(n ast.Node) ast.Action {
		if _, isDoc := n.(*ast.Document); isDoc {
			return ast.Continue
		}
		switch rng.Intn(12) {
		case 0:
			return ast.Remove
		case 1:
			if s, ok := n.(*ast.Strong); ok {
				e := &ast.Emphasis{Children: s.Children}
				ast.CopyMetadata(e, s)
				return ast.Replace(e)
			}
		case 2:
			if x, ok := n.(*ast.Text); ok {
				x.Value += ` <&"edited">`
			}
		case 3:
			n.Attributes().Set("data-x", "1<2")
			n.Attributes().AddClass("added")
		case 4:
			switch x := n.(type) {
			case *ast.Link:
				x.Destination, x.DestinationSet = "", rng.Intn(2) == 0
			case *ast.Image:
				x.Destination = "new.png"
			case *ast.Heading:
				x.Level = 1 + rng.Intn(6)
			case *ast.BulletList:
				x.Tight = !x.Tight
			case *ast.OrderedList:
				x.Start, x.Tight = rng.Intn(50), !x.Tight
			case *ast.TaskListItem:
				x.Checked = !x.Checked
			case *ast.CodeBlock:
				x.Language, x.Text = "zz", x.Text+"<x>"
			case *ast.TableCell:
				x.Alignment = ast.CellAlign(rng.Intn(4))
			case *ast.Footnote:
				x.Label = "renamed"
			}
		case 5:
			ast.SetSpan(n, ast.SourceSpan{})
		case 6:
			if p, ok := n.(*ast.Paragraph); ok {
				p.Children = append(p.Children, &ast.Text{Value: " appended"}, &ast.Symbol{Name: "x"})
			}
		}
		return ast.Continue
	})
	if out == nil {
		return &ast.Document{}
	}
	return out.(*ast.Document)
}

// A compiled tree must render exactly as the typed-tree renderer renders it,
// including after arbitrary edits.
func TestCompiledTapeMatchesTreeRenderer(t *testing.T) {
	for _, src := range compileSeeds {
		for seed := int64(0); seed < 200; seed++ {
			tree := mutateForTest(Parse(src).AST(), seed)
			want := renderTreeHTMLForTest(NewDoc(tree))
			if got := renderCompiledForTest(tree); got != want {
				t.Fatalf("seed %d: compiled tape differs\ninput: %q\nwant: %q\n got: %q", seed, src, want, got)
			}
		}
	}
}

// Compiling and materializing are inverses: compiling a tree, building a tree
// back, and compiling again gives an identical tape.
func TestCompileMaterializeRoundTrip(t *testing.T) {
	for _, src := range compileSeeds {
		for seed := int64(0); seed < 50; seed++ {
			tree := mutateForTest(Parse(src).AST(), seed)
			first := compileSemanticTape(tree, tree.References)
			rebuilt := first.materializeAST()
			if second := compileSemanticTape(rebuilt, first.materializeReferences()); !reflect.DeepEqual(first, second) {
				t.Fatalf("seed %d: round trip changed the tape\ninput: %q", seed, src)
			}
		}
	}
}

func TestNewDocScratchTree(t *testing.T) {
	text := func(s string) *ast.Text { return &ast.Text{Value: s} }
	tree := &ast.Document{Children: []ast.Block{
		&ast.Heading{Level: 2, Children: []ast.Inline{text("Built "), &ast.Strong{Children: []ast.Inline{text("by hand")}}}},
		&ast.Paragraph{Children: []ast.Inline{
			&ast.Link{Destination: "https://x", DestinationSet: true, Children: []ast.Inline{text("link")}},
			text(" and a note"), &ast.FootnoteReference{Label: "n"},
		}},
		&ast.BulletList{Tight: true, Marker: '-', Items: []*ast.ListItem{
			{Children: []ast.Block{&ast.Paragraph{Children: []ast.Inline{text("one")}}}},
		}},
		&ast.Footnote{Label: "n", Children: []ast.Block{&ast.Paragraph{Children: []ast.Inline{text("the note")}}}},
		&ast.Paragraph{},
		&ast.Section{},
	}}
	doc := NewDoc(tree)
	if got, want := RenderHTML(doc), renderTreeHTMLForTest(doc); got != want {
		t.Fatalf("hand-built tree differs\nwant: %q\n got: %q", want, got)
	}
}

// Spans do not affect HTML, so a negative offset set through ast.SetSpan must
// not stop a tree from rendering.
func TestNewDocNegativeSpan(t *testing.T) {
	tree := &ast.Document{Children: []ast.Block{&ast.Paragraph{Children: []ast.Inline{&ast.Text{Value: "x"}}}}}
	ast.SetSpan(tree.Children[0], ast.SourceSpan{Start: ast.Pos{Offset: -5}, End: ast.Pos{Offset: -1}})
	if got := RenderHTML(NewDoc(tree)); got != "<p>x</p>\n" {
		t.Fatalf("got %q", got)
	}
}

func FuzzCompiledTapeMatchesTreeRenderer(f *testing.F) {
	for i, s := range compileSeeds {
		f.Add(s, int64(i))
	}
	f.Fuzz(func(t *testing.T, src string, seed int64) {
		tree := mutateForTest(Parse(src).AST(), seed)
		want := renderTreeHTMLForTest(NewDoc(tree))
		if got := renderCompiledForTest(tree); got != want {
			t.Fatalf("compiled tape differs\ninput: %q seed %d\nwant: %q\n got: %q", src, seed, want, got)
		}
	})
}
