package djot

// Action controls the walker's behavior after visiting a node.
// It is the return type for the [Continue], [SkipChildren], and [Remove] constants.
type Action int

const (
	Continue     Action = iota // recurse into children
	SkipChildren               // do not visit children
	Remove                     // remove this node from its parent
)

// ReplaceAction is returned by filters that swap a node for a replacement.
type ReplaceAction struct {
	Replacement *Node
}

// FilterFunc is called for each node during a walk.
// Return Continue, SkipChildren, Remove, or call Replace().
type FilterFunc func(n *Node) any

// Replace returns an action that swaps the current node for a replacement.
// The walker will visit the replacement's children.
func Replace(n *Node) ReplaceAction {
	return ReplaceAction{Replacement: n}
}

// Walk traverses the tree top-down (pre-order). The filter sees each node
// before its children. Supports Replace, SkipChildren, Remove, Continue.
func Walk(n *Node, fn FilterFunc) {
	walkChildren(n, fn)
}

func walkChildren(n *Node, fn FilterFunc) {
	i := 0
	for i < len(n.Children) {
		child := n.Children[i]
		result := fn(child)

		switch v := result.(type) {
		case Action:
			switch v {
			case Continue:
				walkChildren(child, fn)
				i++
			case SkipChildren:
				i++
			case Remove:
				n.Children = append(n.Children[:i], n.Children[i+1:]...)
				// don't increment i
			}
		case ReplaceAction:
			n.Children[i] = v.Replacement
			walkChildren(v.Replacement, fn)
			i++
		default:
			walkChildren(child, fn)
			i++
		}
	}
}

// WalkBottomUp traverses the tree bottom-up (post-order). Children are
// visited before their parent. Best for collection and validation passes
// where you want children fully resolved before inspecting the parent.
// Does not support Replace — mutations should happen between passes.
func WalkBottomUp(n *Node, fn func(n *Node)) {
	for _, child := range n.Children {
		WalkBottomUp(child, fn)
	}
	fn(n)
}
