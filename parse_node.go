package djot

import "github.com/danielledeleo/djot-go/ast"

type parseNode struct {
	Kind      ast.Kind
	Children  []*parseNode
	Attrs     map[string]string
	attrOrder []string
	Start     ast.Pos
	End       ast.Pos

	plainBracesUntil int
	Text             string
	*parsePayload
}

type parsePayload struct {
	Level int

	Target    string
	HasTarget bool
	Name      string
	Lang      string
	Format    string

	tight     bool
	Marker    byte
	ListStyle ast.ListStyle
	ListStart int
	Checked   bool
	CellAlign ast.CellAlign
	IsHeader  bool
	Label     string
}

type parseNodeSpec struct {
	Kind      ast.Kind
	Children  []*parseNode
	Attrs     map[string]string
	attrOrder []string
	Start     ast.Pos
	End       ast.Pos

	plainBracesUntil int
	Text             string
	Level            int
	Target           string
	HasTarget        bool
	Name             string
	Lang             string
	Format           string
	tight            bool
	Marker           byte
	ListStyle        ast.ListStyle
	ListStart        int
	Checked          bool
	CellAlign        ast.CellAlign
	IsHeader         bool
	Label            string
}

func parseNodeNeedsPayload(kind ast.Kind) bool {
	switch kind {
	case ast.KindHeading, ast.KindLink, ast.KindImage, ast.KindSymbol, ast.KindVerbatim, ast.KindCodeBlock, ast.KindRawBlock, ast.KindRawInline,
		ast.KindBulletList, ast.KindOrderedList, ast.KindTaskList, ast.KindDefinitionList, ast.KindTaskListItem,
		ast.KindTableRow, ast.KindTableCell, ast.KindFootnote, ast.KindFootnoteReference:
		return true
	default:
		return false
	}
}

func (n *parseNode) Attr(key string) string {
	if n.Attrs == nil {
		return ""
	}
	return n.Attrs[key]
}

func (n *parseNode) SetAttr(key, value string) bool {
	if !isValidAttrKey(key) {
		return false
	}
	if n.Attrs == nil {
		n.Attrs = make(map[string]string)
	}
	if _, exists := n.Attrs[key]; !exists {
		n.attrOrder = append(n.attrOrder, key)
	}
	n.Attrs[key] = value
	return true
}

func (n *parseNode) AddClass(class string) {
	if existing := n.Attr("class"); existing != "" {
		n.SetAttr("class", existing+" "+class)
	} else {
		n.SetAttr("class", class)
	}
}

type parseSlab[T any] struct {
	chunk []T
	next  int
}

const (
	parseArenaMinChunk = 32
	parseArenaMaxChunk = 2048
)

func (a *parseSlab[T]) alloc() *T {
	if len(a.chunk) == cap(a.chunk) {
		size := a.next
		if size < parseArenaMinChunk {
			size = parseArenaMinChunk
		}
		a.chunk = make([]T, 0, size)
		if size < parseArenaMaxChunk {
			a.next = size * 2
		}
	}
	a.chunk = append(a.chunk, *new(T))
	return &a.chunk[len(a.chunk)-1]
}

type parseNodeArena struct {
	nodes    parseSlab[parseNode]
	payloads parseSlab[parsePayload]
}

func (a *parseNodeArena) new(src parseNodeSpec) *parseNode {
	dst := a.nodes.alloc()
	dst.Kind = src.Kind
	dst.Start = src.Start
	dst.End = src.End
	dst.Text = src.Text
	dst.plainBracesUntil = src.plainBracesUntil
	dst.Children = src.Children
	if len(src.Attrs) != 0 {
		dst.Attrs = src.Attrs
		dst.attrOrder = src.attrOrder
	}
	if parseNodeNeedsPayload(src.Kind) {
		payload := a.payloads.alloc()
		payload.Level = src.Level
		payload.Target = src.Target
		payload.HasTarget = src.HasTarget
		payload.Name = src.Name
		payload.Lang = src.Lang
		payload.Format = src.Format
		payload.tight = src.tight
		payload.Marker = src.Marker
		payload.ListStyle = src.ListStyle
		payload.ListStart = src.ListStart
		payload.Checked = src.Checked
		payload.CellAlign = src.CellAlign
		payload.IsHeader = src.IsHeader
		payload.Label = src.Label
		dst.parsePayload = payload
	}
	return dst
}

func walkParse(root *parseNode, fn func(*parseNode)) {
	for _, child := range root.Children {
		fn(child)
		walkParse(child, fn)
	}
}
