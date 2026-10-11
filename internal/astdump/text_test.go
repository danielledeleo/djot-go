package astdump_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/internal/astdump"
)

var errWriteLimited = errors.New("write limit reached")

// limitWriter fails once more than limit bytes have been written.
type limitWriter struct{ limit, written int }

func (w *limitWriter) Write(p []byte) (int, error) {
	if w.written+len(p) > w.limit {
		return 0, errWriteLimited
	}
	w.written += len(p)
	return len(p), nil
}

func TestWriteErrors(t *testing.T) {
	doc := djot.Parse("Hello *world* this is a paragraph with enough text to trigger the limit")
	for name, write := range map[string]func(*limitWriter) error{
		"text": func(w *limitWriter) error { return astdump.Text(w, doc, false) },
		"json": func(w *limitWriter) error { return astdump.JSON(w, doc, false) },
	} {
		if err := write(&limitWriter{limit: 5}); !errors.Is(err, errWriteLimited) {
			t.Errorf("%s: got %v, want errWriteLimited", name, err)
		}
	}
}

func TestNoPositionsWithoutSource(t *testing.T) {
	doc := djot.NewDoc(djot.Parse("hi").AST())
	var b strings.Builder
	if err := astdump.Text(&b, doc, true); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "doc\n  para\n    str text=\"hi\"\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
