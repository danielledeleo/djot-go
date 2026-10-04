package djot

import (
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unsafe"

	. "github.com/danielledeleo/djot-go/ast"
)

func renderTreeHTMLForTest(doc *Doc, opts ...RenderOption) string {
	var b strings.Builder
	r := newHTMLRendererWithConfig(&b, doc.AST(), makeRenderConfig(opts))
	r.renderDocument()
	return b.String()
}

// RenderTreeHTMLForTest bypasses the compact tape for external tests.
func RenderTreeHTMLForTest(doc *Doc, opts ...RenderOption) string {
	return renderTreeHTMLForTest(doc, opts...)
}

func findTypedNode[T Node](root Node) T {
	var found T
	have := false
	Preorder(root, func(node Node) bool {
		if candidate, ok := node.(T); ok && !have {
			found = candidate
			have = true
		}
		return !have
	})
	return found
}

func TestSemanticTapeDefaultRendering(t *testing.T) {
	inputs := []string{
		"# Heading\n\nText with *emphasis* and [a link](https://example.com).\n",
		"- tight\n- list\n\n[^a]: note\n\nreference[^a]\n",
		"| left | right |\n|:-----|------:|\n| a | b |\n",
		"::: warning\nraw `{=html} and :symbol:\n:::\n",
	}
	for _, input := range inputs {
		doc := Parse(input)
		if doc.semantic == nil {
			t.Fatal("Parse did not attach a semantic tape")
		}
		if got, want := renderSemanticHTML(doc.semantic), renderTreeHTMLForTest(doc); got != want {
			t.Fatalf("tape output differs from tree renderer\nwant: %q\n got: %q", want, got)
		}
	}
}

// AST returns a new tree on each call, and nothing done to it reaches the
// document.
func TestASTReturnsIndependentTrees(t *testing.T) {
	doc := Parse("text[^a] and [label][]\n\n[^a]: note\n\n[label]: /target\n")
	before := RenderHTML(doc)
	first, second := doc.AST(), doc.AST()
	if first == second {
		t.Fatal("AST returned the same tree twice")
	}
	if findTypedNode[*Footnote](first) == nil {
		t.Fatal("AST has no footnote definition")
	}
	if ref := first.References["label"]; ref == nil || ref.Destination != "/target" {
		t.Fatalf("References[label] = %#v", ref)
	}
	findTypedNode[*Text](first).Value = "changed"
	first.References["label"].Destination = "/changed"
	if got := RenderHTML(doc); got != before {
		t.Fatalf("editing an AST changed the document\nbefore: %q\n after: %q", before, got)
	}
	if third := doc.AST(); findTypedNode[*Text](third).Value != "text" || third.References["label"].Destination != "/target" {
		t.Fatal("editing an AST changed later trees")
	}
}

// NewDoc copies what it needs from the tree: later edits to the tree do not
// reach the document, and references survive the trip.
func TestNewDocIsIndependentOfItsTree(t *testing.T) {
	tree := &Document{
		Children:   []Block{&Paragraph{Children: []Inline{&Text{Value: "custom"}}}},
		References: map[string]*Reference{"custom": {Destination: "/custom", DestinationSet: true}},
	}
	doc := NewDoc(tree)
	if got, want := RenderHTML(doc), "<p>custom</p>\n"; got != want {
		t.Fatalf("RenderHTML(NewDoc(...)) = %q, want %q", got, want)
	}
	findTypedNode[*Text](tree).Value = "changed"
	if got, want := RenderHTML(doc), "<p>custom</p>\n"; got != want {
		t.Fatalf("editing the tree changed the document: %q", got)
	}
	if got, want := RenderHTML(NewDoc(tree)), "<p>changed</p>\n"; got != want {
		t.Fatalf("RenderHTML(NewDoc(edited)) = %q, want %q", got, want)
	}
	if ref := doc.AST().References["custom"]; ref == nil || ref.Destination != "/custom" {
		t.Fatalf("References did not survive NewDoc: %#v", ref)
	}
	if got := RenderHTML(NewDoc(nil)); got != "" {
		t.Fatalf("RenderHTML(NewDoc(nil)) = %q, want empty", got)
	}
	if got := RenderHTML(&Doc{}); got != "" {
		t.Fatalf("RenderHTML of a zero Doc = %q, want empty", got)
	}
}

// A Doc is immutable, so concurrent rendering and tree building need no
// coordination. Run under -race.
func TestDocConcurrentUse(t *testing.T) {
	doc := Parse("reference[^note]\n\n[^note]: body\n\n[label]: /target\n")
	want := RenderHTML(doc)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				if got := RenderHTML(doc); got != want {
					t.Errorf("concurrent render differs: %q", got)
				}
			case 1:
				if doc.AST().References["label"] == nil {
					t.Errorf("References[label] is nil")
				}
			case 2:
				_ = doc.Position(doc.AST().Children[0].Span().Start)
			}
		}(i)
	}
	wg.Wait()
}

// The largest accepted enumerator round-trips through the tape, and the first
// value past the bound is not a list at all. Both hold on every platform:
// maxOrderedEnum is MaxInt32, so neither case depends on the int width.
func TestSemanticTapePreservesWideOrderedListStart(t *testing.T) {
	const start = 2147483647
	doc := Parse("2147483647. item\n")
	if got, want := RenderHTML(doc), "<ol start=\"2147483647\">\n<li>\nitem\n</li>\n</ol>\n"; got != want {
		t.Fatalf("wide ordered-list render differs\nwant: %q\n got: %q", want, got)
	}
	list := findTypedNode[*OrderedList](doc.AST())
	if list == nil || list.Start != start {
		t.Fatalf("materialized ListStart = %v, want %d", list, start)
	}

	past := Parse("2147483648. item\n")
	if got := RenderHTML(past); strings.Contains(got, "<ol") {
		t.Fatalf("enumerator past maxOrderedEnum became a list: %q", got)
	}
}

func TestSemanticTapeSparseCapacityMatchesStructure(t *testing.T) {
	doc := Parse(strings.Repeat("a", 1<<20))
	if got, want := cap(doc.semantic.records), len(doc.semantic.records); got != want {
		t.Fatalf("sparse record capacity = %d, want exact %d", got, want)
	}
	if got, want := cap(doc.semantic.attributes), len(doc.semantic.attributes); got != want {
		t.Fatalf("sparse attribute capacity = %d, want exact %d", got, want)
	}
}

func TestSemanticTapeCheckedLimitsPanicInsteadOfCorrupting(t *testing.T) {
	assertPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		fn()
	}
	assertPanic("negative uint32", func() { checkedSemanticUint32(-1, "test") })
	assertPanic("negative text index", func() { checkedSemanticTextIndex(-1, "test") })
	// Both upper limits sit at or above MaxInt32, so on a 32-bit build no int
	// can reach them and only the negative cases above are exercisable. The
	// values go through variables so the conversions stay runtime ones: as
	// constant expressions they would not compile for a 32-bit int.
	if strconv.IntSize == 64 {
		highBit := uint64(semanticStoredText)
		assertPanic("text high bit", func() { checkedSemanticTextIndex(int(highBit), "test") })
		pastUint32 := uint64(^uint32(0)) + 1
		assertPanic("uint32 overflow", func() { checkedSemanticUint32(int(pastUint32), "test") })
	}
}

func TestSemanticHTMLToSparseDocumentDoesNotBufferOutput(t *testing.T) {
	doc := Parse(strings.Repeat("a", 1<<20))
	var writer firstWriteRecorder
	allocs := testing.AllocsPerRun(5, func() {
		writer = firstWriteRecorder{}
		if err := RenderHTMLTo(&writer, doc); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 100 {
		t.Fatalf("RenderHTMLTo allocations = %.0f, want bounded small count", allocs)
	}
	if writer.writes < 2 || writer.firstWrite > 1024 {
		t.Fatalf("RenderHTMLTo write shape = %d writes, first %d bytes; output appears buffered", writer.writes, writer.firstWrite)
	}
}

type firstWriteRecorder struct {
	writes     int
	firstWrite int
}

func (w *firstWriteRecorder) Write(p []byte) (int, error) {
	if w.writes == 0 {
		w.firstWrite = len(p)
	}
	w.writes++
	return len(p), nil
}

// Every field the renderer reads survives NewDoc: a tree edited through AST
// and passed to NewDoc renders exactly as the tree renderer renders it.
func TestNewDocRendersEditedTrees(t *testing.T) {
	mutateFirst := func(kind Kind, mutate func(Node)) func(*Document) {
		return func(tree *Document) {
			var found Node
			Walk(tree, func(node Node) Action {
				if found == nil && node.Kind() == kind {
					found = node
				}
				return Continue
			})
			if found == nil {
				panic("test input did not produce requested node kind")
			}
			mutate(found)
		}
	}
	tests := []struct {
		name   string
		input  string
		mutate func(*Document)
	}{
		{
			name:  "text",
			input: "hello\n",
			mutate: func(tree *Document) {
				findTypedNode[*Text](tree).Value = "changed"
			},
		},
		{
			name:  "attributes",
			input: "# heading\n",
			mutate: func(tree *Document) {
				findTypedNode[*Section](tree).Attributes().Set("class", "changed")
			},
		},
		{
			name:  "target",
			input: "[link](old)\n",
			mutate: func(tree *Document) {
				findTypedNode[*Link](tree).Destination = "new"
			},
		},
		{
			name:  "children",
			input: "one\n",
			mutate: func(tree *Document) {
				tree.Children = append(tree.Children, &Paragraph{
					Children: []Inline{&Text{Value: "two"}},
				})
			},
		},
		{
			name:   "heading level",
			input:  "# heading\n",
			mutate: mutateFirst(KindHeading, func(node Node) { node.(*Heading).Level = 2 }),
		},
		{
			name:  "ordered list style and start",
			input: "2. item\n",
			mutate: mutateFirst(KindOrderedList, func(node Node) {
				node.(*OrderedList).Style = ListAlphaUpper
				node.(*OrderedList).Start = 27
			}),
		},
		{
			name:   "tight list",
			input:  "- one\n- two\n",
			mutate: mutateFirst(KindBulletList, func(node Node) { node.(*BulletList).Tight = false }),
		},
		{
			name:   "task checked",
			input:  "- [ ] item\n",
			mutate: mutateFirst(KindTaskListItem, func(node Node) { node.(*TaskListItem).Checked = true }),
		},
		{
			name:  "table cell flags",
			input: "| a |\n|:--:|\n| b |\n",
			mutate: mutateFirst(KindTableCell, func(node Node) {
				node.(*TableCell).Alignment, node.(*TableCell).Header = AlignRight, false
			}),
		},
		{
			name:   "explicit empty target",
			input:  "[link]()\n",
			mutate: mutateFirst(KindLink, func(node Node) { node.(*Link).DestinationSet = false }),
		},
		{
			name:  "code block payload",
			input: "``` go\ncode\n```\n",
			mutate: mutateFirst(KindCodeBlock, func(node Node) {
				node.(*CodeBlock).Text = "changed"
				node.(*CodeBlock).Language = "rust"
			}),
		},
		{
			name:  "raw payload",
			input: "``` =html\n<b>raw</b>\n```\n",
			mutate: mutateFirst(KindRawBlock, func(node Node) {
				node.(*RawBlock).Format = "latex"
			}),
		},
		{
			name:   "symbol name",
			input:  ":symbol:\n",
			mutate: mutateFirst(KindSymbol, func(node Node) { node.(*Symbol).Name = "changed" }),
		},
		{
			name:   "footnote label",
			input:  "reference[^a]\n\n[^a]: note\n",
			mutate: mutateFirst(KindFootnoteReference, func(node Node) { node.(*FootnoteReference).Label = "changed" }),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tree := Parse(test.input).AST()
			test.mutate(tree)
			doc := NewDoc(tree)
			if before := RenderHTML(Parse(test.input)); RenderHTML(doc) == before {
				t.Fatal("the edit did not change the output")
			}
			if got, want := RenderHTML(doc), renderTreeHTMLForTest(doc); got != want {
				t.Fatalf("edited tree output differs\nwant: %q\n got: %q", want, got)
			}
		})
	}
}

func TestSemanticTapeFootnoteOptionsStayCompact(t *testing.T) {
	const input = "reference[^a] again[^a]\n\n[^a]: note\n"
	options := []RenderOption{
		WithMultiBacklinks(),
		WithFootnotePrefix(`p"<&`),
		WithFootnoteBacklinkLabel(func(_, k, _ int) string { return "<" + strconv.Itoa(k) + ">" }),
	}
	want := renderTreeHTMLForTest(Parse(input), options...)
	doc := Parse(input)
	got := RenderHTML(doc, options...)
	if got != want {
		t.Fatalf("compact option output differs\nwant: %q\n got: %q", want, got)
	}
}

func TestSemanticTapeSourceTextAndStoredText(t *testing.T) {
	doc := Parse("plain text and \\*escaped\\* plus 'quotes'\n")
	spans, values := len(doc.semantic.textSpans)-1, len(doc.semantic.textValues)-1
	if spans == 0 || values == 0 {
		t.Fatalf("expected source-backed and stored text, got spans=%d values=%d", spans, values)
	}
	if got, want := RenderHTML(doc), renderTreeHTMLForTest(doc); got != want {
		t.Fatalf("mixed text storage differs\nwant: %q\n got: %q", want, got)
	}
}

func FuzzSemanticTapeHTMLParity(f *testing.F) {
	for _, seed := range []string{
		"plain text\n",
		"# heading\n\n*emphasis* and [link](target)\n",
		"reference[^a]\n\n[^a]: note\n",
		"| a | b |\n|---|---|\n| c | d |\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		doc := Parse(input)
		got := RenderHTML(doc)
		want := renderTreeHTMLForTest(doc)
		if got != want {
			t.Fatalf("tape output differs from AST renderer\ninput: %q\nwant: %q\n got: %q", input, want, got)
		}
	})
}

func FuzzSemanticTapeFootnoteOptionParity(f *testing.F) {
	for _, seed := range []string{
		"plain text\n",
		"reference[^a] again[^a]\n\n[^a]: note\n",
		"outer[^a]\n\n[^a]: nested[^b]\n\n[^b]: second\n",
	} {
		f.Add(seed)
	}
	options := []RenderOption{
		WithMultiBacklinks(),
		WithFootnotePrefix(`p"<&`),
		WithFootnoteBacklinkLabel(func(_, k, _ int) string { return "<" + strconv.Itoa(k) + ">" }),
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := RenderHTML(Parse(input), options...)
		want := renderTreeHTMLForTest(Parse(input), options...)
		if got != want {
			t.Fatalf("compact footnote options differ from tree renderer\ninput: %q\nwant: %q\n got: %q", input, want, got)
		}
	})
}

func BenchmarkProductionSemanticFastPath(b *testing.B) {
	input := strings.Repeat("A paragraph with *emphasis*, **strong**, and [a link](https://example.com).\n\n", 2048)
	compactDoc := Parse(input)
	treeDoc := Parse(input)
	discardDoc := Parse(input)
	footnoteOptionDoc := Parse(input)
	footnoteOption := WithFootnotePrefix("post-")
	nodeHookDoc := Parse(input)
	nodeHook := WithNodeRenderer(KindCodeBlock, func(_ Node, r NodeRenderer) { r.Default() })
	b.Run("CompactTape", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = RenderHTML(compactDoc)
		}
	})
	b.Run("TreeAST", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = renderTreeHTMLForTest(treeDoc)
		}
	})
	b.Run("TapeToDiscard", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, discardDoc); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("FootnoteOptionToDiscard", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, footnoteOptionDoc, footnoteOption); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("AbsentNodeHookToDiscard", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, nodeHookDoc, nodeHook); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkInternalParseNodeLayout(b *testing.B) {
	b.ReportMetric(float64(unsafe.Sizeof(parseNode{})), "common-B/node")
	b.ReportMetric(float64(unsafe.Sizeof(parsePayload{})), "rare-payload-B")
	for i := 0; i < b.N; i++ {
	}
}

func BenchmarkMaterializeTypedAST(b *testing.B) {
	parsed := Parse(benchmarkHugeDocument())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		doc := &Doc{semantic: parsed.semantic}
		root := doc.AST()
		if root == nil {
			b.Fatal("materialized nil root")
		}
	}
}

func benchmarkHugeDocument() string {
	return strings.Repeat("A paragraph with *emphasis*, **strong**, and [a link](https://example.com).\n\n", 14000)
}

func BenchmarkSparseDocument(b *testing.B) {
	input := strings.Repeat("a", 10<<20)
	b.Run("Parse10MB", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = Parse(input)
		}
	})
	doc := Parse(input)
	b.Run("RenderHTMLToDiscard10MB", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc); err != nil {
				b.Fatal(err)
			}
		}
	})
}
