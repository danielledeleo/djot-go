package djot_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
)

func TestRenderASTTo(t *testing.T) {
	doc := djot.Parse("# Hi\n\nHello *world*")
	var b strings.Builder
	if err := djot.RenderASTTo(&b, doc, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := b.String(), djot.RenderAST(doc, false); got != want {
		t.Errorf("RenderASTTo output differs from RenderAST:\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderASTToWithPositions(t *testing.T) {
	doc := djot.Parse("hi")
	var b strings.Builder
	if err := djot.RenderASTTo(&b, doc, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := b.String(), djot.RenderAST(doc, true); got != want {
		t.Errorf("RenderASTTo(positions=true) differs from RenderAST")
	}
}

func TestRenderASTToErrorPropagation(t *testing.T) {
	doc := djot.Parse("Hello *world* this is a paragraph with enough text to trigger the limit")
	w := &errWriter{limit: 5}
	err := djot.RenderASTTo(w, doc, false)
	if err == nil {
		t.Fatal("expected error from failing writer, got nil")
	}
	if !errors.Is(err, errWriteLimited) {
		t.Fatalf("expected errWriteLimited, got: %v", err)
	}
}
