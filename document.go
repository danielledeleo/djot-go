package djot

import (
	"sync"

	"github.com/danielledeleo/djot-go/ast"
)

// Doc is the top-level parsed document. Its compact semantic representation is
// rendered directly; Root, Footnotes, and References materialize typed views on
// demand.
type Doc struct {
	Files []ast.FileInfo

	mu            sync.RWMutex
	root          *ast.Document
	rootRequested bool
	references    map[string]*ast.Reference
	semantic      *semanticTape

	parseRoot       *parseNode
	parseReferences map[string]*parseNode
}

// NewDoc constructs a document from an existing mutable typed tree.
func NewDoc(root *ast.Document) *Doc {
	return &Doc{root: root, rootRequested: true}
}

// Root returns the shared mutable typed tree, materializing it on first use.
// Repeated calls return the same root. Concurrent read-only materialization is
// safe; callers must synchronize subsequent tree mutations with other users of
// the document.
func (d *Doc) Root() *ast.Document {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rootRequested = true
	return d.materializeRootLocked()
}

func (d *Doc) materializeRootLocked() *ast.Document {
	if d.root == nil && d.semantic != nil {
		d.root = d.semantic.materializeAST()
	}
	return d.root
}

// SetRoot replaces the document tree and discards compact rendering caches.
func (d *Doc) SetRoot(root *ast.Document) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.root = root
	d.rootRequested = true
	d.semantic = nil
	d.references = nil
}

func (d *Doc) semanticRenderSnapshot() (tape *semanticTape, root *ast.Document, direct bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.semantic != nil && !d.rootRequested {
		return d.semantic, nil, true
	}
	return d.semantic, d.root, false
}

// Position resolves a source position to a filename, line, and column.
func (d *Doc) Position(position ast.Pos) (file string, line, col int) {
	info := &d.Files[position.File]
	line, col = info.Position(position.Offset)
	return info.Path, line, col
}

// Footnotes returns a newly built footnote-definition index for the current
// typed tree. Mutating the returned map does not modify the tree; changing a
// Footnote through Root is reflected the next time Footnotes is called.
func (d *Doc) Footnotes() map[string]*ast.Footnote {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rootRequested = true
	footnotes := make(map[string]*ast.Footnote)
	if root := d.materializeRootLocked(); root != nil {
		ast.Preorder(root, func(node ast.Node) bool {
			if footnote, ok := node.(*ast.Footnote); ok {
				footnotes[footnote.Label] = footnote
			}
			return true
		})
	}
	return footnotes
}

// References returns the shared mutable resolved-reference index,
// materializing it on first use. Reference definitions are document metadata;
// links and images already contain their resolved destinations.
func (d *Doc) References() map[string]*ast.Reference {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.references == nil {
		if d.semantic != nil {
			d.references = d.semantic.materializeReferences()
		} else {
			d.references = make(map[string]*ast.Reference)
		}
	}
	return d.references
}
