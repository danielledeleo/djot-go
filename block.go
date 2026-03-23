package djot

import (
	"strings"
)

type blockParser struct {
	input        string
	lines        []blockLine
	pos          int
	pendingAttrs map[string]string
}

type blockLine struct {
	start  int    // byte offset in input
	end    int    // byte offset (exclusive, before newline)
	text   string // full line content (no newline)
	indent int    // count of leading spaces
	strip  string // content after stripping indent
}

func newBlockParser(input string) *blockParser {
	bp := &blockParser{input: input}
	bp.splitLines()
	return bp
}

func (bp *blockParser) splitLines() {
	offset := 0
	for offset < len(bp.input) {
		end := strings.IndexByte(bp.input[offset:], '\n')
		var lineEnd int
		if end == -1 {
			lineEnd = len(bp.input)
		} else {
			lineEnd = offset + end
		}
		text := bp.input[offset:lineEnd]
		indent := countLeadingSpaces(text)
		bp.lines = append(bp.lines, blockLine{
			start:  offset,
			end:    lineEnd,
			text:   text,
			indent: indent,
			strip:  text,
		})
		if end == -1 {
			break
		}
		offset = lineEnd + 1
	}
}

func (bp *blockParser) parse() *Node {
	root := &Node{Kind: Document}
	bp.parseBlocks(root, 0, "")
	return root
}

// parseBlocks parses block-level elements and appends them to parent.
// baseIndent is the minimum indentation for content in this container.
// prefix is the string prefix to strip (e.g., "> " for blockquotes).
func (bp *blockParser) parseBlocks(parent *Node, baseIndent int, prefix string) {
	for bp.pos < len(bp.lines) {
		if bp.parseBlock(parent, baseIndent, prefix) {
			continue
		}
		break
	}
}

func (bp *blockParser) parseBlock(parent *Node, baseIndent int, prefix string) bool {
	if bp.pos >= len(bp.lines) {
		return false
	}

	line := bp.currentLine()

	// Blank line.
	if isBlankLine(line.text) {
		bp.pos++
		return true
	}

	// Strip prefix (for blockquote continuation).
	text := line.text
	if prefix != "" {
		if strings.HasPrefix(text, prefix) {
			text = text[len(prefix):]
		} else {
			return false
		}
	}

	stripped := strings.TrimLeft(text, " \t")
	indent := len(text) - len(stripped)

	// Block-level attributes.
	if len(stripped) > 0 && stripped[0] == '{' && strings.HasSuffix(strings.TrimRight(stripped, " \t"), "}") {
		inner := strings.TrimRight(stripped, " \t")
		inner = inner[1 : len(inner)-1]
		attrs := ParseAttrs(inner)
		if attrs != nil {
			bp.pendingAttrs = mergeAttrs(bp.pendingAttrs, attrs)
			bp.pos++
			return true
		}
	}

	// Thematic break: must be preceded by blank line or at start, and
	// consists of 3+ of the same char (*, -, or _) with optional spaces.
	if isThematicBreak(stripped) && bp.isPrecededByBlank(parent) {
		node := &Node{Kind: ThematicBreak}
		bp.attachPendingAttrs(node)
		parent.Children = append(parent.Children, node)
		bp.pos++
		return true
	}

	// Heading: # followed by space (or end of line).
	if level := headingLevel(stripped); level > 0 {
		bp.parseHeading(parent, level, stripped, prefix)
		return true
	}

	// Code block: ``` or ~~~
	if isCodeFenceOpen(stripped) {
		bp.parseCodeBlock(parent, stripped, indent, prefix)
		return true
	}

	// Block quote: > followed by space.
	if len(stripped) > 0 && stripped[0] == '>' && (len(stripped) == 1 || stripped[1] == ' ') {
		bp.parseBlockQuote(parent, indent, prefix)
		return true
	}

	// Fenced div: ::: followed by optional class.
	if isDivFenceOpen(stripped) {
		bp.parseFencedDiv(parent, stripped, indent, prefix)
		return true
	}

	// Bullet list.
	if marker, after, ok := bulletListMarker(stripped); ok {
		bp.parseBulletList(parent, marker, after, indent, prefix)
		return true
	}

	// Ordered list.
	if start, style, after, ok := orderedListMarker(stripped); ok {
		bp.parseOrderedList(parent, start, style, after, indent, prefix)
		return true
	}

	// Paragraph (default).
	bp.parseParagraph(parent, prefix)
	return true
}

func (bp *blockParser) currentLine() blockLine {
	return bp.lines[bp.pos]
}

func (bp *blockParser) parseHeading(parent *Node, level int, stripped, prefix string) {
	// Content starts after the # markers and any whitespace.
	content := strings.TrimSpace(stripped[level:])

	node := &Node{Kind: Heading, Level: level}
	bp.attachPendingAttrs(node)

	bp.pos++

	// Continuation lines: any line that doesn't start a new block.
	var textBuf strings.Builder
	textBuf.WriteString(content)

	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				// Lazy continuation (no prefix).
				if isBlankLine(text) {
					break
				}
			}
		}
		if isBlankLine(text) {
			break
		}
		s := strings.TrimLeft(text, " \t")
		// If this is another block-level element, stop.
		if headingLevel(s) > 0 && headingLevel(s) != level {
			break
		}
		if isCodeFenceOpen(s) || isDivFenceOpen(s) || isThematicBreak(s) {
			break
		}
		if len(s) > 0 && s[0] == '>' && (len(s) == 1 || s[1] == ' ') {
			break
		}
		if len(s) > 0 && s[0] == '{' && strings.HasSuffix(strings.TrimRight(s, " \t"), "}") {
			break
		}
		// Same-level heading markers continue the heading.
		var line_content string
		if headingLevel(s) == level {
			line_content = strings.TrimSpace(s[level:])
		} else {
			line_content = strings.TrimRight(s, " \t")
		}
		if textBuf.Len() > 0 {
			textBuf.WriteByte('\n')
		}
		textBuf.WriteString(line_content)
		bp.pos++
	}

	node.Text = textBuf.String()
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseCodeBlock(parent *Node, stripped string, baseIndent int, prefix string) {
	fence := stripped
	fenceChar := fence[0]
	fenceLen := 0
	for i := 0; i < len(fence) && fence[i] == fenceChar; i++ {
		fenceLen++
	}

	lang := strings.TrimSpace(fence[fenceLen:])

	node := &Node{Kind: CodeBlock, Lang: lang}
	bp.attachPendingAttrs(node)

	bp.pos++

	var textBuf strings.Builder
	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				break
			}
		}

		s := strings.TrimLeft(text, " \t")
		// Check for closing fence.
		if isClosingCodeFence(s, fenceChar, fenceLen) {
			bp.pos++
			break
		}

		// Strip base indentation.
		content := stripIndent(text, baseIndent)
		textBuf.WriteString(content)
		textBuf.WriteByte('\n')
		bp.pos++
	}

	node.Text = textBuf.String()
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBlockQuote(parent *Node, indent int, outerPrefix string) {
	node := &Node{Kind: BlockQuote}
	bp.attachPendingAttrs(node)

	var contentLines []string

	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if outerPrefix != "" {
			if strings.HasPrefix(text, outerPrefix) {
				text = text[len(outerPrefix):]
			} else {
				break
			}
		}

		stripped := strings.TrimLeft(text, " \t")

		if isBlankLine(text) {
			// A blank line ends the block quote (unless it's "> " blank).
			break
		}

		if len(stripped) > 0 && stripped[0] == '>' {
			if len(stripped) == 1 {
				contentLines = append(contentLines, "")
			} else if stripped[1] == ' ' {
				contentLines = append(contentLines, stripped[2:])
			} else {
				// >text without space — not a blockquote continuation.
				break
			}
		} else {
			// Lazy continuation.
			contentLines = append(contentLines, stripped)
		}
		bp.pos++
	}

	// Parse the collected content as blocks.
	subInput := strings.Join(contentLines, "\n")
	subBP := newBlockParser(subInput)
	subBP.parseBlocks(node, 0, "")

	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseFencedDiv(parent *Node, stripped string, baseIndent int, prefix string) {
	fenceLen := 0
	for i := 0; i < len(stripped) && stripped[i] == ':'; i++ {
		fenceLen++
	}

	className := strings.TrimSpace(stripped[fenceLen:])

	node := &Node{Kind: Div}
	bp.attachPendingAttrs(node)
	if className != "" {
		node.AddClass(className)
	}

	bp.pos++

	// Collect inner content, parse as blocks.
	var contentLines []string
	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				break
			}
		}
		s := strings.TrimLeft(text, " \t")

		// Closing fence: at least fenceLen colons, nothing else.
		if isClosingDivFence(s, fenceLen) {
			bp.pos++
			break
		}

		contentLines = append(contentLines, text)
		bp.pos++
	}

	subInput := strings.Join(contentLines, "\n")
	subBP := newBlockParser(subInput)
	subBP.parseBlocks(node, 0, "")

	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBulletList(parent *Node, marker byte, afterMarker string, indent int, prefix string) {
	node := &Node{Kind: BulletList}
	bp.attachPendingAttrs(node)

	// Track tightness.
	tight := true

	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				break
			}
		}
		stripped := strings.TrimLeft(text, " \t")
		m, after, ok := bulletListMarker(stripped)
		if !ok || m != marker {
			break
		}

		item := &Node{Kind: ListItem}
		bp.pos++

		// Collect item content.
		var contentLines []string
		contentLines = append(contentLines, after)

		contentIndent := len(text) - len(stripped) + 2 // marker + space

		for bp.pos < len(bp.lines) {
			nextLine := bp.currentLine()
			nextText := nextLine.text
			if prefix != "" {
				if strings.HasPrefix(nextText, prefix) {
					nextText = nextText[len(prefix):]
				} else {
					break
				}
			}

			if isBlankLine(nextText) {
				// Check if next non-blank line is still indented.
				if bp.pos+1 < len(bp.lines) {
					peekText := bp.lines[bp.pos+1].text
					if prefix != "" && strings.HasPrefix(peekText, prefix) {
						peekText = peekText[len(prefix):]
					}
					peekIndent := countLeadingSpaces(peekText)
					if peekIndent >= contentIndent && !isBlankLine(peekText) {
						tight = false
						contentLines = append(contentLines, "")
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent >= contentIndent {
				contentLines = append(contentLines, nextText[contentIndent:])
				bp.pos++
			} else {
				// Check if it's a new list item at same level.
				ns := strings.TrimLeft(nextText, " \t")
				_, _, isItem := bulletListMarker(ns)
				if isItem {
					break
				}
				// Lazy continuation.
				contentLines = append(contentLines, strings.TrimLeft(nextText, " \t"))
				bp.pos++
			}
		}

		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		subBP.parseBlocks(item, 0, "")

		node.Children = append(node.Children, item)
	}

	// If tight, unwrap single paragraphs in items.
	if tight {
		node.SetAttr("tight", "true")
	}

	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseOrderedList(parent *Node, start int, style ListStyle, afterMarker string, indent int, prefix string) {
	node := &Node{Kind: OrderedList, ListStart: start, ListStyle: style}
	bp.attachPendingAttrs(node)

	tight := true

	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				break
			}
		}
		stripped := strings.TrimLeft(text, " \t")
		_, _, after, ok := orderedListMarker(stripped)
		if !ok {
			break
		}

		item := &Node{Kind: ListItem}
		bp.pos++

		var contentLines []string
		contentLines = append(contentLines, after)

		// Find the column where content starts.
		markerEnd := len(text) - len(stripped)
		for i := 0; i < len(stripped); i++ {
			if stripped[i] == '.' || stripped[i] == ')' {
				markerEnd += i + 2 // past marker + space
				break
			}
		}
		contentIndent := markerEnd

		for bp.pos < len(bp.lines) {
			nextLine := bp.currentLine()
			nextText := nextLine.text
			if prefix != "" {
				if strings.HasPrefix(nextText, prefix) {
					nextText = nextText[len(prefix):]
				} else {
					break
				}
			}

			if isBlankLine(nextText) {
				if bp.pos+1 < len(bp.lines) {
					peekText := bp.lines[bp.pos+1].text
					if prefix != "" && strings.HasPrefix(peekText, prefix) {
						peekText = peekText[len(prefix):]
					}
					peekIndent := countLeadingSpaces(peekText)
					if peekIndent >= contentIndent && !isBlankLine(peekText) {
						tight = false
						contentLines = append(contentLines, "")
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent >= contentIndent {
				contentLines = append(contentLines, nextText[contentIndent:])
				bp.pos++
			} else {
				ns := strings.TrimLeft(nextText, " \t")
				_, _, _, isItem := orderedListMarker(ns)
				if isItem {
					break
				}
				contentLines = append(contentLines, strings.TrimLeft(nextText, " \t"))
				bp.pos++
			}
		}

		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		subBP.parseBlocks(item, 0, "")

		node.Children = append(node.Children, item)
	}

	if tight {
		node.SetAttr("tight", "true")
	}

	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseParagraph(parent *Node, prefix string) {
	var textBuf strings.Builder

	for bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				break
			}
		}

		if isBlankLine(text) {
			break
		}

		stripped := strings.TrimLeft(text, " \t")

		// Stop if we see a block-level element that can interrupt a paragraph.
		if headingLevel(stripped) > 0 {
			break
		}
		if isCodeFenceOpen(stripped) {
			break
		}
		if len(stripped) > 0 && stripped[0] == '>' && (len(stripped) == 1 || stripped[1] == ' ') {
			break
		}
		if len(stripped) > 0 && stripped[0] == '{' && strings.HasSuffix(strings.TrimRight(stripped, " \t"), "}") {
			inner := strings.TrimRight(stripped, " \t")
			inner = inner[1 : len(inner)-1]
			if attrs := ParseAttrs(inner); attrs != nil {
				break
			}
		}

		// Thematic breaks and divs cannot interrupt paragraphs.
		// (They require a preceding blank line.)

		if textBuf.Len() > 0 {
			textBuf.WriteByte('\n')
		}
		textBuf.WriteString(text)
		bp.pos++
	}

	if textBuf.Len() > 0 {
		node := &Node{Kind: Paragraph, Text: strings.TrimRight(textBuf.String(), " \t")}
		bp.attachPendingAttrs(node)
		parent.Children = append(parent.Children, node)
	}
}

func (bp *blockParser) attachPendingAttrs(node *Node) {
	if bp.pendingAttrs != nil {
		for k, v := range bp.pendingAttrs {
			if k == "class" {
				node.AddClass(v)
			} else {
				node.SetAttr(k, v)
			}
		}
		bp.pendingAttrs = nil
	}
}

func (bp *blockParser) isPrecededByBlank(parent *Node) bool {
	if bp.pos == 0 {
		return true // start of document counts
	}
	prev := bp.lines[bp.pos-1]
	return isBlankLine(prev.text)
}

// Block detection helpers.

func isBlankLine(text string) bool {
	for _, c := range text {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}

func countLeadingSpaces(text string) int {
	n := 0
	for _, c := range text {
		if c == ' ' {
			n++
		} else if c == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}

func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	// Must be followed by space or end of line.
	if n < len(s) && s[n] != ' ' {
		return 0
	}
	return n
}

func isThematicBreak(s string) bool {
	if len(s) < 3 {
		return false
	}
	// Must be 3+ of the same character (* - _) optionally with spaces.
	var char byte
	count := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c == '*' || c == '-' {
			if char == 0 {
				char = c
			} else if c != char {
				return false
			}
			count++
		} else {
			return false
		}
	}
	return count >= 3
}

func isCodeFenceOpen(s string) bool {
	if len(s) < 3 {
		return false
	}
	char := s[0]
	if char != '`' && char != '~' {
		return false
	}
	n := 0
	for n < len(s) && s[n] == char {
		n++
	}
	if n < 3 {
		return false
	}
	// For backtick fences, the info string must not contain backticks.
	if char == '`' {
		rest := s[n:]
		// If the rest also ends with backticks and has content, it's inline code.
		if strings.ContainsRune(rest, '`') {
			return false
		}
	}
	return true
}

func isClosingCodeFence(s string, char byte, minLen int) bool {
	n := 0
	for n < len(s) && s[n] == char {
		n++
	}
	if n < minLen {
		return false
	}
	// Must be only fence chars and optional whitespace.
	rest := strings.TrimSpace(s[n:])
	return rest == ""
}

func isDivFenceOpen(s string) bool {
	n := 0
	for n < len(s) && s[n] == ':' {
		n++
	}
	return n >= 3
}

func isClosingDivFence(s string, minLen int) bool {
	n := 0
	for n < len(s) && s[n] == ':' {
		n++
	}
	if n < minLen {
		return false
	}
	rest := strings.TrimSpace(s[n:])
	return rest == ""
}

func bulletListMarker(s string) (marker byte, after string, ok bool) {
	if len(s) < 2 {
		return 0, "", false
	}
	if (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return s[0], s[2:], true
	}
	return 0, "", false
}

func orderedListMarker(s string) (start int, style ListStyle, after string, ok bool) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) {
		return 0, 0, "", false
	}
	if s[i] != '.' && s[i] != ')' {
		return 0, 0, "", false
	}
	if i+1 >= len(s) || s[i+1] != ' ' {
		return 0, 0, "", false
	}

	num := 0
	for _, c := range s[:i] {
		num = num*10 + int(c-'0')
	}

	return num, ListDecimal, s[i+2:], true
}

func stripIndent(text string, n int) string {
	stripped := 0
	for i := 0; i < len(text) && stripped < n; i++ {
		if text[i] == ' ' {
			stripped++
		} else if text[i] == '\t' {
			stripped += 4
		} else {
			return text[i:]
		}
		if stripped >= n {
			return text[i+1:]
		}
	}
	return text
}

func mergeAttrs(dst, src map[string]string) map[string]string {
	if dst == nil {
		return src
	}
	for k, v := range src {
		if k == "class" {
			if existing, ok := dst["class"]; ok {
				dst["class"] = existing + " " + v
			} else {
				dst["class"] = v
			}
		} else {
			dst[k] = v
		}
	}
	return dst
}
