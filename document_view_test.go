package djot_test

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestDocumentRendererBuildsTOCBeforeOutput(t *testing.T) {
	doc := djot.Parse("# First\n\nText.\n\n## Second *heading*\n\nMore.\n")
	got := djot.RenderHTML(doc, djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
		headings := document.Headings()
		r.Write("<nav><ol>")
		for _, heading := range headings {
			r.Write(`<li data-level="` + strconv.Itoa(heading.Level()) + `"><a href="#`)
			r.Write(heading.ID())
			r.Write(`">` + heading.Text() + `</a></li>`)
		}
		r.Write("</ol></nav>\n")
		r.Default()
	}))
	for _, fragment := range []string{
		`<nav><ol><li data-level="1"><a href="#First">First</a></li>`,
		`<li data-level="2"><a href="#Second-heading">Second heading</a></li></ol></nav>`,
		`<section id="First">`,
		`<h2>Second <strong>heading</strong></h2>`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("document output missing %q:\n%s", fragment, got)
		}
	}
}

func TestDocumentViewHeadingParity(t *testing.T) {
	type headingSnapshot struct {
		level int
		text  string
		id    string
		span  djot.SourceSpan
	}
	input := "# Hello :star: `code` -- ...\n\n::: note\n## “Nested”\n:::\n"
	var wantHeadings []headingSnapshot

	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			var headings []headingSnapshot
			djot.RenderHTML(doc, djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
				first := document.Headings()
				second := document.Headings()
				if len(first) > 0 && &first[0] != &second[0] {
					t.Fatal("Headings index was rebuilt within one render")
				}
				for _, heading := range first {
					headings = append(headings, headingSnapshot{
						level: heading.Level(), text: heading.Text(),
						id: heading.ID(), span: heading.Span(),
					})
				}
				r.Default()
			}))
			if backend == "tape" {
				wantHeadings = headings
				return
			}
			if len(headings) != len(wantHeadings) {
				t.Fatalf("heading lengths differ: %d/%d", len(headings), len(wantHeadings))
			}
			for i := range headings {
				if headings[i] != wantHeadings[i] {
					t.Fatalf("heading %d differs\nwant: %#v\n got: %#v", i, wantHeadings[i], headings[i])
				}
			}
		})
	}
	if len(wantHeadings) != 2 {
		t.Fatalf("heading count = %d, want 2", len(wantHeadings))
	}
	if wantHeadings[0].text != "Hello :star: code – …" || wantHeadings[1].text != "“Nested”" {
		t.Fatalf("heading text = %#v", wantHeadings)
	}
}

func TestDocumentViewKindCounts(t *testing.T) {
	const input = "# Heading\n\n::: note\n:star:\n:::\n"
	var want []int
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			var counts []int
			djot.RenderHTML(doc, djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
				for kind := djot.KindDocument; kind <= djot.KindEnDash; kind++ {
					count := document.Count(kind)
					if document.Contains(kind) != (count != 0) {
						t.Fatalf("Contains(%v) disagrees with Count=%d", kind, count)
					}
					counts = append(counts, count)
				}
				if document.Count(djot.Kind(-1)) != 0 || document.Count(djot.KindEnDash+1) != 0 {
					t.Fatal("out-of-range kind count was nonzero")
				}
				r.Default()
			}))
			if backend == "tape" {
				want = counts
				return
			}
			if !equalDocumentSummary(counts, want) {
				t.Fatalf("tree counts differ\nwant: %v\n got: %v", want, counts)
			}
		})
	}
	for kind, count := range map[djot.Kind]int{
		djot.KindDocument:  1,
		djot.KindSection:   1,
		djot.KindHeading:   1,
		djot.KindDiv:       1,
		djot.KindParagraph: 1,
		djot.KindSymbol:    1,
	} {
		if got := want[int(kind)]; got != count {
			t.Fatalf("Count(%v) = %d, want %d", kind, got, count)
		}
	}
}

func TestDocumentRendererWrapsEndnotes(t *testing.T) {
	const input = "reference[^a]\n\n[^a]: note\n"
	for _, backend := range []string{"tape", "tree", "configured tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(input)
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			options := []djot.RenderOption{djot.WithDocumentRenderer(func(_ djot.DocumentView, r djot.DocumentRenderer) {
				r.Write("<article>")
				r.Default()
				r.Write("</article>")
			})}
			if backend == "configured tree" {
				options = append(options, djot.WithMultiBacklinks())
			}
			got := djot.RenderHTML(doc, options...)
			endnotes := strings.Index(got, `role="doc-endnotes"`)
			closing := strings.Index(got, "</article>")
			if !strings.HasPrefix(got, "<article>") || endnotes < 0 || closing < endnotes {
				t.Fatalf("endnotes escaped document wrapper:\n%s", got)
			}
		})
	}
}

func TestDocumentRendererDefaultReplaySuppressionAndPrecedence(t *testing.T) {
	doc := djot.Parse("text[^a]\n\n[^a]: note\n")
	want := djot.RenderHTML(doc)
	replayed := djot.RenderHTML(doc, djot.WithDocumentRenderer(func(_ djot.DocumentView, r djot.DocumentRenderer) {
		r.Default()
		r.Default()
	}))
	if replayed != want+want {
		t.Fatalf("replayed document differs\nwant: %q\n got: %q", want+want, replayed)
	}

	suppressed := djot.RenderHTML(doc, djot.WithDocumentRenderer(func(djot.DocumentView, djot.DocumentRenderer) {}))
	if suppressed != "" {
		t.Fatalf("suppressed document output = %q", suppressed)
	}

	last := djot.RenderHTML(doc,
		djot.WithDocumentRenderer(func(djot.DocumentView, djot.DocumentRenderer) {
			t.Fatal("replaced document hook ran")
		}),
		djot.WithDocumentRenderer(func(_ djot.DocumentView, r djot.DocumentRenderer) { r.Write("last") }),
	)
	if last != "last" {
		t.Fatalf("last document hook output = %q", last)
	}
}

func TestDocumentRendererComposesElementAndSubtreeHooks(t *testing.T) {
	doc := djot.Parse("::: note\n:star:\n:::\n")
	got := djot.RenderHTML(doc,
		djot.WithDocumentRenderer(func(_ djot.DocumentView, r djot.DocumentRenderer) {
			r.Write("<main>")
			r.Default()
			r.Write("</main>")
		}),
		djot.WithSubtreeRenderer(djot.KindDiv, func(_ djot.SubtreeView, r djot.ElementRenderer) {
			r.Children()
		}),
		djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
			r.Write("[" + symbol.Name + "]")
		}),
	)
	if got != "<main><p>[star]</p>\n</main>" {
		t.Fatalf("composed document output = %q", got)
	}
}

func TestDocumentRendererConcurrentRendering(t *testing.T) {
	doc := djot.Parse("# Heading\n\ntext\n")
	option := djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
		r.Write(strconv.Itoa(len(document.Headings())))
		r.Default()
	})
	want := djot.RenderHTML(doc, option)
	const workers = 32
	var wait sync.WaitGroup
	errors := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if got := djot.RenderHTML(doc, option); got != want {
				errors <- got
			}
		}()
	}
	wait.Wait()
	close(errors)
	for got := range errors {
		t.Fatalf("concurrent output differs\nwant: %q\n got: %q", want, got)
	}
}

func TestDocumentRendererHTMLToParity(t *testing.T) {
	doc := djot.Parse("# Heading\n\ntext\n")
	option := djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
		r.Write("headings=" + strconv.Itoa(len(document.Headings())) + "\n")
		r.Default()
	})
	want := djot.RenderHTML(doc, option)
	var output strings.Builder
	if err := djot.RenderHTMLTo(&output, doc, option); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != want {
		t.Fatalf("RenderHTMLTo differs\nwant: %q\n got: %q", want, got)
	}
}

func TestDocumentRendererWriterErrorStopsDescendantHooks(t *testing.T) {
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			doc := djot.Parse(":star:\n")
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			writer := &errWriter{limit: 0}
			symbolCalls := 0
			err := djot.RenderHTMLTo(writer, doc,
				djot.WithDocumentRenderer(func(_ djot.DocumentView, r djot.DocumentRenderer) {
					r.Write("<main>")
					r.Default()
				}),
				djot.WithSymbolRenderer(func(djot.SymbolView, djot.ElementRenderer) {
					symbolCalls++
				}),
			)
			if err == nil {
				t.Fatal("expected writer error")
			}
			if symbolCalls != 0 {
				t.Fatalf("descendant callbacks after writer error = %d", symbolCalls)
			}
		})
	}
}

func TestDocumentRendererPanicPropagates(t *testing.T) {
	const marker = "document hook panic"
	for _, backend := range []string{"tape", "tree"} {
		t.Run(backend, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != marker {
					t.Fatalf("recovered %v, want %q", recovered, marker)
				}
			}()
			doc := djot.Parse("text\n")
			if backend == "tree" {
				doc = djot.NewDoc(doc.Root())
			}
			djot.RenderHTML(doc, djot.WithDocumentRenderer(func(djot.DocumentView, djot.DocumentRenderer) {
				panic(marker)
			}))
		})
	}
}

func FuzzDocumentViewMatchesTree(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"# heading\n\ntext\n",
		"# :symbol: `code`\n\n## second\n",
		"reference[^a]\n\n[^a]: note\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		type summary struct {
			levels []int
			texts  []string
		}
		render := func(doc *djot.Doc) (string, summary) {
			var result summary
			output := djot.RenderHTML(doc, djot.WithDocumentRenderer(func(document djot.DocumentView, r djot.DocumentRenderer) {
				for _, heading := range document.Headings() {
					result.levels = append(result.levels, heading.Level())
					result.texts = append(result.texts, heading.Text())
				}
				r.Default()
			}))
			return output, result
		}

		tapeOutput, tapeSummary := render(djot.Parse(input))
		treeParsed := djot.Parse(input)
		treeOutput, treeSummary := render(djot.NewDoc(treeParsed.Root()))
		if tapeOutput != treeOutput || !equalDocumentSummary(tapeSummary.levels, treeSummary.levels) ||
			!equalDocumentSummary(tapeSummary.texts, treeSummary.texts) {
			t.Fatalf("document view differs\ninput: %q\ntape output: %q\ntree output: %q\ntape: %#v\ntree: %#v", input, tapeOutput, treeOutput, tapeSummary, treeSummary)
		}
	})
}

func equalDocumentSummary[T comparable](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
