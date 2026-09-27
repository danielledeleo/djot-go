package djot_test

import (
	"sync"
	"testing"

	"github.com/danielledeleo/djot-go"
)

const parserTestInput = "# Title\n\nSee [the title][Title] and a note.[^n]\n\n[^n]: *Footnote* text.\n"

// Parse, a Parser from NewParser, and the zero Parser all use the default
// configuration, so they must produce identical documents.
func TestParserMatchesParse(t *testing.T) {
	want := djot.RenderAST(djot.Parse(parserTestInput), true)

	var zero djot.Parser
	for name, parser := range map[string]*djot.Parser{
		"NewParser": djot.NewParser(),
		"zero":      &zero,
	} {
		if got := djot.RenderAST(parser.Parse(parserTestInput), true); got != want {
			t.Errorf("%s: AST differs from Parse\ngot:\n%s\nwant:\n%s", name, got, want)
		}
	}
}

// A single Parser is documented as safe for concurrent use. Run under -race.
func TestParserConcurrentUse(t *testing.T) {
	parser := djot.NewParser()
	want := djot.RenderHTML(parser.Parse(parserTestInput))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := djot.RenderHTML(parser.Parse(parserTestInput)); got != want {
				t.Errorf("concurrent parse differs:\ngot:  %q\nwant: %q", got, want)
			}
		}()
	}
	wg.Wait()
}
