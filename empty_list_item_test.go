package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// A list marker alone on its line opens an item with no content. The line that
// follows is a new block, not a lazy continuation of nothing, and a bare
// enumerator still opens a list. Expected output is djot.js's, captured from
// the reference implementation.
func TestEmptyListItemEndsTheList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"bullet dash",
			"-\ntext\n",
			"<ul>\n<li>\n</li>\n</ul>\n<p>text</p>\n",
		},
		{
			"bullet star",
			"*\ntext\n",
			"<ul>\n<li>\n</li>\n</ul>\n<p>text</p>\n",
		},
		{
			"marker with trailing space mid-list",
			"- a\n- \nb\n",
			"<ul>\n<li>\na\n</li>\n<li>\n</li>\n</ul>\n<p>b</p>\n",
		},
		{
			"ordered dot",
			"1.\ntext\n",
			"<ol>\n<li>\n</li>\n</ol>\n<p>text</p>\n",
		},
		{
			"ordered paren",
			"1)\ntext\n",
			"<ol>\n<li>\n</li>\n</ol>\n<p>text</p>\n",
		},
		{
			"ordered roman keeps its style",
			"i.\ntext\n",
			"<ol type=\"i\">\n<li>\n</li>\n</ol>\n<p>text</p>\n",
		},
		{
			"ordered wrapped in parens",
			"(1)\ntext\n",
			"<ol>\n<li>\n</li>\n</ol>\n<p>text</p>\n",
		},
		{
			"ordered wrapped roman keeps its style",
			"(i)\ntext\n",
			"<ol type=\"i\">\n<li>\n</li>\n</ol>\n<p>text</p>\n",
		},
		{
			"task list",
			"- [ ]\ntext\n",
			"<ul class=\"task-list\">\n<li>\n<input disabled=\"\" type=\"checkbox\"/>\n</li>\n</ul>\n<p>text</p>\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
				t.Errorf("input %q:\nwant: %q\n got: %q", tc.in, tc.want, got)
			}
		})
	}
}

// Once an item has content, an unindented line is a lazy continuation again.
// These already matched djot.js and must keep doing so.
func TestListLazyContinuationStillApplies(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"content on the marker line",
			"- item\ntext\n",
			"<ul>\n<li>\nitem\ntext\n</li>\n</ul>\n",
		},
		{
			"content arrives on an indented line",
			"-\n  text\nmore\n",
			"<ul>\n<li>\ntext\nmore\n</li>\n</ul>\n",
		},
		{
			"empty item followed by a real item",
			"-\n- second\n",
			"<ul>\n<li>\n</li>\n<li>\nsecond\n</li>\n</ul>\n",
		},
		{
			"blank line already separated them",
			"-\n\ntext\n",
			"<ul>\n<li>\n</li>\n</ul>\n<p>text</p>\n",
		},
		{
			"task list with content",
			"- [ ] a\n",
			"<ul class=\"task-list\">\n<li>\n<input disabled=\"\" type=\"checkbox\"/>\na\n</li>\n</ul>\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := djot.RenderHTML(djot.Parse(tc.in)); got != tc.want {
				t.Errorf("input %q:\nwant: %q\n got: %q", tc.in, tc.want, got)
			}
		})
	}
}
