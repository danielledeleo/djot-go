package djot

import (
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

// ParseOption configures a [Parser]. Pass to [NewParser].
type ParseOption func(*parseConfig)

// parseConfig holds the settings a Parser applies. It has no fields yet; it
// exists so that parsing settings can be added without changing the API.
type parseConfig struct{}

// Parser parses djot documents with a fixed configuration. A Parser is
// immutable once created and safe for concurrent use by multiple goroutines.
// The zero value parses with the default configuration, as [Parse] does.
type Parser struct {
	config parseConfig
}

// NewParser returns a Parser configured by opts.
func NewParser(opts ...ParseOption) *Parser {
	p := &Parser{}
	for _, opt := range opts {
		opt(&p.config)
	}
	return p
}

var defaultParser = NewParser()

// Parse parses a djot document with the default configuration. It is
// shorthand for NewParser().Parse(input); see [Parser.Parse].
func Parse(input string) *Doc {
	return defaultParser.Parse(input)
}

// Parse parses a djot document with resolved references, footnotes, and
// auto-generated section IDs.
func (p *Parser) Parse(input string) *Doc {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")

	bp := newBlockParser(input)
	root := bp.parse()
	tightenBlockEnds(root)

	doc := &Doc{
		parseRoot:       root,
		source:          &sourceFile{text: input},
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
	register := func(heading *parseNode, id string) {
		label := collapseWhitespace(collectParseText(heading))
		if id == "" || label == "" {
			return
		}
		if _, exists := doc.parseReferences[label]; !exists {
			doc.parseReferences[label] = &parseNode{
				Kind:         ast.KindLink,
				parsePayload: &parsePayload{Target: "#" + id, HasTarget: true, Label: label},
			}
		}
	}
	walkParse(doc.parseRoot, func(n *parseNode) {
		switch n.Kind {
		case ast.KindSection:
			for _, child := range n.Children {
				if child.Kind == ast.KindHeading {
					register(child, n.Attr("id"))
					break
				}
			}
		case ast.KindHeading:
			register(n, n.Attr("id"))
		}
	})
}

// resolveUnresolvedRefs walks the parse tree looking for link/image nodes with empty
// Target (emitted when the inline parser couldn't resolve a reference) and
// resolves them against the now-complete reference map.
func resolveUnresolvedRefs(doc *Doc) {
	walkParse(doc.parseRoot, func(n *parseNode) {
		if (n.Kind == ast.KindLink || n.Kind == ast.KindImage) && n.Target == "" && !n.HasTarget {
			label := n.Label
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
	case ast.KindBulletList, ast.KindOrderedList, ast.KindTaskList, ast.KindDefinitionList,
		ast.KindListItem, ast.KindTaskListItem, ast.KindBlockQuote, ast.KindTable, ast.KindFootnote:
		if len(n.Children) > 0 {
			if last := n.Children[len(n.Children)-1]; last.End.Offset > 0 {
				n.End = last.End
			}
		}
	}
}
