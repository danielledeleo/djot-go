package djot

import "reflect"

func requireNode(node Node) {
	if node == nil || reflect.ValueOf(node).IsNil() {
		panic("djot: nil node")
	}
}

// walkRead visits node and all descendants in preorder without allocating.
func walkRead(node Node, visit func(Node)) {
	requireNode(node)
	visit(node)
	forEachChild(node, func(child Node) { walkRead(child, visit) })
}

func forEachChild(node Node, visit func(Node)) {
	switch node := node.(type) {
	case *Document:
		for _, child := range node.Children {
			visit(child)
		}
	case *Section:
		for _, child := range node.Children {
			visit(child)
		}
	case *Paragraph:
		for _, child := range node.Children {
			visit(child)
		}
	case *Heading:
		for _, child := range node.Children {
			visit(child)
		}
	case *BlockQuote:
		for _, child := range node.Children {
			visit(child)
		}
	case *Div:
		for _, child := range node.Children {
			visit(child)
		}
	case *BulletList:
		for _, child := range node.Items {
			visit(child)
		}
	case *OrderedList:
		for _, child := range node.Items {
			visit(child)
		}
	case *TaskList:
		for _, child := range node.Items {
			visit(child)
		}
	case *ListItem:
		for _, child := range node.Children {
			visit(child)
		}
	case *TaskListItem:
		for _, child := range node.Children {
			visit(child)
		}
	case *DefinitionList:
		for _, child := range node.Children {
			visit(child)
		}
	case *Term:
		for _, child := range node.Children {
			visit(child)
		}
	case *Definition:
		for _, child := range node.Children {
			visit(child)
		}
	case *Table:
		for _, child := range node.Children {
			visit(child)
		}
	case *TableRow:
		for _, child := range node.Cells {
			visit(child)
		}
	case *TableCell:
		for _, child := range node.Children {
			visit(child)
		}
	case *Caption:
		for _, child := range node.Children {
			visit(child)
		}
	case *Footnote:
		for _, child := range node.Children {
			visit(child)
		}
	case *Emphasis:
		for _, child := range node.Children {
			visit(child)
		}
	case *Strong:
		for _, child := range node.Children {
			visit(child)
		}
	case *Superscript:
		for _, child := range node.Children {
			visit(child)
		}
	case *Subscript:
		for _, child := range node.Children {
			visit(child)
		}
	case *Insert:
		for _, child := range node.Children {
			visit(child)
		}
	case *Delete:
		for _, child := range node.Children {
			visit(child)
		}
	case *Mark:
		for _, child := range node.Children {
			visit(child)
		}
	case *Link:
		for _, child := range node.Children {
			visit(child)
		}
	case *Image:
		for _, child := range node.Children {
			visit(child)
		}
	case *Span:
		for _, child := range node.Children {
			visit(child)
		}
	case *DoubleQuoted:
		for _, child := range node.Children {
			visit(child)
		}
	case *SingleQuoted:
		for _, child := range node.Children {
			visit(child)
		}
	case *ThematicBreak, *CodeBlock, *RawBlock, *Text, *SoftBreak,
		*HardBreak, *NonBreakingSpace, *Verbatim, *InlineMath, *DisplayMath,
		*RawInline, *Symbol, *FootnoteReference, *Ellipsis, *EmDash, *EnDash:
		return
	default:
		panic("djot: unknown node implementation")
	}
}

func appendTypedChild(parent, child Node) {
	requireNode(parent)
	requireNode(child)
	switch parent := parent.(type) {
	case *Document:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Section:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Paragraph:
		parent.Children = append(parent.Children, requireInline(child))
	case *Heading:
		parent.Children = append(parent.Children, requireInline(child))
	case *BlockQuote:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Div:
		parent.Children = append(parent.Children, requireBlock(child))
	case *BulletList:
		item, ok := child.(*ListItem)
		if !ok {
			panic("djot: bullet-list child must be *ListItem")
		}
		parent.Items = append(parent.Items, item)
	case *OrderedList:
		item, ok := child.(*ListItem)
		if !ok {
			panic("djot: ordered-list child must be *ListItem")
		}
		parent.Items = append(parent.Items, item)
	case *TaskList:
		item, ok := child.(*TaskListItem)
		if !ok {
			panic("djot: task-list child must be *TaskListItem")
		}
		parent.Items = append(parent.Items, item)
	case *ListItem:
		parent.Children = append(parent.Children, requireBlock(child))
	case *TaskListItem:
		parent.Children = append(parent.Children, requireBlock(child))
	case *DefinitionList:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Term:
		parent.Children = append(parent.Children, requireInline(child))
	case *Definition:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Table:
		parent.Children = append(parent.Children, requireBlock(child))
	case *TableRow:
		cell, ok := child.(*TableCell)
		if !ok {
			panic("djot: table-row child must be *TableCell")
		}
		parent.Cells = append(parent.Cells, cell)
	case *TableCell:
		parent.Children = append(parent.Children, requireInline(child))
	case *Caption:
		parent.Children = append(parent.Children, requireInline(child))
	case *Footnote:
		parent.Children = append(parent.Children, requireBlock(child))
	case *Emphasis:
		parent.Children = append(parent.Children, requireInline(child))
	case *Strong:
		parent.Children = append(parent.Children, requireInline(child))
	case *Superscript:
		parent.Children = append(parent.Children, requireInline(child))
	case *Subscript:
		parent.Children = append(parent.Children, requireInline(child))
	case *Insert:
		parent.Children = append(parent.Children, requireInline(child))
	case *Delete:
		parent.Children = append(parent.Children, requireInline(child))
	case *Mark:
		parent.Children = append(parent.Children, requireInline(child))
	case *Link:
		parent.Children = append(parent.Children, requireInline(child))
	case *Image:
		parent.Children = append(parent.Children, requireInline(child))
	case *Span:
		parent.Children = append(parent.Children, requireInline(child))
	case *DoubleQuoted:
		parent.Children = append(parent.Children, requireInline(child))
	case *SingleQuoted:
		parent.Children = append(parent.Children, requireInline(child))
	default:
		panic("djot: cannot append a child to a leaf node")
	}
}

func requireBlock(node Node) Block {
	block, ok := node.(Block)
	if !ok {
		panic("djot: block child position requires a Block")
	}
	return block
}

func requireInline(node Node) Inline {
	inline, ok := node.(Inline)
	if !ok {
		panic("djot: inline child position requires an Inline")
	}
	return inline
}
