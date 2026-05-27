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

// TestSetAttrRejectsInvalidKeys verifies that Node.SetAttr returns false and
// leaves the node unmodified when given a key that would produce malformed
// HTML. SetAttr accepts arbitrary strings at the call site, but keys
// containing whitespace, quotes, '>', '/', '=', or control bytes are not
// valid HTML attribute names.
func TestSetAttrRejectsInvalidKeys(t *testing.T) {
	invalid := []struct {
		name string
		key  string
	}{
		{"contains space", "evil key"},
		{"contains double quote", `evil"key`},
		{"contains single quote", "evil'key"},
		{"contains gt", "evil>key"},
		{"contains slash", "evil/key"},
		{"contains equals", "evil=key"},
		{"contains tab", "evil\tkey"},
		{"contains newline", "evil\nkey"},
		{"contains null byte", "evil\x00key"},
		{"contains control byte", "evil\x01key"},
		{"injection via quote break", `x"><script>alert(1)</script>`},
		{"injection via space and equals", `onclick=alert(1) data-x`},
		{"empty", ""},
		{"starts with digit", "1foo"},
		{"starts with hyphen", "-foo"},
	}

	baseline := djot.RenderHTML(djot.Parse("# x"))

	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			d := djot.Parse("# x")
			var sect *djot.Node
			for _, child := range d.Root.Children {
				if child.Kind == djot.Section {
					sect = child
				}
			}
			if sect == nil {
				t.Fatal("expected Section child")
			}

			if ok := sect.SetAttr(tc.key, "v"); ok {
				t.Errorf("SetAttr(%q, _) returned true, want false", tc.key)
			}
			if got := sect.Attr(tc.key); got != "" {
				t.Errorf("after rejected SetAttr, Attr(%q) = %q, want empty", tc.key, got)
			}
			if got := djot.RenderHTML(d); got != baseline {
				t.Errorf("rejected SetAttr leaked into output:\n got:      %q\n baseline: %q", got, baseline)
			}
		})
	}
}

func TestSetAttrAcceptsValidKeys(t *testing.T) {
	valid := []string{
		"data-track",
		"aria-label",
		"xml:lang",
		"id",
		"class",
		"role",
		"_internal",
		":scoped",
		"a",
		"a1",
		"abc-123_xyz",
	}

	for _, k := range valid {
		t.Run(k, func(t *testing.T) {
			d := djot.Parse("# x")
			var sect *djot.Node
			for _, child := range d.Root.Children {
				if child.Kind == djot.Section {
					sect = child
				}
			}
			if sect == nil {
				t.Fatal("expected Section child")
			}
			if ok := sect.SetAttr(k, "v"); !ok {
				t.Errorf("SetAttr(%q, _) returned false, want true", k)
			}
			if got := sect.Attr(k); got != "v" {
				t.Errorf("Attr(%q) = %q, want %q", k, got, "v")
			}
			got := djot.RenderHTML(d)
			if !strings.Contains(got, k+`="v"`) {
				t.Errorf("expected %s=\"v\" in output, got:\n%s", k, got)
			}
		})
	}
}

func TestSetAttrOverwriteThenRejectKeepsOriginal(t *testing.T) {
	// Calling SetAttr with an invalid key after a valid SetAttr on a different
	// key must not mutate any state.
	d := djot.Parse("# x")
	var sect *djot.Node
	for _, child := range d.Root.Children {
		if child.Kind == djot.Section {
			sect = child
		}
	}
	if sect == nil {
		t.Fatal("expected Section child")
	}
	sect.SetAttr("data-good", "v1")
	if ok := sect.SetAttr("evil key", "v2"); ok {
		t.Fatal("expected SetAttr to reject invalid key")
	}
	if got := sect.Attr("data-good"); got != "v1" {
		t.Errorf("data-good was disturbed: got %q, want %q", got, "v1")
	}
}

func TestAddClassWithSpecialValueStaysWellFormed(t *testing.T) {
	d := djot.Parse("# x")
	for _, child := range d.Root.Children {
		if child.Kind == djot.Section {
			child.AddClass(`"><script>alert(1)</script>`)
		}
	}
	got := djot.RenderHTML(d)
	if strings.Contains(got, "<script>") {
		t.Errorf("class value should be escaped, got raw <script>:\n%s", got)
	}
}

func TestParsedAttrsStillRender(t *testing.T) {
	d := djot.Parse("{data-foo=\"v\" aria-label=\"x\"}\n# heading\n")
	got := djot.RenderHTML(d)
	for _, want := range []string{`data-foo="v"`, `aria-label="x"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}
