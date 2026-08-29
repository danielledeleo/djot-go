package djot

import "github.com/danielledeleo/djot-go/ast"

func (t *semanticTape) materializeAST() *ast.Document {
	if t == nil || len(t.records) < 2 {
		return nil
	}
	var arena typedNodeArena
	root, _ := t.materializeNode(0, &arena)
	document, ok := root.(*ast.Document)
	if !ok {
		panic("djot: semantic tape root is not a document")
	}
	return document
}

func (t *semanticTape) materializeNode(index int, arena *typedNodeArena) (ast.Node, int) {
	record := t.records[index]
	node := allocateTypedNode(ast.Kind(record.kind), arena)
	position := t.positions[index]
	ast.SetSpan(node, ast.SourceSpan{
		Start: ast.Pos{Offset: int(position.start)},
		End:   ast.Pos{Offset: int(position.end)},
	})
	for j, end := record.attrStart, t.records[index+1].attrStart; j < end; j++ {
		attribute := t.attributes[j]
		node.Attributes().Set(attribute.key, attribute.value)
	}

	switch node := node.(type) {
	case *ast.Heading:
		node.Level = int(record.small)
	case *ast.BulletList:
		node.Marker = byte(record.small)
		node.Tight = record.flags&semanticTight != 0
	case *ast.OrderedList:
		node.Style = ast.ListStyle(record.small)
		node.Start = t.listStarts[record.payload]
		node.Tight = record.flags&semanticTight != 0
	case *ast.TaskList:
		node.Tight = record.flags&semanticTight != 0
	case *ast.DefinitionList:
		node.Tight = record.flags&semanticTight != 0
	case *ast.TaskListItem:
		node.Checked = record.flags&semanticChecked != 0
	case *ast.TableRow:
		node.Header = record.flags&semanticHeader != 0
	case *ast.TableCell:
		node.Alignment = ast.CellAlign(record.small)
		node.Header = record.flags&semanticHeader != 0
	case *ast.Text:
		node.Value = t.text(record.payload)
	case *ast.Verbatim:
		node.Text = t.text(record.payload)
	case *ast.InlineMath:
		node.Text = t.text(record.payload)
	case *ast.DisplayMath:
		node.Text = t.text(record.payload)
	case *ast.Symbol:
		node.Name = t.text(record.payload)
	case *ast.Link:
		node.DestinationSet = record.flags&semanticHasTarget != 0
		if record.payload != 0 {
			node.Destination = t.targets[record.payload]
		}
	case *ast.Image:
		node.DestinationSet = record.flags&semanticHasTarget != 0
		if record.payload != 0 {
			node.Destination = t.targets[record.payload]
		}
	case *ast.CodeBlock:
		payload := t.textExtras[record.payload]
		node.Text, node.Language = payload.text, payload.extra
	case *ast.RawBlock:
		payload := t.textExtras[record.payload]
		node.Text, node.Format = payload.text, payload.extra
	case *ast.RawInline:
		payload := t.textExtras[record.payload]
		node.Text, node.Format = payload.text, payload.extra
	case *ast.Footnote:
		node.Label = t.labels[record.payload]
	case *ast.FootnoteReference:
		node.Label = t.labels[record.payload]
	}

	next := index + 1
	for next < int(record.subtreeEnd) {
		child, after := t.materializeNode(next, arena)
		ast.AppendChild(node, child)
		next = after
	}
	return node, next
}

func allocateTypedNode(kind ast.Kind, arena *typedNodeArena) ast.Node {
	switch kind {
	case ast.KindDocument:
		return &ast.Document{}
	case ast.KindSection:
		return &ast.Section{}
	case ast.KindParagraph:
		return arena.paragraphs.alloc()
	case ast.KindHeading:
		return &ast.Heading{}
	case ast.KindThematicBreak:
		return &ast.ThematicBreak{}
	case ast.KindCodeBlock:
		return &ast.CodeBlock{}
	case ast.KindRawBlock:
		return &ast.RawBlock{}
	case ast.KindBlockQuote:
		return &ast.BlockQuote{}
	case ast.KindDiv:
		return &ast.Div{}
	case ast.KindBulletList:
		return &ast.BulletList{}
	case ast.KindOrderedList:
		return &ast.OrderedList{}
	case ast.KindTaskList:
		return &ast.TaskList{}
	case ast.KindListItem:
		return arena.listItems.alloc()
	case ast.KindTaskListItem:
		return &ast.TaskListItem{}
	case ast.KindDefinitionList:
		return &ast.DefinitionList{}
	case ast.KindTerm:
		return &ast.Term{}
	case ast.KindDefinition:
		return &ast.Definition{}
	case ast.KindTable:
		return &ast.Table{}
	case ast.KindTableRow:
		return &ast.TableRow{}
	case ast.KindTableCell:
		return arena.tableCells.alloc()
	case ast.KindCaption:
		return &ast.Caption{}
	case ast.KindFootnote:
		return &ast.Footnote{}
	case ast.KindText:
		return arena.texts.alloc()
	case ast.KindSoftBreak:
		return &ast.SoftBreak{}
	case ast.KindHardBreak:
		return &ast.HardBreak{}
	case ast.KindNonBreakingSpace:
		return &ast.NonBreakingSpace{}
	case ast.KindEmphasis:
		return &ast.Emphasis{}
	case ast.KindStrong:
		return arena.strongs.alloc()
	case ast.KindSuperscript:
		return &ast.Superscript{}
	case ast.KindSubscript:
		return &ast.Subscript{}
	case ast.KindInsert:
		return &ast.Insert{}
	case ast.KindDelete:
		return &ast.Delete{}
	case ast.KindMark:
		return &ast.Mark{}
	case ast.KindLink:
		return arena.links.alloc()
	case ast.KindImage:
		return &ast.Image{}
	case ast.KindSpan:
		return &ast.Span{}
	case ast.KindVerbatim:
		return &ast.Verbatim{}
	case ast.KindInlineMath:
		return &ast.InlineMath{}
	case ast.KindDisplayMath:
		return &ast.DisplayMath{}
	case ast.KindRawInline:
		return &ast.RawInline{}
	case ast.KindSymbol:
		return &ast.Symbol{}
	case ast.KindFootnoteReference:
		return &ast.FootnoteReference{}
	case ast.KindDoubleQuoted:
		return &ast.DoubleQuoted{}
	case ast.KindSingleQuoted:
		return &ast.SingleQuoted{}
	case ast.KindEllipsis:
		return &ast.Ellipsis{}
	case ast.KindEmDash:
		return &ast.EmDash{}
	case ast.KindEnDash:
		return &ast.EnDash{}
	default:
		panic("djot: unknown semantic node kind")
	}
}

func (t *semanticTape) materializeReferences() map[string]*ast.Reference {
	result := make(map[string]*ast.Reference, len(t.references))
	for _, named := range t.references {
		source := named.semanticReference
		reference := &ast.Reference{
			Destination: source.target, DestinationSet: source.hasTarget,
		}
		for _, attribute := range source.attrs {
			reference.Attributes.Set(attribute.key, attribute.value)
		}
		result[named.name] = reference
	}
	return result
}

func (t *semanticTape) matchesAST(root *ast.Document) bool {
	if t == nil || root == nil || len(t.records) < 2 {
		return false
	}
	index, ok := t.matchNode(root, 0)
	return ok && index == len(t.records)-1
}

func (t *semanticTape) matchNode(node ast.Node, index int) (int, bool) {
	if index+1 >= len(t.records) || node == nil {
		return index, false
	}
	record := t.records[index]
	if ast.Kind(record.kind) != node.Kind() || !t.matchAttributes(node, index) {
		return index, false
	}
	position := t.positions[index]
	span := node.Span()
	if span.Start.File != 0 || span.End.File != 0 ||
		span.Start.Offset != int(position.start) || span.End.Offset != int(position.end) {
		return index, false
	}

	var flags uint8
	switch node := node.(type) {
	case *ast.BulletList:
		if node.Tight {
			flags |= semanticTight
		}
	case *ast.OrderedList:
		if node.Tight {
			flags |= semanticTight
		}
	case *ast.TaskList:
		if node.Tight {
			flags |= semanticTight
		}
	case *ast.DefinitionList:
		if node.Tight {
			flags |= semanticTight
		}
	case *ast.TaskListItem:
		if node.Checked {
			flags |= semanticChecked
		}
	case *ast.TableRow:
		if node.Header {
			flags |= semanticHeader
		}
	case *ast.TableCell:
		if node.Header {
			flags |= semanticHeader
		}
	case *ast.Link:
		if node.DestinationSet {
			flags |= semanticHasTarget
		}
	case *ast.Image:
		if node.DestinationSet {
			flags |= semanticHasTarget
		}
	}
	if flags != record.flags {
		return index, false
	}

	switch node := node.(type) {
	case *ast.Heading:
		if node.Level != int(record.small) {
			return index, false
		}
	case *ast.BulletList:
		if node.Marker != byte(record.small) {
			return index, false
		}
	case *ast.OrderedList:
		if node.Style != ast.ListStyle(record.small) || node.Start != t.listStarts[record.payload] {
			return index, false
		}
	case *ast.TableCell:
		if node.Alignment != ast.CellAlign(record.small) {
			return index, false
		}
	case *ast.Text:
		if node.Value != t.text(record.payload) {
			return index, false
		}
	case *ast.Verbatim:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *ast.InlineMath:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *ast.DisplayMath:
		if node.Text != t.text(record.payload) {
			return index, false
		}
	case *ast.Symbol:
		if node.Name != t.text(record.payload) {
			return index, false
		}
	case *ast.Link:
		if node.Destination != t.semanticTarget(record.payload) {
			return index, false
		}
	case *ast.Image:
		if node.Destination != t.semanticTarget(record.payload) {
			return index, false
		}
	case *ast.CodeBlock:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Language != payload.extra {
			return index, false
		}
	case *ast.RawBlock:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Format != payload.extra {
			return index, false
		}
	case *ast.RawInline:
		payload := t.textExtras[record.payload]
		if node.Text != payload.text || node.Format != payload.extra {
			return index, false
		}
	case *ast.Footnote:
		if node.Label != t.labels[record.payload] {
			return index, false
		}
	case *ast.FootnoteReference:
		if node.Label != t.labels[record.payload] {
			return index, false
		}
	}

	next, ok := index+1, true
	ast.ForEachChild(node, func(child ast.Node) {
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

func (t *semanticTape) matchAttributes(node ast.Node, index int) bool {
	start := int(t.records[index].attrStart)
	end := int(t.records[index+1].attrStart)
	attributes := node.Attributes().Entries()
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
