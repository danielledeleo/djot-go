package djot

import (
	"testing"

	. "github.com/danielledeleo/djot-go/ast"
)

// TestParseOrderedEnumAs exercises the enumerator reader one style at a time.
//
// It is a unit test rather than a parse-and-render one because parsing cannot
// reach all of it: a run of digits is classed decimal the moment it is seen, so
// by the time a list asks "could this enumerator be decimal too?" the answer is
// always no, and the decimal branch only ever returns false in practice. The
// contract is still worth stating, since the branch is one edit away from
// mattering.
func TestParseOrderedEnumAs(t *testing.T) {
	tests := []struct {
		name  string
		enum  string
		style ListStyle
		want  int
		ok    bool
	}{
		// Decimal takes any run of digits, including a leading zero: the spec
		// takes a list's start from its first item, whatever that number is.
		{"decimal one", "1", ListDecimal, 1, true},
		{"decimal multi-digit", "42", ListDecimal, 42, true},
		{"decimal zero", "0", ListDecimal, 0, true},
		{"decimal leading zero", "007", ListDecimal, 7, true},
		{"decimal rejects letters", "a", ListDecimal, 0, false},
		{"decimal rejects mixed", "1a", ListDecimal, 0, false},

		// Alpha enumerators are a single letter, so they run a to z only.
		{"lower alpha first", "a", ListAlphaLower, 1, true},
		{"lower alpha last", "z", ListAlphaLower, 26, true},
		{"lower alpha rejects upper", "A", ListAlphaLower, 0, false},
		{"lower alpha rejects two letters", "aa", ListAlphaLower, 0, false},
		{"lower alpha rejects digit", "1", ListAlphaLower, 0, false},
		{"upper alpha first", "A", ListAlphaUpper, 1, true},
		{"upper alpha last", "Z", ListAlphaUpper, 26, true},
		{"upper alpha rejects lower", "a", ListAlphaUpper, 0, false},
		{"upper alpha rejects two letters", "AA", ListAlphaUpper, 0, false},

		// Roman accepts both spellings of 4 but not a run that spills over.
		{"lower roman one", "i", ListRomanLower, 1, true},
		{"lower roman subtractive", "iv", ListRomanLower, 4, true},
		{"lower roman additive", "iiii", ListRomanLower, 4, true},
		{"lower roman nineteen", "xix", ListRomanLower, 19, true},
		{"lower roman rejects five ones", "iiiii", ListRomanLower, 0, false},
		{"lower roman rejects upper", "I", ListRomanLower, 0, false},
		{"lower roman rejects non-numeral", "a", ListRomanLower, 0, false},
		{"upper roman one", "I", ListRomanUpper, 1, true},
		{"upper roman year", "MCMXCIV", ListRomanUpper, 1994, true},
		{"upper roman rejects lower", "i", ListRomanUpper, 0, false},
		{"upper roman rejects five ones", "IIIII", ListRomanUpper, 0, false},

		// An empty enumerator reads as nothing, whatever the style.
		{"empty decimal", "", ListDecimal, 0, false},
		{"empty lower alpha", "", ListAlphaLower, 0, false},
		{"empty lower roman", "", ListRomanLower, 0, false},

		// A style outside the set reads as nothing.
		{"unknown style", "1", ListStyle(99), 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOrderedEnumAs(tt.enum, tt.style)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseOrderedEnumAs(%q, %v) = (%d, %t), want (%d, %t)",
					tt.enum, tt.style, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// TestParseRoman covers the numeral rule directly: additive and subtractive
// spellings both count, but a digit may not repeat past the point where the
// next digit up takes over.
func TestParseRoman(t *testing.T) {
	tests := []struct {
		enum string
		want int
		ok   bool
	}{
		{"I", 1, true},
		{"IV", 4, true},
		{"IIII", 4, true}, // clock-face 4
		{"V", 5, true},
		{"VIIII", 9, true},
		{"IX", 9, true},
		{"XL", 40, true},
		{"XXXX", 40, true},
		{"CD", 400, true},
		{"CCCC", 400, true},
		{"MMMM", 4000, true},
		{"MCMXCIV", 1994, true},

		// Five of a "one" digit is the next digit up, not a numeral.
		{"IIIII", 0, false},
		{"XXXXX", 0, false},
		{"CCCCC", 0, false},
		// The "five" digits never repeat: VV is X, LL is C, DD is M.
		{"VV", 0, false},
		{"LL", 0, false},
		{"DD", 0, false},

		{"", 0, false},
		{"A", 0, false},
		{"i", 0, false}, // parseRoman takes upper case only
	}

	for _, tt := range tests {
		t.Run(tt.enum, func(t *testing.T) {
			got, ok := parseRoman(tt.enum)
			if got != tt.want || ok != tt.ok {
				t.Errorf("parseRoman(%q) = (%d, %t), want (%d, %t)",
					tt.enum, got, ok, tt.want, tt.ok)
			}
		})
	}
}
