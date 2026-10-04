package djot_test

import (
	"strings"
	"testing"
)

func TestDeferredHeadingReferenceLabels(t *testing.T) {
	for _, link := range []string{"[Wrong][Target]", "![Wrong][Target]"} {
		attr := `href="#Target"`
		if strings.HasPrefix(link, "!") {
			attr = `src="#Target"`
		}
		checkBothHTML(t, "# Wrong\n\n# Target\n\n"+link, attr)
	}
	checkBothHTML(t, "[Read more][Target]\n\n# Target", `href="#Target"`)
	checkBothHTML(t, "# Target\n\n[Target][Missing]", "<a>Target</a>")
}

func TestNestedHeadingIDs(t *testing.T) {
	for _, input := range []string{
		"> - # Nested\n",
		"- - # Nested\n",
		": term\n\n  # Nested\n",
		"text[^n]\n\n[^n]: # Nested\n",
	} {
		checkBothHTML(t, input, `<h1 id="Nested">Nested</h1>`)
	}
}

func TestContainerHeadingReferences(t *testing.T) {
	for _, heading := range []string{"> # Heading", "> - # Heading", ": term\n\n  # Heading"} {
		checkBothHTML(t, "[Read more][Heading]\n\n"+heading, `href="#Heading"`)
		checkBothHTML(t, heading+"\n\n[Heading][]\n\n[Heading]: /override", `href="/override"`)
	}
}

// Generated identifiers follow djot.js: they are assigned in document order,
// and an explicit id counts as taken only from where it appears. An explicit
// id after a heading can therefore repeat that heading's generated id.
func TestGeneratedIDsFollowDocumentOrder(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"# Same\n\n{#Same}\n# Other", `<section id="Same">
<h1>Same</h1>
</section>
<section id="Same">
<h1>Other</h1>`},
		{"> # Same\n\n{#Same}\n# Other", `<h1 id="Same">Same</h1>
</blockquote>
<section id="Same">`},
		{"# !!!\n\n{#s-1}\n# Other", `<section id="s-1">
<h1>!!!</h1>
</section>
<section id="s-1">`},
		{"{#Same}\n# First\n\n# Same\n\n# Same", `<section id="Same-1">
<h1>Same</h1>
</section>
<section id="Same-2">`},
		{"# Foo\n\n> # Foo\n\n# Foo", `<section id="Foo">
<h1>Foo</h1>
<blockquote>
<h1 id="Foo-1">Foo</h1>
</blockquote>
</section>
<section id="Foo-2">`},
		{"{#foo}\na\n\n{#foo}\nb\n\n# foo", `<section id="foo-1">`},
		{"#\n\n#\n\n# s", `<section id="s-1">
<h1></h1>
</section>
<section id="s-2">
<h1></h1>
</section>
<section id="s">`},
	} {
		checkBothHTML(t, tt.input, tt.want)
	}
}

// The text rule for generated identifiers, checked against djot.js.
func TestGeneratedIDText(t *testing.T) {
	for heading, id := range map[string]string{
		"foo: bar":                      "foo:-bar",
		"Section 1.2":                   "Section-1-2",
		"a(b)c":                         "a-b-c",
		"50% off & more":                "50-off-more",
		"a---b":                         "a---b",
		"a--b":                          "a--b",
		"a...b":                         "a-b",
		"x--y---z----w":                 "x--y---z----w",
		"---":                           "---",
		"Don't panic":                   "Don't-panic",
		"It's 'nested \"mixed\"' stuff": "It's-nested-mixed-stuff",
		"\"Quoted\" text":               "Quoted-text",
		"/{'":                           "'",
		"x :sym: y":                     "x-y",
		"a\\ b":                         "ab",
		"a\u00a0b":                      "a-b",
		"semi;colon":                    "semi;colon",
		"C++ & C#":                      "C-C",
		"<https://auto.link>":           "https:-auto-link",
		"`code` and $`math`":            "code-and-math",
		"note[^1]":                      "note",
		"Ünïcödé Ωmega 日本":              "Ünïcödé-Ωmega-日本",
		"!!!":                           "s-1",
	} {
		checkBothHTML(t, "# "+heading+"\n", `<section id="`+strings.ReplaceAll(id, `"`, "&quot;")+`">`)
	}
}

// Heading references use the same text as identifiers, so a link label written
// with a straight quote finds a heading whose text has a smart one.
func TestHeadingReferenceWithApostrophe(t *testing.T) {
	checkBothHTML(t, "See [Don't panic][].\n\n# Don't panic\n", `<a href="#Don't-panic">`)
}

func TestReferenceDefinitionLabelNormalization(t *testing.T) {
	for _, label := range []string{"two  words", " two words ", "two\twords"} {
		checkBothHTML(t, "[x][two words]\n\n["+label+"]: /url", `href="/url"`)
		checkBothHTML(t, "![x][two words]\n\n["+label+"]: /url", `src="/url"`)
	}
	checkBothHTML(t, "[two  words][]\n\n[two words]: /url", `href="/url"`)
	checkBothHTML(t, "# two  words\n\n[more][two words]", `href="#two-words"`)
	checkBothHTML(t, "[x][two words]\n\n[two words]: /first\n[two  words]: /last", `href="/last"`)
}
