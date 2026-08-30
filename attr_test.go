package djot

import (
	"testing"
)

func TestParseAttrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		// Basic shorthand
		{
			name:     "class shorthand",
			input:    ".warning",
			expected: map[string]string{"class": "warning"},
		},
		{
			name:     "id shorthand",
			input:    "#main",
			expected: map[string]string{"id": "main"},
		},
		{
			name:     "multiple classes",
			input:    ".foo .bar",
			expected: map[string]string{"class": "foo bar"},
		},
		{
			name:     "class and id",
			input:    ".warning #alert",
			expected: map[string]string{"class": "warning", "id": "alert"},
		},

		// Key-value pairs
		{
			name:     "quoted value double",
			input:    `key="value"`,
			expected: map[string]string{"key": "value"},
		},
		{
			// Only double quotes quote a value. djot.js rejects single quotes
			// too, and accepting them made the whole specifier vanish from the
			// output instead of staying literal text.
			name:     "single quotes do not quote a value",
			input:    `key='value'`,
			expected: nil,
		},
		{
			name:     "bare value",
			input:    "key=value",
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "boolean attribute",
			input:    "hidden",
			expected: map[string]string{"hidden": ""},
		},

		// The bugs we found in php-collective/djot-php

		// Dot inside quoted value must NOT be parsed as class shorthand.
		// This was the bug in PR #105.
		{
			name:     "dot in quoted value is literal",
			input:    `include="note.dj"`,
			expected: map[string]string{"include": "note.dj"},
		},
		{
			name:     "hash in quoted value is literal",
			input:    `key="value #not-an-id"`,
			expected: map[string]string{"key": "value #not-an-id"},
		},
		{
			name:     "dot in single quoted value is still not a value",
			input:    `include='note.dj'`,
			expected: nil,
		},

		// Bare values must only contain [a-zA-Z0-9:_-].
		// Dots and slashes are NOT valid bare value characters.
		// This was the secondary bug we found (PR #106).
		{
			name:     "bare value with dot is invalid",
			input:    "include=note.dj",
			expected: nil,
		},
		{
			name:     "bare value with slash is invalid",
			input:    "path=/foo/bar",
			expected: nil,
		},
		{
			name:     "bare value with valid chars",
			input:    "data-role=tab-panel",
			expected: map[string]string{"data-role": "tab-panel"},
		},

		// Escapes in quoted values
		{
			name:     "escaped backslash in quoted value",
			input:    `key="\\\\"`,
			expected: map[string]string{"key": `\\`},
		},
		{
			name:     "escaped quote in quoted value",
			input:    `key="\""`,
			expected: map[string]string{"key": `"`},
		},

		// Braces inside quoted values (must not confuse the parser)
		{
			name:     "braces in quoted value",
			input:    `key="{#hi"`,
			expected: map[string]string{"key": "{#hi"},
		},

		// Comments
		{
			name:     "comment",
			input:    ".foo %comment% .bar",
			expected: map[string]string{"class": "foo bar"},
		},

		// Complex combinations
		{
			name:     "class id and key-value",
			input:    `.note #sidebar key="val"`,
			expected: map[string]string{"class": "note", "id": "sidebar", "key": "val"},
		},

		// Character set edge cases (verified against JS reference)
		{
			name:     "class with colon",
			input:    ".foo:bar",
			expected: map[string]string{"class": "foo:bar"},
		},
		{
			name:     "id with unicode",
			input:    "#\xc3\xbcber", // über
			expected: map[string]string{"id": "\xc3\xbcber"},
		},
		{
			name:     "id with colon",
			input:    "#foo:bar",
			expected: map[string]string{"id": "foo:bar"},
		},
		{
			name:     "id with hyphen",
			input:    "#my-id",
			expected: map[string]string{"id": "my-id"},
		},

		// Empty and whitespace
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: map[string]string{},
		},

		// Invalid inputs
		{
			name:     "dot with no class name",
			input:    ". ",
			expected: nil,
		},
		{
			name:     "hash with no id",
			input:    "# ",
			expected: nil,
		},
		{
			name:     "unclosed quote",
			input:    `key="value`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAttrs(tt.input)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("expected %v, got nil", tt.expected)
			}

			if len(got) != len(tt.expected) {
				t.Errorf("expected %d attrs, got %d\nexpected: %v\ngot:      %v", len(tt.expected), len(got), tt.expected, got)
				return
			}

			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("attr %q: expected %q, got %q", k, v, got[k])
				}
			}
		})
	}
}

// The syntax reference allows key="value" or a bare value; single quotes do
// not quote a value. A specifier using them is not an attribute, so it stays
// literal text rather than being consumed. Accepting it silently swallowed the
// line, since an attribute block that attaches to nothing renders as nothing.
func TestSingleQuotedAttributeValueIsNotAnAttribute(t *testing.T) {
	if got := ParseAttrs("key='single quoted'"); got != nil {
		t.Errorf("ParseAttrs accepted a single-quoted value: %v", got)
	}
	if got := ParseAttrs(`key="double quoted"`); got["key"] != "double quoted" {
		t.Errorf("ParseAttrs(double quoted) = %v, want key=\"double quoted\"", got)
	}

	// The line survives instead of disappearing. Expected output is djot.js's.
	const in = "text\n{key='single quoted'}\nmore\n"
	const want = "<p>text\n{key=\u2019single quoted\u2019\nmore</p>\n"
	if got := RenderHTML(Parse(in)); got != want {
		t.Errorf("input %q:\nwant: %q\n got: %q", in, want, got)
	}
}
