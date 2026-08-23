package djot

func (t *semanticTape) materializeAST() *Document {
	if t == nil || len(t.records) < 2 {
		return nil
	}
	var arena typedNodeArena
	root, _ := t.materializeNode(0, &arena)
	document, ok := root.(*Document)
	if !ok {
		panic("djot: semantic tape root is not a document")
	}
	return document
}

func (t *semanticTape) materializeNode(index int, arena *typedNodeArena) (Node, int) {
	record := t.records[index]
	node := allocateTypedNode(Kind(record.kind), arena)
	position := t.positions[index]
	node.base().span = SourceSpan{
		Start: Pos{Offset: int(position.start)},
		End:   Pos{Offset: int(position.end)},
	}
	for j, end := record.attrStart, t.records[index+1].attrStart; j < end; j++ {
		attribute := t.attributes[j]
		node.base().attrs.items = append(node.base().attrs.items, Attribute{
			Key: attribute.key, Value: attribute.value,
		})
	}

	switch node := node.(type) {
	case *Heading:
		node.Level = int(record.small)
	case *BulletList:
		node.Marker = byte(record.small)
		node.Tight = record.flags&semanticTight != 0
	case *OrderedList:
		node.Style = ListStyle(record.small)
		node.Start = t.listStarts[record.payload]
		node.Tight = record.flags&semanticTight != 0
	case *TaskList:
		node.Tight = record.flags&semanticTight != 0
	case *DefinitionList:
		node.Tight = record.flags&semanticTight != 0
	case *TaskListItem:
		node.Checked = record.flags&semanticChecked != 0
	case *TableRow:
		node.Header = record.flags&semanticHeader != 0
	case *TableCell:
		node.Alignment = CellAlign(record.small)
		node.Header = record.flags&semanticHeader != 0
	case *Text:
		node.Value = t.text(record.payload)
	case *Verbatim:
		node.Text = t.text(record.payload)
	case *InlineMath:
		node.Text = t.text(record.payload)
	case *DisplayMath:
		node.Text = t.text(record.payload)
	case *Symbol:
		node.Name = t.text(record.payload)
	case *Link:
		node.DestinationSet = record.flags&semanticHasTarget != 0
		if record.payload != 0 {
			node.Destination = t.targets[record.payload]
		}
	case *Image:
		node.DestinationSet = record.flags&semanticHasTarget != 0
		if record.payload != 0 {
			node.Destination = t.targets[record.payload]
		}
	case *CodeBlock:
		payload := t.textExtras[record.payload]
		node.Text, node.Language = payload.text, payload.extra
	case *RawBlock:
		payload := t.textExtras[record.payload]
		node.Text, node.Format = payload.text, payload.extra
	case *RawInline:
		payload := t.textExtras[record.payload]
		node.Text, node.Format = payload.text, payload.extra
	case *Footnote:
		node.Label = t.labels[record.payload]
	case *FootnoteReference:
		node.Label = t.labels[record.payload]
	}

	next := index + 1
	for next < int(record.subtreeEnd) {
		child, after := t.materializeNode(next, arena)
		appendTypedChild(node, child)
		next = after
	}
	return node, next
}

func allocateTypedNode(kind Kind, arena *typedNodeArena) Node {
	switch kind {
	case KindDocument:
		return &Document{}
	case KindSection:
		return &Section{}
	case KindParagraph:
		return arena.paragraphs.alloc()
	case KindHeading:
		return &Heading{}
	case KindThematicBreak:
		return &ThematicBreak{}
	case KindCodeBlock:
		return &CodeBlock{}
	case KindRawBlock:
		return &RawBlock{}
	case KindBlockQuote:
		return &BlockQuote{}
	case KindDiv:
		return &Div{}
	case KindBulletList:
		return &BulletList{}
	case KindOrderedList:
		return &OrderedList{}
	case KindTaskList:
		return &TaskList{}
	case KindListItem:
		return arena.listItems.alloc()
	case KindTaskListItem:
		return &TaskListItem{}
	case KindDefinitionList:
		return &DefinitionList{}
	case KindTerm:
		return &Term{}
	case KindDefinition:
		return &Definition{}
	case KindTable:
		return &Table{}
	case KindTableRow:
		return &TableRow{}
	case KindTableCell:
		return arena.tableCells.alloc()
	case KindCaption:
		return &Caption{}
	case KindFootnote:
		return &Footnote{}
	case KindText:
		return arena.texts.alloc()
	case KindSoftBreak:
		return &SoftBreak{}
	case KindHardBreak:
		return &HardBreak{}
	case KindNonBreakingSpace:
		return &NonBreakingSpace{}
	case KindEmphasis:
		return &Emphasis{}
	case KindStrong:
		return arena.strongs.alloc()
	case KindSuperscript:
		return &Superscript{}
	case KindSubscript:
		return &Subscript{}
	case KindInsert:
		return &Insert{}
	case KindDelete:
		return &Delete{}
	case KindMark:
		return &Mark{}
	case KindLink:
		return arena.links.alloc()
	case KindImage:
		return &Image{}
	case KindSpan:
		return &Span{}
	case KindVerbatim:
		return &Verbatim{}
	case KindInlineMath:
		return &InlineMath{}
	case KindDisplayMath:
		return &DisplayMath{}
	case KindRawInline:
		return &RawInline{}
	case KindSymbol:
		return &Symbol{}
	case KindFootnoteReference:
		return &FootnoteReference{}
	case KindDoubleQuoted:
		return &DoubleQuoted{}
	case KindSingleQuoted:
		return &SingleQuoted{}
	case KindEllipsis:
		return &Ellipsis{}
	case KindEmDash:
		return &EmDash{}
	case KindEnDash:
		return &EnDash{}
	default:
		panic("djot: unknown semantic node kind")
	}
}

func (t *semanticTape) materializeReferences() map[string]*Reference {
	result := make(map[string]*Reference, len(t.references))
	for name, source := range t.references {
		reference := &Reference{
			Destination: source.target, DestinationSet: source.hasTarget,
		}
		for _, attribute := range source.attrs {
			reference.Attributes.items = append(reference.Attributes.items, Attribute{
				Key: attribute.key, Value: attribute.value,
			})
		}
		result[name] = reference
	}
	return result
}

// matchesAST compares all state that can affect default HTML rendering.
func (t *semanticTape) matchesAST(root *Document) bool {
	if t == nil || root == nil || len(t.records) < 2 {
		return false
	}
	index, ok := t.matchNode(root, 0)
	return ok && index == len(t.records)-1
}

func (t *semanticTape) matchNode(node Node, index int) (int, bool) {
	if index+1 >= len(t.records) || node == nil {
		return index, false
	}
	record := t.records[index]
	if Kind(record.kind) != node.Kind() || !t.matchAttributes(node, index) {
		return index, false
	}

	var flags uint8
	switch node := node.(type) {
	case *BulletList:
		if node.Tight {
			flags |= semanticTight
		}
	case *OrderedList:
		if node.Tight {
			flags |= semanticTight
		}
	case *TaskList:
		if node.Tight {
			flags |= semanticTight
		}
	case *DefinitionList:
		if node.Tight {
			flags |= semanticTight
		}
	case *TaskListItem:
		if node.Checked {
			flags |= semanticChecked
		}
	case *TableRow:
		if node.Header {
			flags |= semanticHeader
		}
	case *TableCell:
		if node.Header {
			flags |= semanticHeader
		}
	case *Link:
		if node.DestinationSet {
			flags |= semanticHasTarget
		}
	case *Image:
		if node.DestinationSet {
			flags |= semanticHasTarget
		}
	}
	if flags != record.flags {
		return index, false
	}

	switch node := node.(type) {
	case *Heading:
		if node.Level != int(record.small) {
			return index, false
		}
	case *BulletList:
		if node.Marker != byte(record.small) {
			return index, false
		}
	case *OrderedList:
		if node.Style != ListStyle(record.small) || node.Start != t.listStarts[record.payload] {
			return index, false
		}
	case *TableCell:
		if node.Alignment != CellAlign(record.small) {
			return index, false
		}
	case *Text:
		if node.Value != t.text(record.payload) {
			return index, false
		}
	case *Verbatim:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *InlineMath:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *DisplayMath:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *Symbol:
		if node.Name != t.text(record.payload) {
			return index, false
		}
	case *Link:
		if node.Destination != t.semanticTarget(record.payload) {
			return index, false
		}
	case *Image:
		if node.Destination != t.semanticTarget(record.payload) {
			return index, false
		}
	case *CodeBlock:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Language != payload.extra {
			return index, false
		}
	case *RawBlock:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Format != payload.extra {
			return index, false
		}
	case *RawInline:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Format != payload.extra {
			return index, false
		}
	case *Footnote:
		if node.Label != t.labels[record.payload] {
			return index, false
		}
	case *FootnoteReference:
		if node.Label != t.labels[record.payload] {
			return index, false
		}
	}

	next, ok := index+1, true
	forEachChild(node, func(child Node) {
		if !ok {
			return
		}
		next, ok = t.matchNode(child, next)
	})
	if !ok || next != int(record.subtreeEnd) {
		return index, false
	}
	return next, true
}

func (t *semanticTape) semanticTarget(index uint32) string {
	if index == 0 {
		return ""
	}
	return t.targets[index]
}

func (t *semanticTape) matchAttributes(node Node, index int) bool {
	start := int(t.records[index].attrStart)
	end := int(t.records[index+1].attrStart)
	attributes := node.Attributes().items
	if len(attributes) != end-start {
		return false
	}
	for i, attribute := range attributes {
		expected := t.attributes[start+i]
		if attribute.Key != expected.key || attribute.Value != expected.value {
			return false
		}
	}
	return true
}
