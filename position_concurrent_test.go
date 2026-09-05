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
				_, line, col := doc.Position(ast.Pos{Offset: 3000})
				if line != 501 || col != 1 {
					t.Errorf("position = %d:%d, want 501:1", line, col)
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
