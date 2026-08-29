package djot

import (
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

// parseAllInlines walks the AST and parses inline content for all blocks
// that contain raw text (paragraphs, headings).
func parseAllInlines(root *parseNode, doc *Doc, arena *parseNodeArena) {
	p := &inlineParser{
		openers:   make(map[byte][]opener, 4),
		openerIdx: make(map[int]bool, 8),
		arena:     arena,
	}
	walkParse(root, func(n *parseNode) {
		switch n.Kind {
		case ast.KindParagraph, ast.KindHeading, ast.KindTerm, ast.KindTableCell, ast.KindCaption:
			if n.Text != "" {
				baseOffset := n.Start.Offset
				if n.Kind == ast.KindHeading && doc != nil && len(doc.Files) > 0 {
					src := doc.Files[0].Source
					off := n.Start.Offset
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
				} else if n.Kind == ast.KindParagraph && doc != nil && len(doc.Files) > 0 {
					src := doc.Files[0].Source
					off := n.Start.Offset
					for off < len(src) && (src[off] == ' ' || src[off] == '\t') {
						off++
					}
					baseOffset = off
				}
				n.Children = p.parseInline(n.Text, doc, baseOffset, n.plainBracesUntil)
				n.Text = ""
				n.plainBracesUntil = 0
			}
		}
	})
}

// parseInline parses a djot inline string into a list of inline nodes.
// baseOffset is the source byte offset corresponding to input[0].
// plainBracesUntil is an offset before which braces are ordinary characters.
func (p *inlineParser) parseInline(input string, doc *Doc, baseOffset, plainBracesUntil int) []*parseNode {
	p.input = input
	p.pos = 0
	p.nodes = p.scratch[:0]
	clear(p.openers)
	clear(p.openerIdx)
	p.doc = doc
	p.baseOffset = baseOffset
	p.plainBracesUntil = plainBracesUntil
	out := p.parse()
	p.scratch = out[:0]
	return p.slices.clone(out)
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
	nodes      []*parseNode
	openers    map[byte][]opener
	openerIdx  map[int]bool // set of nodeIdx values that are opener placeholders
	doc        *Doc
	baseOffset int             // source byte offset corresponding to input[0]
	arena      *parseNodeArena // shared node allocator for the whole document
	slices     nodeSliceArena  // shared allocator for child slices
	scratch    []*parseNode    // reused node buffer across blocks

	plainBracesUntil int
}

// srcPos returns an ast.Pos for a position in the inline parser's input.
func (p *inlineParser) srcPos(offset int) ast.Pos {
	return ast.Pos{Offset: p.baseOffset + offset}
}

func (p *inlineParser) parse() []*parseNode {
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
			p.parseDelimiterPair(c, ast.KindSuperscript)
		case '~':
			p.parseDelimiterPair(c, ast.KindSubscript)
		case '[':
			p.parseBracketOpen()
		case '!':
			p.parseImageOpen()
		case ']':
			p.parseBracketClose()
		case '{', '}':
			// Block parsing already tried and failed to read these as a block
			// attribute, so here they are just characters.
			if p.pos < p.plainBracesUntil {
				p.addTextChar(c)
				p.pos++
			} else if c == '{' {
				p.parseOpenBrace()
			} else {
				p.parseCloseBrace()
			}
		case '<':
			p.parseAutolink()
		case '"':
			p.parseSmartQuote('"', ast.KindDoubleQuoted)
		case '\'':
			p.parseSmartQuote('\'', ast.KindSingleQuoted)
		case '+', '=':
			p.parseMarkedCloserChar(c)
		case '-':
			p.parseDashes()
		case '.':
			p.parseEllipsis()
		case ':':
			p.parseSymbol()
		case '\n':
			p.add(parseNodeSpec{Kind: ast.KindSoftBreak})
			p.pos++
		default:
			p.parseText()
		}

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

	p.resolveUnclosedOpeners()

	return p.nodes
}

func (p *inlineParser) parseEscape() {
	if p.pos+1 >= len(p.input) {
		// Trailing backslash at end of input = hard break.
		p.trimTrailingSpaces()
		p.add(parseNodeSpec{Kind: ast.KindHardBreak})
		p.pos++
		return
	}

	next := p.input[p.pos+1]

	// Escaped newline = hard break.
	if next == '\n' {
		p.trimTrailingSpaces()
		p.add(parseNodeSpec{Kind: ast.KindHardBreak})
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
			p.add(parseNodeSpec{Kind: ast.KindHardBreak})
			p.pos = j + 1
			return
		}
	}

	// Escaped space = non-breaking space.
	if next == ' ' {
		p.add(parseNodeSpec{Kind: ast.KindNonBreakingSpace})
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
			node := p.arena.new(parseNodeSpec{Kind: ast.KindVerbatim, Text: content})
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
	p.add(parseNodeSpec{Kind: ast.KindVerbatim, Text: content})
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
				kind := ast.KindInlineMath
				if dollars >= 2 {
					kind = ast.KindDisplayMath
				}
				p.add(parseNodeSpec{Kind: kind, Text: content})
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

func (p *inlineParser) parseDelimiterPair(char byte, kind ast.Kind) {
	start := p.pos
	p.pos++

	// Check if this is a marked closer: ^} or ~}
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
		return
	}

	canClose := p.canCloseDelimiter(start)
	canOpen := p.canOpenDelimiter(start)

	if canClose {
		if openers, ok := p.openers[char]; ok && len(openers) > 0 {
			// Only check the most recent non-marked opener.
			var op opener
			opIdx := -1
			for i := len(openers) - 1; i >= 0; i-- {
				if !openers[i].marked {
					op = openers[i]
					opIdx = i
					break
				}
			}
			if opIdx >= 0 {
				children := p.nodes[op.nodeIdx+1:]
				if len(children) > 0 {
					p.openers[char] = append(openers[:opIdx], openers[opIdx+1:]...)
					childCopy := p.slices.clone(children)
					p.invalidateOpenersFrom(op.nodeIdx)
					p.nodes = p.nodes[:op.nodeIdx]
					node := p.arena.new(parseNodeSpec{Kind: kind, Children: childCopy})
					node.Start = p.srcPos(op.pos)
					node.End = p.srcPos(p.pos - 1)
					p.addNode(node)
					return
				}
			}
		}
	}

	if canOpen {
		idx := len(p.nodes)
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
	}
}

func (p *inlineParser) parseImageOpen() {
	// Check for ![
	if p.pos+1 < len(p.input) && p.input[p.pos+1] == '[' {
		// "![^" is a footnote reference behind a literal "!", not an image, so
		// leave the bracket to open a plain one.
		if p.pos+2 < len(p.input) && p.input[p.pos+2] == '^' {
			p.addTextChar('!')
			p.pos++
			return
		}
		idx := len(p.nodes)
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "!["})
		p.openerIdx[idx] = true
		p.openers['['] = append(p.openers['['], opener{
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
		p.add(parseNodeSpec{Kind: ast.KindLink, Target: content, Children: []*parseNode{{Kind: ast.KindText, Text: content}}})
		p.pos += end + 1
		return
	}

	// Check for email autolink (contains @ and no spaces)
	if strings.Contains(content, "@") && !strings.Contains(content, " ") {
		p.add(parseNodeSpec{Kind: ast.KindLink, Target: "mailto:" + content, Children: []*parseNode{{Kind: ast.KindText, Text: content}}})
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

	kind := ast.KindEmphasis
	if char == '*' {
		kind = ast.KindStrong
	}

	// Check if this delimiter is followed by } — if so, it's a marked closer.
	// We handle marked closers in parseCloseBrace when we encounter }, so
	// we must NOT process this as a regular closer or opener.
	if p.pos < len(p.input) && p.input[p.pos] == '}' {
		// This is part of a _} or *} marked closer. Don't process here;
		// just emit the delimiter char as text and let parseCloseBrace handle it.
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
		return
	}

	canClose := p.canCloseDelimiter(start)
	canOpen := p.canOpenDelimiter(start)

	// Like djot.js, only the most recent non-marked opener may close.
	if canClose {
		if openers, ok := p.openers[char]; ok && len(openers) > 0 {
			var op opener
			opIdx := -1
			for i := len(openers) - 1; i >= 0; i-- {
				if !openers[i].marked {
					op = openers[i]
					opIdx = i
					break
				}
			}
			if opIdx >= 0 {
				children := p.nodes[op.nodeIdx+1:]
				if len(children) > 0 {
					p.openers[char] = append(openers[:opIdx], openers[opIdx+1:]...)

					childCopy := p.slices.clone(children)

					p.invalidateOpenersFrom(op.nodeIdx)
					p.nodes = p.nodes[:op.nodeIdx]

					node := p.arena.new(parseNodeSpec{Kind: kind, Children: childCopy})
					node.Start = p.srcPos(op.pos)
					node.End = p.srcPos(p.pos - 1)
					p.addNode(node)
					return
				}
			}
		}
	}

	if canOpen {
		idx := len(p.nodes)
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
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
//
// Openers are ordered by nodeIdx, so every surviving set is a prefix.
func (p *inlineParser) invalidateOpenersFrom(fromIdx int) {
	for ch, openers := range p.openers {
		// Find the first opener with nodeIdx >= fromIdx.
		lo, hi := 0, len(openers)
		for lo < hi {
			mid := int(uint(lo+hi) >> 1)
			if openers[mid].nodeIdx < fromIdx {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		if lo == len(openers) {
			continue
		}
		for _, op := range openers[lo:] {
			delete(p.openerIdx, op.nodeIdx)
		}
		p.openers[ch] = openers[:lo]
	}
}

func (p *inlineParser) parseBracketOpen() {
	idx := len(p.nodes)
	p.add(parseNodeSpec{Kind: ast.KindText, Text: "["})
	p.openerIdx[idx] = true
	p.openers['['] = append(p.openers['['], opener{
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
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "]"})
		return
	}

	op := openers[len(openers)-1]
	p.openers['['] = openers[:len(openers)-1]
	// The opener is no longer live: if its "["/"![" ends up as literal text,
	// following text may merge with it again.
	delete(p.openerIdx, op.nodeIdx)

	if op.nodeIdx >= len(p.nodes) {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "]"})
		return
	}

	if op.char == '[' && op.pos+1 < len(p.input) && p.input[op.pos+1] == '^' {
		// References are normalized like reference-link labels, collapsing runs
		// of whitespace so a label broken across lines still matches its
		// definition. An empty label is a valid reference, though never a valid
		// definition, so it resolves to nothing.
		label := collapseWhitespace(p.input[op.pos+2 : p.pos-1])
		// A nested "[" would give a label no definition could ever carry, since
		// a definition's label ends at its first "]". Leave the run literal
		// instead of inventing a reference that can never resolve.
		if !strings.Contains(label, "[") {
			p.invalidateOpenersFrom(op.nodeIdx)
			p.nodes = p.nodes[:op.nodeIdx]
			p.add(parseNodeSpec{Kind: ast.KindFootnoteReference, Label: label})
			return
		}
	}

	if p.pos >= len(p.input) || (p.input[p.pos] != '(' && p.input[p.pos] != '[' && p.input[p.pos] != '{') {
		p.invalidateOpenersFrom(op.nodeIdx)
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "]"})
		return
	}

	isImage := op.char == '!'

	children := p.nodes[op.nodeIdx+1:]
	childCopy := p.slices.clone(children)
	p.invalidateOpenersFrom(op.nodeIdx)
	p.nodes = p.nodes[:op.nodeIdx]

	linkText := collectNodesText(childCopy)

	if p.pos < len(p.input) && p.input[p.pos] == '(' {
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
				node := p.arena.new(parseNodeSpec{Kind: ast.KindImage, Target: target, HasTarget: true, Children: childCopy})
				node.Start = linkStart
				node.End = linkEnd
				p.addNode(node)
			} else {
				node := p.arena.new(parseNodeSpec{Kind: ast.KindLink, Target: target, HasTarget: true, Children: childCopy})
				node.Start = linkStart
				node.End = linkEnd
				p.addNode(node)
			}
			return
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '[' {
		refEnd := strings.IndexByte(p.input[p.pos:], ']')
		if refEnd != -1 {
			refLabel := p.input[p.pos+1 : p.pos+refEnd]
			refLabel = collapseWhitespace(refLabel)
			if refLabel == "" {
				refLabel = linkText
			}
			p.pos = p.pos + refEnd + 1
			if p.resolveReference(refLabel, childCopy, isImage) {
				return
			}
			if isImage {
				p.add(parseNodeSpec{Kind: ast.KindImage, Children: childCopy})
			} else {
				p.add(parseNodeSpec{Kind: ast.KindLink, Children: childCopy})
			}
			return
		}
	}

	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		end := findClosingBrace(p.input, p.pos)
		if end != -1 {
			inner := p.input[p.pos+1 : end]
			attrs, attrOrder := parseAttrsOrdered(inner)
			if attrs != nil {
				node := p.arena.new(parseNodeSpec{Kind: ast.KindSpan, Children: childCopy, Attrs: attrs, attrOrder: attrOrder})
				p.addNode(node)
				p.pos = end + 1
				return
			}
		}
	}

	if isImage {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "!["})
	} else {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "["})
	}
	p.nodes = append(p.nodes, childCopy...)
	p.add(parseNodeSpec{Kind: ast.KindText, Text: "]"})
}

// resolveReference looks up a reference label and creates a link or image node.
func (p *inlineParser) resolveReference(label string, children []*parseNode, isImage bool) bool {
	if p.doc == nil || p.doc.parseReferences == nil {
		return false
	}
	ref, ok := p.doc.parseReferences[label]
	if !ok {
		return false
	}
	target := ref.Target
	if isImage {
		node := p.arena.new(parseNodeSpec{Kind: ast.KindImage, Target: target, HasTarget: true, Children: children})
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
		node := p.arena.new(parseNodeSpec{Kind: ast.KindLink, Target: target, HasTarget: true, Children: children})
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
func collectNodesText(nodes []*parseNode) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(collectParseText(n))
	}
	return b.String()
}

func (p *inlineParser) parseOpenBrace() {
	// Check if this is a marked opener: {_ or {* or {" or {' or {+ or {- or {= or {^ or {~
	if p.pos+1 < len(p.input) {
		next := p.input[p.pos+1]
		// {= after a verbatim node should be parsed as a raw format specifier,
		// not as a marked opener.
		if next == '=' && len(p.nodes) > 0 && p.nodes[len(p.nodes)-1].Kind == ast.KindVerbatim {
			p.parseInlineAttr()
			return
		}
		if next == '_' || next == '*' || next == '"' || next == '\'' ||
			next == '+' || next == '=' || next == '^' || next == '~' {
			char := next
			start := p.pos + 1
			p.pos += 2

			idx := len(p.nodes)
			p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
			p.openerIdx[idx] = true
			p.openers[char] = append(p.openers[char], opener{
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
			p.add(parseNodeSpec{Kind: ast.KindText, Text: "-"})
			p.openerIdx[idx] = true
			p.openers[char] = append(p.openers[char], opener{
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
		if prev.Kind == ast.KindText && len(prev.Text) == 1 {
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
								if last.Kind == ast.KindText && last.Text == string(char) {
									children = children[:len(children)-1]
								}
							}

							if len(children) == 0 {
								break
							}

							childCopy := p.slices.clone(children)

							p.openers[char] = append(openers[:i], openers[i+1:]...)
							p.invalidateOpenersFrom(op.nodeIdx)
							p.nodes = p.nodes[:op.nodeIdx]

							node := p.arena.new(parseNodeSpec{Kind: ast.Kind(markedKind), Children: childCopy})
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
		return int(ast.KindEmphasis)
	case '*':
		return int(ast.KindStrong)
	case '+':
		return int(ast.KindInsert)
	case '-':
		return int(ast.KindDelete)
	case '=':
		return int(ast.KindMark)
	case '^':
		return int(ast.KindSuperscript)
	case '~':
		return int(ast.KindSubscript)
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
			if prev.Kind == ast.KindVerbatim {
				prev.Kind = ast.KindRawInline
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
		if prev.Kind == ast.KindText {
			text := prev.Text
			lastSpace := strings.LastIndexByte(text, ' ')
			if lastSpace == -1 {
				p.nodes = p.nodes[:len(p.nodes)-1]
				span := p.arena.new(parseNodeSpec{Kind: ast.KindSpan, Attrs: attrs, attrOrder: attrOrder, Children: []*parseNode{{Kind: ast.KindText, Text: text}}})
				p.addNode(span)
			} else {
				word := text[lastSpace+1:]
				if word == "" {
					// No word after space — discard the attribute block.
					p.pos = end + 1
					return
				}
				prev.Text = text[:lastSpace+1]
				span := p.arena.new(parseNodeSpec{Kind: ast.KindSpan, Attrs: attrs, attrOrder: attrOrder, Children: []*parseNode{{Kind: ast.KindText, Text: word}}})
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
		p.add(parseNodeSpec{Kind: ast.KindSymbol, Name: name})
		return
	}

	p.pos = start
	p.addTextChar(':')
	p.pos++
}

func (p *inlineParser) parseSmartQuote(char byte, kind ast.Kind) {
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
					// The children may legitimately end with a literal quote
					// character (e.g. an escaped quote in {'\''}); it is part
					// of the quoted content, not the closing delimiter
					// (matching djot.js).
					children := p.nodes[op.nodeIdx+1:]
					childCopy := p.slices.clone(children)
					p.openers[qchar] = append(openers[:i], openers[i+1:]...)
					p.invalidateOpenersFrom(op.nodeIdx)
					p.nodes = p.nodes[:op.nodeIdx]
					node := p.arena.new(parseNodeSpec{Kind: kind, Children: childCopy})
					node.Start = p.srcPos(op.pos - 1)
					p.pos++ // skip }
					node.End = p.srcPos(p.pos - 1)
					p.addNode(node)
					return
				}
			}
		}
		// No marked opener to pair with, but the marker still forces this quote
		// to close, and it applies only here: a later unmatched quote is
		// unaffected and still opens.
		if char == '\'' {
			p.add(parseNodeSpec{Kind: ast.KindText, Text: "’"}) // right single quote
		} else {
			p.add(parseNodeSpec{Kind: ast.KindText, Text: "”"}) // right double quote
		}
		p.pos++ // skip }
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
				childCopy := p.slices.clone(children)
				p.openers[char] = append(openers[:i], openers[i+1:]...)
				p.invalidateOpenersFrom(op.nodeIdx)
				p.nodes = p.nodes[:op.nodeIdx]
				node := p.arena.new(parseNodeSpec{Kind: kind, Children: childCopy})
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
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(char)})
		p.openerIdx[idx] = true
		p.openers[char] = append(p.openers[char], opener{
			char:    char,
			pos:     start,
			nodeIdx: idx,
		})
	} else {
		// Could not open or close (or close with no matching opener).
		// Single quotes become apostrophes; double quotes become left double quotes.
		if char == '\'' {
			p.add(parseNodeSpec{Kind: ast.KindText, Text: "\u2019"}) // right single quote / apostrophe
		} else {
			p.add(parseNodeSpec{Kind: ast.KindText, Text: "\u201c"}) // left double quote
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
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "-"})
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
			p.add(parseNodeSpec{Kind: ast.KindEmDash})
		}
		for i := 0; i < en; i++ {
			p.add(parseNodeSpec{Kind: ast.KindEnDash})
		}
	}

	if trailingCloser {
		p.add(parseNodeSpec{Kind: ast.KindText, Text: "-"})
	}
}

func (p *inlineParser) parseEllipsis() {
	// Check for three dots.
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == '.' && p.input[p.pos+2] == '.' {
		p.add(parseNodeSpec{Kind: ast.KindEllipsis})
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
		p.add(parseNodeSpec{Kind: ast.KindText, Text: string(c)})
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
		p.add(parseNodeSpec{Kind: ast.KindText, Text: p.input[start:p.pos]})
	}
}

func (p *inlineParser) addNode(n *parseNode) {
	p.nodes = append(p.nodes, n)
}

// add appends a node built from n, allocated from the shared arena.
func (p *inlineParser) add(n parseNodeSpec) {
	p.nodes = append(p.nodes, p.arena.new(n))
}

func (p *inlineParser) addTextChar(c byte) {
	// Merge with previous text node if possible, but NOT with opener placeholders.
	if len(p.nodes) > 0 {
		idx := len(p.nodes) - 1
		prev := p.nodes[idx]
		if prev.Kind == ast.KindText && !p.openerIdx[idx] {
			prev.Text = prev.Text + string(c)
			return
		}
	}
	p.add(parseNodeSpec{Kind: ast.KindText, Text: string(c)})
}

// addTextByte adds a byte as text, always creating a new node or merging
// with previous non-opener text. Same as addTextChar but clearer name.
func (p *inlineParser) addTextByte(c byte) {
	p.addTextChar(c)
}

func (p *inlineParser) trimTrailingSpaces() {
	if len(p.nodes) > 0 {
		prev := p.nodes[len(p.nodes)-1]
		if prev.Kind == ast.KindText {
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
				if node.Kind == ast.KindText {
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
						// followed by a span, absorb the opener text into it.
						if !op.marked && op.nodeIdx+1 < len(p.nodes) {
							next := p.nodes[op.nodeIdx+1]
							if next.Kind == ast.KindSpan {
								next.Children = append([]*parseNode{{Kind: ast.KindText, Text: node.Text}}, next.Children...)
								node.Text = "" // clear the opener text
							}
						}
					}
				}
			}
		}
	}
}

// findClosingBrace finds the matching } for a { at pos, respecting quoted strings.
// findBalancedParen finds the closing ')' that matches the '(' at pos,
// tracking nested parentheses. Returns the index of the closing ')' or -1.
func findBalancedParen(input string, pos int) int {
	depth := 0
	for i := pos; i < len(input); i++ {
		switch input[i] {
		case '\\':
			i++ // an escaped paren neither opens nor closes
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

// nodeSliceArena bump-allocates runs of []*parseNode from shared chunks, so the
// child lists of inline containers cost one allocation per chunk instead of
// one per container. Returned slices are capped to their length, so any later
// append reallocates instead of writing into the next run.
type nodeSliceArena struct {
	chunk []*parseNode
	next  int
}

const (
	sliceArenaMinChunk = 256
	sliceArenaMaxChunk = 8192
)

func (a *nodeSliceArena) clone(src []*parseNode) []*parseNode {
	if len(src) == 0 {
		return nil
	}
	if len(a.chunk)+len(src) > cap(a.chunk) {
		size := a.next
		if size < sliceArenaMinChunk {
			size = sliceArenaMinChunk
		}
		if size < len(src) {
			size = len(src)
		}
		a.chunk = make([]*parseNode, 0, size)
		if size < sliceArenaMaxChunk {
			a.next = size * 2
		}
	}
	start := len(a.chunk)
	a.chunk = append(a.chunk, src...)
	return a.chunk[start:len(a.chunk):len(a.chunk)]
}
