package djot

import "strings"

// Parse parses a djot document and returns the complete AST with resolved
// references, footnotes, and auto-generated section IDs.
func Parse(input string) *Doc {
	// Normalize line endings.
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	bp := newBlockParser(input)
	root := bp.parse()
	tightenBlockEnds(root)

	doc := &Doc{
		Root:       root,
		Files:      []FileInfo{{Path: "<input>", Source: []byte(input)}},
		Footnotes:  make(map[string]*Node),
		References: bp.references,
	}

	// Phase 2: parse inline content in all blocks that contain it.
	parseAllInlines(root, doc, bp.arena)

	// Phase 3: wrap headings in sections and assign IDs.
	wrapSections(root, bp.arena)

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

// tightenBlockEnds trims container end positions back to where their content
// actually ends. A block closed lazily — by a less-indented line, a blank line,
// or the end of input — otherwise records an end reaching into whatever closed
// it, so a list item spanning one line appears to run into the next.
//
// Runs post-order, so a container picks up an end its last child has already
// had tightened.
func tightenBlockEnds(n *Node) {
	for _, c := range n.Children {
		tightenBlockEnds(c)
	}
	switch n.Kind {
	case BulletList, OrderedList, TaskList, DefinitionList,
		ListItem, TaskListItem, BlockQuote, Table, Footnote:
		if len(n.Children) > 0 {
			if last := n.Children[len(n.Children)-1]; last.End.Offset > 0 {
				n.End = last.End
			}
		}
	}
}
