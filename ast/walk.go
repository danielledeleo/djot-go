package ast

type actionKind uint8

const (
	actionContinue actionKind = iota
	actionSkipChildren
	actionRemove
	actionReplace
)

// Action controls typed tree transformation during Walk.
type Action struct {
	kind        actionKind
	replacement Node
}

var (
	// Continue keeps the node and visits its children.
	Continue = Action{kind: actionContinue}
	// SkipChildren keeps the node without visiting its children.
	SkipChildren = Action{kind: actionSkipChildren}
	// Remove deletes the node from its occupied tree slot.
	Remove = Action{kind: actionRemove}
)

// Replace returns an action that substitutes node in its occupied tree slot.
// The replacement must have the same grammar category: document, block, or
// inline. Walk visits the replacement's children but does not call the filter
// on the replacement itself.
func Replace(node Node) Action {
	requireNode(node)
	return Action{kind: actionReplace, replacement: node}
}

// FilterFunc is called for each node during Walk.
type FilterFunc func(Node) Action

// Preorder visits root and its descendants without mutating the tree. It stops
// when visit returns false. Preorder performs no allocations.
func Preorder(root Node, visit func(Node) bool) {
	if root == nil {
		return
	}
	preorder(root, visit)
}

func preorder(node Node, visit func(Node) bool) bool {
	requireNode(node)
	if !visit(node) {
		return false
	}
	keepGoing := true
	ForEachChild(node, func(child Node) {
		if keepGoing {
			keepGoing = preorder(child, visit)
		}
	})
	return keepGoing
}

// Walk transforms a tree top-down and returns the possibly replaced root. The
// supplied root is visited. Removing it returns nil. Callers replacing a
// document root should type-assert the result to *Document before passing it
// to djot.Doc.SetRoot.
func Walk(root Node, fn FilterFunc) Node {
	if root == nil {
		return nil
	}
	result, keep := walkNode(root, fn, nodeCategory(root))
	if !keep {
		return nil
	}
	return result
}

func walkNode(node Node, fn FilterFunc, category childCategory) (Node, bool) {
	requireNode(node)
	action := fn(node)
	switch action.kind {
	case actionContinue:
		walkTypedChildren(node, fn)
		return node, true
	case actionSkipChildren:
		return node, true
	case actionRemove:
		return nil, false
	case actionReplace:
		replacement := action.replacement
		if nodeCategory(replacement) != category {
			panic("djot: replacement node has the wrong grammar category")
		}
		walkTypedChildren(replacement, fn)
		return replacement, true
	default:
		panic("djot: invalid Walk action")
	}
}

type childCategory uint8

const (
	categoryDocument childCategory = iota
	categoryBlock
	categoryInline
)

func nodeCategory(node Node) childCategory {
	switch node.(type) {
	case *Document:
		return categoryDocument
	case Block:
		return categoryBlock
	case Inline:
		return categoryInline
	default:
		panic("djot: unknown node category")
	}
}

func walkTypedChildren(node Node, fn FilterFunc) {
	switch node := node.(type) {
	case *Document:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Section:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Paragraph:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Heading:
		node.Children = walkInlineSlice(node.Children, fn)
	case *BlockQuote:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Div:
		node.Children = walkBlockSlice(node.Children, fn)
	case *BulletList:
		node.Items = walkListItemSlice(node.Items, fn)
	case *OrderedList:
		node.Items = walkListItemSlice(node.Items, fn)
	case *TaskList:
		node.Items = walkTaskListItemSlice(node.Items, fn)
	case *ListItem:
		node.Children = walkBlockSlice(node.Children, fn)
	case *TaskListItem:
		node.Children = walkBlockSlice(node.Children, fn)
	case *DefinitionList:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Term:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Definition:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Table:
		node.Children = walkBlockSlice(node.Children, fn)
	case *TableRow:
		node.Cells = walkTableCellSlice(node.Cells, fn)
	case *TableCell:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Caption:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Footnote:
		node.Children = walkBlockSlice(node.Children, fn)
	case *Emphasis:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Strong:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Superscript:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Subscript:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Insert:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Delete:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Mark:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Link:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Image:
		node.Children = walkInlineSlice(node.Children, fn)
	case *Span:
		node.Children = walkInlineSlice(node.Children, fn)
	case *DoubleQuoted:
		node.Children = walkInlineSlice(node.Children, fn)
	case *SingleQuoted:
		node.Children = walkInlineSlice(node.Children, fn)
	}
}

func walkBlockSlice(children []Block, fn FilterFunc) []Block {
	result := children[:0]
	for _, child := range children {
		next, keep := walkNode(child, fn, categoryBlock)
		if keep {
			result = append(result, requireBlock(next))
		}
	}
	return result
}

func walkInlineSlice(children []Inline, fn FilterFunc) []Inline {
	result := children[:0]
	for _, child := range children {
		next, keep := walkNode(child, fn, categoryInline)
		if keep {
			result = append(result, requireInline(next))
		}
	}
	return result
}

func walkListItemSlice(children []*ListItem, fn FilterFunc) []*ListItem {
	result := children[:0]
	for _, child := range children {
		next, keep := walkNode(child, fn, categoryBlock)
		if keep {
			item, ok := next.(*ListItem)
			if !ok {
				panic("djot: list replacement must be *ListItem")
			}
			result = append(result, item)
		}
	}
	return result
}

func walkTaskListItemSlice(children []*TaskListItem, fn FilterFunc) []*TaskListItem {
	result := children[:0]
	for _, child := range children {
		next, keep := walkNode(child, fn, categoryBlock)
		if keep {
			item, ok := next.(*TaskListItem)
			if !ok {
				panic("djot: task-list replacement must be *TaskListItem")
			}
			result = append(result, item)
		}
	}
	return result
}

func walkTableCellSlice(children []*TableCell, fn FilterFunc) []*TableCell {
	result := children[:0]
	for _, child := range children {
		next, keep := walkNode(child, fn, categoryBlock)
		if keep {
			cell, ok := next.(*TableCell)
			if !ok {
				panic("djot: table-row replacement must be *TableCell")
			}
			result = append(result, cell)
		}
	}
	return result
}

// WalkBottomUp visits every node postorder, including root.
func WalkBottomUp(root Node, fn func(Node)) {
	if root == nil {
		return
	}
	ForEachChild(root, func(child Node) { WalkBottomUp(child, fn) })
	fn(root)
}
