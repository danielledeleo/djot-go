package djot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/danielledeleo/djot-go"
)

// TestPathological guards against super-linear parsing on inputs that pile up
// inline openers. Quadratic behavior blows well past these budgets — a run of
// "[^" took ~7s before openers were invalidated by binary search, versus
// ~250ms now — while leaving an order of magnitude of headroom for a slow
// machine.
func TestPathological(t *testing.T) {
	if testing.Short() {
		t.Skip("pathological inputs are megabyte-scale")
	}

	tests := []struct {
		name   string
		input  string
		budget time.Duration
	}{
		{"footnote reference starts", strings.Repeat("[^", 800*1024), 3 * time.Second},
		{"link starts", strings.Repeat("[a", 800*1024), 3 * time.Second},
		{"emphasis starts", strings.Repeat("*a ", 400*1024), 3 * time.Second},
		// Deeply nested brackets once copied the children accumulated so far on
		// every close, summing to O(n^2): this took ~2.9s before the literal
		// close path stopped copying, and ~5ms after. Sized and budgeted to
		// match djot.js, which runs this shape at 25k brackets a side.
		{"balanced brackets", strings.Repeat("[", 25*1024) + "a" + strings.Repeat("]", 25*1024), time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan time.Duration, 1)
			go func() {
				start := time.Now()
				djot.RenderHTML(djot.Parse(tt.input))
				done <- time.Since(start)
			}()

			select {
			case elapsed := <-done:
				t.Logf("%d bytes in %v", len(tt.input), elapsed)
				if elapsed > tt.budget {
					t.Errorf("parsing %d bytes took %v, over the %v budget; "+
						"inline parsing is super-linear in this shape",
						len(tt.input), elapsed, tt.budget)
				}
			case <-time.After(tt.budget):
				t.Fatalf("parsing %d bytes exceeded the %v budget; "+
					"inline parsing is super-linear in this shape",
					len(tt.input), tt.budget)
			}
		})
	}
}
