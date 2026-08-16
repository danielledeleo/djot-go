package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestOrderedListStyleContinuation covers enumerators that read as more than one
// style. Alone, "c" is roman 100 and "i" is roman 1, and that is how a list
// opening on them is read — but a list already running as lower alpha carries on
// through those letters rather than breaking into a second list.
//
// Expectations are the output of the reference implementation (jgm/djot.js) at
// f0191eb, captured by running its renderer over the same input.
func TestOrderedListStyleContinuation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{{
		name:  "alpha list runs through c",
		input: "a. one\nb. two\nc. three",
		want: `<ol type="a">
<li>
one
</li>
<li>
two
</li>
<li>
three
</li>
</ol>
`,
	}, {
		name:  "alpha list runs through d",
		input: "a. one\nb. two\nd. four",
		want: `<ol type="a">
<li>
one
</li>
<li>
two
</li>
<li>
four
</li>
</ol>
`,
	}, {
		name:  "alpha list runs through i",
		input: "a. one\ni. nested",
		want: `<ol type="a">
<li>
one
</li>
<li>
nested
</li>
</ol>
`,
	}, {
		name:  "alpha list starting at b runs through c",
		input: "b. one\nc. two",
		want: `<ol start="2" type="a">
<li>
one
</li>
<li>
two
</li>
</ol>
`,
	}, {
		// Opening on an ambiguous letter still reads as roman, and stays roman.
		name:  "list opening on c is roman",
		input: "c. one\nd. two",
		want: `<ol start="100" type="i">
<li>
one
</li>
<li>
two
</li>
</ol>
`,
	}, {
		// A second item that is unambiguously alpha settles the first one.
		name:  "second item settles an ambiguous first",
		input: "i. one\nb. two",
		want: `<ol start="9" type="a">
<li>
one
</li>
<li>
two
</li>
</ol>
`,
	}, {
		name:  "roman list stays roman",
		input: "i. one\nii. two\niii. three",
		want: `<ol type="i">
<li>
one
</li>
<li>
two
</li>
<li>
three
</li>
</ol>
`,
	}, {
		// The deciding marker can arrive well after the second item: while every
		// enumerator so far is ambiguous the style stays open, and settling it
		// rereads the whole list rather than splitting it in two.
		name:  "a late marker settles an ambiguous run",
		input: "i. one\ni. two\nb. three",
		want: `<ol start="9" type="a">
<li>
one
</li>
<li>
two
</li>
<li>
three
</li>
</ol>
`,
	}, {
		name:  "same, upper case",
		input: "I. one\nI. two\nA. three",
		want: `<ol start="9" type="A">
<li>
one
</li>
<li>
two
</li>
<li>
three
</li>
</ol>
`,
	}, {
		name:  "upper alpha runs through C",
		input: "A. one\nB. two\nC. three",
		want: `<ol type="A">
<li>
one
</li>
<li>
two
</li>
<li>
three
</li>
</ol>
`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := djot.RenderHTML(djot.Parse(tt.input)); got != tt.want {
				t.Errorf("input:\n%s\n\nwant:\n%s\ngot:\n%s", tt.input, tt.want, got)
			}
		})
	}
}

// TestOrderedListStartZero pins a deliberate difference from djot.js. The spec
// says the start number "will be determined by the number of its first item",
// so a list opening on "0." starts at 0. djot.js drops the attribute here —
// src/html.ts guards with `node.start && node.start !== 1`, and 0 is falsy in
// JavaScript — which renders the list as starting at 1 instead.
func TestOrderedListStartZero(t *testing.T) {
	const want = `<ol start="0">
<li>
one
</li>
<li>
two
</li>
</ol>
`
	if got := djot.RenderHTML(djot.Parse("0. one\n1. two")); got != want {
		t.Errorf("want:\n%s\ngot:\n%s", want, got)
	}
}
