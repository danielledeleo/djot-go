package djot

// Parse parses a djot document and returns the AST.
func Parse(input string) *Doc {
	bp := newBlockParser(input)
	root := bp.parse()

	doc := &Doc{
		Root:       root,
		Files:      []FileInfo{{Path: "<input>", Source: []byte(input)}},
		Footnotes:  make(map[string]*Node),
		References: bp.references,
	}

	// Phase 2: parse inline content in all blocks that contain it.
	parseAllInlines(root, doc)

	// Phase 3: wrap headings in sections and assign IDs.
	wrapSections(root)

	// Phase 4: create implicit heading references.
	registerHeadingRefs(doc)

	// Phase 5: resolve any references that were unresolved during inline parsing
	// (e.g., implicit heading references that weren't available yet).
	resolveUnresolvedRefs(doc)

	// Collect footnotes.
	collectFootnotesAndRefs(doc)

	return doc
}

// registerHeadingRefs creates implicit reference definitions for headings,
// mapping the heading's text content to the section's (or heading's) ID.
func registerHeadingRefs(doc *Doc) {
	Walk(doc.Root, func(n *Node) any {
		if n.Kind == Section {
			id := n.Attr("id")
			if id == "" {
				return Continue
			}
			// Find the heading child.
			for _, child := range n.Children {
				if child.Kind == Heading {
					label := collectText(child)
					if label != "" {
						if _, exists := doc.References[label]; !exists {
							doc.References[label] = &Node{Kind: Link, Target: "#" + id, Label: label}
						}
					}
					break
				}
			}
		}
		return Continue
	})
}

// resolveUnresolvedRefs walks the AST looking for Link/Image nodes with empty
// Target (emitted when the inline parser couldn't resolve a reference) and
// resolves them against the now-complete reference map.
func resolveUnresolvedRefs(doc *Doc) {
	Walk(doc.Root, func(n *Node) any {
		if (n.Kind == Link || n.Kind == Image) && n.Target == "" && !n.HasTarget {
			label := collectText(n)
			if ref, ok := doc.References[label]; ok {
				n.Target = ref.Target
				n.HasTarget = true
			}
		}
		return Continue
	})
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

