package djot

// parseNode is the parser's private mutable workspace. Common tree and text
// state stays inline; fields used only by a minority of kinds are promoted from
// an arena-backed payload. Public Nodes are materialized from the final semantic
// tape and are never used while parsing.
type parseNode struct {
	Kind      Kind
	Children  []*parseNode
	Attrs     map[string]string
	attrOrder []string
	Start     Pos
	End       Pos

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
	ListStyle ListStyle
	ListStart int
	Checked   bool
	CellAlign CellAlign
	IsHeader  bool
	Label     string
}

// parseNodeSpec is a short-lived constructor value. It preserves readable
// parser literals without forcing every live node to carry every payload field.
type parseNodeSpec struct {
	Kind      Kind
	Children  []*parseNode
	Attrs     map[string]string
	attrOrder []string
	Start     Pos
	End       Pos

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
	ListStyle        ListStyle
	ListStart        int
	Checked          bool
	CellAlign        CellAlign
	IsHeader         bool
	Label            string
}

func parseNodeNeedsPayload(kind Kind) bool {
	switch kind {
	case KindHeading, KindLink, KindImage, KindSymbol, KindVerbatim, KindCodeBlock, KindRawBlock, KindRawInline,
		KindBulletList, KindOrderedList, KindTaskList, KindDefinitionList, KindTaskListItem,
		KindTableRow, KindTableCell, KindFootnote, KindFootnoteReference:
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

// new accepts the old Node-shaped constructor value to keep parser creation
// sites concise while storing the result in the private compact shape.
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
