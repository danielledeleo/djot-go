package djot_test

import (
	"testing"

	"github.com/danielledeleo/djot-go"
)

// Every official fixture and benchmark document renders the same after a
// round trip through the typed tree and NewDoc.
func TestNewDocRoundTrip(t *testing.T) {
	for file, cases := range loadOfficialTests(t) {
		for _, tc := range cases {
			want := djot.RenderHTML(djot.Parse(tc.Input))
			if got := djot.RenderHTML(djot.NewDoc(djot.Parse(tc.Input).Root())); got != want {
				t.Errorf("%s:%d round trip differs\ninput: %q\nwant: %q\n got: %q", file, tc.Line, tc.Input, want, got)
			}
		}
	}
	for _, src := range []string{smallDoc(), mediumDoc(), largeDoc(), hugeDoc()} {
		if djot.RenderHTML(djot.NewDoc(djot.Parse(src).Root())) != djot.RenderHTML(djot.Parse(src)) {
			t.Errorf("benchmark document round trip differs (%d bytes)", len(src))
		}
	}
}
