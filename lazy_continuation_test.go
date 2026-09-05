package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// A lazy line continues a list item or block quote only while its innermost
// block is an open paragraph; after a fence, table, heading, or an item made
// only of markers, and at any block start, the container ends. Expected HTML
// is djot.js 0.3.2's output for the same input.
func TestLazyContinuationNeedsOpenParagraph(t *testing.T) {
	cases := []struct{ in, want string }{
		{"- ```\nlazy\n", "<ul>\n<li>\n<pre><code></code></pre>\n</li>\n</ul>\n<p>lazy</p>\n"},
		{"1. ```\nlazy\n", "<ol>\n<li>\n<pre><code></code></pre>\n</li>\n</ol>\n<p>lazy</p>\n"},
		{"- [ ] ```\nlazy\n", "<ul class=\"task-list\">\n<li>\n<input disabled=\"\" type=\"checkbox\"/>\n<pre><code></code></pre>\n</li>\n</ul>\n<p>lazy</p>\n"},
		{"- |t|\nlazy\n", "<ul>\n<li>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n</li>\n</ul>\n<p>lazy</p>\n"},
		{"- # h\nlazy\n", "<ul>\n<li>\n<h1 id=\"h-lazy\">h\nlazy</h1>\n</li>\n</ul>\n"},
		{"- - -\nlazy\n", "<hr>\n<p>lazy</p>\n"},
		{"> ```\nlazy\n", "<blockquote>\n<pre><code></code></pre>\n</blockquote>\n<p>lazy</p>\n"},
		{"> * *\n*\n", "<blockquote>\n<ul>\n<li>\n<ul>\n<li>\n</li>\n</ul>\n</li>\n</ul>\n</blockquote>\n<ul>\n<li>\n</li>\n</ul>\n"},
		{"> |t|\nlazy\n", "<blockquote>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n</blockquote>\n<p>lazy</p>\n"},
		{"> a\nlazy\n", "<blockquote>\n<p>a\nlazy</p>\n</blockquote>\n"},
		{"> - a\nlazy\n", "<blockquote>\n<ul>\n<li>\na\nlazy\n</li>\n</ul>\n</blockquote>\n"},
		{"> a\n- b\n", "<blockquote>\n<p>a</p>\n</blockquote>\n<ul>\n<li>\nb\n</li>\n</ul>\n"},
		{"- a\n> q\n", "<ul>\n<li>\na\n</li>\n</ul>\n<blockquote>\n<p>q</p>\n</blockquote>\n"},
		{"- a\n|t|\n", "<ul>\n<li>\na\n</li>\n</ul>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n"},
		{"* :\n:", "<ul>\n<li>\n<dl>\n<dt></dt>\n<dd>\n</dd>\n</dl>\n</li>\n</ul>\n<dl>\n<dt></dt>\n<dd>\n</dd>\n</dl>\n"},
	}
	for _, tc := range cases {
		if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
}

// Only a blank line ends a paragraph: a line that merely looks like a table
// row, fence, break, div fence, or attribute list inside an open paragraph
// is paragraph text, and the paragraph stays open for lazy continuation.
// After a blank the same lines do open blocks. Expected HTML is djot.js's.
func TestLazyContinuationKeepsOpenParagraph(t *testing.T) {
	cases := []struct{ in, want string }{
		{"> a\n> |t|\nlazy\n", "<blockquote>\n<p>a\n|t|\nlazy</p>\n</blockquote>\n"},
		{"> a\n> ```\nlazy\n", "<blockquote>\n<p>a\n<code>\nlazy</code></p>\n</blockquote>\n"},
		{"> a\n> ***\nlazy\n", "<blockquote>\n<p>a\n***\nlazy</p>\n</blockquote>\n"},
		{"> a\n> :::\nlazy\n", "<blockquote>\n<p>a\n:::\nlazy</p>\n</blockquote>\n"},
		{"> a\n> {.c}\nlazy\n", "<blockquote>\n<p>a\n\nlazy</p>\n</blockquote>\n"},
		{"- a\n  |t|\nlazy\n", "<ul>\n<li>\na\n|t|\nlazy\n</li>\n</ul>\n"},
		{"- a\n  ```\nlazy\n", "<ul>\n<li>\na\n<code>\nlazy</code>\n</li>\n</ul>\n"},
		{"- a\n  ***\nlazy\n", "<ul>\n<li>\na\n***\nlazy\n</li>\n</ul>\n"},
		{"x[^a]\n\n[^a]: a\n  |t|\nlazy\n", "<p>x<a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p>a\n|t|\nlazy<a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n"},
		{"x[^a]\n\n[^a]: a\n  ```\nlazy\n", "<p>x<a id=\"fnref1\" href=\"#fn1\" role=\"doc-noteref\"><sup>1</sup></a></p>\n<section role=\"doc-endnotes\">\n<hr>\n<ol>\n<li id=\"fn1\">\n<p>a\n<code>\nlazy</code><a href=\"#fnref1\" role=\"doc-backlink\">↩︎</a></p>\n</li>\n</ol>\n</section>\n"},
		{"> a\n>\n> |t|\nlazy\n", "<blockquote>\n<p>a</p>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n</blockquote>\n<p>lazy</p>\n"},
		{"> a\n>\n> ```\nlazy\n", "<blockquote>\n<p>a</p>\n<pre><code></code></pre>\n</blockquote>\n<p>lazy</p>\n"},
		{"- a\n\n  |t|\nlazy\n", "<ul>\n<li>\n<p>a</p>\n<table>\n<tr>\n<td>t</td>\n</tr>\n</table>\n</li>\n</ul>\n<p>lazy</p>\n"},
	}
	for _, tc := range cases {
		if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", tc.in, got, tc.want)
		}
	}
}
