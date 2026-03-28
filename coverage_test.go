package djot_test

import (
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestDocPosition(t *testing.T) {
	doc := djot.Parse("hello\nworld\nthird")
	// "world" starts at offset 6, which is line 2, col 1.
	file, line, col := doc.Position(doc.Root.Children[0].Start)
	if file != "<input>" {
		t.Errorf("file = %q, want <input>", file)
	}
	if line != 1 || col != 1 {
		t.Errorf("position = %d:%d, want 1:1", line, col)
	}
}

func TestAutoEmail(t *testing.T) {
	doc := djot.Parse("<user@example.com>")
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `href="mailto:user@example.com"`) {
		t.Errorf("expected mailto link, got:\n%s", html)
	}
	if !strings.Contains(html, ">user@example.com<") {
		t.Errorf("expected email as link text, got:\n%s", html)
	}
}

func TestAutoURL(t *testing.T) {
	doc := djot.Parse("<https://example.com>")
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `href="https://example.com"`) {
		t.Errorf("expected URL link, got:\n%s", html)
	}
}

func TestEscapeAttrEdgeCases(t *testing.T) {
	// Exercise single-quote escaping in attribute values.
	doc := djot.Parse(`[text](url){title="it's <nice> & \"cool\""}`)
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, "&amp;") {
		t.Errorf("expected escaped &, got:\n%s", html)
	}
	if !strings.Contains(html, "&quot;") {
		t.Errorf("expected escaped quote, got:\n%s", html)
	}
}

func TestTableAlignments(t *testing.T) {
	input := "| L | C | R |\n|:--|:-:|--:|\n| a | b | c |"
	doc := djot.Parse(input)
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `text-align: left`) {
		t.Errorf("expected left align, got:\n%s", html)
	}
	if !strings.Contains(html, `text-align: center`) {
		t.Errorf("expected center align, got:\n%s", html)
	}
	if !strings.Contains(html, `text-align: right`) {
		t.Errorf("expected right align, got:\n%s", html)
	}
}

func TestTableAlignmentsAST(t *testing.T) {
	input := "| L | C | R |\n|:--|:-:|--:|\n| a | b | c |"
	doc := djot.Parse(input)
	ast := djot.RenderAST(doc, false)
	if !strings.Contains(ast, `align="left"`) {
		t.Errorf("expected left in AST, got:\n%s", ast)
	}
	if !strings.Contains(ast, `align="center"`) {
		t.Errorf("expected center in AST, got:\n%s", ast)
	}
	if !strings.Contains(ast, `align="right"`) {
		t.Errorf("expected right in AST, got:\n%s", ast)
	}
}

func TestOrderedListStyles(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"decimal", "1. first\n2. second", "<ol>"},
		{"lower alpha", "a. first\nb. second", `type="a"`},
		{"upper alpha", "A. first\nB. second", `type="A"`},
		{"lower roman", "i. first\nii. second", `type="i"`},
		{"upper roman", "I. first\nII. second", `type="I"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.input)
			html := djot.RenderHTML(doc)
			if !strings.Contains(html, tc.want) {
				t.Errorf("expected %s, got:\n%s", tc.want, html)
			}
		})
	}
}

func TestOrderedListStylesAST(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"decimal", "1. first", `style="1."`},
		{"lower alpha", "a. first", `style="a."`},
		{"lower roman", "i. first\nii. second", `style="i."`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := djot.Parse(tc.input)
			ast := djot.RenderAST(doc, false)
			if !strings.Contains(ast, tc.want) {
				t.Errorf("expected %s in AST, got:\n%s", tc.want, ast)
			}
		})
	}
}

func TestDisplayMath(t *testing.T) {
	doc := djot.Parse("$$`E = mc^2`")
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `class="math display"`) {
		t.Errorf("expected display math, got:\n%s", html)
	}
	if !strings.Contains(html, "E = mc^2") {
		t.Errorf("expected math content, got:\n%s", html)
	}
}

func TestInlineMath(t *testing.T) {
	doc := djot.Parse("$`x^2`")
	html := djot.RenderHTML(doc)
	if !strings.Contains(html, `class="math inline"`) {
		t.Errorf("expected inline math, got:\n%s", html)
	}
}
