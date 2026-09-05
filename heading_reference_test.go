package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
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

func TestGeneratedIDsReserveExplicitHeadings(t *testing.T) {
	for _, input := range []string{
		"# Same\n\n{#Same}\n# Other",
		"> # Same\n\n{#Same}\n# Other",
		"# Same\n\n> {#Same}\n> # Other",
	} {
		checkBothHTML(t, input, `id="Same-1"`)
		if got := djot.RenderHTML(djot.Parse(input)); strings.Count(got, `id="Same"`) != 1 {
			t.Errorf("explicit ID duplicated:\n%s", got)
		}
	}
	checkBothHTML(t, "# !!!\n\n{#s-1}\n# Other", `id="s-2"`)
	checkBothHTML(t, "{#Same}\n# First\n\n# Same\n\n# Same", `id="Same-2"`)
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
