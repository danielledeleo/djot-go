package djot

// Parse parses a djot document and returns the AST.
func Parse(input string) *Doc {
	bp := newBlockParser(input)
	root := bp.parse()

	// Phase 2: parse inline content in all blocks that contain it.
	parseAllInlines(root, input)

	// Phase 3: wrap headings in sections and assign IDs.
	wrapSections(root)

	doc := &Doc{
		Root:       root,
		Files:      []FileInfo{{Path: "<input>", Source: []byte(input)}},
		Footnotes:  make(map[string]*Node),
		References: make(map[string]*Node),
	}

	// Collect footnotes and references.
	collectFootnotesAndRefs(doc)

	return doc
}

func collectFootnotesAndRefs(doc *Doc) {
	Walk(doc.Root, func(n *Node) any {
		switch n.Kind {
		case Footnote:
			doc.Footnotes[n.Label] = n
		}
		return Continue
	})
}
