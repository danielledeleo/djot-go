package djot

import (
	"strings"
)

// wrapSections wraps headings and their content in <section> elements.
// Each heading creates a section that contains everything up to the next
// heading of the same or higher level. Sections are only created at the
// document root level. Headings inside block containers (blockquotes, divs,
// list items) get their auto-ID placed directly on the heading element.
func wrapSections(root *Node) {
	// Pre-populate ID set with any explicitly-set IDs on non-heading nodes.
	usedIDs := make(map[string]int)
	Walk(root, func(n *Node) any {
		if n.Kind != Heading {
			if id := n.Attr("id"); id != "" {
				usedIDs[id]++
			}
		}
		return Continue
	})
	// Assign IDs to headings inside block containers (no section wrapping).
	assignContainerHeadingIDs(root, usedIDs)
	// Wrap top-level headings in sections.
	root.Children = buildSections(root.Children, usedIDs)
}

// isBlockContainer returns true for node kinds that are block containers
// where headings should NOT be wrapped in sections.
func isBlockContainer(kind NodeKind) bool {
	switch kind {
	case BlockQuote, Div, ListItem, TaskListItem:
		return true
	}
	return false
}

// assignContainerHeadingIDs walks into block containers and assigns auto-IDs
// directly to headings found inside them (without section wrapping).
func assignContainerHeadingIDs(node *Node, idCounts map[string]int) {
	for _, child := range node.Children {
		if isBlockContainer(child.Kind) {
			assignHeadingIDsInContainer(child, idCounts)
		} else if child.Kind != Heading {
			// Recurse into non-heading, non-container nodes too
			// (e.g. the document root itself on first call).
			assignContainerHeadingIDs(child, idCounts)
		}
	}
}

// assignHeadingIDsInContainer assigns auto-IDs to all headings within a
// block container, recursing into nested containers.
func assignHeadingIDsInContainer(node *Node, idCounts map[string]int) {
	for _, child := range node.Children {
		if child.Kind == Heading {
			id := child.Attr("id")
			explicit := id != ""
			if !explicit {
				id = autoID(child)
				id = uniqueID(id, idCounts)
			} else {
				idCounts[id]++
			}
			child.SetAttr("id", id)
		} else if isBlockContainer(child.Kind) {
			assignHeadingIDsInContainer(child, idCounts)
		}
	}
}

func buildSections(nodes []*Node, idCounts map[string]int) []*Node {
	var result []*Node
	i := 0

	for i < len(nodes) {
		node := nodes[i]
		if node.Kind != Heading {
			result = append(result, node)
			i++
			continue
		}

		// Create a section wrapping this heading and subsequent content.
		section := &Node{Kind: Section}

		// Generate or use provided ID.
		id := node.Attr("id")
		explicit := id != ""
		if !explicit {
			id = autoID(node)
		}
		// Only deduplicate auto-generated IDs.
		if !explicit {
			id = uniqueID(id, idCounts)
		} else {
			// Register explicit ID so auto-IDs won't collide.
			idCounts[id]++
		}

		section.SetAttr("id", id)

		// Move any non-id attributes from heading to section,
		// preserving insertion order.
		if node.Attrs != nil {
			for _, k := range node.attrOrder {
				if k != "id" {
					section.SetAttr(k, node.Attrs[k])
				}
			}
		}
		// Clear heading attrs (ID lives on the section now).
		node.Attrs = nil
		node.attrOrder = nil

		section.Children = append(section.Children, node)
		i++

		// Collect subsequent nodes that belong in this section.
		for i < len(nodes) {
			next := nodes[i]
			if next.Kind == Heading && next.Level <= node.Level {
				break
			}
			section.Children = append(section.Children, next)
			i++
		}

		// Recursively build sub-sections within this section.
		if len(section.Children) > 1 {
			rest := section.Children[1:]
			section.Children = append(section.Children[:1], buildSections(rest, idCounts)...)
		}

		// Set section position: start from heading, end from last child.
		if len(section.Children) > 0 {
			section.Start = section.Children[0].Start
			section.End = section.Children[len(section.Children)-1].End
		}

		result = append(result, section)
	}

	return result
}

// autoID generates a section ID from heading text content.
func autoID(heading *Node) string {
	text := collectText(heading)
	// Replace whitespace runs with single hyphen, keep alphanumeric and hyphens.
	var b strings.Builder
	prevWasSpace := false
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevWasSpace && b.Len() > 0 {
				b.WriteByte('-')
			}
			prevWasSpace = true
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			prevWasSpace = false
		} else {
			prevWasSpace = false
		}
	}
	id := strings.TrimRight(b.String(), "-")
	if id == "" {
		return "s"
	}
	return id
}

// collectText extracts all text content from a node and its children.
func collectText(n *Node) string {
	switch n.Kind {
	case Text:
		return n.Text
	case SoftBreak, HardBreak:
		return " "
	case NonBreakingSpace:
		return " "
	}
	var b strings.Builder
	for _, child := range n.Children {
		b.WriteString(collectText(child))
	}
	if b.Len() == 0 && n.Text != "" {
		return n.Text
	}
	return b.String()
}

// uniqueID deduplicates an ID by appending -1, -2, etc.
// Anonymous section IDs (base "s") always get a counter.
func uniqueID(id string, counts map[string]int) string {
	if id == "s" {
		counts[id]++
		return id + "-" + itoa(counts[id])
	}
	if counts[id] > 0 {
		// Already used. Find the next available suffix.
		suffix := counts[id]
		for {
			candidate := id + "-" + itoa(suffix)
			if counts[candidate] == 0 {
				counts[candidate]++
				counts[id]++
				return candidate
			}
			suffix++
		}
	}
	counts[id]++
	return id
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
