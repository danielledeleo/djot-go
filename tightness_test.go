package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// TestListTightnessBlankBetweenItems pins tight/loose decisions to djot.js
// behavior. A blank line between items makes a list loose regardless of what
// the items contain; a blank line directly before a nested sublist does not
// count. (Previously an item containing any non-paragraph block cancelled
// looseness, so "- a\n\n- b\n\n  - c" rendered tight.)
func TestListTightnessBlankBetweenItems(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		loose bool
	}{
		{"no-blanks", "- a\n- b\n", false},
		{"blank-between-simple", "- a\n\n- b\n", true},
		{"blank-before-own-sublist", "- a\n\n  - b\n", false},
		{"blank-between-with-sublist", "- a\n\n- b\n\n  - c\n", true},
		{"blank-before-code", "- a\n\n  ```\n  x\n  ```\n", true},
		{"blank-between-break-item", "* a\n\n* {}\n  ***\n", true},
		{"ordered-blank-between-with-sublist", "1. a\n\n1. b\n\n   1. c\n", true},
		// A blank before an item marker does not count when the previous
		// item ends with a sublist: the blank belongs to the inner list,
		// and trailing blanks of a list never affect tightness.
		{"blank-after-sublist", "- a\n\n  - b\n\n- d\n", false},
		{"blank-after-sublist-official", "- a\n  - b\n\n  - c\n\n- d\n", false},
		{"blank-after-para", "- a\n  - b\n\n- d\n", true},
		{"blank-after-code", "- a\n\n  ```\n  x\n  ```\n\n- d\n", true},
		{"blank-after-trailing-para", "- a\n\n  - b\n\n  x\n\n- d\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			html := djot.RenderHTML(djot.Parse(tc.in))
			gotLoose := strings.Contains(html, "<p>")
			if gotLoose != tc.loose {
				t.Errorf("input %q: loose = %v, want %v\nhtml:\n%s",
					tc.in, gotLoose, tc.loose, html)
			}
		})
	}
}

// TestTaskListTightnessMatchesBulletRules: the task-list collector shares
// the bullet/ordered tightness rules, including the trailing-sublist
// exception.
func TestTaskListTightnessMatchesBulletRules(t *testing.T) {
	html := djot.RenderHTML(djot.Parse("- [ ] a\n\n  - b\n\n- [ ] d\n"))
	if strings.Contains(html, "<p>") {
		t.Errorf("task list with trailing-sublist blanks should be tight:\n%s", html)
	}
}

// TestBlankBeforeNestedDefinitionList: a blank before a nested definition
// list does not count toward looseness, same as bullet/ordered sublists.
func TestBlankBeforeNestedDefinitionList(t *testing.T) {
	for _, in := range []string{"- a\n\n  : b\n", "- a\n\n  : b\n\n- d\n"} {
		if html := djot.RenderHTML(djot.Parse(in)); strings.Contains(html, "<p>") {
			t.Errorf("input %q: loose, want tight:\n%s", in, html)
		}
	}
}
