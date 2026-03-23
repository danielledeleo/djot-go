package djot

import (
	"strings"
)

// parseAllInlines walks the AST and parses inline content for all blocks
// that contain raw text (paragraphs, headings).
func parseAllInlines(root *Node, source string) {
	Walk(root, func(n *Node) any {
		switch n.Kind {
		case Paragraph, Heading:
			if n.Text != "" {
				children := parseInline(n.Text)
				n.Children = children
				n.Text = ""
			}
		}
		return Continue
	})
}

// parseInline parses a djot inline string into a list of inline nodes.
func parseInline(input string) []*Node {
	p := &inlineParser{
		input:   input,
		pos:     0,
		openers: make(map[byte][]*opener),
	}
	return p.parse()
}

type opener struct {
	char     byte
	pos      int   // position in input
	nodeIdx  int   // index in nodes slice (placeholder)
	children int   // number of children after this opener
}

type inlineParser struct {
	input   string
	pos     int
	nodes   []*Node
	openers map[byte][]*opener
}

func (p *inlineParser) parse() []*Node {
	for p.pos < len(p.input) {
		c := p.input[p.pos]

		switch c {
		case '\\':
			p.parseEscape()
		case '`':
			p.parseVerbatim()
		case '*':
			p.parseDelimiter('*', Strong)
		case '_':
			p.parseDelimiter('_', Emphasis)
		case '[':
			p.parseBracketOpen()
		case ']':
			p.parseBracketClose()
		case '{':
			p.parseInlineAttr()
		case ':':
			p.parseSymbol()
		case '\n':
			p.addNode(&Node{Kind: SoftBreak})
			p.pos++
		default:
			p.parseText()
		}
	}

	// Close any unclosed openers — they become literal text.
	p.resolveUnclosedOpeners()

	return p.nodes
}

func (p *inlineParser) parseEscape() {
	if p.pos+1 >= len(p.input) {
		p.addTextChar('\\')
		p.pos++
		return
	}

	next := p.input[p.pos+1]

	// Escaped newline = hard break.
	if next == '\n' {
		// Trim trailing whitespace from previous text node.
		p.trimTrailingSpaces()
		p.addNode(&Node{Kind: HardBreak})
		p.pos += 2
		return
	}

	// Escaped space = non-breaking space.
	if next == ' ' {
		p.addNode(&Node{Kind: NonBreakingSpace})
		p.pos += 2
		return
	}

	// Escaped trailing whitespace before newline = hard break.
	if next == ' ' || next == '\t' {
		j := p.pos + 1
		for j < len(p.input) && (p.input[j] == ' ' || p.input[j] == '\t') {
			j++
		}
		if j < len(p.input) && p.input[j] == '\n' {
			p.trimTrailingSpaces()
			p.addNode(&Node{Kind: HardBreak})
			p.pos = j + 1
			return
		}
	}

	// ASCII punctuation can be escaped.
	if isASCIIPunctuation(next) {
		p.addTextChar(next)
		p.pos += 2
		return
	}

	// Non-escapable: output the backslash literally.
	p.addTextChar('\\')
	p.pos++
}

func (p *inlineParser) parseVerbatim() {
	// Count opening backticks.
	start := p.pos
	n := 0
	for p.pos < len(p.input) && p.input[p.pos] == '`' {
		n++
		p.pos++
	}

	// Find matching closing backticks.
	for i := p.pos; i <= len(p.input)-n; i++ {
		match := true
		for j := 0; j < n; j++ {
			if p.input[i+j] != '`' {
				match = false
				break
			}
		}
		if match {
			// Check that the closing backticks are exactly n (not more).
			endAfter := i + n
			if endAfter < len(p.input) && p.input[endAfter] == '`' {
				continue
			}

			content := p.input[p.pos:i]
			p.addNode(&Node{Kind: Verbatim, Text: content})
			p.pos = endAfter
			return
		}
	}

	// No closing backticks found — verbatim extends to end of inline content.
	content := p.input[start+n:]
	// Include opening backtick in displayed content for unclosed.
	p.addNode(&Node{Kind: Verbatim, Text: p.input[start+n:]})
	_ = content
	p.pos = len(p.input)
}

func (p *inlineParser) parseDelimiter(char byte, kind NodeKind) {
	start := p.pos
	p.pos++

	// Check if this can close an existing opener.
	if openers, ok := p.openers[char]; ok && len(openers) > 0 {
		op := openers[len(openers)-1]
		p.openers[char] = openers[:len(openers)-1]

		// Gather children between opener and closer.
		children := p.nodes[op.nodeIdx+1:]
		childCopy := make([]*Node, len(children))
		copy(childCopy, children)

		// Remove opener placeholder and children from nodes.
		p.nodes = p.nodes[:op.nodeIdx]

		// Create the container node.
		node := &Node{Kind: kind, Children: childCopy}
		p.addNode(node)
		return
	}

	// Otherwise, record as potential opener.
	// Check flanking rules: opener must not be followed by whitespace.
	canOpen := p.pos < len(p.input) && !isUnicodeWhitespace(p.input[p.pos])
	// Must not be preceded by ASCII whitespace.
	if start > 0 && p.input[start-1] == ' ' {
		canOpen = false
	}

	if canOpen {
		idx := len(p.nodes)
		p.addNode(&Node{Kind: Text, Text: string(char)})
		p.openers[char] = append(p.openers[char], &opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		p.addNode(&Node{Kind: Text, Text: string(char)})
	}
}

func (p *inlineParser) parseBracketOpen() {
	idx := len(p.nodes)
	p.addNode(&Node{Kind: Text, Text: "["})
	p.openers['['] = append(p.openers['['], &opener{
		char:    '[',
		pos:     p.pos,
		nodeIdx: idx,
	})
	p.pos++
}

func (p *inlineParser) parseBracketClose() {
	p.pos++ // skip ]

	openers, ok := p.openers['[']
	if !ok || len(openers) == 0 {
		p.addNode(&Node{Kind: Text, Text: "]"})
		return
	}

	op := openers[len(openers)-1]
	p.openers['['] = openers[:len(openers)-1]

	// Gather children between [ and ].
	children := p.nodes[op.nodeIdx+1:]
	childCopy := make([]*Node, len(children))
	copy(childCopy, children)
	p.nodes = p.nodes[:op.nodeIdx]

	// Check what follows: (url), {attrs}, or nothing.
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		// Inline link.
		end := strings.IndexByte(p.input[p.pos:], ')')
		if end != -1 {
			target := p.input[p.pos+1 : p.pos+end]
			// Handle multi-line URLs.
			target = strings.ReplaceAll(target, "\n", "")
			target = strings.TrimSpace(target)
			node := &Node{Kind: Link, Target: target, Children: childCopy}
			p.addNode(node)
			p.pos = p.pos + end + 1
			return
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		// Span with attributes.
		end := strings.IndexByte(p.input[p.pos:], '}')
		if end != -1 {
			inner := p.input[p.pos+1 : p.pos+end]
			attrs := ParseAttrs(inner)
			if attrs != nil {
				node := &Node{Kind: Span, Children: childCopy, Attrs: attrs}
				p.addNode(node)
				p.pos = p.pos + end + 1
				return
			}
		}
	}

	// No special syntax follows — just emit literal brackets with content.
	p.addNode(&Node{Kind: Text, Text: "["})
	p.nodes = append(p.nodes, childCopy...)
	p.addNode(&Node{Kind: Text, Text: "]"})
}

func (p *inlineParser) parseInlineAttr() {
	// Look for closing brace.
	end := findClosingBrace(p.input, p.pos)
	if end == -1 {
		p.addTextChar('{')
		p.pos++
		return
	}

	inner := p.input[p.pos+1 : end]
	attrs := ParseAttrs(inner)
	if attrs == nil {
		p.addTextChar('{')
		p.pos++
		return
	}

	// Attach to the preceding element.
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		// If prev is a text node, wrap the last word in a span.
		if prev.Kind == Text {
			text := prev.Text
			// Find the last word boundary.
			lastSpace := strings.LastIndexByte(text, ' ')
			if lastSpace == -1 {
				// Entire text becomes a span.
				p.nodes = p.nodes[:len(p.nodes)-1]
				span := &Node{Kind: Span, Attrs: attrs, Children: []*Node{{Kind: Text, Text: text}}}
				p.addNode(span)
			} else {
				prev.Text = text[:lastSpace+1]
				word := text[lastSpace+1:]
				span := &Node{Kind: Span, Attrs: attrs, Children: []*Node{{Kind: Text, Text: word}}}
				p.addNode(span)
			}
		} else {
			// Attach to previous complex node.
			for k, v := range attrs {
				if k == "class" {
					prev.AddClass(v)
				} else {
					prev.SetAttr(k, v)
				}
			}
		}
	}

	p.pos = end + 1
}

func (p *inlineParser) parseSymbol() {
	// :name: syntax. Name chars: [a-zA-Z0-9_+-]
	start := p.pos
	p.pos++ // skip opening :

	nameStart := p.pos
	for p.pos < len(p.input) && isSymbolChar(p.input[p.pos]) {
		p.pos++
	}

	if p.pos < len(p.input) && p.input[p.pos] == ':' && p.pos > nameStart {
		name := p.input[nameStart:p.pos]
		p.pos++ // skip closing :
		p.addNode(&Node{Kind: Symbol, Name: name})
		return
	}

	// Not a valid symbol, output as text.
	p.pos = start
	p.addTextChar(':')
	p.pos++
}

func (p *inlineParser) parseText() {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '\\' || c == '`' || c == '*' || c == '_' || c == '[' || c == ']' ||
			c == '{' || c == ':' || c == '\n' {
			break
		}
		p.pos++
	}
	if p.pos > start {
		p.addNode(&Node{Kind: Text, Text: p.input[start:p.pos]})
	}
}

func (p *inlineParser) addNode(n *Node) {
	p.nodes = append(p.nodes, n)
}

func (p *inlineParser) addTextChar(c byte) {
	// Merge with previous text node if possible.
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		if prev.Kind == Text {
			prev.Text += string(c)
			return
		}
	}
	p.addNode(&Node{Kind: Text, Text: string(c)})
}

func (p *inlineParser) trimTrailingSpaces() {
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		if prev.Kind == Text {
			prev.Text = strings.TrimRight(prev.Text, " \t")
		}
	}
}

func (p *inlineParser) resolveUnclosedOpeners() {
	// Nothing to do — unclosed openers are already text nodes in p.nodes.
	// Just clear the openers map.
	p.openers = nil
}

// findClosingBrace finds the matching } for a { at pos, respecting quoted strings.
func findClosingBrace(input string, pos int) int {
	depth := 0
	inQuote := byte(0)
	escaped := false

	for i := pos; i < len(input); i++ {
		c := input[i]

		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = c
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func isSymbolChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '+' || c == '-'
}

func isASCIIPunctuation(c byte) bool {
	return (c >= '!' && c <= '/') || (c >= ':' && c <= '@') ||
		(c >= '[' && c <= '`') || (c >= '{' && c <= '~')
}

func isUnicodeWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
