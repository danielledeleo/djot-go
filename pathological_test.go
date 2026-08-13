package djot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danielledeleo/djot-go"
)

// TestPathological guards against super-linear parsing on inputs that pile up
// inline openers without ever closing them. Each case is large enough that
// quadratic behavior blows well past the budget — a run of "[^" took ~7s here
// before openers were invalidated by binary search, versus ~250ms now — while
// leaving an order of magnitude of headroom for a slow machine.
func TestPathological(t *testing.T) {
	if testing.Short() {
		t.Skip("pathological inputs are megabyte-scale")
	}

	const budget = 3 * time.Second

	cases := []struct {
		name  string
		input string
	}{
		{"footnote reference starts", strings.Repeat("[^", 800*1024)},
		{"link starts", strings.Repeat("[a", 800*1024)},
		{"emphasis starts", strings.Repeat("*a ", 400*1024)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan time.Duration, 1)
			go func() {
				start := time.Now()
				djot.RenderHTML(djot.Parse(tc.input))
				done <- time.Since(start)
			}()

			select {
			case elapsed := <-done:
				t.Logf("%d bytes in %v", len(tc.input), elapsed)
				if elapsed > budget {
					t.Errorf("parsing %d bytes took %v, over the %v budget; "+
						"inline parsing has likely regressed to super-linear",
						len(tc.input), elapsed, budget)
				}
			case <-time.After(budget):
				t.Fatalf("parsing %d bytes exceeded the %v budget; "+
					"inline parsing has likely regressed to super-linear",
					len(tc.input), budget)
			}
		})
	}
}
