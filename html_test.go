package djot_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// errWriter is an io.Writer that fails after n bytes.
type errWriter struct {
	limit   int
	written int
}

var errWriteLimited = errors.New("write limit reached")

func (w *errWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		remaining := w.limit - w.written
		if remaining > 0 {
			w.written += remaining
			return remaining, errWriteLimited
		}
		return 0, errWriteLimited
	}
	w.written += len(p)
	return len(p), nil
}

func TestRenderHTMLToSuccess(t *testing.T) {
	doc := djot.Parse("Hello *world*")
	var buf strings.Builder
	err := djot.RenderHTMLTo(&buf, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	html := buf.String()
	if html != djot.RenderHTML(doc) {
		t.Errorf("RenderHTMLTo and RenderHTML produced different output:\n  To:     %q\n  HTML:   %q", html, djot.RenderHTML(doc))
	}
}

func TestRenderHTMLToErrorPropagation(t *testing.T) {
	doc := djot.Parse("Hello *world* this is a paragraph with enough text to trigger the limit")
	w := &errWriter{limit: 5}
	err := djot.RenderHTMLTo(w, doc)
	if err == nil {
		t.Fatal("expected error from failing writer, got nil")
	}
	if !errors.Is(err, errWriteLimited) {
		t.Fatalf("expected errWriteLimited, got: %v", err)
	}
}

func TestRenderHTMLToZeroLimit(t *testing.T) {
	doc := djot.Parse("Hello")
	w := &errWriter{limit: 0}
	err := djot.RenderHTMLTo(w, doc)
	if err == nil {
		t.Fatal("expected error from zero-limit writer, got nil")
	}
}

func TestLinkBalancedParentheses(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"nested parens",
			"[link](https://example.com/wiki/Foo_(bar))",
			`<p><a href="https://example.com/wiki/Foo_(bar)">link</a></p>` + "\n",
		},
		{
			"double nested",
			"[link](url_(a_(b)))",
			`<p><a href="url_(a_(b))">link</a></p>` + "\n",
		},
		{
			"no parens",
			"[link](https://example.com)",
			`<p><a href="https://example.com">link</a></p>` + "\n",
		},
		{
			"empty target",
			"[link]()",
			`<p><a href="">link</a></p>` + "\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.input)
			got := djot.RenderHTML(doc)
			if got != tc.want {
				t.Errorf("got:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestReferenceDefinitionScoping(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"ref inside blockquote used outside",
			"> [foo]: https://example.com\n\n[click][foo]\n",
			"<blockquote>\n</blockquote>\n<p><a href=\"https://example.com\">click</a></p>\n",
		},
		{
			"ref inside list item used outside",
			"- [bar]: https://example.com\n\n[click][bar]\n",
			"<ul>\n<li>\n</li>\n</ul>\n<p><a href=\"https://example.com\">click</a></p>\n",
		},
		{
			"ref at top level used inside blockquote",
			"[baz]: https://example.com\n\n> [click][baz]\n",
			"<blockquote>\n<p><a href=\"https://example.com\">click</a></p>\n</blockquote>\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.input)
			got := djot.RenderHTML(doc)
			if got != tc.want {
				t.Errorf("got:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestRenderHTMLToDiscard(t *testing.T) {
	// Verify no panic when writing to io.Discard.
	doc := djot.Parse("# Heading\n\nParagraph with *emphasis* and a [link](url).\n")
	err := djot.RenderHTMLTo(io.Discard, doc)
	if err != nil {
		t.Fatalf("unexpected error writing to Discard: %v", err)
	}
}
