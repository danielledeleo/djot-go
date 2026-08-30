package djot

import (
	"math"
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

type blockParser struct {
	input            string
	lines            []blockLine
	pos              int
	pendingAttrs     map[string]string
	pendingAttrOrder []string
	references       map[string]*parseNode // label → ref node (for link reference definitions)
	arena            *parseNodeArena       // shared node allocator for the whole document
}

type blockLine struct {
	start  int    // byte offset in input
	end    int    // byte offset (exclusive, before newline)
	text   string // full line content (no newline)
	indent int    // count of leading spaces
}

func newBlockParser(input string) *blockParser {
	bp := &blockParser{input: input, references: make(map[string]*parseNode), arena: &parseNodeArena{}}
	bp.splitLines()
	return bp
}

type contentLines []contentLine

type contentLine struct {
	text  string
	start int // byte offset in original source where content begins
	end   int // byte offset in original source where the line ends
}

func (cl *contentLines) add(text string, srcStart, srcEnd int) {
	*cl = append(*cl, contentLine{text: text, start: srcStart, end: srcEnd})
}

func (cl *contentLines) addBlank(srcStart, srcEnd int) {
	cl.add("", srcStart, srcEnd)
}

func (cl contentLines) subParser(refs map[string]*parseNode, arena *parseNodeArena) *blockParser {
	lines := make([]blockLine, len(cl))
	for i, line := range cl {
		lines[i] = blockLine{
			start:  line.start,
			end:    line.end,
			text:   line.text,
			indent: countLeadingSpaces(line.text),
		}
	}
	return &blockParser{lines: lines, references: refs, arena: arena}
}

func (bp *blockParser) splitLines() {
	n := strings.Count(bp.input, "\n") + 1
	bp.lines = make([]blockLine, 0, n)
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
		})
		if end == -1 {
			break
		}
		offset = lineEnd + 1
	}
}

func (bp *blockParser) parse() *parseNode {
	root := bp.arena.new(parseNodeSpec{Kind: ast.KindDocument})
	root.Start = ast.Pos{Offset: 0}
	bp.parseBlocks(root, 0, "")
	if len(bp.lines) > 0 {
		root.End = ast.Pos{Offset: bp.lines[len(bp.lines)-1].end}
	}
	return root
}

// parseBlocks parses block-level elements and appends them to parent.
// baseIndent is the minimum indentation for content in this container.
// prefix is the string prefix to strip (e.g., "> " for blockquotes).
func (bp *blockParser) parseBlocks(parent *parseNode, baseIndent int, prefix string) {
	for bp.pos < len(bp.lines) {
		if bp.parseBlock(parent, baseIndent, prefix) {
			continue
		}
		break
	}
}

func (bp *blockParser) parseBlock(parent *parseNode, baseIndent int, prefix string) bool {
	if bp.pos >= len(bp.lines) {
		return false
	}

	line := bp.currentLine()

	if isBlankLine(line.text) {
		bp.pendingAttrs = nil
		bp.pendingAttrOrder = nil
		bp.pos++
		return true
	}

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

	// literalLines carries a failed attempt down to parseParagraph: djot.js
	// re-emits the text it consumed as literal inline content, so a "{" line
	// that isn't a valid attribute block never gets a second reading as an
	// inline attribute or comment.
	literalLines := 0
	if len(stripped) > 0 && stripped[0] == '{' {
		attrContent, lines := bp.tryBlockAttr(stripped, prefix)
		if attrContent != "" {
			inner := attrContent[1 : len(attrContent)-1] // strip { and }
			attrs, attrOrder := parseAttrsOrdered(inner)
			if attrs != nil {
				bp.pendingAttrs, bp.pendingAttrOrder = mergeAttrsOrdered(bp.pendingAttrs, bp.pendingAttrOrder, attrs, attrOrder)
				bp.pos += lines
				return true
			}
		} else {
			literalLines = lines
		}
	}

	// Paragraphs consume continuation lines before dispatch, so a thematic
	// break reaching here needs no preceding blank.
	if isThematicBreak(stripped) {
		node := bp.arena.new(parseNodeSpec{Kind: ast.KindThematicBreak})
		node.Start = ast.Pos{Offset: line.start}
		node.End = ast.Pos{Offset: line.end}
		bp.attachPendingAttrs(node)
		parent.Children = append(parent.Children, node)
		bp.pos++
		return true
	}

	if level := headingLevel(stripped); level > 0 {
		bp.parseHeading(parent, level, stripped, prefix)
		return true
	}

	if isCodeFenceOpen(stripped) {
		bp.parseCodeBlock(parent, stripped, indent, prefix)
		return true
	}

	if len(stripped) > 0 && stripped[0] == '>' && (len(stripped) == 1 || stripped[1] == ' ') {
		bp.parseBlockQuote(parent, indent, prefix)
		return true
	}

	if isDivFenceOpen(stripped) {
		bp.parseFencedDiv(parent, stripped, indent, prefix)
		return true
	}

	if marker, after, ok := bulletListMarker(stripped); ok {
		if isTaskListItem(after) {
			bp.parseTaskList(parent, marker, indent, prefix)
			return true
		}
		bp.parseBulletList(parent, marker, after, indent, prefix)
		return true
	}

	if start, style, after, ok := orderedListMarker(stripped); ok {
		bp.parseOrderedList(parent, start, style, after, indent, prefix)
		return true
	}

	if isReferenceDefinition(stripped) {
		bp.parseReferenceDefinition(parent, stripped, indent, prefix)
		return true
	}

	if isFootnoteDefinition(stripped) {
		bp.parseFootnoteDefinition(parent, stripped, indent, prefix)
		return true
	}

	if isDefinitionListMarker(stripped) {
		bp.parseDefinitionList(parent, indent, prefix)
		return true
	}

	if isTableRow(stripped) {
		bp.parseTable(parent, stripped, indent, prefix)
		return true
	}

	bp.parseParagraph(parent, prefix, literalLines)
	return true
}

func (bp *blockParser) currentLine() blockLine {
	return bp.lines[bp.pos]
}

func (bp *blockParser) parseHeading(parent *parseNode, level int, stripped, prefix string) {
	content := strings.TrimSpace(stripped[level:])

	startLine := bp.currentLine()
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindHeading, Level: level})
	node.Start = ast.Pos{Offset: startLine.start}
	bp.attachPendingAttrs(node)

	lastEnd := startLine.end
	bp.pos++

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
		// Recognized block starts end headings, matching djot.js.
		if _, _, ok := bulletListMarker(s); ok {
			break
		}
		if _, _, _, ok := orderedListMarker(s); ok {
			break
		}
		if isDefinitionListMarker(s) || isTableRow(s) {
			break
		}
		if isReferenceDefinition(s) || isFootnoteDefinition(s) {
			break
		}
		// Mirror the dispatcher: valid attributes and multiline attempts end
		// headings; invalid single-line attempts remain heading text.
		if len(s) > 0 && s[0] == '{' {
			if attrContent, lines := bp.tryBlockAttr(s, prefix); attrContent != "" {
				if attrs, _ := parseAttrsOrdered(attrContent[1 : len(attrContent)-1]); attrs != nil {
					break
				}
			} else if lines > 0 {
				break
			}
		}
		// Same-level heading markers continue the heading.
		var line_content string
		if headingLevel(s) == level {
			line_content = strings.TrimSpace(s[level:])
		} else {
			line_content = strings.TrimRight(s, " \t")
		}
		// A bare marker line contributes no content, and no newline either:
		// the lines around it join as if it weren't there.
		if line_content == "" {
			lastEnd = line.end
			bp.pos++
			continue
		}
		if textBuf.Len() > 0 {
			textBuf.WriteByte('\n')
		}
		textBuf.WriteString(line_content)
		lastEnd = line.end
		bp.pos++
	}

	node.End = ast.Pos{Offset: lastEnd}
	node.Text = textBuf.String()
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseCodeBlock(parent *parseNode, stripped string, baseIndent int, prefix string) {
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
		node := bp.arena.new(parseNodeSpec{Kind: ast.KindRawBlock, Format: format})
		node.Start = ast.Pos{Offset: openLine.start}
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
		node.End = ast.Pos{Offset: lastEnd}
		node.Text = textBuf.String()
		parent.Children = append(parent.Children, node)
		return
	}

	node := bp.arena.new(parseNodeSpec{Kind: ast.KindCodeBlock, Lang: lang})
	node.Start = ast.Pos{Offset: openLine.start}
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

	node.End = ast.Pos{Offset: lastEnd}
	node.Text = textBuf.String()
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBlockQuote(parent *parseNode, indent int, outerPrefix string) {
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindBlockQuote})
	startLine := bp.currentLine()
	node.Start = ast.Pos{Offset: startLine.start}
	bp.attachPendingAttrs(node)

	var content contentLines
	lastEnd := startLine.start
	prefixLen := len(outerPrefix)

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
		leadingWS := len(text) - len(stripped)

		if isBlankLine(text) {
			// A blank line ends the block quote (unless it's "> " blank).
			break
		}

		if len(stripped) > 0 && stripped[0] == '>' && (len(stripped) == 1 || stripped[1] == ' ') {
			if len(stripped) == 1 {
				content.addBlank(line.start+prefixLen+leadingWS+1, line.end)
			} else {
				content.add(stripped[2:],
					line.start+prefixLen+leadingWS+2, line.end)
			}
		} else {
			// Lazy continuation.
			content.add(stripped,
				line.start+prefixLen+leadingWS, line.end)
		}
		lastEnd = line.end
		bp.pos++
	}

	// Parse the collected content as blocks.
	subBP := content.subParser(bp.references, bp.arena)
	subBP.parseBlocks(node, 0, "")

	node.End = ast.Pos{Offset: lastEnd}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseFencedDiv(parent *parseNode, stripped string, baseIndent int, prefix string) {
	fenceLen := 0
	for i := 0; i < len(stripped) && stripped[i] == ':'; i++ {
		fenceLen++
	}

	className := strings.TrimSpace(stripped[fenceLen:])

	openLine := bp.currentLine()
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindDiv})
	node.Start = ast.Pos{Offset: openLine.start}
	bp.attachPendingAttrs(node)
	if className != "" {
		node.AddClass(className)
	}

	bp.pos++

	// Collect inner content, parse as blocks.
	// Track code fences so that ::: inside a code block doesn't close the div.
	var content contentLines
	lastEnd := openLine.end
	prefixLen := len(prefix)
	inCodeFence := false
	codeFenceChar := byte(0)
	codeFenceLen := 0
	inParagraph := false
	inHeading := false
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

		// A fence line only counts in fresh block position or after a
		// heading (which fences interrupt); an open paragraph absorbs it
		// as text (nothing interrupts a paragraph). A closing div fence,
		// however, outranks paragraph continuation.
		closed := false
		switch {
		case inCodeFence:
			if isClosingCodeFence(s, codeFenceChar, codeFenceLen) {
				inCodeFence = false
			}
		case isBlankLine(text):
			inParagraph, inHeading = false, false
		case !inParagraph && isCodeFenceOpen(s):
			inCodeFence = true
			inHeading = false
			codeFenceChar = s[0]
			codeFenceLen = 0
			for codeFenceLen < len(s) && s[codeFenceLen] == codeFenceChar {
				codeFenceLen++
			}
		case isClosingDivFence(s, fenceLen):
			// Closing fence: at least fenceLen colons, nothing else.
			lastEnd = line.end
			bp.pos++
			closed = true
		case !inParagraph && !inHeading && headingLevel(s) > 0:
			inHeading = true
		case !inHeading:
			inParagraph = true
		}
		if closed {
			break
		}

		content.add(text, line.start+prefixLen, line.end)
		lastEnd = line.end
		bp.pos++
	}

	subBP := content.subParser(bp.references, bp.arena)
	subBP.parseBlocks(node, 0, "")

	node.End = ast.Pos{Offset: lastEnd}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseBulletList(parent *parseNode, marker byte, afterMarker string, indent int, prefix string) {
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindBulletList})
	line := bp.currentLine()
	// Position at the list marker, not the leading whitespace.
	node.Start = ast.Pos{Offset: line.start + indent}
	node.Marker = marker
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

		if blanksBefore > 0 && blankCountsBetweenItems(node) {
			hasBlankBetweenItems = true
		}

		item := bp.arena.new(parseNodeSpec{Kind: ast.KindListItem})
		// Position at the list marker, not the leading whitespace.
		item.Start = ast.Pos{Offset: line.start + itemIndent}
		bp.pos++

		var content contentLines
		// Strip all continuation lines by stripAmount (markerIndent + 1),
		// which preserves relative indentation for sublists at varying depths.
		// Prepend padding to `after` so it aligns with content at contentIndent.
		stripAmount := itemIndent + 1
		contentIndent := itemIndent + 2 // marker + space
		padding := strings.Repeat(" ", contentIndent-stripAmount)
		prefixLen := len(prefix)
		content.add(padding+after,
			line.start+prefixLen+contentIndent, line.end)

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
						if !isBullet && !isOrd && !isDefinitionListMarker(peekStripped) {
							hasBlankWithinItem = true
						}
						content.addBlank(nextLine.start, nextLine.end)
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent > markerIndent {
				rest := stripIndent(nextText, stripAmount)
				content.add(rest,
					nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
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
				trimmedNext := strings.TrimLeft(nextText, " \t")
				content.add(trimmedNext,
					nextLine.start+prefixLen+(len(nextText)-len(trimmedNext)), nextLine.end)
				bp.pos++
			}
		}

		subBP := content.subParser(bp.references, bp.arena)
		subBP.parseBlocks(item, 0, "")

		// Set item end past the newline that terminates the last consumed
		// line. For the last line in the input (no trailing newline), end
		// is at the line's end position (one past last char).
		if bp.pos > 0 {
			prevIdx := bp.pos - 1
			lastLine := bp.lines[prevIdx]
			endOffset := lastLine.end
			// If there's a following line, the end includes the newline.
			if prevIdx+1 < len(bp.lines) {
				endOffset = bp.lines[prevIdx+1].start
			}
			item.End = ast.Pos{Offset: endOffset}
		}
		node.Children = append(node.Children, item)
	}

	// A list is loose when blank lines separate its items or appear within
	// an item. Blank lines directly before a sublist are already excluded
	// during item collection, and blankCountsBetweenItems excludes blank
	// lines after a trailing sublist.
	if !hasBlankWithinItem && !hasBlankBetweenItems {
		node.tight = true
	}

	// Set list end from last item.
	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseOrderedList(parent *parseNode, start int, style ast.ListStyle, afterMarker string, indent int, prefix string) {
	// Extract the first item's enum text and delimiter for potential reinterpretation.
	firstLine := bp.lines[bp.pos].text
	if prefix != "" && strings.HasPrefix(firstLine, prefix) {
		firstLine = firstLine[len(prefix):]
	}
	firstStripped := strings.TrimLeft(firstLine, " \t")
	firstEnum, firstDelim, _ := extractOrderedMarkerParts(firstStripped)
	// Every enumerator taken so far. While they are all ambiguous the list's
	// style is still open to revision — see the reinterpretation below.
	enums := []string{firstEnum}

	node := bp.arena.new(parseNodeSpec{Kind: ast.KindOrderedList, ListStart: start, ListStyle: style})
	node.Start = ast.Pos{Offset: bp.currentLine().start}
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
		itemEnum, itemDelim, _ := extractOrderedMarkerParts(stripped)
		if itemDelim != firstDelim {
			bp.pos -= blanksBefore
			break
		}
		if itemStyle != style {
			// The marker was classified on its own, where roman wins the tie for
			// letters that are also numerals — "c", "d", "i". A list already
			// under way wins that tie instead: if the enumerator reads as the
			// style in play, it goes on being that style.
			if _, ok2 := parseOrderedEnumAs(itemEnum, style); ok2 {
				itemStyle = style
			} else if allParseAs(enums, itemStyle) {
				// Otherwise every enumerator so far was ambiguous, and this one
				// settles the question: reread the list in the style that keeps
				// it going. The spec asks for the reading that yields one
				// continuous list rather than two, and takes the start number
				// from the first item under that reading.
				newNum, _ := parseOrderedEnumAs(enums[0], itemStyle)
				style = itemStyle
				node.ListStyle = style
				node.ListStart = newNum
			} else {
				bp.pos -= blanksBefore
				break
			}
		}

		if blanksBefore > 0 && blankCountsBetweenItems(node) {
			hasBlankBetweenItems = true
		}

		enums = append(enums, itemEnum)

		item := bp.arena.new(parseNodeSpec{Kind: ast.KindListItem})
		item.Start = ast.Pos{Offset: line.start}
		bp.pos++

		var content contentLines
		prefixLen := len(prefix)

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
		content.add(padding+after,
			line.start+prefixLen+contentIndent, line.end)

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
						if !isBullet && !isOrd && !isDefinitionListMarker(peekStripped) {
							hasBlankWithinItem = true
						}
						content.addBlank(nextLine.start, nextLine.end)
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent > markerIndent {
				rest := stripIndent(nextText, stripAmount)
				// Offsets are bytes; the stripped indentation's byte length
				// can differ from its column count when tabs are present.
				content.add(rest,
					nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
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
				trimmedNext := strings.TrimLeft(nextText, " \t")
				content.add(trimmedNext,
					nextLine.start+prefixLen+(len(nextText)-len(trimmedNext)), nextLine.end)
				bp.pos++
			}
		}

		subBP := content.subParser(bp.references, bp.arena)
		subBP.parseBlocks(item, 0, "")

		if bp.pos > 0 {
			item.End = ast.Pos{Offset: bp.lines[bp.pos-1].end}
		}
		node.Children = append(node.Children, item)
	}

	// Same tight/loose rule as bullet lists.
	if !hasBlankWithinItem && !hasBlankBetweenItems {
		node.tight = true
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseParagraph(parent *parseNode, prefix string, literalLines int) {
	var textBuf strings.Builder
	startOffset := bp.currentLine().start
	lastEnd := bp.currentLine().end
	taken, literalBytes := 0, 0

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

		// Nothing interrupts a paragraph; only a blank line ends it.

		if textBuf.Len() > 0 {
			textBuf.WriteByte('\n')
		}
		textBuf.WriteString(strings.TrimLeft(text, " \t"))
		lastEnd = line.end
		bp.pos++
		taken++
		if taken == literalLines {
			literalBytes = textBuf.Len()
		}
	}

	if textBuf.Len() > 0 {
		text := strings.TrimRight(textBuf.String(), " \t")
		// If the paragraph broke off early, everything it took is literal.
		if literalLines > 0 && taken < literalLines {
			literalBytes = len(text)
		}
		node := bp.arena.new(parseNodeSpec{Kind: ast.KindParagraph, Text: text})
		node.Start = ast.Pos{Offset: startOffset}
		node.End = ast.Pos{Offset: lastEnd}
		node.plainBracesUntil = min(literalBytes, len(text))
		bp.attachPendingAttrs(node)
		parent.Children = append(parent.Children, node)
	}
}

func (bp *blockParser) attachPendingAttrs(node *parseNode) {
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

// tryBlockAttr attempts to read a block attribute starting at the current line.
// On success it returns the brace-delimited content and the number of lines it
// spans.
//
// On failure it returns "" and, when the attempt died because a continuation
// line wasn't indented, the number of lines it had already taken. Those lines
// are literal text: djot.js re-emits them as plain inline content rather than
// letting "{" be read again as an inline attribute or comment. Failing for want
// of a closing brace instead reports 0, leaving the whole paragraph to normal
// inline parsing, which already falls back to literal text for a "{" that opens
// nothing valid.
func (bp *blockParser) tryBlockAttr(stripped, prefix string) (string, int) {
	trimmed := strings.TrimRight(stripped, " \t")
	// Single-line case: {attrs}
	if strings.HasSuffix(trimmed, "}") {
		return trimmed, 1
	}

	// A closing brace on the opening line rules out a multiline attempt.
	if strings.ContainsRune(trimmed, '}') {
		return "", 0
	}
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
			return "", lines
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
	}
	// Language and =format info strings are single tokens.
	if strings.ContainsAny(rest, " \t") {
		return false
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
	if n < 3 {
		return false
	}
	// Validate the class name: only [a-zA-Z0-9_-] allowed.
	className := strings.TrimSpace(s[n:])
	for i := 0; i < len(className); i++ {
		c := className[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
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

// blankCountsBetweenItems reports whether a blank line before the next item
// marker of a list makes the list loose. It does not when the previous item
// ends with a sublist: the blank belongs to that inner list, and trailing
// blank lines of a list never affect the tightness of its parent.
func blankCountsBetweenItems(list *parseNode) bool {
	if len(list.Children) == 0 {
		return false
	}
	prev := list.Children[len(list.Children)-1]
	if len(prev.Children) == 0 {
		return true
	}
	switch prev.Children[len(prev.Children)-1].Kind {
	case ast.KindBulletList, ast.KindOrderedList, ast.KindTaskList, ast.KindDefinitionList:
		return false
	}
	return true
}

func bulletListMarker(s string) (marker byte, after string, ok bool) {
	if len(s) == 0 {
		return 0, "", false
	}
	if s[0] == '-' || s[0] == '*' || s[0] == '+' {
		if len(s) == 1 {
			return s[0], "", true // marker at end of input (empty item)
		}
		if s[1] == ' ' {
			return s[0], s[2:], true
		}
	}
	return 0, "", false
}

func orderedListMarker(s string) (start int, style ast.ListStyle, after string, ok bool) {
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

// maxOrderedEnum bounds decimal enumerators at the widest value an HTML ol
// start attribute can carry. Capping here rather than at the platform int
// maximum keeps parsing identical on 32- and 64-bit builds; a larger start
// would not survive the output format anyway.
const maxOrderedEnum uint64 = math.MaxInt32

// parseDecimalEnum parses an all-digit enumerator, rejecting values past
// maxOrderedEnum.
func parseDecimalEnum(s string) (int, bool) {
	n := uint64(0)
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		// n stays <= maxOrderedEnum here, so this cannot overflow uint64.
		n = n*10 + uint64(c-'0')
		if n > maxOrderedEnum {
			return 0, false
		}
	}
	// maxOrderedEnum fits every platform's int, so this conversion is exact.
	return int(n), true
}

// parseOrderedEnumAs tries to parse an enum string as a specific style.
func parseOrderedEnumAs(s string, style ast.ListStyle) (int, bool) {
	// No style reads an empty enumerator; without this the decimal case would
	// fall out of its loop and call it zero.
	if s == "" {
		return 0, false
	}
	switch style {
	case ast.ListDecimal:
		return parseDecimalEnum(s)
	case ast.ListAlphaLower:
		if len(s) == 1 && s[0] >= 'a' && s[0] <= 'z' {
			return int(s[0]-'a') + 1, true
		}
		return 0, false
	case ast.ListAlphaUpper:
		if len(s) == 1 && s[0] >= 'A' && s[0] <= 'Z' {
			return int(s[0]-'A') + 1, true
		}
		return 0, false
	case ast.ListRomanLower:
		if isAllLower(s) {
			return parseRoman(strings.ToUpper(s))
		}
		return 0, false
	case ast.ListRomanUpper:
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
func parseOrderedEnum(s string) (num int, style ast.ListStyle, ok bool) {
	if len(s) == 0 {
		return 0, 0, false
	}

	// Decimal
	if s[0] >= '0' && s[0] <= '9' {
		n, ok := parseDecimalEnum(s)
		if !ok {
			return 0, 0, false
		}
		return n, ast.ListDecimal, true
	}

	// Lower letters: try roman first, then alpha
	if isAllLower(s) {
		if rn, ok2 := parseRoman(strings.ToUpper(s)); ok2 {
			return rn, ast.ListRomanLower, true
		}
		if len(s) == 1 {
			return int(s[0]-'a') + 1, ast.ListAlphaLower, true
		}
		return 0, 0, false
	}

	// Upper letters: try roman first, then alpha
	if isAllUpper(s) {
		if rn, ok2 := parseRoman(s); ok2 {
			return rn, ast.ListRomanUpper, true
		}
		if len(s) == 1 {
			return int(s[0]-'A') + 1, ast.ListAlphaUpper, true
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
var romanValues = map[byte]int{
	'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000,
}

// maxRomanRun is how many times a digit may repeat in a row: the "one" digits
// four times, the "five" digits not at all, since VV is only ever a mistake
// for X.
func maxRomanRun(val int) int {
	switch val {
	case 5, 50, 500:
		return 1
	}
	return 4
}

// parseRoman converts a Roman numeral to its value. Additive and subtractive
// spellings are both accepted — IIII and IV are each 4, as on a clock face —
// but not runs that spill into the next digit up: IIIII is not 5, it is a
// mistake for V.
func parseRoman(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	total, prev, run := 0, 0, 0
	for i := len(s) - 1; i >= 0; i-- {
		val, exists := romanValues[s[i]]
		if !exists {
			return 0, false
		}
		if val == prev {
			run++
		} else {
			run = 1
		}
		if run > maxRomanRun(val) {
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
	return total, true
}

// stripIndent removes up to n indentation columns. Callers derive the number
// of source bytes consumed from the returned suffix because tabs span columns.
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

// isReferenceDefinition reports whether s opens a reference definition,
// "[label]: url".
//
// Whatever follows the colon must be a single run of non-whitespace reaching
// the end of the line, matching pattReferenceDefinition in djot.js. A title, a
// stray trailing word, even a trailing space means the line is not a definition
// at all: it stays a paragraph, and uses of the label go unresolved. The URL
// may also be empty here and supplied by continuation lines.
func isReferenceDefinition(s string) bool {
	if len(s) < 3 || s[0] != '[' {
		return false
	}
	// A footnote definition takes precedence, but only a real one: "[^]:" has
	// no footnote label, so it is a reference definition labelled "^".
	if isFootnoteDefinition(s) {
		return false
	}
	closeBracket := strings.IndexByte(s, ']')
	if closeBracket < 1 {
		return false
	}
	if closeBracket+1 >= len(s) || s[closeBracket+1] != ':' {
		return false
	}
	rest := s[closeBracket+2:]
	if rest == "" {
		return true
	}
	url := strings.TrimLeft(rest, " \t")
	if len(url) == len(rest) {
		return false // the URL must be separated from the colon by whitespace
	}
	return !strings.ContainsAny(url, " \t")
}

func (bp *blockParser) parseReferenceDefinition(parent *parseNode, stripped string, indent int, prefix string) {
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
		trimmed := strings.TrimLeft(nextText, " \t")
		// If continuation looks like another ref def, stop
		if isReferenceDefinition(trimmed) || isFootnoteDefinition(trimmed) {
			break
		}
		// Like the first line, a continuation chunk must be one unbroken run of
		// non-whitespace. A chunk with a space in it ends the definition and is
		// parsed as its own block instead.
		if strings.ContainsAny(trimmed, " \t") {
			break
		}
		urlParts = append(urlParts, trimmed)
		bp.pos++
	}

	url := strings.Join(urlParts, "")
	ref := bp.arena.new(parseNodeSpec{Kind: ast.KindLink, Target: url, Label: label})
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

func (bp *blockParser) parseFootnoteDefinition(parent *parseNode, stripped string, indent int, prefix string) {
	closeBracket := strings.IndexByte(stripped, ']')
	label := stripped[2:closeBracket]

	after := ""
	if closeBracket+2 < len(stripped) {
		after = strings.TrimSpace(stripped[closeBracket+2:])
	}

	node := bp.arena.new(parseNodeSpec{Kind: ast.KindFootnote, Label: label})
	node.Start = ast.Pos{Offset: bp.currentLine().start}
	bp.pos++

	var content contentLines
	prefixLen := len(prefix)
	fnLine := bp.lines[bp.pos-1] // we already advanced past the footnote line
	if after != "" {
		// after is trimmed from stripped[closeBracket+2:]; content starts after "]: "
		afterStart := fnLine.start + prefixLen + indent + closeBracket + 2
		// Find start of trimmed content (skip leading whitespace in the after portion)
		rawAfter := stripped[closeBracket+2:]
		afterStart += len(rawAfter) - len(strings.TrimLeft(rawAfter, " \t"))
		content.add(after, afterStart, fnLine.end)
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
					content.addBlank(nextLine.start, nextLine.end)
					bp.pos++
					continue
				}
			}
			break
		}

		nextIndent := countLeadingSpaces(nextText)
		if nextIndent >= contentIndent {
			rest := stripIndent(nextText, contentIndent)
			content.add(rest,
				nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
			bp.pos++
		} else {
			break
		}
	}

	if len(content) > 0 {
		subBP := content.subParser(bp.references, bp.arena)
		subBP.parseBlocks(node, 0, "")
	}

	if bp.pos > 0 {
		node.End = ast.Pos{Offset: bp.lines[bp.pos-1].end}
	}
	parent.Children = append(parent.Children, node)
}

func isTaskListItem(after string) bool {
	return (strings.HasPrefix(after, "[ ] ") || strings.HasPrefix(after, "[x] ") ||
		strings.HasPrefix(after, "[X] "))
}

func (bp *blockParser) parseTaskList(parent *parseNode, marker byte, indent int, prefix string) {
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindTaskList})
	node.Start = ast.Pos{Offset: bp.currentLine().start}
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

		if blanksBefore > 0 && blankCountsBetweenItems(node) {
			tight = false
		}

		checked := after[1] == 'x' || after[1] == 'X'
		afterCheckbox := after[4:] // skip "[ ] " or "[x] "

		item := bp.arena.new(parseNodeSpec{Kind: ast.KindTaskListItem, Checked: checked})
		item.Start = ast.Pos{Offset: line.start}
		bp.pos++

		var content contentLines
		prefixLen := len(prefix)

		contentIndent := len(text) - len(stripped) + 2 // marker + space
		// afterCheckbox starts after "- [ ] " = marker(2) + checkbox(4) = 6 chars from stripped
		content.add(afterCheckbox,
			line.start+prefixLen+contentIndent+4, line.end)

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
						// A blank before a nested list does not make the
						// task list loose (same rule as bullet/ordered).
						peekStripped := strings.TrimLeft(peekText, " \t")
						_, _, isBullet := bulletListMarker(peekStripped)
						_, _, _, isOrd := orderedListMarker(peekStripped)
						if !isBullet && !isOrd && !isDefinitionListMarker(peekStripped) {
							tight = false
						}
						content.addBlank(nextLine.start, nextLine.end)
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent >= contentIndent {
				rest := stripIndent(nextText, contentIndent)
				content.add(rest,
					nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
				bp.pos++
			} else {
				ns := strings.TrimLeft(nextText, " \t")
				_, _, isItem := bulletListMarker(ns)
				if isItem {
					break
				}
				trimmedNext := strings.TrimLeft(nextText, " \t")
				content.add(trimmedNext,
					nextLine.start+prefixLen+(len(nextText)-len(trimmedNext)), nextLine.end)
				bp.pos++
			}
		}

		subBP := content.subParser(bp.references, bp.arena)
		subBP.parseBlocks(item, 0, "")

		if bp.pos > 0 {
			item.End = ast.Pos{Offset: bp.lines[bp.pos-1].end}
		}
		node.Children = append(node.Children, item)
	}

	if tight {
		node.tight = true
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func isDefinitionListMarker(s string) bool {
	// Definition-list item starts with ":" followed by a space or end of line.
	if len(s) == 0 || s[0] != ':' {
		return false
	}
	return len(s) == 1 || s[1] == ' '
}

func isTableRow(s string) bool {
	if len(s) == 0 || s[0] != '|' {
		return false
	}
	// A row line must end with a pipe, ignoring trailing whitespace.
	if t := strings.TrimRight(s, " \t"); len(t) < 2 || t[len(t)-1] != '|' {
		return false
	}
	// Count unescaped, un-backticked pipe characters.
	// A valid table row needs at least 2 (the leading | plus one cell
	// separator). Verbatim spans open on a backtick run and close only on a
	// run of the same length (matching inline parsing); an unclosed span
	// swallows the rest of the line, pipes included.
	pipes := 0
	escaped := false
	openTicks := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '`' {
			run := 1
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			i += run - 1
			if openTicks == 0 {
				openTicks = run
			} else if run == openTicks {
				openTicks = 0
			}
			continue
		}
		if openTicks > 0 {
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '|' {
			pipes++
			if pipes >= 2 {
				return true
			}
		}
	}
	return false
}

func (bp *blockParser) parseDefinitionList(parent *parseNode, indent int, prefix string) {
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindDefinitionList})
	node.Start = ast.Pos{Offset: bp.currentLine().start}
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
		afterMarker := ""
		if len(stripped) > 2 {
			afterMarker = stripped[2:]
		}
		itemStartOffset := line.start
		bp.pos++

		var content contentLines
		prefixLen := len(prefix)

		contentIndent := itemIndent + 2 // marker + space
		content.add(afterMarker,
			line.start+prefixLen+contentIndent, line.end)

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
						content.addBlank(nextLine.start, nextLine.end)
						bp.pos++
						continue
					}
				}
				break
			}

			nextIndent := countLeadingSpaces(nextText)
			if nextIndent >= contentIndent {
				rest := stripIndent(nextText, contentIndent)
				content.add(rest,
					nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
				bp.pos++
			} else {
				ns := strings.TrimLeft(nextText, " \t")
				ni := len(nextText) - len(ns)
				if isDefinitionListMarker(ns) && ni == markerIndent {
					break
				}
				// Lazy continuation (indented beyond marker but less than content).
				if nextIndent > markerIndent {
					rest := stripIndent(nextText, markerIndent+1)
					content.add(rest,
						nextLine.start+prefixLen+(len(nextText)-len(rest)), nextLine.end)
					bp.pos++
				} else {
					break
				}
			}
		}

		// Parse collected content as blocks.
		subBP := content.subParser(bp.references, bp.arena)
		itemNode := bp.arena.new(parseNodeSpec{Kind: ast.KindDocument})
		subBP.parseBlocks(itemNode, 0, "")

		// Split: first paragraph is the term, rest is the definition.
		itemEndOffset := itemStartOffset
		if bp.pos > 0 {
			itemEndOffset = bp.lines[bp.pos-1].end
		}
		term := bp.arena.new(parseNodeSpec{Kind: ast.KindTerm})
		term.Start = ast.Pos{Offset: itemStartOffset}
		def := bp.arena.new(parseNodeSpec{Kind: ast.KindDefinition})

		if len(itemNode.Children) > 0 && itemNode.Children[0].Kind == ast.KindParagraph {
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
			def.Start = ast.Pos{Offset: itemStartOffset}
			def.Children = itemNode.Children
		}
		def.End = ast.Pos{Offset: itemEndOffset}

		node.Children = append(node.Children, term, def)
	}

	if tight {
		node.tight = true
	}

	if len(node.Children) > 0 {
		node.End = node.Children[len(node.Children)-1].End
	}
	parent.Children = append(parent.Children, node)
}

func (bp *blockParser) parseTable(parent *parseNode, stripped string, indent int, prefix string) {
	node := bp.arena.new(parseNodeSpec{Kind: ast.KindTable})
	node.Start = ast.Pos{Offset: bp.currentLine().start}
	bp.attachPendingAttrs(node)

	// Track current alignment from the most recent separator row.
	var currentAligns []ast.CellAlign

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
				if lastRow.Kind == ast.KindTableRow {
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

		row := parseTableRow(stripped, bp.arena)
		row.Start = ast.Pos{Offset: line.start}
		row.End = ast.Pos{Offset: line.end}
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

	// Look for caption blocks: each starts with "^ ". A later caption
	// replaces an earlier one, even if separated by blank lines.
	var captionNode *parseNode
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
		trimmed := strings.TrimLeft(text, " \t")
		if !(len(trimmed) >= 2 && trimmed[0] == '^' && trimmed[1] == ' ') {
			break
		}
		// Found a caption start. Collect this caption's lines.
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
			// A new ^ line within the same block replaces the caption.
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
		captionNode = bp.arena.new(parseNodeSpec{Kind: ast.KindCaption, Text: captionText})
		captionNode.Start = ast.Pos{Offset: captionStart}
		captionNode.End = ast.Pos{Offset: captionLastEnd}
		// Skip blank lines before checking for another caption.
		for bp.pos < len(bp.lines) {
			bl := bp.currentLine()
			bt := bl.text
			if prefix != "" {
				if strings.HasPrefix(bt, prefix) {
					bt = bt[len(prefix):]
				} else {
					break
				}
			}
			if !isBlankLine(bt) {
				break
			}
			bp.pos++
		}
	}
	if captionNode != nil {
		// Prepend caption as first child of table.
		node.Children = append([]*parseNode{captionNode}, node.Children...)
	}

	// If no caption was found, restore position (blank lines consumed should stay consumed
	// only if caption was found). Actually, blank lines between table and non-caption
	// content are fine to consume — but let's be safe.
	if len(node.Children) == 0 || node.Children[0].Kind != ast.KindCaption {
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

// isTableSeparator reports whether s is a table separator row such as |---|
// or |:--:|---:|, optionally carrying alignment colons.
//
// Cells must begin immediately after the leading "|": with a space, as in
// "| --- |", the line is an ordinary row and the row above it stays a body row.
// Whitespace between later cells is fine, since it sits after a "|" that the
// preceding cell already consumed. This mirrors pattRowSep in djot.js.
func isTableSeparator(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '|' {
		return false
	}
	pos := 1
	for {
		if pos < len(s) && s[pos] == ':' {
			pos++
		}
		dashes := pos
		for pos < len(s) && s[pos] == '-' {
			pos++
		}
		if pos == dashes {
			return false
		}
		if pos < len(s) && s[pos] == ':' {
			pos++
		}
		for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		if pos >= len(s) || s[pos] != '|' {
			return false
		}
		pos++
		for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		if pos == len(s) {
			return true
		}
	}
}

func parseTableAlignments(s string) []ast.CellAlign {
	s = strings.TrimSpace(s)
	if len(s) > 0 && s[0] == '|' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '|' {
		s = s[:len(s)-1]
	}
	parts := strings.Split(s, "|")
	var aligns []ast.CellAlign
	for _, part := range parts {
		part = strings.TrimSpace(part)
		left := len(part) > 0 && part[0] == ':'
		right := len(part) > 0 && part[len(part)-1] == ':'
		switch {
		case left && right:
			aligns = append(aligns, ast.AlignCenter)
		case left:
			aligns = append(aligns, ast.AlignLeft)
		case right:
			aligns = append(aligns, ast.AlignRight)
		default:
			aligns = append(aligns, ast.AlignDefault)
		}
	}
	return aligns
}

func parseTableRow(s string, arena *parseNodeArena) *parseNode {
	row := arena.new(parseNodeSpec{Kind: ast.KindTableRow})
	s = strings.TrimSpace(s)
	if len(s) > 0 && s[0] == '|' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '|' {
		s = s[:len(s)-1]
	}

	cells := splitTableCells(s)
	for _, cellText := range cells {
		cell := arena.new(parseNodeSpec{Kind: ast.KindTableCell, Text: strings.TrimSpace(cellText)})
		row.Children = append(row.Children, cell)
	}
	return row
}

func splitTableCells(s string) []string {
	var cells []string
	var current strings.Builder
	escaped := false
	openTicks := 0

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '`' {
			// Same run-aware verbatim tracking as isTableRow.
			run := 1
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			current.WriteString(s[i : i+run])
			i += run - 1
			if openTicks == 0 {
				openTicks = run
			} else if run == openTicks {
				openTicks = 0
			}
			continue
		}
		if openTicks > 0 {
			current.WriteByte(c)
			continue
		}
		if c == '\\' {
			escaped = true
			current.WriteByte(c)
			continue
		}
		if c == '|' {
			cells = append(cells, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}
	cells = append(cells, current.String())
	return cells
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

// allParseAs reports whether every enumerator can be read in the given style.
// A list only changes style when the items already taken can all be reread that
// way, so that the change explains the whole list rather than splitting it.
func allParseAs(enums []string, style ast.ListStyle) bool {
	for _, e := range enums {
		if _, ok := parseOrderedEnumAs(e, style); !ok {
			return false
		}
	}
	return true
}
