package djot_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/danielledeleo/djot-go"
)

// TestPathological guards against super-linear inline parsing on inputs that
// pile up openers.
//
// It compares timings at two input sizes rather than against a wall clock. An
// absolute budget depends on the machine and on whether -race is set, which
// costs roughly 6x and is how CI runs; growth rate is the property actually
// worth asserting.
//
// Four times the input measures at 4.3-5.1x here, with or without -race. The
// shapes below were each quadratic at some point, which costs about 16x, so the
// threshold sits clear of both. n is large enough that the smaller timing stays
// well above the noise that makes short runs unreliable.
func TestPathological(t *testing.T) {
	if testing.Short() {
		t.Skip("pathological inputs run to hundreds of KB")
	}

	const (
		growth   = 4
		maxRatio = 10 // linear measures ~5, quadratic ~16
		reps     = 2
		n        = 64 * 1024

		// Quadratic parsing takes minutes at these sizes, so both measurements
		// are bounded. The ratio below is still the assertion; these only stop
		// the test waiting on an answer it can already give. No linear parse of
		// 128KB comes near this, even on a machine 100x slower than this one.
		sanityCap = 10 * time.Second
	)

	// parse reports how long one parse took, or false if it ran past cap.
	parse := func(in string, cap time.Duration) (time.Duration, bool) {
		done := make(chan time.Duration, 1)
		go func() {
			start := time.Now()
			djot.RenderHTML(djot.Parse(in))
			done <- time.Since(start)
		}()
		select {
		case d := <-done:
			return d, true
		case <-time.After(cap):
			return 0, false
		}
	}

	// Fastest of several runs: an unlucky sample on a loaded CI runner inflates
	// a duration but never deflates one, so the minimum is the stable statistic.
	fastest := func(in string, cap time.Duration) (time.Duration, bool) {
		best := time.Duration(math.MaxInt64)
		for i := 0; i < reps; i++ {
			d, ok := parse(in, cap)
			if !ok {
				return 0, false
			}
			if d < best {
				best = d
			}
		}
		return best, true
	}

	tests := []struct {
		name  string
		build func(n int) string
	}{
		{"footnote reference starts", func(n int) string { return strings.Repeat("[^", n) }},
		{"link starts", func(n int) string { return strings.Repeat("[a", n) }},
		{"emphasis starts", func(n int) string { return strings.Repeat("*a ", n) }},
		{"balanced brackets", func(n int) string {
			return strings.Repeat("[", n) + "a" + strings.Repeat("]", n)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			small, ok := fastest(tt.build(n), sanityCap)
			if !ok {
				t.Fatalf("n=%d took over %v to parse; inline parsing is super-linear in this shape",
					n, sanityCap)
			}

			// Too quick to divide meaningfully. n is chosen to keep this from
			// happening, so say so rather than assert on noise.
			if small < time.Millisecond {
				t.Skipf("n=%d parsed in %v, too fast to measure growth against", n, small)
			}

			// Anything slower than this has already failed the ratio check.
			cap := time.Duration(float64(small) * growth * maxRatio)
			large, ok := fastest(tt.build(n*growth), cap)
			if !ok {
				t.Fatalf("%dx the input ran past %v, over %dx the %v baseline; "+
					"inline parsing is super-linear in this shape",
					growth, cap, growth*maxRatio, small)
			}

			ratio := float64(large) / float64(small)
			t.Logf("n=%d %v -> n=%d %v (%.1fx for %dx the input)",
				n, small, n*growth, large, ratio, growth)

			if ratio > maxRatio {
				t.Errorf("%dx the input cost %.1fx the time (n=%d %v -> n=%d %v); "+
					"inline parsing is super-linear in this shape",
					growth, ratio, n, small, n*growth, large)
			}
		})
	}
}
