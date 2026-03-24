package djot

import (
	"strings"
)

type blockParser struct {
	input            string
	lines            []blockLine
	pos              int
	pendingAttrs     map[string]string
	pendingAttrOrder []string
	references       map[string]*Node // label → ref node (for link reference definitions)
}

type blockLine struct {
	start  int    // byte offset in input
	end    int    // byte offset (exclusive, before newline)
	text   string // full line content (no newline)
	indent int    // count of leading spaces
	strip  string // content after stripping indent
}

func newBlockParser(input string) *blockParser {
	bp := &blockParser{input: input, references: make(map[string]*Node)}
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
	root.Start = Pos{Offset: 0}
	bp.parseBlocks(root, 0, "")
	if len(bp.lines) > 0 {
		root.End = Pos{Offset: bp.lines[len(bp.lines)-1].end}
	}
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
		// Discard pending block attributes — they can't attach across blank lines.
		bp.pendingAttrs = nil
		bp.pendingAttrOrder = nil
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
	if len(stripped) > 0 && stripped[0] == '{' {
		if attrContent, lines := bp.tryBlockAttr(stripped, prefix); attrContent != "" {
			inner := attrContent[1 : len(attrContent)-1] // strip { and }
			attrs, attrOrder := parseAttrsOrdered(inner)
			if attrs != nil {
				bp.pendingAttrs, bp.pendingAttrOrder = mergeAttrsOrdered(bp.pendingAttrs, bp.pendingAttrOrder, attrs, attrOrder)
				bp.pos += lines
				return true
			}
		}
	}

	// Thematic break: must be preceded by blank line or at start (or block attrs), and
	// consists of 3+ of the same char (*, -, or _) with optional spaces.
	if isThematicBreak(stripped) && (bp.isPrecededByBlank(parent) || bp.pendingAttrs != nil) {
		node := &Node{Kind: ThematicBreak}
		node.Start = Pos{Offset: line.start}
		node.End = Pos{Offset: line.end}
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

	// Bullet list (or task list).
	if marker, after, ok := bulletListMarker(stripped); ok {
		// Check for task list: "- [ ] " or "- [x] "
		if isTaskListItem(after) {
			bp.parseTaskList(parent, marker, indent, prefix)
			return true
		}
		bp.parseBulletList(parent, marker, after, indent, prefix)
		return true
	}

	// Ordered list.
	if start, style, after, ok := orderedListMarker(stripped); ok {
		bp.parseOrderedList(parent, start, style, after, indent, prefix)
		return true
	}

	// Reference definition: [label]: url
	if isReferenceDefinition(stripped) {
		bp.parseReferenceDefinition(parent, stripped, indent, prefix)
		return true
	}

	// Footnote definition: [^label]:
	if isFootnoteDefinition(stripped) {
		bp.parseFootnoteDefinition(parent, stripped, indent, prefix)
		return true
	}

	// Definition list.
	if isDefinitionListMarker(stripped) {
		bp.parseDefinitionList(parent, indent, prefix)
		return true
	}

	// Table.
	if isTableRow(stripped) && bp.isPrecededByBlank(parent) {
		bp.parseTable(parent, stripped, indent, prefix)
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

	startLine := bp.currentLine()
	node := &Node{Kind: Heading, Level: level}
	node.Start = Pos{Offset: startLine.start}
	bp.attachPendingAttrs(node)

	lastEnd := startLine.end
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
		lastEnd = line.end
		bp.pos++
	}

	node.End = Pos{Offset: lastEnd}
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
	openLine := bp.currentLine()

	// Check for raw block: ``` =html
	if len(lang) > 1 && lang[0] == '=' {
		format := lang[1:]
		node := &Node{Kind: RawBlock, Format: format}
		node.Start = Pos{Offset: openLine.start}
		// Don't attach pending attrs to raw blocks.
		bp.pendingAttrs = nil
		bp.pos++
		lastEnd := openLine.end
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
			if isClosingCodeFence(s, fenceChar, fenceLen) {
				lastEnd = line.end
				bp.pos++
				break
			}
			content := stripIndent(text, baseIndent)
			textBuf.WriteString(content)
			textBuf.WriteByte('\n')
			lastEnd = line.end
			bp.pos++
		}
		node.End = Pos{Offset: lastEnd}
		node.Text = textBuf.String()
		parent.Children = append(parent.Children, node)
		return
	}

	node := &Node{Kind: CodeBlock, Lang: lang}
	node.Start = Pos{Offset: openLine.start}
	bp.attachPendingAttrs(node)

	bp.pos++

	lastEnd := openLine.end
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
			lastEnd = line.end
			bp.pos++
			break
		}

		// Strip base indentation.
		content := stripIndent(text, baseIndent)
		textBuf.WriteString(content)
		textBuf.WriteByte('\n')
		lastEnd = line.end
		bp.pos++
	}

	node.End = Pos{Offset: lastEnd}
	node.Text = textBuf.String()
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBlockQuote(parent *Node, indent int, outerPrefix string) {
	node := &Node{Kind: BlockQuote}
	startLine := bp.currentLine()
	node.Start = Pos{Offset: startLine.start}
	bp.attachPendingAttrs(node)

	var contentLines []string
	lastEnd := startLine.start

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

		if len(stripped) > 0 && stripped[0] == '>' && (len(stripped) == 1 || stripped[1] == ' ') {
			if len(stripped) == 1 {
				contentLines = append(contentLines, "")
			} else {
				contentLines = append(contentLines, stripped[2:])
			}
		} else {
			// Lazy continuation.
			contentLines = append(contentLines, stripped)
		}
		lastEnd = line.end
		bp.pos++
	}

	// Parse the collected content as blocks.
	subInput := strings.Join(contentLines, "\n")
	subBP := newBlockParser(subInput)
	subBP.parseBlocks(node, 0, "")

	node.End = Pos{Offset: lastEnd}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseFencedDiv(parent *Node, stripped string, baseIndent int, prefix string) {
	fenceLen := 0
	for i := 0; i < len(stripped) && stripped[i] == ':'; i++ {
		fenceLen++
	}

	className := strings.TrimSpace(stripped[fenceLen:])

	openLine := bp.currentLine()
	node := &Node{Kind: Div}
	node.Start = Pos{Offset: openLine.start}
	bp.attachPendingAttrs(node)
	if className != "" {
		node.AddClass(className)
	}

	bp.pos++

	// Collect inner content, parse as blocks.
	// Track code fences so that ::: inside a code block doesn't close the div.
	var contentLines []string
	lastEnd := openLine.end
	inCodeFence := false
	codeFenceChar := byte(0)
	codeFenceLen := 0
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

		if inCodeFence {
			// Check for closing code fence.
			if isClosingCodeFence(s, codeFenceChar, codeFenceLen) {
				inCodeFence = false
			}
		} else {
			// Check for opening code fence.
			if isCodeFenceOpen(s) {
				inCodeFence = true
				codeFenceChar = s[0]
				codeFenceLen = 0
				for codeFenceLen < len(s) && s[codeFenceLen] == codeFenceChar {
					codeFenceLen++
				}
			} else if isClosingDivFence(s, fenceLen) {
				// Closing fence: at least fenceLen colons, nothing else.
				lastEnd = line.end
				bp.pos++
				break
			}
		}

		contentLines = append(contentLines, text)
		lastEnd = line.end
		bp.pos++
	}

	subInput := strings.Join(contentLines, "\n")
	subBP := newBlockParser(subInput)
	subBP.parseBlocks(node, 0, "")

	node.End = Pos{Offset: lastEnd}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBulletList(parent *Node, marker byte, afterMarker string, indent int, prefix string) {
	node := &Node{Kind: BulletList}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	hasBlankBetweenItems := false
	hasBlankWithinItem := false // blank within item followed by non-sublist content
	markerIndent := indent      // indent level of the list marker

	for bp.pos < len(bp.lines) {
		// Skip blank lines between items.
		blanksBefore := 0
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
			if !isBlankLine(text) {
				break
			}
			blanksBefore++
			bp.pos++
		}

		if bp.pos >= len(bp.lines) {
			break
		}

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
		itemIndent := len(text) - len(stripped)
		m, after, ok := bulletListMarker(stripped)
		if !ok || m != marker || itemIndent != markerIndent {
			// Put back the blank lines we consumed.
			bp.pos -= blanksBefore
			break
		}

		if blanksBefore > 0 && len(node.Children) > 0 {
			hasBlankBetweenItems = true
		}

		item := &Node{Kind: ListItem}
		item.Start = Pos{Offset: line.start}
		bp.pos++

		var contentLines []string
		// Strip all continuation lines by stripAmount (markerIndent + 1),
		// which preserves relative indentation for sublists at varying depths.
		// Prepend padding to `after` so it aligns with content at contentIndent.
		stripAmount := itemIndent + 1
		contentIndent := itemIndent + 2 // marker + space
		padding := strings.Repeat(" ", contentIndent-stripAmount)
		contentLines = append(contentLines, padding+after)

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
				// Check if next non-blank line is still indented (continuation).
				if bp.pos+1 < len(bp.lines) {
					peekText := bp.lines[bp.pos+1].text
					if prefix != "" && strings.HasPrefix(peekText, prefix) {
						peekText = peekText[len(prefix):]
					}
					peekIndent := countLeadingSpaces(peekText)
					// Continue item if next line is indented beyond marker start.
					if peekIndent > markerIndent && !isBlankLine(peekText) {
						// Check if the next line starts a sublist — if so,
						// the blank line doesn't make the parent list loose.
						peekStripped := strings.TrimLeft(peekText, " \t")
						_, _, isBullet := bulletListMarker(peekStripped)
						_, _, _, isOrd := orderedListMarker(peekStripped)
						if !isBullet && !isOrd {
							hasBlankWithinItem = true
						}
						contentLines = append(contentLines, "")
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent > markerIndent {
				contentLines = append(contentLines, stripIndent(nextText, stripAmount))
				bp.pos++
			} else {
				// Check if it's a new list item at the SAME indent.
				ns := strings.TrimLeft(nextText, " \t")
				ni := len(nextText) - len(ns)
				_, _, isItem := bulletListMarker(ns)
				if isItem && ni == markerIndent {
					break
				}
				_, _, _, isOrdItem := orderedListMarker(ns)
				if isOrdItem && ni == markerIndent {
					break
				}
				// Not a same-level item. Could be lazy continuation
				// only if it doesn't look like a block element.
				if headingLevel(ns) > 0 || isCodeFenceOpen(ns) {
					break
				}
				contentLines = append(contentLines, strings.TrimLeft(nextText, " \t"))
				bp.pos++
			}
		}

		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		subBP.parseBlocks(item, 0, "")

		// Set item end to last consumed line.
		if bp.pos > 0 {
			item.End = Pos{Offset: bp.lines[bp.pos-1].end}
		}
		node.Children = append(node.Children, item)
	}

	// Determine tight/loose:
	// A list is loose if any item has multiple paragraph children (blank within item),
	// or if there are blank lines between items and NO item contains block-level
	// children like sublists (meaning all items are simple text).
	tight := true
	if hasBlankWithinItem {
		tight = false
	}
	if hasBlankBetweenItems && tight {
		// Check if any item has non-paragraph block children (sublists, etc.).
		anyBlockChildren := false
		for _, child := range node.Children {
			for _, gc := range child.Children {
				if gc.Kind != Paragraph {
					anyBlockChildren = true
					break
				}
			}
			if anyBlockChildren {
				break
			}
		}
		if !anyBlockChildren {
			tight = false
		}
	}

	if tight {
		node.SetAttr("tight", "true")
	}

	// Set list end from last item.
	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseOrderedList(parent *Node, start int, style ListStyle, afterMarker string, indent int, prefix string) {
	// Extract the first item's enum text and delimiter for potential reinterpretation.
	firstLine := bp.lines[bp.pos].text
	if prefix != "" && strings.HasPrefix(firstLine, prefix) {
		firstLine = firstLine[len(prefix):]
	}
	firstStripped := strings.TrimLeft(firstLine, " \t")
	firstEnum, firstDelim, _ := extractOrderedMarkerParts(firstStripped)

	node := &Node{Kind: OrderedList, ListStart: start, ListStyle: style}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	hasBlankBetweenItems := false
	hasBlankWithinItem := false
	markerIndent := indent

	for bp.pos < len(bp.lines) {
		// Skip blank lines between items.
		blanksBefore := 0
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
			if !isBlankLine(text) {
				break
			}
			blanksBefore++
			bp.pos++
		}

		if bp.pos >= len(bp.lines) {
			break
		}

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
		itemIndent := len(text) - len(stripped)
		_, itemStyle, after, ok := orderedListMarker(stripped)
		if !ok || itemIndent != markerIndent {
			bp.pos -= blanksBefore
			break
		}
		// Check delimiter type matches.
		_, itemDelim, _ := extractOrderedMarkerParts(stripped)
		if itemDelim != firstDelim {
			bp.pos -= blanksBefore
			break
		}
		if itemStyle != style {
			// Try to reinterpret: if this is the second item and the first
			// item's enumerator can be parsed as the new style, switch.
			if len(node.Children) == 1 {
				if newNum, ok2 := parseOrderedEnumAs(firstEnum, itemStyle); ok2 {
					style = itemStyle
					node.ListStyle = style
					node.ListStart = newNum
				} else {
					bp.pos -= blanksBefore
					break
				}
			} else {
				bp.pos -= blanksBefore
				break
			}
		}

		if blanksBefore > 0 && len(node.Children) > 0 {
			hasBlankBetweenItems = true
		}

		item := &Node{Kind: ListItem}
		item.Start = Pos{Offset: line.start}
		bp.pos++

		var contentLines []string

		// Find the column where content starts.
		markerEnd := itemIndent
		for i := 0; i < len(stripped); i++ {
			if stripped[i] == '.' || stripped[i] == ')' {
				markerEnd += i + 2 // past marker + space
				break
			}
		}
		contentIndent := markerEnd

		// Strip all continuation lines by stripAmount to preserve relative indentation.
		stripAmount := itemIndent + 1
		padding := strings.Repeat(" ", contentIndent-stripAmount)
		contentLines = append(contentLines, padding+after)

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
					if peekIndent > markerIndent && !isBlankLine(peekText) {
						// Check if the next line starts a sublist — if so,
						// the blank line doesn't make the parent list loose.
						peekStripped := strings.TrimLeft(peekText, " \t")
						_, _, isBullet := bulletListMarker(peekStripped)
						_, _, _, isOrd := orderedListMarker(peekStripped)
						if !isBullet && !isOrd {
							hasBlankWithinItem = true
						}
						contentLines = append(contentLines, "")
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent > markerIndent {
				contentLines = append(contentLines, stripIndent(nextText, stripAmount))
				bp.pos++
			} else {
				ns := strings.TrimLeft(nextText, " \t")
				ni := len(nextText) - len(ns)
				_, _, _, isItem := orderedListMarker(ns)
				if isItem && ni == markerIndent {
					break
				}
				_, _, isBulletItem := bulletListMarker(ns)
				if isBulletItem && ni == markerIndent {
					break
				}
				if headingLevel(ns) > 0 || isCodeFenceOpen(ns) {
					break
				}
				contentLines = append(contentLines, strings.TrimLeft(nextText, " \t"))
				bp.pos++
			}
		}

		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		subBP.parseBlocks(item, 0, "")

		if bp.pos > 0 {
			item.End = Pos{Offset: bp.lines[bp.pos-1].end}
		}
		node.Children = append(node.Children, item)
	}

	// Determine tight/loose using same rules as bullet lists.
	tight := true
	if hasBlankWithinItem {
		tight = false
	}
	if hasBlankBetweenItems && tight {
		anyBlockChildren := false
		for _, child := range node.Children {
			for _, gc := range child.Children {
				if gc.Kind != Paragraph {
					anyBlockChildren = true
					break
				}
			}
			if anyBlockChildren {
				break
			}
		}
		if !anyBlockChildren {
			tight = false
		}
	}

	if tight {
		node.SetAttr("tight", "true")
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseParagraph(parent *Node, prefix string) {
	var textBuf strings.Builder
	openBraces := 0
	startOffset := bp.currentLine().start
	lastEnd := bp.currentLine().end

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

		// Only break for block-level elements if we don't have unclosed braces.
		// Unclosed { means we're inside an inline attribute that spans lines,
		// so the next line is a continuation, not a new block.
		if openBraces == 0 {
			// Stop if we see a block-level element that can interrupt a paragraph.
			if headingLevel(stripped) > 0 {
				break
			}
			// Note: code fences do NOT interrupt paragraphs in djot.
		}

		// Note: {.class} lines do NOT break paragraphs. Block attributes only
		// apply before a block, not within a paragraph.

		// Thematic breaks and divs cannot interrupt paragraphs.
		// (They require a preceding blank line.)

		if textBuf.Len() > 0 {
			textBuf.WriteByte('\n')
		}
		textBuf.WriteString(strings.TrimLeft(text, " \t"))
		// Track unclosed braces (respecting quotes/escapes).
		openBraces = countOpenBraces(textBuf.String())
		lastEnd = line.end
		bp.pos++
	}

	if textBuf.Len() > 0 {
		node := &Node{Kind: Paragraph, Text: strings.TrimRight(textBuf.String(), " \t")}
		node.Start = Pos{Offset: startOffset}
		node.End = Pos{Offset: lastEnd}
		bp.attachPendingAttrs(node)
		parent.Children = append(parent.Children, node)
	}
}

// countOpenBraces counts the number of unclosed { in a string,
// respecting quoted strings and backslash escapes.
func countOpenBraces(s string) int {
	depth := 0
	inQuote := byte(0)
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
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
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func (bp *blockParser) attachPendingAttrs(node *Node) {
	if bp.pendingAttrs != nil {
		// Apply in order.
		for _, k := range bp.pendingAttrOrder {
			v := bp.pendingAttrs[k]
			if k == "class" {
				node.AddClass(v)
			} else {
				node.SetAttr(k, v)
			}
		}
		bp.pendingAttrs = nil
		bp.pendingAttrOrder = nil
	}
}

func (bp *blockParser) isPrecededByBlank(parent *Node) bool {
	if bp.pos == 0 {
		return true // start of document counts
	}
	prev := bp.lines[bp.pos-1]
	return isBlankLine(prev.text)
}

// tryBlockAttr checks if the current line starts a block-level attribute.
// Returns the full attribute content (including braces) and the number of lines consumed.
// If not a valid attribute block, returns ("", 0).
func (bp *blockParser) tryBlockAttr(stripped, prefix string) (string, int) {
	trimmed := strings.TrimRight(stripped, " \t")
	// Single-line case: {attrs}
	if strings.HasSuffix(trimmed, "}") {
		return trimmed, 1
	}

	// Multi-line case: { starts an attr block, continuation lines must be indented.
	var buf strings.Builder
	buf.WriteString(stripped)
	lines := 1
	for i := bp.pos + 1; i < len(bp.lines); i++ {
		line := bp.lines[i]
		text := line.text
		if prefix != "" {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
			} else {
				return "", 0
			}
		}
		contStripped := strings.TrimLeft(text, " \t")
		contIndent := len(text) - len(contStripped)
		// Continuation lines must be indented (at least 1 space).
		if contIndent == 0 || isBlankLine(text) {
			return "", 0
		}
		buf.WriteByte(' ')
		buf.WriteString(strings.TrimSpace(text))
		lines++
		trimmedBuf := strings.TrimRight(buf.String(), " \t")
		if strings.HasSuffix(trimmedBuf, "}") {
			return trimmedBuf, lines
		}
	}

	return "", 0
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
	// Must be 3+ of * or - (any mix) optionally with spaces.
	count := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c == '*' || c == '-' {
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
	rest := strings.TrimSpace(s[n:])
	if char == '`' {
		// For backtick fences, the info string must not contain backticks.
		if strings.ContainsRune(rest, '`') {
			return false
		}
		// The info string (language) must be a single word (no spaces).
		// If it contains spaces, this is inline code, not a code fence.
		if strings.ContainsRune(rest, ' ') {
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
	// Try parenthesized form: (num), (a), (A), (i), (I)
	if len(s) > 0 && s[0] == '(' {
		closeParen := strings.IndexByte(s, ')')
		if closeParen > 1 && closeParen+1 < len(s) && s[closeParen+1] == ' ' {
			inner := s[1:closeParen]
			if num, sty, ok2 := parseOrderedEnum(inner); ok2 {
				return num, sty, s[closeParen+2:], true
			}
		}
		return 0, 0, "", false
	}

	// Try suffix form: num. num) a. a) i. i) etc.
	i := 0
	for i < len(s) && !isSuffixDelim(s[i]) {
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

	enum := s[:i]
	num, sty, ok2 := parseOrderedEnum(enum)
	if !ok2 {
		return 0, 0, "", false
	}
	return num, sty, s[i+2:], true
}

func isSuffixDelim(c byte) bool {
	return c == '.' || c == ')'
}

// orderedDelim describes the delimiter format for ordered lists.
type orderedDelim int

const (
	delimDot   orderedDelim = iota // "1."
	delimParen                     // "1)"
	delimWrap                      // "(1)"
)

// extractOrderedMarkerParts returns the raw enum string and delimiter type from a stripped line.
func extractOrderedMarkerParts(s string) (enum string, delim orderedDelim, ok bool) {
	if len(s) > 0 && s[0] == '(' {
		closeParen := strings.IndexByte(s, ')')
		if closeParen > 1 && closeParen+1 < len(s) && s[closeParen+1] == ' ' {
			return s[1:closeParen], delimWrap, true
		}
		return "", 0, false
	}
	i := 0
	for i < len(s) && !isSuffixDelim(s[i]) {
		i++
	}
	if i == 0 || i >= len(s) {
		return "", 0, false
	}
	if s[i] == '.' {
		return s[:i], delimDot, true
	}
	if s[i] == ')' {
		return s[:i], delimParen, true
	}
	return "", 0, false
}

// parseOrderedEnumAs tries to parse an enum string as a specific style.
func parseOrderedEnumAs(s string, style ListStyle) (int, bool) {
	switch style {
	case ListDecimal:
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, false
			}
		}
		n := 0
		for _, c := range s {
			n = n*10 + int(c-'0')
		}
		return n, true
	case ListAlphaLower:
		if len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
			return int(s[0]-'a') + 1, true
		}
		return 0, false
	case ListAlphaUpper:
		if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
			return int(s[0]-'A') + 1, true
		}
		return 0, false
	case ListRomanLower:
		if isAllLower(s) {
			return parseRoman(strings.ToUpper(s))
		}
		return 0, false
	case ListRomanUpper:
		if isAllUpper(s) {
			return parseRoman(s)
		}
		return 0, false
	}
	return 0, false
}

// parseOrderedEnum parses an ordered list enumerator string (without delimiters)
// and returns the numeric value and style. It handles decimal, lower/upper alpha,
// and lower/upper roman numerals. When ambiguous (e.g. "i" could be alpha or roman),
// roman numerals are preferred.
func parseOrderedEnum(s string) (num int, style ListStyle, ok bool) {
	if len(s) == 0 {
		return 0, 0, false
	}

	// Decimal
	if s[0] >= '0' && s[0] <= '9' {
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, 0, false
			}
		}
		n := 0
		for _, c := range s {
			n = n*10 + int(c-'0')
		}
		return n, ListDecimal, true
	}

	// Lower letters: try roman first, then alpha
	if isAllLower(s) {
		if rn, ok2 := parseRoman(strings.ToUpper(s)); ok2 {
			return rn, ListRomanLower, true
		}
		if len(s) == 1 {
			return int(s[0]-'a') + 1, ListAlphaLower, true
		}
		return 0, 0, false
	}

	// Upper letters: try roman first, then alpha
	if isAllUpper(s) {
		if rn, ok2 := parseRoman(s); ok2 {
			return rn, ListRomanUpper, true
		}
		if len(s) == 1 {
			return int(s[0]-'A') + 1, ListAlphaUpper, true
		}
		return 0, 0, false
	}

	return 0, 0, false
}

func isAllLower(s string) bool {
	for _, c := range s {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return len(s) > 0
}

func isAllUpper(s string) bool {
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

// parseRoman parses an uppercase Roman numeral string and returns its value.
func parseRoman(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	romanValues := map[byte]int{
		'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000,
	}
	total := 0
	prev := 0
	for i := len(s) - 1; i >= 0; i-- {
		val, exists := romanValues[s[i]]
		if !exists {
			return 0, false
		}
		if val < prev {
			total -= val
		} else {
			total += val
		}
		prev = val
	}
	if total <= 0 {
		return 0, false
	}
	// Validate by checking that the roman numeral round-trips
	if toRoman(total) != s {
		return 0, false
	}
	return total, true
}

// toRoman converts an integer to an uppercase Roman numeral string.
func toRoman(n int) string {
	if n <= 0 {
		return ""
	}
	type pair struct {
		value  int
		symbol string
	}
	pairs := []pair{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}
	var buf strings.Builder
	for _, p := range pairs {
		for n >= p.value {
			buf.WriteString(p.symbol)
			n -= p.value
		}
	}
	return buf.String()
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

func isReferenceDefinition(s string) bool {
	if len(s) < 4 || s[0] != '[' {
		return false
	}
	// Must not be a footnote definition [^...]
	if len(s) > 1 && s[1] == '^' {
		return false
	}
	closeBracket := strings.IndexByte(s, ']')
	if closeBracket < 2 {
		return false
	}
	if closeBracket+1 >= len(s) || s[closeBracket+1] != ':' {
		return false
	}
	return true
}

func (bp *blockParser) parseReferenceDefinition(parent *Node, stripped string, indent int, prefix string) {
	closeBracket := strings.IndexByte(stripped, ']')
	label := stripped[1:closeBracket]

	after := ""
	if closeBracket+2 < len(stripped) {
		after = strings.TrimSpace(stripped[closeBracket+2:])
	}

	bp.pos++

	// URL can continue on following lines if they start with whitespace
	var urlParts []string
	if after != "" {
		urlParts = append(urlParts, after)
	}
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
			break
		}
		// Continuation lines must start with whitespace
		if nextText[0] != ' ' && nextText[0] != '\t' {
			break
		}
		trimmed := strings.TrimSpace(nextText)
		// If continuation looks like another ref def, stop
		if isReferenceDefinition(trimmed) || isFootnoteDefinition(trimmed) {
			break
		}
		urlParts = append(urlParts, trimmed)
		bp.pos++
	}

	url := strings.Join(urlParts, "")
	ref := &Node{Kind: Link, Target: url, Label: label}
	bp.attachPendingAttrs(ref)
	bp.references[label] = ref
}

func isFootnoteDefinition(s string) bool {
	if len(s) < 5 || s[0] != '[' || s[1] != '^' {
		return false
	}
	closeBracket := strings.IndexByte(s, ']')
	if closeBracket < 3 {
		return false
	}
	if closeBracket+1 >= len(s) || s[closeBracket+1] != ':' {
		return false
	}
	return true
}

func (bp *blockParser) parseFootnoteDefinition(parent *Node, stripped string, indent int, prefix string) {
	closeBracket := strings.IndexByte(stripped, ']')
	label := stripped[2:closeBracket]

	after := ""
	if closeBracket+2 < len(stripped) {
		after = strings.TrimSpace(stripped[closeBracket+2:])
	}

	node := &Node{Kind: Footnote, Label: label}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.pos++

	var contentLines []string
	if after != "" {
		contentLines = append(contentLines, after)
	}

	// Footnote continuation lines must be indented. Use a fixed indent
	// of 2 spaces (like list item continuation in djot).
	contentIndent := indent + 2

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
			break
		}
	}

	if len(contentLines) > 0 {
		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		subBP.parseBlocks(node, 0, "")
	}

	if bp.pos > 0 {
		node.End = Pos{Offset: bp.lines[bp.pos-1].end}
	}
	parent.Children = append(parent.Children, node)
}

func isTaskListItem(after string) bool {
	return (strings.HasPrefix(after, "[ ] ") || strings.HasPrefix(after, "[x] ") ||
		strings.HasPrefix(after, "[X] "))
}

func (bp *blockParser) parseTaskList(parent *Node, marker byte, indent int, prefix string) {
	node := &Node{Kind: TaskList}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	tight := true
	markerIndent := indent

	for bp.pos < len(bp.lines) {
		// Skip blank lines between items (they make the list loose).
		blanksBefore := 0
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
			if !isBlankLine(text) {
				break
			}
			blanksBefore++
			bp.pos++
		}

		if bp.pos >= len(bp.lines) {
			break
		}

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
		itemIndent := len(text) - len(stripped)
		m, after, ok := bulletListMarker(stripped)
		if !ok || m != marker || !isTaskListItem(after) || itemIndent != markerIndent {
			bp.pos -= blanksBefore
			break
		}

		if blanksBefore > 0 && len(node.Children) > 0 {
			tight = false
		}

		checked := after[1] == 'x' || after[1] == 'X'
		afterCheckbox := after[4:] // skip "[ ] " or "[x] "

		item := &Node{Kind: TaskListItem, Checked: checked}
		item.Start = Pos{Offset: line.start}
		bp.pos++

		var contentLines []string
		contentLines = append(contentLines, afterCheckbox)

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
				_, _, isItem := bulletListMarker(ns)
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

		if bp.pos > 0 {
			item.End = Pos{Offset: bp.lines[bp.pos-1].end}
		}
		node.Children = append(node.Children, item)
	}

	if tight {
		node.SetAttr("tight", "true")
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func isDefinitionListMarker(s string) bool {
	// Definition list item starts with ": " (colon + space)
	return len(s) >= 2 && s[0] == ':' && s[1] == ' '
}

func isTableRow(s string) bool {
	if len(s) == 0 || s[0] != '|' {
		return false
	}
	// Count unescaped, un-backticked pipe characters.
	// A valid table row needs at least 2 (the leading | plus one cell separator).
	pipes := 0
	escaped := false
	inBacktick := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '`' {
			inBacktick = !inBacktick
			continue
		}
		if c == '|' && !inBacktick {
			pipes++
			if pipes >= 2 {
				return true
			}
		}
	}
	return false
}

func (bp *blockParser) parseDefinitionList(parent *Node, indent int, prefix string) {
	node := &Node{Kind: DefinitionList}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	tight := true
	markerIndent := indent

	for bp.pos < len(bp.lines) {
		// Skip blank lines between items (they make the list loose).
		blanksBefore := 0
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
			if !isBlankLine(text) {
				break
			}
			blanksBefore++
			bp.pos++
		}

		if bp.pos >= len(bp.lines) {
			break
		}

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
		itemIndent := len(text) - len(stripped)
		if !isDefinitionListMarker(stripped) || itemIndent != markerIndent {
			bp.pos -= blanksBefore
			break
		}

		if blanksBefore > 0 && len(node.Children) > 0 {
			tight = false
		}

		// Collect item content lines (like a bullet list item).
		afterMarker := stripped[2:]
		itemStartOffset := line.start
		bp.pos++

		var contentLines []string
		contentLines = append(contentLines, afterMarker)

		contentIndent := itemIndent + 2 // marker + space

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
				ni := len(nextText) - len(ns)
				if isDefinitionListMarker(ns) && ni == markerIndent {
					break
				}
				// Lazy continuation (indented beyond marker but less than content).
				if nextIndent > markerIndent {
					contentLines = append(contentLines, nextText[markerIndent+1:])
					bp.pos++
				} else {
					break
				}
			}
		}

		// Parse collected content as blocks.
		subInput := strings.Join(contentLines, "\n")
		subBP := newBlockParser(subInput)
		itemNode := &Node{Kind: Document} // temporary container
		subBP.parseBlocks(itemNode, 0, "")

		// Split: first paragraph is the term, rest is the definition.
		itemEndOffset := itemStartOffset
		if bp.pos > 0 {
			itemEndOffset = bp.lines[bp.pos-1].end
		}
		term := &Node{Kind: Term}
		term.Start = Pos{Offset: itemStartOffset}
		def := &Node{Kind: Definition}

		if len(itemNode.Children) > 0 && itemNode.Children[0].Kind == Paragraph {
			// The first paragraph's text becomes the term.
			term.Text = itemNode.Children[0].Text
			term.Children = itemNode.Children[0].Children
			term.Attrs = itemNode.Children[0].Attrs
			term.End = term.Start // term is first line
			def.Start = term.Start
			for _, rest := range itemNode.Children[1:] {
				def.Children = append(def.Children, rest)
			}
		} else {
			// No paragraph — empty term, everything is definition.
			term.End = term.Start
			def.Start = Pos{Offset: itemStartOffset}
			def.Children = itemNode.Children
		}
		def.End = Pos{Offset: itemEndOffset}

		node.Children = append(node.Children, term, def)
	}

	if tight {
		node.SetAttr("tight", "true")
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseTable(parent *Node, stripped string, indent int, prefix string) {
	node := &Node{Kind: Table}
	node.Start = Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	// Track current alignment from the most recent separator row.
	var currentAligns []CellAlign

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
		if !isTableRow(stripped) {
			break
		}

		// Check if this is a separator row (alignment indicators).
		if isTableSeparator(stripped) {
			// Apply alignment to all cells in the previous row (mark as header).
			aligns := parseTableAlignments(stripped)
			if len(node.Children) > 0 {
				lastRow := node.Children[len(node.Children)-1]
				if lastRow.Kind == TableRow {
					for i, cell := range lastRow.Children {
						cell.IsHeader = true
						if i < len(aligns) {
							cell.CellAlign = aligns[i]
						}
					}
				}
			}
			currentAligns = aligns
			bp.pos++
			continue
		}

		row := parseTableRow(stripped)
		row.Start = Pos{Offset: line.start}
		row.End = Pos{Offset: line.end}
		// Apply current alignment to data row cells.
		if currentAligns != nil {
			for i, cell := range row.Children {
				if i < len(currentAligns) {
					cell.CellAlign = currentAligns[i]
				}
			}
		}
		node.Children = append(node.Children, row)
		bp.pos++
	}

	// Check for caption: skip blank lines, then look for ^ prefix.
	savedPos := bp.pos
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
			bp.pos++
			continue
		}
		break
	}

	// Check if the next non-blank line starts with "^ "
	if bp.pos < len(bp.lines) {
		line := bp.currentLine()
		text := line.text
		if prefix != "" && strings.HasPrefix(text, prefix) {
			text = text[len(prefix):]
		}
		trimmed := strings.TrimLeft(text, " \t")
		if len(trimmed) >= 2 && trimmed[0] == '^' && trimmed[1] == ' ' {
			// Collect caption lines: first line after "^ ", then continuation lines.
			captionStart := line.start
			var captionLines []string
			captionLines = append(captionLines, trimmed[2:])
			captionLastEnd := line.end
			bp.pos++
			for bp.pos < len(bp.lines) {
				cLine := bp.currentLine()
				cText := cLine.text
				if prefix != "" {
					if strings.HasPrefix(cText, prefix) {
						cText = cText[len(prefix):]
					} else {
						break
					}
				}
				if isBlankLine(cText) {
					break
				}
				cTrimmed := strings.TrimLeft(cText, " \t")
				// A new caption line starting with ^ replaces the current caption.
				if len(cTrimmed) >= 2 && cTrimmed[0] == '^' && cTrimmed[1] == ' ' {
					captionStart = cLine.start
					captionLines = []string{cTrimmed[2:]}
					captionLastEnd = cLine.end
					bp.pos++
					continue
				}
				captionLines = append(captionLines, cTrimmed)
				captionLastEnd = cLine.end
				bp.pos++
			}
			captionText := strings.Join(captionLines, "\n")
			captionNode := &Node{Kind: Caption, Text: captionText}
			captionNode.Start = Pos{Offset: captionStart}
			captionNode.End = Pos{Offset: captionLastEnd}
			// Prepend caption as first child of table.
			node.Children = append([]*Node{captionNode}, node.Children...)
		}
	}

	// If no caption was found, restore position (blank lines consumed should stay consumed
	// only if caption was found). Actually, blank lines between table and non-caption
	// content are fine to consume — but let's be safe.
	if len(node.Children) == 0 || node.Children[0].Kind != Caption {
		bp.pos = savedPos
		// Re-skip blank lines (they are normally consumed by the main loop).
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
				bp.pos++
				continue
			}
			break
		}
	}

	// Set table end from last child (caption or last row).
	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func isTableSeparator(s string) bool {
	// A separator row: |---|---|, possibly with : for alignment
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '|' {
		return false
	}
	for _, c := range s {
		if c != '|' && c != '-' && c != ':' && c != ' ' && c != '\t' {
			return false
		}
	}
	// Must have at least one -
	return strings.ContainsRune(s, '-')
}

func parseTableAlignments(s string) []CellAlign {
	s = strings.TrimSpace(s)
	if len(s) > 0 && s[0] == '|' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '|' {
		s = s[:len(s)-1]
	}
	parts := strings.Split(s, "|")
	var aligns []CellAlign
	for _, part := range parts {
		part = strings.TrimSpace(part)
		left := len(part) > 0 && part[0] == ':'
		right := len(part) > 0 && part[len(part)-1] == ':'
		switch {
		case left && right:
			aligns = append(aligns, AlignCenter)
		case left:
			aligns = append(aligns, AlignLeft)
		case right:
			aligns = append(aligns, AlignRight)
		default:
			aligns = append(aligns, AlignDefault)
		}
	}
	return aligns
}

func parseTableRow(s string) *Node {
	row := &Node{Kind: TableRow}
	s = strings.TrimSpace(s)
	if len(s) > 0 && s[0] == '|' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '|' {
		s = s[:len(s)-1]
	}

	cells := splitTableCells(s)
	for _, cellText := range cells {
		cell := &Node{Kind: TableCell, Text: strings.TrimSpace(cellText)}
		row.Children = append(row.Children, cell)
	}
	return row
}

func splitTableCells(s string) []string {
	var cells []string
	var current strings.Builder
	escaped := false
	inBacktick := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			current.WriteByte(c)
			continue
		}
		if c == '`' {
			inBacktick = !inBacktick
			current.WriteByte(c)
			continue
		}
		if c == '|' && !inBacktick {
			cells = append(cells, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	cells = append(cells, current.String())
	return cells
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

func mergeAttrsOrdered(dst map[string]string, dstOrder []string, src map[string]string, srcOrder []string) (map[string]string, []string) {
	if dst == nil {
		return src, srcOrder
	}
	for _, k := range srcOrder {
		v := src[k]
		if k == "class" {
			if existing, ok := dst["class"]; ok {
				dst["class"] = existing + " " + v
			} else {
				dstOrder = append(dstOrder, "class")
				dst["class"] = v
			}
		} else {
			if _, exists := dst[k]; !exists {
				dstOrder = append(dstOrder, k)
			}
			dst[k] = v
		}
	}
	return dst, dstOrder
}
