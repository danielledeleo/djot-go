package djot

import "strings"

// Parse parses a djot document with resolved references, footnotes, and
// auto-generated section IDs. The mutable AST is materialized lazily by Root.
func Parse(input string) *Doc {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	bp := newBlockParser(input)
	root := bp.parse()
	tightenBlockEnds(root)

	doc := &Doc{
		parseRoot:       root,
		Files:           []FileInfo{{Path: "<input>", Source: []byte(input)}},
		parseReferences: bp.references,
	}

	parseAllInlines(root, doc, bp.arena)
	wrapSections(root, bp.arena)
	registerHeadingRefs(doc)
	resolveUnresolvedRefs(doc)
	doc.semantic = newSemanticTape(doc.parseRoot, input)
	doc.semantic.captureReferences(doc.parseReferences)
	doc.parseRoot = nil
	doc.parseReferences = nil

	return doc
}

// registerHeadingRefs creates implicit reference definitions for headings,
// mapping the heading's text content to the section's (or heading's) ID.
func registerHeadingRefs(doc *Doc) {
	walkParse(doc.parseRoot, func(n *parseNode) {
		if n.Kind == KindSection {
			id := n.Attr("id")
			if id == "" {
				return
			}
			for _, child := range n.Children {
				if child.Kind == KindHeading {
					label := collectParseText(child)
					if label != "" {
						if _, exists := doc.parseReferences[label]; !exists {
							doc.parseReferences[label] = &parseNode{
								Kind:         KindLink,
								parsePayload: &parsePayload{Target: "#" + id, Label: label},
							}
						}
					}
					break
				}
			}
		}
	})
}

// resolveUnresolvedRefs walks the parse tree looking for link/image nodes with empty
// Target (emitted when the inline parser couldn't resolve a reference) and
// resolves them against the now-complete reference map.
func resolveUnresolvedRefs(doc *Doc) {
	walkParse(doc.parseRoot, func(n *parseNode) {
		if (n.Kind == KindLink || n.Kind == KindImage) && n.Target == "" && !n.HasTarget {
			label := collectParseText(n)
			if ref, ok := doc.parseReferences[label]; ok {
				n.Target = ref.Target
				n.HasTarget = true
			}
		}
	})
}

// tightenBlockEnds trims container end positions back to where their content
// actually ends. A block closed lazily — by a less-indented line, a blank line,
// or the end of input — otherwise records an end reaching into whatever closed
// it, so a list item spanning one line appears to run into the next.
//
// Runs post-order, so a container picks up an end its last child has already
// had tightened.
func tightenBlockEnds(n *parseNode) {
	for _, c := range n.Children {
		tightenBlockEnds(c)
	}
	switch n.Kind {
	case KindBulletList, KindOrderedList, KindTaskList, KindDefinitionList,
		KindListItem, KindTaskListItem, KindBlockQuote, KindTable, KindFootnote:
		if len(n.Children) > 0 {
			if last := n.Children[len(n.Children)-1]; last.End.Offset > 0 {
				n.End = last.End
			}
		}
	}
}
