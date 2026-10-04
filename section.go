package djot

import (
	"strings"
	"unicode/utf8"

	"github.com/danielledeleo/djot-go/ast"
)

// wrapSections wraps headings and their content in <section> elements.
// Each heading creates a section that contains everything up to the next
// heading of the same or higher level. Sections are only created at the
// document root level. Headings inside block containers (blockquotes, divs,
// list items) get their auto-ID placed directly on the heading element.
func wrapSections(root *parseNode, arena *parseNodeArena) {
	assignHeadingIDs(root, make(map[string]bool))
	root.Children = buildSections(root.Children, arena)
}

// assignHeadingIDs gives every heading without an explicit id one derived from
// its text. Like djot.js, it works in document order, and an identifier is
// taken only once it has appeared: an explicit id later in the document does
// not affect an earlier heading.
func assignHeadingIDs(node *parseNode, used map[string]bool) {
	for _, child := range node.Children {
		if child.Kind == ast.KindHeading {
			// Identifiers inside the heading's own content come first.
			assignHeadingIDs(child, used)
			id := child.Attr("id")
			if id == "" {
				id = uniqueID(autoID(child), used)
				child.SetAttr("id", id)
			}
			used[id] = true
			continue
		}
		if id := child.Attr("id"); id != "" {
			used[id] = true
		}
		assignHeadingIDs(child, used)
	}
}

func buildSections(nodes []*parseNode, arena *parseNodeArena) []*parseNode {
	var result []*parseNode
	i := 0

	for i < len(nodes) {
		node := nodes[i]
		if node.Kind != ast.KindHeading {
			result = append(result, node)
			i++
			continue
		}

		// Create a section wrapping this heading and subsequent content.
		section := arena.new(parseNodeSpec{Kind: ast.KindSection})

		// The heading's id, explicit or generated, moves to the section.
		section.SetAttr("id", node.Attr("id"))

		// Remove id from heading (it lives on the section now).
		// All other attributes stay on the heading.
		if node.Attrs != nil {
			delete(node.Attrs, "id")
			// Rebuild attrOrder without "id".
			filtered := node.attrOrder[:0]
			for _, k := range node.attrOrder {
				if k != "id" {
					filtered = append(filtered, k)
				}
			}
			node.attrOrder = filtered
		}

		section.Children = append(section.Children, node)
		i++

		// Collect subsequent nodes that belong in this section.
		for i < len(nodes) {
			next := nodes[i]
			if next.Kind == ast.KindHeading && next.Level <= node.Level {
				break
			}
			section.Children = append(section.Children, next)
			i++
		}

		// Recursively build sub-sections within this section.
		if len(section.Children) > 1 {
			rest := section.Children[1:]
			section.Children = append(section.Children[:1], buildSections(rest, arena)...)
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

// autoID derives an identifier from a heading's text the way djot.js does:
// each run of whitespace or of the characters ][~!@#$%^&*(){}`,.<>\|=+/? becomes
// a single separator, and other characters are kept.
func autoID(heading *parseNode) string {
	var b strings.Builder
	pending := false
	for _, r := range collectParseText(heading) {
		if isJSSpace(r) || strings.ContainsRune("][~!@#$%^&*(){}`,.<>\\|=+/?", r) {
			pending = b.Len() > 0
			continue
		}
		if pending {
			b.WriteByte('-')
			pending = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// isJSSpace reports whether r is whitespace to a JavaScript regular
// expression (\s), which is what djot.js uses for identifiers.
func isJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029',
		'\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return r >= '\u2000' && r <= '\u200a'
}

// collectParseText returns a node's string content as djot.js computes it for
// heading identifiers and reference labels: smart punctuation keeps its source
// characters, line breaks become newlines, and footnote references, symbols,
// and non-breaking spaces contribute nothing.
func collectParseText(n *parseNode) string {
	var b strings.Builder
	appendParseText(&b, n)
	return b.String()
}

func appendParseText(b *strings.Builder, n *parseNode) {
	switch n.Kind {
	case ast.KindText:
		if n.parsePayload != nil && n.quote != 0 {
			// The text starts with a smart quote; use the source quote instead.
			_, size := utf8.DecodeRuneInString(n.Text)
			b.WriteByte(n.quote)
			b.WriteString(n.Text[size:])
			return
		}
		b.WriteString(n.Text)
		return
	case ast.KindSoftBreak, ast.KindHardBreak:
		b.WriteByte('\n')
		return
	case ast.KindEmDash:
		b.WriteString("---")
		return
	case ast.KindEnDash:
		b.WriteString("--")
		return
	case ast.KindEllipsis:
		b.WriteString("...")
		return
	case ast.KindNonBreakingSpace, ast.KindFootnoteReference, ast.KindSymbol:
		return
	}
	start := b.Len()
	for _, child := range n.Children {
		appendParseText(b, child)
	}
	if b.Len() == start && n.Text != "" {
		b.WriteString(n.Text)
	}
}

// uniqueID returns base, or base-1, base-2, ... if base is already used. An
// empty base becomes s-1, s-2, ...
func uniqueID(base string, used map[string]bool) string {
	id := base
	for i := 1; id == "" || used[id]; i++ {
		prefix := base
		if prefix == "" {
			prefix = "s"
		}
		id = prefix + "-" + itoa(i)
	}
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
