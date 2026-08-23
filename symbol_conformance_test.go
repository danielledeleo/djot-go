package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// flattenInlines walks the first paragraph's inline children and returns a
// compact []"Kind:value" representation, merging adjacent Text nodes the way
// the reference djot.js coalesces str runs. Used to compare djot-go symbol
// parsing against the canonical implementation.
func flattenInlines(t *testing.T, doc *djot.Node) []string {
	t.Helper()
	if len(doc.Children) == 0 || doc.Children[0].Kind != djot.Paragraph {
		t.Fatalf("expected a leading paragraph, got %+v", doc.Children)
	}
	var out []string
	var textBuf strings.Builder
	flush := func() {
		if textBuf.Len() > 0 {
			out = append(out, "str:"+textBuf.String())
			textBuf.Reset()
		}
	}
	for _, n := range doc.Children[0].Children {
		switch n.Kind {
		case djot.Text:
			textBuf.WriteString(n.Text)
		case djot.Symbol:
			flush()
			out = append(out, "symb:"+n.Name)
		default:
			flush()
			out = append(out, n.Kind.String())
		}
	}
	flush()
	return out
}

// TestSymbolConformance pins djot-go's symbol (:name:) parsing to the behavior
// of the reference implementation (jgm/djot.js). The reference tokenizes
// symbols with the pattern `:[\w_+-]+:` anchored at the trigger colon (find.ts
// uses the sticky 'y' regex flag), with NO flanking/word-boundary rule. The
// char class is identical to inline.go's isSymbolChar.
//
// Expected values below were produced by running the reference:
//
//	printf '%s\n' INPUT | npx @djot/djot -t astpretty
//
// Notably the reference DOES extract symb"30" from the timestamp 10:30:00 and
// symb"b" from a:b:c. This greedy behavior is spec-conformant, not a bug, so a
// downstream consumer cannot treat a leftover Symbol as necessarily intentional
// — incidental digits:digits:digits prose produces real Symbol nodes.
func TestSymbolConformance(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		// Timestamp: reference yields str"10", symb"30", str"00".
		{"timestamp", "10:30:00", []string{"str:10", "symb:30", "str:00"}},
		// Ratio with a single colon pair and no closing colon: no symbol.
		{"ratio", "3:2", []string{"str:3:2"}},
		// Three colon-separated letters: middle becomes a symbol.
		{"abc", "a:b:c", []string{"str:a", "symb:b", "str:c"}},
		// Overlapping colons: first pair wins, trailing colon is literal.
		{"adjacent", ":ice:scream:", []string{"symb:ice", "str:scream:"}},
		// Two well-formed symbols separated by a space.
		{"two_symbols", ":+1: :scream:", []string{"symb:+1", "str: ", "symb:scream"}},
		// Empty colon pair is not a symbol (requires >= 1 name char).
		{"empty", "::", []string{"str:::"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenInlines(t, djot.Parse(tc.in).Root())
			if !equalStrings(got, tc.want) {
				t.Errorf("Parse(%q) inlines = %v, want %v (reference djot.js)", tc.in, got, tc.want)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
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
