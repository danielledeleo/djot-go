package djot_test

import (
	"testing"

	"github.com/danielledeleo/homedoc/djot"
)

// TestOfficial runs all official djot spec tests.
func TestOfficial(t *testing.T) {
	allTests := loadOfficialTests(t)

	for file, cases := range allTests {
		t.Run(file, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.Name, func(t *testing.T) {
					if tc.IsAST {
						t.Skip("AST-format test, not supported by HTML test runner")
					}
					doc := djot.Parse(tc.Input)
					got := djot.RenderHTML(doc)

					// Trim trailing newline for comparison.
					got = trimTrailingNewline(got)
					expected := trimTrailingNewline(tc.Expected)

					if got != expected {
						t.Errorf("file: %s line: %d\ninput:\n%s\n\nexpected:\n%s\n\ngot:\n%s",
							tc.File, tc.Line, tc.Input, expected, got)
					}
				})
			}
		})
	}
}

func trimTrailingNewline(s string) string {
	for len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}
