package djot

import (
	"strconv"
	"testing"

	"github.com/danielledeleo/djot-go/ast"
)

// hookOptionsForTest registers a hook on each kind selected by seed, with a
// behavior also chosen by seed: default rendering, a wrapper around Default
// or Children, a replacement, or suppression.
func hookOptionsForTest(seed uint64) []RenderOption {
	var opts []RenderOption
	for kind := ast.KindSection; kind <= ast.KindEnDash; kind++ {
		if kind == ast.KindFootnote {
			continue
		}
		bits := (seed >> (uint(kind) % 60)) ^ (seed >> 7 * uint64(kind))
		if bits%5 != 0 {
			continue
		}
		name := kind.String()
		switch behavior := (bits / 5) % 5; behavior {
		case 0:
			opts = append(opts, WithNodeRenderer(kind, func(_ ast.Node, r NodeRenderer) { r.Default() }))
		case 1:
			opts = append(opts, WithNodeRenderer(kind, func(_ ast.Node, r NodeRenderer) {
				r.Write("<" + name + ">")
				r.Children()
				r.Write("</" + name + ">")
			}))
		case 2:
			opts = append(opts, WithNodeRenderer(kind, func(n ast.Node, r NodeRenderer) {
				r.Write("[" + name + " " + strconv.Itoa(n.Span().Start.Offset) + "]")
			}))
		case 3:
			opts = append(opts, WithNodeRenderer(kind, func(ast.Node, NodeRenderer) {}))
		case 4:
			opts = append(opts, WithNodeRenderer(kind, func(_ ast.Node, r NodeRenderer) {
				r.Write("<x>")
				r.Default()
				r.Write("</x>")
			}))
		}
	}
	if seed&1 != 0 {
		opts = append(opts, WithMultiBacklinks())
	}
	return opts
}

var hookParitySeeds = []string{
	"# Title\n\n*strong* _em_ [link](u) ![img](i.png) :sym: `code` $`m`\n",
	"- a\n- b\n\n1. x\n2. y\n\n- [ ] t\n- [x] u\n",
	"| a | b |\n|:--|--:|\n| c | d |\n^ caption\n",
	"ref[^a] again[^a]\n\n[^a]: note\n\n  second para[^b]\n\n[^b]: inner\n",
	"::: note\n> quote\n\n``` go\ncode\n```\n:::\n\nterm\n: def\n",
	"- > first\n  >\n  > second\n- last\n",
}

// Hooks must behave identically on the compact form and on the typed tree.
func TestHookedRenderMatchesTreeRenderer(t *testing.T) {
	for _, src := range hookParitySeeds {
		doc := Parse(src)
		for seed := uint64(0); seed < 400; seed++ {
			opts := hookOptionsForTest(seed * 0x9e3779b97f4a7c15)
			if got, want := RenderHTML(doc, opts...), renderTreeHTMLForTest(doc, opts...); got != want {
				t.Fatalf("seed %d: hooked render differs\ninput: %q\nwant: %q\n got: %q", seed, src, want, got)
			}
		}
	}
}

func FuzzHookedRenderMatchesTreeRenderer(f *testing.F) {
	for i, s := range hookParitySeeds {
		f.Add(s, uint64(i)*0x9e3779b97f4a7c15)
	}
	f.Fuzz(func(t *testing.T, src string, seed uint64) {
		doc := Parse(src)
		opts := hookOptionsForTest(seed)
		if got, want := RenderHTML(doc, opts...), renderTreeHTMLForTest(doc, opts...); got != want {
			t.Fatalf("hooked render differs\ninput: %q seed %d\nwant: %q\n got: %q", src, seed, want, got)
		}
	})
}
