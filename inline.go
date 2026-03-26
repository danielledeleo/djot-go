package djot

import (
	"strings"
)

// parseAllInlines walks the AST and parses inline content for all blocks
// that contain raw text (paragraphs, headings).
func parseAllInlines(root *Node, doc *Doc) {
	Walk(root, func(n *Node) any {
		switch n.Kind {
		case Paragraph, Heading, Term, TableCell, Caption:
			if n.Text != "" {
				// Compute the source offset for the inline text content.
				// For headings, text starts after "# " markers.
				// For other blocks, text starts at the block's content area.
				baseOffset := n.Start.Offset
				if n.Kind == Heading && doc != nil && len(doc.Files) > 0 {
					src := doc.Files[0].Source
					off := n.Start.Offset
					// Skip leading whitespace, then skip '#' markers and space.
					for off < len(src) && (src[off] == ' ' || src[off] == '\t') {
						off++
					}
					for off < len(src) && src[off] == '#' {
						off++
					}
					for off < len(src) && (src[off] == ' ' || src[off] == '\t') {
						off++
					}
					baseOffset = off
				} else if n.Kind == Paragraph && doc != nil && len(doc.Files) > 0 {
					src := doc.Files[0].Source
					off := n.Start.Offset
					// Skip leading whitespace on first line.
					for off < len(src) && (src[off] == ' ' || src[off] == '\t') {
						off++
					}
					baseOffset = off
				}
				children := parseInline(n.Text, doc, baseOffset)
				n.Children = children
				n.Text = ""
			}
		}
		return Continue
	})
}

// parseInline parses a djot inline string into a list of inline nodes.
// baseOffset is the source byte offset corresponding to input[0].
func parseInline(input string, doc *Doc, baseOffset int) []*Node {
	p := &inlineParser{
		input:      input,
		pos:        0,
		openers:    make(map[byte][]*opener),
		openerIdx:  make(map[int]bool),
		doc:        doc,
		baseOffset: baseOffset,
	}
	return p.parse()
}

type opener struct {
	char    byte
	pos     int  // position in input
	nodeIdx int  // index in nodes slice (placeholder)
	marked  bool // true if this was an explicitly marked opener ({_ or {*)
}

type inlineParser struct {
	input      string
	pos        int
	nodes      []*Node
	openers    map[byte][]*opener
	openerIdx  map[int]bool // set of nodeIdx values that are opener placeholders
	doc        *Doc
	baseOffset int // source byte offset corresponding to input[0]
}

// srcPos returns a Pos for a position in the inline parser's input.
func (p *inlineParser) srcPos(offset int) Pos {
	return Pos{Offset: p.baseOffset + offset}
}

func (p *inlineParser) parse() []*Node {
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		startPos := p.pos
		nodesBefore := len(p.nodes)

		switch c {
		case '\\':
			p.parseEscape()
		case '`':
			p.parseVerbatim()
		case '$':
			p.parseMath()
		case '*', '_':
			p.parseDelimiter(c)
		case '^':
			p.parseDelimiterPair(c, Superscript)
		case '~':
			p.parseDelimiterPair(c, Subscript)
		case '[':
			p.parseBracketOpen()
		case '!':
			p.parseImageOpen()
		case ']':
			p.parseBracketClose()
		case '{':
			p.parseOpenBrace()
		case '}':
			p.parseCloseBrace()
		case '<':
			p.parseAutolink()
		case '"':
			p.parseSmartQuote('"', DoubleQuoted)
		case '\'':
			p.parseSmartQuote('\'', SingleQuoted)
		case '+', '=':
			p.parseMarkedCloserChar(c)
		case '-':
			p.parseDashes()
		case '.':
			p.parseEllipsis()
		case ':':
			p.parseSymbol()
		case '\n':
			p.addNode(&Node{Kind: SoftBreak})
			p.pos++
		default:
			p.parseText()
		}

		// Set positions on newly created nodes.
		endPos := p.pos
		for i := nodesBefore; i < len(p.nodes); i++ {
			n := p.nodes[i]
			if n.Start.Offset == 0 && n.End.Offset == 0 {
				n.Start = p.srcPos(startPos)
				if endPos > 0 {
					n.End = p.srcPos(endPos - 1)
				}
			}
		}
	}

	// Close any unclosed openers — they become literal text.
	p.resolveUnclosedOpeners()

	return p.nodes
}

func (p *inlineParser) parseEscape() {
	if p.pos+1 >= len(p.input) {
		// Trailing backslash at end of input = hard break.
		p.trimTrailingSpaces()
		p.addNode(&Node{Kind: HardBreak})
		p.pos++
		return
	}

	next := p.input[p.pos+1]

	// Escaped newline = hard break.
	if next == '\n' {
		p.trimTrailingSpaces()
		p.addNode(&Node{Kind: HardBreak})
		p.pos += 2
		return
	}

	// Backslash followed by whitespace then newline = hard break.
	// Must check this BEFORE the single space case.
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

	// Escaped space = non-breaking space.
	if next == ' ' {
		p.addNode(&Node{Kind: NonBreakingSpace})
		p.pos += 2
		return
	}

	// ASCII punctuation can be escaped.
	if isASCIIPunctuation(next) {
		p.addTextByte(next)
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

	// Find matching closing backticks: exactly n backticks, not preceded or followed by `.
	for i := p.pos; i <= len(p.input)-n; i++ {
		// Check that position before the candidate is not a backtick.
		if i > 0 && p.input[i-1] == '`' {
			continue
		}

		match := true
		for j := 0; j < n; j++ {
			if p.input[i+j] != '`' {
				match = false
				break
			}
		}
		if match {
			endAfter := i + n
			if endAfter < len(p.input) && p.input[endAfter] == '`' {
				continue
			}

			content := p.input[p.pos:i]
			content = stripVerbatimSpaces(content)
			node := &Node{Kind: Verbatim, Text: content}
			node.Start = p.srcPos(start)
			node.End = p.srcPos(endAfter - 1)
			p.addNode(node)
			p.pos = endAfter
			return
		}
	}

	// No closing backticks found — verbatim extends to end of inline content.
	content := p.input[start+n:]
	content = stripVerbatimSpaces(content)
	p.addNode(&Node{Kind: Verbatim, Text: content})
	p.pos = len(p.input)
}

func (p *inlineParser) parseMath() {
	// $`...` = inline math, $$`...` = display math
	start := p.pos
	dollars := 0
	for p.pos < len(p.input) && p.input[p.pos] == '$' {
		dollars++
		p.pos++
	}

	if p.pos < len(p.input) && p.input[p.pos] == '`' {
		// Count backticks
		btStart := p.pos
		btCount := 0
		for p.pos < len(p.input) && p.input[p.pos] == '`' {
			btCount++
			p.pos++
		}

		// Find matching closing backticks
		for i := p.pos; i <= len(p.input)-btCount; i++ {
			if i > 0 && p.input[i-1] == '`' {
				continue
			}
			match := true
			for j := 0; j < btCount; j++ {
				if p.input[i+j] != '`' {
					match = false
					break
				}
			}
			if match {
				endAfter := i + btCount
				if endAfter < len(p.input) && p.input[endAfter] == '`' {
					continue
				}
				content := p.input[p.pos:i]
				kind := InlineMath
				if dollars >= 2 {
					kind = DisplayMath
				}
				p.addNode(&Node{Kind: kind, Text: content})
				p.pos = endAfter
				return
			}
		}
		// No match - reset
		p.pos = btStart
	}

	// Not followed by backtick - emit $ signs as text
	p.pos = start
	for i := 0; i < dollars; i++ {
		p.addTextChar('$')
	}
	p.pos = start + dollars
}

func (p *inlineParser) parseDelimiterPair(char byte, kind NodeKind) {
	start := p.pos
	p.pos++

	// Check if this is a marked closer: ^} or ~}
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		p.addNode(&Node{Kind: Text, Text: string(char)})
		return
	}

	canClose := p.canCloseDelimiter(start)
	canOpen := p.canOpenDelimiter(start)

	if canClose {
		if openers, ok := p.openers[char]; ok && len(openers) > 0 {
			for i := len(openers) - 1; i >= 0; i-- {
				op := openers[i]
				if op.marked {
					continue
				}
				children := p.nodes[op.nodeIdx+1:]
				if len(children) == 0 {
					continue
				}
				p.openers[char] = append(openers[:i], openers[i+1:]...)
				childCopy := make([]*Node, len(children))
				copy(childCopy, children)
				p.invalidateOpenersFrom(op.nodeIdx)
				p.nodes = p.nodes[:op.nodeIdx]
				node := &Node{Kind: kind, Children: childCopy}
				node.Start = p.srcPos(op.pos)
				node.End = p.srcPos(p.pos - 1)
				p.addNode(node)
				return
			}
		}
	}

	if canOpen {
		idx := len(p.nodes)
		p.addNode(&Node{Kind: Text, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], &opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		p.addNode(&Node{Kind: Text, Text: string(char)})
	}
}

func (p *inlineParser) parseImageOpen() {
	// Check for ![
	if p.pos+1 < len(p.input) && p.input[p.pos+1] == '[' {
		idx := len(p.nodes)
		p.addNode(&Node{Kind: Text, Text: "!["})
		p.openers['['] = append(p.openers['['], &opener{
			char:    '!', // special marker for image opener
			pos:     p.pos,
			nodeIdx: idx,
		})
		p.pos += 2
		return
	}
	p.addTextChar('!')
	p.pos++
}

func (p *inlineParser) parseAutolink() {
	// Check for <url> or <email> autolink
	start := p.pos
	p.pos++
	end := strings.IndexByte(p.input[p.pos:], '>')
	if end == -1 {
		p.addTextChar('<')
		return
	}

	content := p.input[p.pos : p.pos+end]

	// Don't allow newlines in autolinks
	if strings.ContainsAny(content, "\n") {
		p.pos = start
		p.addTextChar('<')
		p.pos++
		return
	}

	// Check for URL autolink (contains ://)
	if strings.Contains(content, "://") {
		p.addNode(&Node{Kind: Link, Target: content, Children: []*Node{{Kind: Text, Text: content}}})
		p.pos += end + 1
		return
	}

	// Check for email autolink (contains @ and no spaces)
	if strings.Contains(content, "@") && !strings.Contains(content, " ") {
		p.addNode(&Node{Kind: Link, Target: "mailto:" + content, Children: []*Node{{Kind: Text, Text: content}}})
		p.pos += end + 1
		return
	}

	// Not a valid autolink
	p.pos = start
	p.addTextChar('<')
	p.pos++
}

func (p *inlineParser) parseDelimiter(char byte) {
	start := p.pos
	p.pos++

	kind := Emphasis
	if char == '*' {
		kind = Strong
	}

	// Check if this delimiter is followed by } — if so, it's a marked closer.
	// We handle marked closers in parseCloseBrace when we encounter }, so
	// we must NOT process this as a regular closer or opener.
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		// This is part of a _} or *} marked closer. Don't process here;
		// just emit the delimiter char as text and let parseCloseBrace handle it.
		p.addNode(&Node{Kind: Text, Text: string(char)})
		return
	}

	canClose := p.canCloseDelimiter(start)
	canOpen := p.canOpenDelimiter(start)

	// Try to close first.
	if canClose {
		if openers, ok := p.openers[char]; ok && len(openers) > 0 {
			// Find the last non-marked opener.
			for i := len(openers) - 1; i >= 0; i-- {
				op := openers[i]
				if op.marked {
					continue // non-marked closer can't close a marked opener
				}

				// Check for empty emphasis (no content between opener and closer).
				children := p.nodes[op.nodeIdx+1:]
				if len(children) == 0 {
					continue // no empty emphasis
				}

				// Also reject if all children are opener placeholders
				// for the same delimiter char (e.g., ___ = all underscores).
				// Only count actual opener placeholders, not escaped chars.
				allOpenerPlaceholders := true
				for ci, ch := range children {
					childIdx := op.nodeIdx + 1 + ci
					if !(ch.Kind == Text && ch.Text == string(char) && p.openerIdx[childIdx]) {
						allOpenerPlaceholders = false
						break
					}
				}
				if allOpenerPlaceholders {
					// Older openers will have a strict superset of these
					// placeholder children, so they'll also be all-placeholders.
					// No point checking further.
					break
				}

				p.openers[char] = append(openers[:i], openers[i+1:]...)

				childCopy := make([]*Node, len(children))
				copy(childCopy, children)

				// Invalidate any openers whose nodeIdx >= op.nodeIdx.
				p.invalidateOpenersFrom(op.nodeIdx)

				// Remove opener placeholder and children from nodes.
				p.nodes = p.nodes[:op.nodeIdx]

				node := &Node{Kind: kind, Children: childCopy}
				node.Start = p.srcPos(op.pos)
				node.End = p.srcPos(p.pos - 1)
				p.addNode(node)
				return
			}
		}
	}

	// Record as potential opener if it can open.
	if canOpen {
		idx := len(p.nodes)
		p.addNode(&Node{Kind: Text, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], &opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		p.addNode(&Node{Kind: Text, Text: string(char)})
	}
}

// canOpenDelimiter checks if a delimiter at position start can open emphasis.
func (p *inlineParser) canOpenDelimiter(start int) bool {
	// Must not be followed by whitespace (or be at end of input).
	if p.pos >= len(p.input) || isUnicodeWhitespace(p.input[p.pos]) {
		return false
	}
	return true
}

// canCloseDelimiter checks if a delimiter at position start can close emphasis.
func (p *inlineParser) canCloseDelimiter(start int) bool {
	if start == 0 {
		return false
	}
	// Must not be preceded by whitespace.
	if isUnicodeWhitespace(p.input[start-1]) {
		return false
	}
	return true
}

// invalidateOpenersFrom removes any openers (for any char) whose nodeIdx >= fromIdx.
func (p *inlineParser) invalidateOpenersFrom(fromIdx int) {
	for ch, openers := range p.openers {
		filtered := openers[:0]
		for _, op := range openers {
			if op.nodeIdx < fromIdx {
				filtered = append(filtered, op)
			} else {
				delete(p.openerIdx, op.nodeIdx)
			}
		}
		p.openers[ch] = filtered
	}
}

func (p *inlineParser) parseBracketOpen() {
	// Check for footnote reference: [^label]
	if p.pos+1 < len(p.input) && p.input[p.pos+1] == '^' {
		end := strings.IndexByte(p.input[p.pos:], ']')
		if end > 2 {
			label := p.input[p.pos+2 : p.pos+end]
			p.addNode(&Node{Kind: FootnoteReference, Label: label})
			p.pos = p.pos + end + 1
			return
		}
	}

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

	// Safety check: if op.nodeIdx is beyond current nodes, the opener was invalidated.
	if op.nodeIdx >= len(p.nodes) {
		p.addNode(&Node{Kind: Text, Text: "]"})
		return
	}

	isImage := op.char == '!'

	// Gather children between [ (or ![) and ].
	children := p.nodes[op.nodeIdx+1:]
	childCopy := make([]*Node, len(children))
	copy(childCopy, children)
	p.invalidateOpenersFrom(op.nodeIdx)
	p.nodes = p.nodes[:op.nodeIdx]

	// Collect inline text for reference lookup.
	linkText := collectNodesText(childCopy)

	// Check what follows: (url), [ref], {attrs}, or nothing.
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		// Inline link/image — find closing ) with balanced parentheses.
		end := findBalancedParen(p.input, p.pos)
		if end != -1 {
			target := p.input[p.pos+1 : end]
			target = strings.ReplaceAll(target, "\n", "")
			target = strings.TrimSpace(target)
			target = processBackslashEscapes(target)
			linkStart := p.srcPos(op.pos)
			p.pos = end + 1
			linkEnd := p.srcPos(p.pos - 1)
			if isImage {
				node := &Node{Kind: Image, Target: target, HasTarget: true, Children: childCopy}
				node.Start = linkStart
				node.End = linkEnd
				p.addNode(node)
			} else {
				node := &Node{Kind: Link, Target: target, HasTarget: true, Children: childCopy}
				node.Start = linkStart
				node.End = linkEnd
				p.addNode(node)
			}
			return
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '[' {
		// Reference link: [text][ref] or [text][]
		refEnd := strings.IndexByte(p.input[p.pos:], ']')
		if refEnd != -1 {
			refLabel := p.input[p.pos+1 : p.pos+refEnd]
			// Collapse newlines and surrounding whitespace in reference labels
			refLabel = collapseWhitespace(refLabel)
			if refLabel == "" {
				refLabel = linkText
			}
			p.pos = p.pos + refEnd + 1
			if p.resolveReference(refLabel, childCopy, isImage) {
				return
			}
			// Reference not found - emit empty link/image
			if isImage {
				p.addNode(&Node{Kind: Image, Children: childCopy})
			} else {
				p.addNode(&Node{Kind: Link, Children: childCopy})
			}
			return
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		// Span with attributes.
		end := findClosingBrace(p.input, p.pos)
		if end != -1 {
			inner := p.input[p.pos+1 : end]
			attrs, attrOrder := parseAttrsOrdered(inner)
			if attrs != nil {
				node := &Node{Kind: Span, Children: childCopy, Attrs: attrs, attrOrder: attrOrder}
				p.addNode(node)
				p.pos = end + 1
				return
			}
		}
	}

	// No special syntax follows — just emit literal brackets with content.
	if isImage {
		p.addNode(&Node{Kind: Text, Text: "!["})
	} else {
		p.addNode(&Node{Kind: Text, Text: "["})
	}
	p.nodes = append(p.nodes, childCopy...)
	p.addNode(&Node{Kind: Text, Text: "]"})
}

// resolveReference looks up a reference label and creates a Link or Image node.
func (p *inlineParser) resolveReference(label string, children []*Node, isImage bool) bool {
	if p.doc == nil || p.doc.References == nil {
		return false
	}
	ref, ok := p.doc.References[label]
	if !ok {
		return false
	}
	target := ref.Target
	if isImage {
		node := &Node{Kind: Image, Target: target, HasTarget: true, Children: children}
		// Copy ref attrs
		if ref.Attrs != nil {
			for k, v := range ref.Attrs {
				if k == "class" {
					node.AddClass(v)
				} else {
					node.SetAttr(k, v)
				}
			}
		}
		p.addNode(node)
	} else {
		node := &Node{Kind: Link, Target: target, HasTarget: true, Children: children}
		if ref.Attrs != nil {
			for k, v := range ref.Attrs {
				if k == "class" {
					node.AddClass(v)
				} else {
					node.SetAttr(k, v)
				}
			}
		}
		p.addNode(node)
	}
	return true
}

// collectNodesText extracts text content from inline nodes.
func collectNodesText(nodes []*Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(collectText(n))
	}
	return b.String()
}

func (p *inlineParser) parseOpenBrace() {
	// Check if this is a marked opener: {_ or {* or {" or {' or {+ or {- or {= or {^ or {~
	if p.pos+1 < len(p.input) {
		next := p.input[p.pos+1]
		// {= after a Verbatim node should be parsed as a raw format specifier,
		// not as a marked opener for Mark.
		if next == '=' && len(p.nodes) > 0 && p.nodes[len(p.nodes)-1].Kind == Verbatim {
			p.parseInlineAttr()
			return
		}
		if next == '_' || next == '*' || next == '"' || next == '\'' ||
			next == '+' || next == '=' || next == '^' || next == '~' {
			char := next
			start := p.pos + 1
			p.pos += 2

			idx := len(p.nodes)
			p.addNode(&Node{Kind: Text, Text: string(char)})
			p.openerIdx[idx] = true
			p.openers[char] = append(p.openers[char], &opener{
				char:    char,
				pos:     start,
				nodeIdx: idx,
				marked:  true,
			})
			return
		}
		// {- is special: could be marked opener for delete
		if next == '-' {
			char := byte('-')
			start := p.pos + 1
			p.pos += 2

			idx := len(p.nodes)
			p.addNode(&Node{Kind: Text, Text: "-"})
			p.openerIdx[idx] = true
			p.openers[char] = append(p.openers[char], &opener{
				char:    char,
				pos:     start,
				nodeIdx: idx,
				marked:  true,
			})
			return
		}
	}

	// Otherwise try to parse as inline attributes.
	p.parseInlineAttr()
}

func (p *inlineParser) parseCloseBrace() {
	// Check if this is a marked closer: _} or *} or +} or -} or =} or ^} or ~}
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		if prev.Kind == Text && len(prev.Text) == 1 {
			char := prev.Text[0]
			markedKind := markedCloserKind(char)
			if markedKind >= 0 {
				// Find the last marked opener for this char.
				if openers, ok := p.openers[char]; ok && len(openers) > 0 {
					for i := len(openers) - 1; i >= 0; i-- {
						if openers[i].marked {
							op := openers[i]

							// Gather children between marked opener and this closer.
							children := p.nodes[op.nodeIdx+1:]

							// Remove the trailing delimiter text before }.
							if len(children) > 0 {
								last := children[len(children)-1]
								if last.Kind == Text && last.Text == string(char) {
									children = children[:len(children)-1]
								}
							}

							if len(children) == 0 {
								break
							}

							childCopy := make([]*Node, len(children))
							copy(childCopy, children)

							p.openers[char] = append(openers[:i], openers[i+1:]...)
							p.invalidateOpenersFrom(op.nodeIdx)
							p.nodes = p.nodes[:op.nodeIdx]

							node := &Node{Kind: NodeKind(markedKind), Children: childCopy}
							// op.pos points to the char after {, so start at op.pos-1.
							node.Start = p.srcPos(op.pos - 1)
							p.pos++ // skip }
							node.End = p.srcPos(p.pos - 1)
							p.addNode(node)
							return
						}
					}
				}

				// No matching marked opener found.
				p.addTextChar('}')
				p.pos++
				return
			}
		}
	}

	p.addTextChar('}')
	p.pos++
}

// markedCloserKind returns the node kind for a marked closer character, or -1 if not valid.
func markedCloserKind(c byte) int {
	switch c {
	case '_':
		return int(Emphasis)
	case '*':
		return int(Strong)
	case '+':
		return int(Insert)
	case '-':
		return int(Delete)
	case '=':
		return int(Mark)
	case '^':
		return int(Superscript)
	case '~':
		return int(Subscript)
	}
	return -1
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

	// Check for raw format: {=format} on a verbatim node.
	if len(inner) > 1 && inner[0] == '=' && len(p.nodes) > 0 {
		format := inner[1:]
		// Only valid if it's JUST the format specifier (no other attrs).
		if !strings.ContainsAny(format, " \t#.") {
			prev := p.nodes[len(p.nodes)-1]
			if prev.Kind == Verbatim {
				prev.Kind = RawInline
				prev.Format = format
				p.pos = end + 1
				return
			}
		}
	}

	attrs, attrOrder := parseAttrsOrdered(inner)
	if attrs == nil {
		p.addTextChar('{')
		p.pos++
		return
	}

	// Empty attributes: no-op (don't wrap in span).
	if len(attrs) == 0 {
		p.pos = end + 1
		return
	}

	// Attach to the preceding element.
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		if prev.Kind == Text {
			text := prev.Text
			lastSpace := strings.LastIndexByte(text, ' ')
			if lastSpace == -1 {
				p.nodes = p.nodes[:len(p.nodes)-1]
				span := &Node{Kind: Span, Attrs: attrs, attrOrder: attrOrder, Children: []*Node{{Kind: Text, Text: text}}}
				p.addNode(span)
			} else {
				word := text[lastSpace+1:]
				if word == "" {
					// No word after space — discard the attribute block.
					p.pos = end + 1
					return
				}
				prev.Text = text[:lastSpace+1]
				span := &Node{Kind: Span, Attrs: attrs, attrOrder: attrOrder, Children: []*Node{{Kind: Text, Text: word}}}
				p.addNode(span)
			}
		} else {
			// Attach attrs to the previous node, preserving order.
			for _, k := range attrOrder {
				v := attrs[k]
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
	start := p.pos
	p.pos++

	nameStart := p.pos
	for p.pos < len(p.input) && isSymbolChar(p.input[p.pos]) {
		p.pos++
	}

	if p.pos < len(p.input) && p.input[p.pos] == ':' && p.pos > nameStart {
		name := p.input[nameStart:p.pos]
		p.pos++
		p.addNode(&Node{Kind: Symbol, Name: name})
		return
	}

	p.pos = start
	p.addTextChar(':')
	p.pos++
}

func (p *inlineParser) parseSmartQuote(char byte, kind NodeKind) {
	start := p.pos
	p.pos++

	// Check if this is a marked closer: "} or '}
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		// Find the last marked opener for this quote char.
		qchar := char
		if openers, ok := p.openers[qchar]; ok && len(openers) > 0 {
			for i := len(openers) - 1; i >= 0; i-- {
				if openers[i].marked {
					op := openers[i]
					children := p.nodes[op.nodeIdx+1:]
					// Remove trailing quote text before }
					if len(children) > 0 {
						last := children[len(children)-1]
						if last.Kind == Text && last.Text == string(char) {
							children = children[:len(children)-1]
						}
					}
					childCopy := make([]*Node, len(children))
					copy(childCopy, children)
					p.openers[qchar] = append(openers[:i], openers[i+1:]...)
					p.invalidateOpenersFrom(op.nodeIdx)
					p.nodes = p.nodes[:op.nodeIdx]
					node := &Node{Kind: kind, Children: childCopy}
					node.Start = p.srcPos(op.pos - 1)
					p.pos++ // skip }
					node.End = p.srcPos(p.pos - 1)
					p.addNode(node)
					return
				}
			}
		}
		// No matching marked opener — emit quote as text.
		p.addNode(&Node{Kind: Text, Text: string(char)})
		return
	}

	canClose := p.canCloseQuote(start)
	canOpen := p.canOpenQuote(start, char)

	// Try to close first.
	if canClose {
		if openers, ok := p.openers[char]; ok && len(openers) > 0 {
			for i := len(openers) - 1; i >= 0; i-- {
				op := openers[i]
				if op.marked {
					continue // non-marked closer can't close a marked opener
				}
				// Reject empty spans (e.g. '' should nest, not close immediately).
				if op.nodeIdx+1 >= len(p.nodes) {
					continue
				}
				children := p.nodes[op.nodeIdx+1:]
				childCopy := make([]*Node, len(children))
				copy(childCopy, children)
				p.openers[char] = append(openers[:i], openers[i+1:]...)
				p.invalidateOpenersFrom(op.nodeIdx)
				p.nodes = p.nodes[:op.nodeIdx]
				node := &Node{Kind: kind, Children: childCopy}
				node.Start = p.srcPos(op.pos)
				node.End = p.srcPos(p.pos - 1)
				p.addNode(node)
				return
			}
		}
	}

	// Record as potential opener if it can open.
	if canOpen {
		idx := len(p.nodes)
		p.addNode(&Node{Kind: Text, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], &opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		// Could not open or close (or close with no matching opener).
		// Single quotes become apostrophes; double quotes become left double quotes.
		if char == '\'' {
			p.addNode(&Node{Kind: Text, Text: "\u2019"}) // right single quote / apostrophe
		} else {
			p.addNode(&Node{Kind: Text, Text: "\u201c"}) // left double quote
		}
	}
}

// canOpenQuote checks if a quote at position start can open.
// For double quotes, the opentest is always true (matching djot.js).
// For single quotes, the character must be preceded by whitespace, or
// one of: " ' - ( [
func (p *inlineParser) canOpenQuote(start int, char byte) bool {
	// Must be followed by a non-space character.
	if p.pos >= len(p.input) || isUnicodeWhitespace(p.input[p.pos]) {
		return false
	}
	if char == '"' {
		return true
	}
	// Single quote: can only open at start or after specific characters.
	if start == 0 {
		return true
	}
	prev := p.input[start-1]
	return prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r' ||
		prev == '"' || prev == '\'' || prev == '-' || prev == '(' || prev == '['
}

// canCloseQuote checks if a quote at position start can close.
func (p *inlineParser) canCloseQuote(start int) bool {
	if start == 0 {
		return false
	}
	// Must be preceded by a non-space character.
	return !isUnicodeWhitespace(p.input[start-1])
}


func (p *inlineParser) parseDashes() {
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
	}
	count := p.pos - start

	// If the dash sequence is followed by '}', reserve the last dash as a
	// standalone text node so parseCloseBrace can detect a marked closer.
	trailingCloser := false
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		trailingCloser = true
		count--
	}

	if count == 0 {
		// Only had 1 dash and it's the closer
		p.addNode(&Node{Kind: Text, Text: "-"})
		return
	}

	if count == 1 {
		p.addTextChar('-')
	} else {
		// Convert sequences of hyphens to em/en dashes.
		// Prefer homogeneous sequences: all em-dashes if divisible by 3,
		// all en-dashes if divisible by 2. Otherwise, em-dashes first,
		// with as few en-dashes as possible.
		em, en := 0, 0
		if count%3 == 0 {
			em = count / 3
		} else if count%2 == 0 {
			en = count / 2
		} else if count%3 == 2 {
			em = (count - 2) / 3
			en = 1
		} else {
			// count%3 == 1: use (count-4)/3 em dashes + 2 en dashes
			em = (count - 4) / 3
			en = 2
		}
		for i := 0; i < em; i++ {
			p.addNode(&Node{Kind: EmDash})
		}
		for i := 0; i < en; i++ {
			p.addNode(&Node{Kind: EnDash})
		}
	}

	if trailingCloser {
		p.addNode(&Node{Kind: Text, Text: "-"})
	}
}

func (p *inlineParser) parseEllipsis() {
	// Check for three dots.
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == '.' && p.input[p.pos+2] == '.' {
		p.addNode(&Node{Kind: Ellipsis})
		p.pos += 3
		return
	}
	p.addTextChar('.')
	p.pos++
}

// parseMarkedCloserChar handles '+' and '=' characters. If the next char is '}',
// emit a standalone text node (so parseCloseBrace can detect the marked closer).
// Otherwise emit as merged text.
func (p *inlineParser) parseMarkedCloserChar(c byte) {
	if p.pos+1 < len(p.input) && p.input[p.pos+1] == '}' {
		p.addNode(&Node{Kind: Text, Text: string(c)})
		p.pos++
		return
	}
	p.addTextChar(c)
	p.pos++
}

func (p *inlineParser) parseText() {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '\\' || c == '`' || c == '$' || c == '*' || c == '_' ||
			c == '^' || c == '~' || c == '[' || c == '!' || c == ']' ||
			c == '{' || c == '}' || c == '<' || c == ':' || c == '\n' ||
			c == '"' || c == '\'' || c == '-' || c == '.' ||
			c == '+' || c == '=' {
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
	// Merge with previous text node if possible, but NOT with opener placeholders.
	if len(p.nodes) > 0 {
		idx := len(p.nodes) - 1
		prev := p.nodes[idx]
		if prev.Kind == Text && !p.openerIdx[idx] {
			prev.Text = prev.Text + string(c)
			return
		}
	}
	p.addNode(&Node{Kind: Text, Text: string(c)})
}

// addTextByte adds a byte as text, always creating a new node or merging
// with previous non-opener text. Same as addTextChar but clearer name.
func (p *inlineParser) addTextByte(c byte) {
	p.addTextChar(c)
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
	// Convert unmatched quote openers to appropriate characters.
	// Unmatched " → left double quote "\u201c
	// Unmatched ' → right single quote (apostrophe) "\u2019
	for _, openers := range p.openers {
		for _, op := range openers {
			if op.nodeIdx < len(p.nodes) {
				node := p.nodes[op.nodeIdx]
				if node.Kind == Text {
					switch op.char {
					case '"':
						node.Text = "\u201c" // left double quote
					case '\'':
						node.Text = "\u2019" // right single quote (apostrophe)
					default:
						// For marked openers, restore the leading '{'.
						if op.marked {
							node.Text = "{" + node.Text
						}
						// For unmarked openers (like * _) that are immediately
						// followed by a Span, absorb the opener text into the span.
						if !op.marked && op.nodeIdx+1 < len(p.nodes) {
							next := p.nodes[op.nodeIdx+1]
							if next.Kind == Span {
								next.Children = append([]*Node{{Kind: Text, Text: node.Text}}, next.Children...)
								node.Text = "" // clear the opener text
							}
						}
					}
				}
			}
		}
	}
	p.openers = nil
	p.openerIdx = nil
}

// findClosingBrace finds the matching } for a { at pos, respecting quoted strings.
// findBalancedParen finds the closing ')' that matches the '(' at pos,
// tracking nested parentheses. Returns the index of the closing ')' or -1.
func findBalancedParen(input string, pos int) int {
	depth := 0
	for i := pos; i < len(input); i++ {
		switch input[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

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

// stripVerbatimSpaces strips one leading and one trailing space from verbatim
// content, but only when the content starts or ends with a backtick after
// stripping. This allows backticks at the edges of code spans.
func stripVerbatimSpaces(s string) string {
	if len(s) < 2 || s[0] != ' ' || s[len(s)-1] != ' ' {
		return s
	}
	// Only strip if content after stripping would start or end with backtick.
	inner := s[1 : len(s)-1]
	if len(inner) > 0 && (inner[0] == '`' || inner[len(inner)-1] == '`') {
		return inner
	}
	return s
}

// collapseWhitespace replaces sequences of whitespace (including newlines)
// with a single space, and trims leading/trailing whitespace.
func collapseWhitespace(s string) string {
	var b strings.Builder
	inSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if !inSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			inSpace = true
		} else {
			inSpace = false
			b.WriteByte(c)
		}
	}
	result := b.String()
	return strings.TrimRight(result, " ")
}

// processBackslashEscapes processes backslash escapes in link destinations.
// Only ASCII punctuation can be escaped.
func processBackslashEscapes(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isASCIIPunctuation(s[i+1]) {
			b.WriteByte(s[i+1])
			i++
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
