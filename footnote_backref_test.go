package djot_test

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

const multiRefInput = "First[^a]. Second[^a]. Third[^a].\n\n[^a]: The note.\n"

// By default the renderer matches djot.js: only the first reference to a
// footnote carries the id (avoiding duplicate HTML ids), and the footnote has a
// single backlink to that first reference.
func TestFootnoteBacklinksDefault(t *testing.T) {
	got := djot.RenderHTML(djot.Parse(multiRefInput))

	wantRefs := `First<a id="fnref1" href="#fn1" role="doc-noteref"><sup>1</sup></a>. ` +
		`Second<a href="#fn1" role="doc-noteref"><sup>1</sup></a>. ` +
		`Third<a href="#fn1" role="doc-noteref"><sup>1</sup></a>.`
	if !strings.Contains(got, wantRefs) {
		t.Errorf("default references wrong.\n got: %s\nwant substring: %s", got, wantRefs)
	}
	wantBack := `<p>The note.<a href="#fnref1" role="doc-backlink">↩︎</a></p>`
	if !strings.Contains(got, wantBack) {
		t.Errorf("default backlink wrong.\n got: %s\nwant substring: %s", got, wantBack)
	}
}

// With WithMultiBacklinks, every reference gets a unique id (fnref1, fnref1-2,
// fnref1-3) and the footnote links back to each with lettered backlinks (a, b, c).
func TestFootnoteBacklinksMulti(t *testing.T) {
	got := djot.RenderHTML(djot.Parse(multiRefInput), djot.WithMultiBacklinks())

	wantRefs := `First<a id="fnref1" href="#fn1" role="doc-noteref"><sup>1</sup></a>. ` +
		`Second<a id="fnref1-2" href="#fn1" role="doc-noteref"><sup>1</sup></a>. ` +
		`Third<a id="fnref1-3" href="#fn1" role="doc-noteref"><sup>1</sup></a>.`
	if !strings.Contains(got, wantRefs) {
		t.Errorf("multi references wrong.\n got: %s\nwant substring: %s", got, wantRefs)
	}
	wantBack := `<p>The note.` +
		`<a href="#fnref1" role="doc-backlink">a</a> ` +
		`<a href="#fnref1-2" role="doc-backlink">b</a> ` +
		`<a href="#fnref1-3" role="doc-backlink">c</a></p>`
	if !strings.Contains(got, wantBack) {
		t.Errorf("multi backlinks wrong.\n got: %s\nwant substring: %s", got, wantBack)
	}
}

var (
	noterefIDRe = regexp.MustCompile(`<a id="([^"]+)" href="#[^"]+" role="doc-noteref">`)
	backlinkRe  = regexp.MustCompile(`<a href="#([^"]+)" role="doc-backlink">`)
)

// assertAnchorIntegrity checks the cross-reference contract on rendered HTML,
// independent of the id scheme: no reference id is emitted twice, and every
// backlink targets an id that actually exists. When bijection is true (multi
// mode) it also requires a one-to-one match between reference ids and backlinks.
func assertAnchorIntegrity(t *testing.T, html string, bijection bool) {
	t.Helper()
	ids := matches(noterefIDRe, html)
	backs := matches(backlinkRe, html)

	// Guard against a vacuous pass if the anchor format ever drifts out from
	// under the regexes: every footnote here is referenced, so there is always
	// at least one backlink (and, in multi mode, at least one reference id).
	if len(backs) == 0 {
		t.Fatalf("no backlinks matched — anchor format may have changed:\n%s", html)
	}
	if bijection && len(ids) == 0 {
		t.Fatalf("no reference ids matched in multi mode — anchor format may have changed:\n%s", html)
	}

	if dup := firstDuplicate(ids); dup != "" {
		t.Errorf("duplicate reference id %q:\n%s", dup, html)
	}
	idSet := toSet(ids)
	for _, b := range backs {
		if !idSet[b] {
			t.Errorf("backlink targets #%s which is not an emitted reference id:\n%s", b, html)
		}
	}
	if bijection {
		if got, want := sortedUnique(backs), sortedUnique(ids); !equal(got, want) {
			t.Errorf("multi mode is not a bijection between refs and backlinks\n refs:      %v\n backlinks: %v\n%s", want, got, html)
		}
	}
}

// TestFootnoteAnchorIntegrity asserts the cross-reference contract that makes
// the feature correct, on the structurally tricky inputs (multiple footnotes,
// a footnote referenced from inside another footnote, and enough references to
// roll the letter labels past "z"):
//
//   - no reference id is emitted twice (the duplicate-id bug we're fixing), and
//   - every backlink targets a reference id that actually exists in the output.
//
// In multi-backlink mode it additionally requires a bijection: every reference
// id has exactly one backlink and vice versa.
func TestFootnoteAnchorIntegrity(t *testing.T) {
	inputs := map[string]string{
		"three refs, one note":  "x[^a][^a][^a].\n\n[^a]: note.\n",
		"two notes, repeated":   "a[^a][^a] b[^b][^b][^b].\n\n[^a]: na.\n[^b]: nb.\n",
		"ref from inside note":  "body[^a] and[^b].\n\n[^a]: na.\n[^b]: refs a again[^a].\n",
		"label rollover past z": "x" + strings.Repeat("[^a]", 27) + ".\n\n[^a]: note.\n",
	}

	for name, in := range inputs {
		for _, multi := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/multi=%v", name, multi), func(t *testing.T) {
				var html string
				if multi {
					html = djot.RenderHTML(djot.Parse(in), djot.WithMultiBacklinks())
				} else {
					html = djot.RenderHTML(djot.Parse(in))
				}

				assertAnchorIntegrity(t, html, multi)
			})
		}
	}
}

// TestFootnoteBacklinkLabelRollover pins the spreadsheet-style label sequence at
// the z→aa boundary through real rendered output (27 references → labels a..z,aa).
func TestFootnoteBacklinkLabelRollover(t *testing.T) {
	in := "x" + strings.Repeat("[^a]", 27) + ".\n\n[^a]: note.\n"
	html := djot.RenderHTML(djot.Parse(in), djot.WithMultiBacklinks())

	if !strings.Contains(html, `<a id="fnref1-26" href="#fn1" role="doc-noteref">`) ||
		!strings.Contains(html, `<a id="fnref1-27" href="#fn1" role="doc-noteref">`) {
		t.Errorf("missing expected reference ids for 26th/27th refs:\n%s", html)
	}
	if !strings.Contains(html, `<a href="#fnref1-26" role="doc-backlink">z</a>`) {
		t.Errorf("26th backlink should be labeled z:\n%s", html)
	}
	if !strings.Contains(html, `<a href="#fnref1-27" role="doc-backlink">aa</a>`) {
		t.Errorf("27th backlink should be labeled aa:\n%s", html)
	}
}

func matches(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func firstDuplicate(xs []string) string {
	seen := map[string]bool{}
	for _, x := range xs {
		if seen[x] {
			return x
		}
		seen[x] = true
	}
	return ""
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func sortedUnique(xs []string) []string {
	out := make([]string, 0, len(xs))
	for k := range toSet(xs) {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// WithFootnotePrefix namespaces every footnote id (both the note id and the
// per-reference ids) while leaving the cross-reference links internally
// consistent.
func TestFootnotePrefix(t *testing.T) {
	got := djot.RenderHTML(djot.Parse(multiRefInput),
		djot.WithMultiBacklinks(), djot.WithFootnotePrefix("p42-"))

	for _, want := range []string{
		`<li id="p42-fn1">`,
		`<a id="p42-fnref1" href="#p42-fn1" role="doc-noteref">`,
		`<a id="p42-fnref1-2" href="#p42-fn1" role="doc-noteref">`,
		`<a href="#p42-fnref1" role="doc-backlink">a</a>`,
		`<a href="#p42-fnref1-3" role="doc-backlink">c</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prefixed output missing %q:\n%s", want, got)
		}
	}
	// No bare (unprefixed) footnote ids should leak through.
	if strings.Contains(got, `id="fn1"`) || strings.Contains(got, `id="fnref1"`) {
		t.Errorf("found unprefixed footnote id:\n%s", got)
	}
	assertAnchorIntegrity(t, got, true)
}

// WithFootnoteBacklinkLabel replaces the default a/b/c labels (here with the
// reference number) without disturbing the underlying ids.
func TestFootnoteCustomBacklinkLabel(t *testing.T) {
	got := djot.RenderHTML(djot.Parse(multiRefInput),
		djot.WithMultiBacklinks(),
		djot.WithFootnoteBacklinkLabel(func(num, k, total int) string {
			return strconv.Itoa(num) + "." + strconv.Itoa(k)
		}))

	want := `<p>The note.` +
		`<a href="#fnref1" role="doc-backlink">1.1</a> ` +
		`<a href="#fnref1-2" role="doc-backlink">1.2</a> ` +
		`<a href="#fnref1-3" role="doc-backlink">1.3</a></p>`
	if !strings.Contains(got, want) {
		t.Errorf("custom labels wrong.\n got: %s\nwant substring: %s", got, want)
	}
	assertAnchorIntegrity(t, got, true)
}

// Fully custom id producers (WithFootnoteID + WithFootnoteRefID) still link up
// correctly: the integrity invariant holds regardless of the chosen scheme.
func TestFootnoteCustomIDs(t *testing.T) {
	got := djot.RenderHTML(djot.Parse(multiRefInput),
		djot.WithMultiBacklinks(),
		djot.WithFootnoteID(func(num int) string {
			return "note-" + strconv.Itoa(num)
		}),
		djot.WithFootnoteRefID(func(num, k int) string {
			return "cite-" + strconv.Itoa(num) + "-" + strconv.Itoa(k)
		}))

	if !strings.Contains(got, `<li id="note-1">`) {
		t.Errorf("custom note id missing:\n%s", got)
	}
	if !strings.Contains(got, `<a id="cite-1-1" href="#note-1" role="doc-noteref">`) ||
		!strings.Contains(got, `<a id="cite-1-2" href="#note-1" role="doc-noteref">`) {
		t.Errorf("custom reference ids missing:\n%s", got)
	}
	if !strings.Contains(got, `<a href="#cite-1-1" role="doc-backlink">a</a>`) {
		t.Errorf("backlink not linked to custom reference id:\n%s", got)
	}
	assertAnchorIntegrity(t, got, true)
}

// Default output is unchanged when no footnote options are supplied.
func TestFootnoteDefaultsUnchanged(t *testing.T) {
	plain := djot.RenderHTML(djot.Parse(multiRefInput))
	if !strings.Contains(plain, `<li id="fn1">`) ||
		!strings.Contains(plain, `<a id="fnref1" href="#fn1" role="doc-noteref">`) {
		t.Errorf("default footnote ids changed:\n%s", plain)
	}
}

// A footnote referenced only once keeps the single ↩︎ backlink even in
// multi-backlink mode (lettered backlinks only kick in for repeated references).
func TestFootnoteBacklinksMultiSingleRef(t *testing.T) {
	got := djot.RenderHTML(djot.Parse("Once[^a].\n\n[^a]: The note.\n"), djot.WithMultiBacklinks())
	wantBack := `<p>The note.<a href="#fnref1" role="doc-backlink">↩︎</a></p>`
	if !strings.Contains(got, wantBack) {
		t.Errorf("single-ref multi backlink wrong.\n got: %s\nwant substring: %s", got, wantBack)
	}
}
