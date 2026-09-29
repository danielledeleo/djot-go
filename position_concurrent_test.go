package djot_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

func TestConcurrentPositionReads(t *testing.T) {
	doc := djot.Parse(strings.Repeat("hello\n", 1000))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			switch i % 3 {
			case 0:
				if got := doc.Position(ast.Pos{Offset: 3000}); got != (ast.Position{Line: 501, Column: 1}) {
					t.Errorf("position = %d:%d, want 501:1", got.Line, got.Column)
				}
			case 1:
				djot.RenderAST(doc, true)
			case 2:
				djot.RenderASTJSON(doc, true)
			}
		}(i)
	}
	close(start)
	wg.Wait()
}
