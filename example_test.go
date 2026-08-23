package djot_test

import (
	"fmt"
	"os"

	"github.com/danielledeleo/djot-go"
)

func ExampleParse() {
	doc := djot.Parse("Hello *world*!")
	fmt.Println(djot.RenderHTML(doc))
	// Output:
	// <p>Hello <strong>world</strong>!</p>
}

func ExampleRenderHTML() {
	doc := djot.Parse("# Heading\n\nA paragraph with a [link](https://djot.net).\n")
	fmt.Println(djot.RenderHTML(doc))
	// Output:
	// <section id="Heading">
	// <h1>Heading</h1>
	// <p>A paragraph with a <a href="https://djot.net">link</a>.</p>
	// </section>
}

func ExampleRenderHTMLTo() {
	doc := djot.Parse("Hello *world*!")
	err := djot.RenderHTMLTo(os.Stdout, doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	// Output:
	// <p>Hello <strong>world</strong>!</p>
}

func ExampleWalk() {
	doc := djot.Parse("Hello *world* and _more_")

	// Count strong and emphasis nodes.
	strong, emphasis := 0, 0
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		switch n.Kind() {
		case djot.KindStrong:
			strong++
		case djot.KindEmphasis:
			emphasis++
		}
		return djot.Continue
	})
	fmt.Printf("strong: %d, emphasis: %d\n", strong, emphasis)
	// Output:
	// strong: 1, emphasis: 1
}

func ExampleWalk_replace() {
	doc := djot.Parse("Hello *world*!")

	// Replace all Strong nodes with Emphasis.
	djot.Walk(doc.Root(), func(n djot.Node) djot.Action {
		if strong, ok := n.(*djot.Strong); ok {
			replacement := &djot.Emphasis{Children: strong.Children}
			djot.CopyMetadata(replacement, strong)
			return djot.Replace(replacement)
		}
		return djot.Continue
	})
	fmt.Println(djot.RenderHTML(doc))
	// Output:
	// <p>Hello <em>world</em>!</p>
}

func ExampleWithNodeRenderer() {
	doc := djot.Parse("Visit [djot](https://djot.net).")

	// Render links with target="_blank".
	html := djot.RenderHTML(doc, djot.WithNodeRenderer(djot.KindLink, func(n djot.Node, r djot.NodeRenderer) {
		link := n.(*djot.Link)
		r.Write(fmt.Sprintf(`<a href="%s" target="_blank">`, link.Destination))
		r.Children()
		r.Write("</a>")
	}))
	fmt.Println(html)
	// Output:
	// <p>Visit <a href="https://djot.net" target="_blank">djot</a>.</p>
}

func ExampleWithSymbolRenderer() {
	doc := djot.Parse("Launch :rocket: or keep :unknown:.")
	html := djot.RenderHTML(doc, djot.WithSymbolRenderer(func(symbol djot.SymbolView, r djot.ElementRenderer) {
		if symbol.Name == "rocket" {
			r.Write("🚀")
			return
		}
		r.Default()
	}))
	fmt.Println(html)
	// Output:
	// <p>Launch 🚀 or keep :unknown:.</p>
}

func ExampleRenderAST() {
	doc := djot.Parse("Hello *world*!")
	fmt.Println(djot.RenderAST(doc, false))
	// Output:
	// doc
	//   para
	//     str text="Hello "
	//     strong
	//       str text="world"
	//     str text="!"
}
