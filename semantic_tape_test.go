package djot

import (
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func renderTreeHTMLForTest(doc *Doc, opts ...RenderOption) string {
	doc.Root()
	tape := doc.semantic
	doc.semantic = nil
	result := RenderHTML(doc, opts...)
	doc.semantic = tape
	return result
}

func findTypedNode[T Node](root Node) T {
	var found T
	have := false
	walkRead(root, func(node Node) {
		if candidate, ok := node.(T); ok && !have {
			found = candidate
			have = true
		}
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
		if !doc.semantic.matchesAST(doc.Root()) {
			t.Fatal("freshly parsed AST does not match its semantic tape")
		}
		if got, want := renderSemanticHTML(doc.semantic), renderTreeHTMLForTest(doc); got != want {
			t.Fatalf("tape output differs from tree renderer\nwant: %q\n got: %q", want, got)
		}
	}
}

func TestDocumentMaterializesLazily(t *testing.T) {
	doc := Parse("text[^a]\n\n[^a]: note\n")
	if doc.root != nil {
		t.Fatal("Parse eagerly materialized the public AST")
	}
	_ = RenderHTML(doc)
	if doc.root != nil {
		t.Fatal("default HTML rendering materialized the public AST")
	}

	root := doc.Root()
	if root == nil || doc.Root() != root {
		t.Fatal("Root did not materialize exactly once")
	}
	footnote := doc.Footnotes()["a"]
	if footnote == nil {
		t.Fatal("Footnotes did not rebuild the parsed definition index")
	}
	found := false
	Walk(root, func(node Node) Action {
		if node == footnote {
			found = true
		}
		return Continue
	})
	if !found {
		t.Fatal("Footnotes returned a node outside the materialized root")
	}
}

func TestDocumentMaterializesReferences(t *testing.T) {
	doc := Parse("[label]: /target\n\n[label][]\n")
	ref := doc.References()["label"]
	if ref == nil || ref.Destination != "/target" {
		t.Fatalf("References()[label] = %#v", ref)
	}
}

func TestNewDocRendersMutableAST(t *testing.T) {
	doc := NewDoc(&Document{
		Children: []Block{&Paragraph{
			Children: []Inline{&Text{Value: "custom"}},
		}},
	})
	if got, want := RenderHTML(doc), "<p>custom</p>\n"; got != want {
		t.Fatalf("RenderHTML(NewDoc(...)) = %q, want %q", got, want)
	}
	findTypedNode[*Text](doc.Root()).Value = "changed"
	if got, want := RenderHTML(doc), "<p>changed</p>\n"; got != want {
		t.Fatalf("RenderHTML after mutation = %q, want %q", got, want)
	}
	if got := NewDoc(nil).Footnotes(); len(got) != 0 {
		t.Fatalf("NewDoc(nil).Footnotes() = %v, want empty", got)
	}
	refs := doc.References()
	refs["custom"] = &Reference{Destination: "/custom", DestinationSet: true}
	if doc.References()["custom"].Destination != "/custom" {
		t.Fatal("NewDoc References map is not mutable and persistent")
	}

	replacement := &Document{Children: []Block{&Paragraph{
		Children: []Inline{&Text{Value: "replacement"}},
	}}}
	doc.SetRoot(replacement)
	if doc.Root() != replacement || RenderHTML(doc) != "<p>replacement</p>\n" {
		t.Fatal("SetRoot did not replace the rendered AST")
	}
}

func TestDocumentLazyStateConcurrentFirstAccess(t *testing.T) {
	doc := Parse("reference[^note]\n\n[^note]: body\n\n[label]: /target\n")
	const workers = 32
	roots := make([]*Document, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			switch i % 4 {
			case 0:
				roots[i] = doc.Root()
			case 1:
				if doc.Footnotes()["note"] == nil {
					t.Errorf("Footnotes()[note] is nil")
				}
				roots[i] = doc.Root()
			case 2:
				if doc.References()["label"] == nil {
					t.Errorf("References()[label] is nil")
				}
				roots[i] = doc.Root()
			case 3:
				_ = RenderHTML(doc)
				roots[i] = doc.Root()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	for i, root := range roots {
		if root == nil || root != roots[0] {
			t.Fatalf("root %d = %p, want shared root %p", i, root, roots[0])
		}
	}
}

func TestSemanticTapePreservesWideOrderedListStart(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("requires a 64-bit int")
	}
	const start = 2147483648
	doc := Parse("2147483648. item\n")
	if got, want := RenderHTML(doc), "<ol start=\"2147483648\">\n<li>\nitem\n</li>\n</ol>\n"; got != want {
		t.Fatalf("wide ordered-list render differs\nwant: %q\n got: %q", want, got)
	}
	list := findTypedNode[*OrderedList](doc.Root())
	if list == nil || list.Start != start {
		t.Fatalf("materialized ListStart = %v, want %d", list, start)
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
	assertPanic("text high bit", func() { checkedSemanticTextIndex(int(semanticStoredText), "test") })
	if strconv.IntSize == 64 {
		assertPanic("uint32 overflow", func() {
			checkedSemanticUint32(int(uint64(^uint32(0))+1), "test")
		})
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

func TestSemanticTapeMutationFallback(t *testing.T) {
	mutateFirst := func(kind Kind, mutate func(Node)) func(*Doc) {
		return func(doc *Doc) {
			var found Node
			Walk(doc.Root(), func(node Node) Action {
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
		mutate func(*Doc)
	}{
		{
			name:  "text",
			input: "hello\n",
			mutate: func(doc *Doc) {
				findTypedNode[*Text](doc.Root()).Value = "changed"
			},
		},
		{
			name:  "attributes",
			input: "# heading\n",
			mutate: func(doc *Doc) {
				findTypedNode[*Section](doc.Root()).Attributes().Set("class", "changed")
			},
		},
		{
			name:  "target",
			input: "[link](old)\n",
			mutate: func(doc *Doc) {
				findTypedNode[*Link](doc.Root()).Destination = "new"
			},
		},
		{
			name:  "children",
			input: "one\n",
			mutate: func(doc *Doc) {
				doc.Root().Children = append(doc.Root().Children, &Paragraph{
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
			doc := Parse(test.input)
			test.mutate(doc)
			if doc.semantic.matchesAST(doc.Root()) {
				t.Fatal("mutated AST still matches immutable tape")
			}
			want := renderTreeHTMLForTest(doc)
			got := RenderHTML(doc)
			if got != want {
				t.Fatalf("fallback output differs\nwant: %q\n got: %q", want, got)
			}
		})
	}
}

func TestSemanticTapeOptionsUseTreeRenderer(t *testing.T) {
	doc := Parse("reference[^a]\n\n[^a]: note\n")
	want := renderTreeHTMLForTest(doc, WithMultiBacklinks())
	got := RenderHTML(doc, WithMultiBacklinks())
	if got != want {
		t.Fatalf("option fallback differs\nwant: %q\n got: %q", want, got)
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

func BenchmarkProductionSemanticFastPath(b *testing.B) {
	input := strings.Repeat("A paragraph with *emphasis*, **strong**, and [a link](https://example.com).\n\n", 2048)
	doc := Parse(input)
	b.Run("CompactTape", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = RenderHTML(doc)
		}
	})
	b.Run("TreeAST", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = renderTreeHTMLForTest(doc)
		}
	})
	b.Run("TapeToDiscard", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if err := RenderHTMLTo(io.Discard, doc); err != nil {
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
		root := doc.Root()
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
