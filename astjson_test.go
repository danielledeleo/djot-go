package djot_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

// compactJSON validates that s is well-formed JSON and returns it with
// insignificant whitespace removed, so tests can assert structure/order without
// depending on indentation.
func compactJSON(t *testing.T, s string) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(s)); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, s)
	}
	return b.String()
}

func TestRenderASTJSON(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"text", "hi",
			`{"tag":"doc","children":[{"tag":"para","children":[{"tag":"str","text":"hi"}]}]}`,
		},
		{
			"symbol", ":smile:",
			`{"tag":"doc","children":[{"tag":"para","children":[{"tag":"symb","alias":"smile"}]}]}`,
		},
		{
			// In djot *...* is strong and _..._ is emphasis.
			"emphasis", "a _b_",
			`{"tag":"doc","children":[{"tag":"para","children":[{"tag":"str","text":"a "},{"tag":"emph","children":[{"tag":"str","text":"b"}]}]}]}`,
		},
		{
			"heading_with_attr", "# Hi",
			`{"tag":"doc","children":[{"tag":"section","attributes":{"id":"Hi"},"children":[{"tag":"heading","level":1,"children":[{"tag":"str","text":"Hi"}]}]}]}`,
		},
		{
			"link_destination", "[x](http://e.com)",
			`{"tag":"doc","children":[{"tag":"para","children":[{"tag":"link","destination":"http://e.com","children":[{"tag":"str","text":"x"}]}]}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compactJSON(t, djot.RenderASTJSON(djot.Parse(tc.in), false))
			if got != tc.want {
				t.Errorf("RenderASTJSON(%q) =\n  %s\nwant:\n  %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestRenderASTJSONIndented(t *testing.T) {
	out := djot.RenderASTJSON(djot.Parse("hi"), false)
	if !strings.Contains(out, "\n  \"tag\"") {
		t.Errorf("expected 2-space indented JSON, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("expected output to end with a newline, got:\n%q", out)
	}
}

func TestRenderASTJSONPositions(t *testing.T) {
	got := compactJSON(t, djot.RenderASTJSON(djot.Parse("hi"), true))
	// The document root carries no position (matches RenderAST).
	if !strings.HasPrefix(got, `{"tag":"doc","children":`) {
		t.Errorf("doc root should have no pos field, got:\n%s", got)
	}
	// Child nodes carry start/end with line, col, and offset.
	if !strings.Contains(got, `"pos":{"start":{"line":1,"col":1,"offset":0},"end":{`) {
		t.Errorf("expected pos objects on child nodes, got:\n%s", got)
	}
}
