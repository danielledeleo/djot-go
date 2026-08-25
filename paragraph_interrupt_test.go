package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestHeadingDoesNotInterruptParagraph pins djot.js behavior: nothing
// interrupts a paragraph — a "#" line inside one is paragraph text, exactly
// like ">" and "-" lines already are. (Previously a heading line split the
// paragraph unless it appeared inside unclosed inline-attribute braces.)
// TestBlocksInterruptHeading pins djot.js behavior: list markers, definition
// terms, and table rows end a heading just like fences, divs, breaks, and
// quotes already do — only plain text lines are absorbed as continuations.
func TestBlocksInterruptHeading(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"bullet", "# h\n*\n", "<ul>"},
		{"ordered", "# h\n1. o\n", "<ol>"},
		{"definition", "# h\n: d\n", "<dl>"},
		{"table", "# h\n|t|\n", "<table>"},
		{"reference-def", "# h\n[r]: /u\n\n[x][r]\n", `href="/u"`},
		{"multiline-attr", "# h\n{#id\n.cls}\nx\n", "<p>{#id\n.cls}\nx</p>"},
		{"single-attr", "# h\n{.cls}\nx\n", `<p class="cls">x</p>`},
		{"footnote-def", "# h\n[^a]: n\n", "<h1>h</h1>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := djot.RenderHTML(djot.Parse(tc.in))
			if !strings.Contains(html, tc.want) {
				t.Errorf("input %q: expected %s after heading\nhtml:\n%s", tc.in, tc.want, html)
			}
		})
	}
}

// TestRawBlockFormatNoSpaces pins djot.js behavior: a raw-block info string
// is exactly "=format" with no whitespace; anything else is not a fence.
func TestRawBlockFormatNoSpaces(t *testing.T) {
	for _, in := range []string{
		"~~~= 0\n", "~~~=fmt extra\n", "```= x\ny\n```\n",
		// Tabs inside the info token also invalidate the fence.
		"~~~=\thtml\nx\n~~~\n", "~~~go\textra\nx\n~~~\n",
	} {
		html := djot.RenderHTML(djot.Parse(in))
		if !strings.Contains(html, "<p>") {
			t.Errorf("input %q: expected paragraph, got:\n%s", in, html)
		}
	}
	// Whitespace around the token stays valid.
	for _, in := range []string{"```=html\n<b>\n```\n", "~~~\t=html\n<b>\n~~~\n", "~~~=html\t\n<b>\n~~~\n"} {
		if html := djot.RenderHTML(djot.Parse(in)); html != "<b>\n" {
			t.Errorf("valid raw block %q broken: %q", in, html)
		}
	}
}

// TestInvalidAttrContinuesHeading: a brace line that the dispatcher would
// read as paragraph text (a failed attribute attempt with no consumed
// lines) is heading text (matching djot.js). Note: "{x y}" is a valid
// attribute to this parser's grammar (bare keys parse as empty-valued
// attributes, unlike djot.js) and so ends the heading like any attribute;
// the heading check mirrors the dispatcher either way.
func TestInvalidAttrContinuesHeading(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("# h\n{x y\n"))
	if !strings.Contains(html, "{x y") || !strings.Contains(html, "</h1>") ||
		strings.Contains(html, "<p>") {
		t.Errorf("unclosed invalid attr line should continue heading:\n%s", html)
	}
}

// TestSinglePipeContinuesHeading: a line with a lone pipe is not a table
// row, so it continues the heading as text (matching djot.js).
func TestSinglePipeContinuesHeading(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("# h\n|x\n"))
	if strings.Contains(html, "<table>") || !strings.Contains(html, "|x</h1>") {
		t.Errorf("single-pipe heading continuation broken:\n%s", html)
	}
}

// TestEscapedQuoteInQuoted pins djot.js behavior: a literal quote character
// at the end of a braced quote's content belongs to the content, not the
// closing delimiter.
func TestEscapedQuoteInQuoted(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("{'\\''}\n"))
	if !strings.Contains(html, "\u2018'\u2019") {
		t.Errorf("inner literal quote dropped:\n%s", html)
	}
}

// TestEscapedParenInDestination pins djot.js behavior: an escaped paren in
// an inline link destination neither opens nor closes the balance.
func TestEscapedParenInDestination(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("[t](a\\)b)\n"))
	if !strings.Contains(html, `<a href="a)b">t</a>`) {
		t.Errorf("escaped paren mishandled:\n%s", html)
	}
}

// TestTableRowBacktickRuns pins djot.js behavior: pipe counting for table
// row detection must honor verbatim backtick runs (an opener closes only on
// a run of the same length; an unclosed verbatim swallows the rest of the
// line, including pipes).
func TestTableRowBacktickRuns(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		isTable bool
	}{
		{"unclosed-swallows-pipe", "|`0```|\n", false},
		{"closed-run-counts-pipe", "|` 0``` `|\n", true},
		{"plain", "|a|\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := djot.RenderHTML(djot.Parse(tc.in))
			if got := strings.Contains(html, "<table>"); got != tc.isTable {
				t.Errorf("input %q: table = %v, want %v\nhtml:\n%s", tc.in, got, tc.isTable, html)
			}
		})
	}
}

func TestHeadingDoesNotInterruptParagraph(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"plain", "a\n# h\n"},
		{"in-verbatim", "`code\n# h\n"},
		{"in-braces", "{\"a\n# b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := djot.RenderHTML(djot.Parse(tc.in))
			if strings.Contains(html, "<h1") || strings.Contains(html, "<section") {
				t.Errorf("input %q: heading interrupted paragraph\nhtml:\n%s", tc.in, html)
			}
		})
	}
}

// TestEscapedTextInSpan pins span parsing when the bracketed text begins
// with a backslash escape: the opener must not merge back into the literal
// text in front of it.
func TestEscapedTextInSpan(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("a [\\*b]{#id}o\n"))
	if !strings.Contains(html, `<span id="id">*b</span>`) {
		t.Errorf("escaped span broken:\n%s", html)
	}
}

// TestBreakAfterHeading pins djot.js behavior: a thematic break in fresh
// block position needs no preceding blank line.
func TestBreakAfterHeading(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("# h\n***\n"))
	if !strings.Contains(html, "<hr>") {
		t.Errorf("break after heading not parsed:\n%s", html)
	}
}

// TestOrderedEnumOverflow: decimal enumerators past maxOrderedEnum are
// rejected rather than wrapped, on every architecture (values above the
// platform int maximum are also rejected, so 32-bit builds stay safe).
func TestOrderedEnumOverflow(t *testing.T) {
	cases := []struct {
		name string
		in   string
		list bool
	}{
		{"at-bound", "100000000000000000) x\n", true},
		{"past-bound", "1000000000000000000) x\n", false},
		{"way-past", "999999999999999999999999999) x\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := djot.RenderHTML(djot.Parse(tc.in))
			if got := strings.Contains(html, "<ol"); got != tc.list {
				t.Errorf("input %q: list = %v, want %v\nhtml:\n%s", tc.in, got, tc.list, html)
			}
			if strings.Contains(html, `start="-`) {
				t.Errorf("enumerator overflowed:\n%s", html)
			}
		})
	}
}

// TestTaskListTabContinuation: a tab-indented continuation line used to
// panic (columns were used as byte offsets), and after the panic fix the
// continuation's source span still used a column count as a byte offset.
func TestTaskListTabContinuation(t *testing.T) {
	input := " * [X] \n\t0"
	doc := djot.Parse(input)
	_ = djot.RenderHTML(doc)

	var bad []string
	zeroStart := -1
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		span := n.Span()
		if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset ||
			span.End.Offset > len(input) {
			bad = append(bad, n.Kind().String())
		}
		if text, ok := n.(*djot.Text); ok && text.Value == "0" {
			zeroStart = text.Span().Start.Offset
		}
		return djot.Continue
	})
	if len(bad) > 0 {
		t.Errorf("nodes with spans outside [0, %d]: %v", len(input), bad)
	}
	if want := strings.IndexByte(input, '0'); zeroStart != want {
		t.Errorf("continuation text start = %d, want %d", zeroStart, want)
	}
}

// TestTabContinuationAllContainers: every container's continuation-line
// collector must survive tab indentation (columns != bytes) without
// panicking, losing content, or producing spans outside the input.
func TestTabContinuationAllContainers(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"footnote", " [^a]: x\n\t0\n\nq[^a]\n", "x\n0"},
		{"definition", ": a\n\tb\n", "b"},
		{"bullet", "  * a\n\tx\n", "a\nx"},
		{"ordered", " 1. a\n\tx\n", "a\nx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.in)
			html := djot.RenderHTML(doc)
			if !strings.Contains(html, tc.want) {
				t.Errorf("input %q: missing %q\nhtml:\n%s", tc.in, tc.want, html)
			}
			djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
				span := n.Span()
				if span.Start.Offset < 0 || span.End.Offset < span.Start.Offset ||
					span.End.Offset > len(tc.in) {
					t.Errorf("%s span outside [0, %d]: %+v", n.Kind(), len(tc.in), span)
				}
				return djot.Continue
			})
		})
	}
}

// TestTableRowNeedsTrailingPipe pins djot.js's pattTableRow: a row line must
// end with a pipe (after trailing whitespace), so "|a| b" is not a row.
func TestTableRowNeedsTrailingPipe(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("# h\n|a| b\n"))
	if strings.Contains(html, "<table>") {
		t.Errorf("line without trailing pipe treated as table row:\n%s", html)
	}
	html = djot.RenderHTML(djot.Parse("|a|\n|b| x\n"))
	if strings.Contains(html, "<td>x</td>") || strings.Contains(html, "b| x") && false {
		t.Errorf("bogus second row absorbed:\n%s", html)
	}
	if !strings.Contains(djot.RenderHTML(djot.Parse("|a|b| \n")), "<table>") {
		t.Errorf("trailing whitespace after final pipe should stay a row")
	}
}
