package djot_test

import (
	"fmt"
	"regexp"
	"sort"
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
	noterefIDRe = regexp.MustCompile(`<a id="(fnref[\w-]+)" href="#fn\d+" role="doc-noteref">`)
	backlinkRe  = regexp.MustCompile(`<a href="#(fnref[\w-]+)" role="doc-backlink">`)
)

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

				ids := matches(noterefIDRe, html)
				backs := matches(backlinkRe, html)

				// Guard against a vacuous pass: every input here references at
				// least one footnote more than once, so a working renderer always
				// emits backlinks (and, in multi mode, multiple reference ids). If
				// these are empty the anchor format drifted out from under the regexes.
				if len(backs) == 0 {
					t.Fatalf("no backlinks matched — anchor format may have changed:\n%s", html)
				}
				if multi && len(ids) < 2 {
					t.Fatalf("expected multiple reference ids in multi mode — regex may be stale:\n%s", html)
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
				if multi {
					if got, want := sortedUnique(backs), sortedUnique(ids); !equal(got, want) {
						t.Errorf("multi mode is not a bijection between refs and backlinks\n refs:      %v\n backlinks: %v\n%s", want, got, html)
					}
				}
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

// A footnote referenced only once keeps the single ↩︎ backlink even in
// multi-backlink mode (lettered backlinks only kick in for repeated references).
func TestFootnoteBacklinksMultiSingleRef(t *testing.T) {
	got := djot.RenderHTML(djot.Parse("Once[^a].\n\n[^a]: The note.\n"), djot.WithMultiBacklinks())
	wantBack := `<p>The note.<a href="#fnref1" role="doc-backlink">↩︎</a></p>`
	if !strings.Contains(got, wantBack) {
		t.Errorf("single-ref multi backlink wrong.\n got: %s\nwant substring: %s", got, wantBack)
	}
}
