package djot

// semanticTape is the compact, immutable rendering representation attached to
// parser-produced documents. It is deliberately private: the public mutable
// AST remains authoritative whenever its render-visible semantics differ.
type semanticTape struct {
	source     string
	records    []semanticRecord
	attributes []semanticAttribute
	textSpans  []semanticSourceSpan
	textValues []string
	targets    []string
	labels     []string
	listStarts []int
	textExtras []semanticTextExtra
	positions  []semanticPosition
	references map[string]semanticReference
}

// semanticRecord is a 16-byte preorder record. payload is a semantic union:
// its meaning is determined by kind.
type semanticRecord struct {
	kind       uint8
	flags      uint8
	small      uint16
	subtreeEnd uint32
	payload    uint32
	attrStart  uint32
}

type semanticAttribute struct {
	key   string
	value string
}

type semanticSourceSpan struct {
	start uint32
	end   uint32
}

type semanticTextExtra struct {
	text  string
	extra string
}

type semanticPosition struct {
	start uint32
	end   uint32
}

type semanticReference struct {
	target    string
	hasTarget bool
	label     string
	attrs     []semanticAttribute
}

const (
	semanticHasTarget = 1 << iota
	semanticChecked
	semanticHeader
	semanticTight
)

const semanticStoredText = uint32(1 << 31)

func checkedSemanticUint32(value int, field string) uint32 {
	if value < 0 || uint64(value) > uint64(^uint32(0)) {
		panic("djot: semantic tape " + field + " exceeds uint32 capacity")
	}
	return uint32(value)
}

func checkedSemanticTextIndex(value int, field string) uint32 {
	if value < 0 || uint64(value) >= uint64(semanticStoredText) {
		panic("djot: semantic tape " + field + " exceeds 31-bit text index capacity")
	}
	return uint32(value)
}

func newSemanticTape(root *parseNode, source string) *semanticTape {
	checkedSemanticUint32(len(source), "source size")
	recordCount, attributeCount := semanticCapacity(root)
	checkedSemanticUint32(recordCount+1, "record count")
	checkedSemanticUint32(attributeCount, "attribute count")
	tape := &semanticTape{
		source:     source,
		records:    make([]semanticRecord, 0, recordCount+1),
		attributes: make([]semanticAttribute, 0, attributeCount),
		textSpans:  []semanticSourceSpan{{}},
		textValues: []string{""},
		targets:    []string{""},
		labels:     []string{""},
		listStarts: []int{0},
		textExtras: []semanticTextExtra{{}},
		positions:  make([]semanticPosition, 0, recordCount),
	}
	tape.appendNode(root)
	// A sentinel supplies the final attribute end offset.
	tape.records = append(tape.records, semanticRecord{
		attrStart: checkedSemanticUint32(len(tape.attributes), "attribute index"),
	})
	return tape
}

// semanticCapacity makes finalization independent of source density. A short
// counting walk avoids both sparse-document over-allocation and dense-document
// slice growth while the private parser tree is still available.
func semanticCapacity(node *parseNode) (records, attributes int) {
	records = 1
	for _, key := range node.attrOrder {
		if _, ok := node.Attrs[key]; ok {
			attributes++
		}
	}
	for _, child := range node.Children {
		childRecords, childAttributes := semanticCapacity(child)
		records += childRecords
		attributes += childAttributes
	}
	return records, attributes
}

func (t *semanticTape) appendNode(node *parseNode) {
	index := len(t.records)
	if node.Kind < 0 || uint64(node.Kind) > uint64(^uint8(0)) {
		panic("djot: semantic tape node kind exceeds uint8 capacity")
	}
	record := semanticRecord{
		kind:      uint8(node.Kind),
		attrStart: checkedSemanticUint32(len(t.attributes), "attribute index"),
	}
	t.positions = append(t.positions, semanticPosition{
		start: checkedSemanticUint32(node.Start.Offset, "start position"),
		end:   checkedSemanticUint32(node.End.Offset, "end position"),
	})
	for _, key := range node.attrOrder {
		if value, ok := node.Attrs[key]; ok {
			t.attributes = append(t.attributes, semanticAttribute{key: key, value: value})
		}
	}
	if node.parsePayload != nil {
		if node.HasTarget {
			record.flags |= semanticHasTarget
		}
		if node.Checked {
			record.flags |= semanticChecked
		}
		if node.IsHeader {
			record.flags |= semanticHeader
		}
		if node.tight {
			record.flags |= semanticTight
		}
	}

	switch node.Kind {
	case Heading:
		if node.Level < 0 || uint64(node.Level) > uint64(^uint16(0)) {
			panic("djot: semantic tape heading level exceeds uint16 capacity")
		}
		record.small = uint16(node.Level)
	case BulletList:
		record.small = uint16(node.Marker)
	case OrderedList:
		record.small = uint16(node.ListStyle)
		record.payload = checkedSemanticUint32(len(t.listStarts), "list-start index")
		t.listStarts = append(t.listStarts, node.ListStart)
	case TableCell:
		record.small = uint16(node.CellAlign)
	case Text, Verbatim, InlineMath, DisplayMath, Symbol:
		value := node.Text
		if node.Kind == Symbol {
			value = node.Name
		}
		record.payload = t.addText(node, value)
	case Link, Image:
		if node.Target != "" {
			record.payload = checkedSemanticUint32(len(t.targets), "target index")
			t.targets = append(t.targets, node.Target)
		}
	case CodeBlock:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: node.Text, extra: node.Lang})
	case RawBlock, RawInline:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: node.Text, extra: node.Format})
	case Footnote, FootnoteReference:
		record.payload = checkedSemanticUint32(len(t.labels), "label index")
		t.labels = append(t.labels, node.Label)
	}

	t.records = append(t.records, record)
	for _, child := range node.Children {
		t.appendNode(child)
	}
	t.records[index].subtreeEnd = checkedSemanticUint32(len(t.records), "record index")
}

func (t *semanticTape) captureReferences(references map[string]*parseNode) {
	if len(references) == 0 {
		return
	}
	t.references = make(map[string]semanticReference, len(references))
	for name, node := range references {
		ref := semanticReference{
			target: node.Target, hasTarget: node.HasTarget, label: node.Label,
		}
		for _, key := range node.attrOrder {
			if value, ok := node.Attrs[key]; ok {
				ref.attrs = append(ref.attrs, semanticAttribute{key: key, value: value})
			}
		}
		t.references[name] = ref
	}
}

func (t *semanticTape) materializeAST() *Node {
	if t == nil || len(t.records) < 2 {
		return nil
	}
	var arena nodeArena
	root, _ := t.materializeNode(0, &arena)
	return root
}

func (t *semanticTape) materializeNode(index int, arena *nodeArena) (*Node, int) {
	record := t.records[index]
	position := t.positions[index]
	node := arena.new(Node{
		Kind:  NodeKind(record.kind),
		Start: Pos{Offset: int(position.start)},
		End:   Pos{Offset: int(position.end)},
	})
	for j, end := record.attrStart, t.records[index+1].attrStart; j < end; j++ {
		attr := t.attributes[j]
		node.SetAttr(attr.key, attr.value)
	}
	node.HasTarget = record.flags&semanticHasTarget != 0
	node.Checked = record.flags&semanticChecked != 0
	node.IsHeader = record.flags&semanticHeader != 0
	node.tight = record.flags&semanticTight != 0

	switch node.Kind {
	case Heading:
		node.Level = int(record.small)
	case BulletList:
		node.Marker = byte(record.small)
	case OrderedList:
		node.ListStyle = ListStyle(record.small)
		node.ListStart = t.listStarts[record.payload]
	case TableCell:
		node.CellAlign = CellAlign(record.small)
	case Text, Verbatim, InlineMath, DisplayMath:
		node.Text = t.text(record.payload)
	case Symbol:
		node.Name = t.text(record.payload)
	case Link, Image:
		if record.payload != 0 {
			node.Target = t.targets[record.payload]
		}
	case CodeBlock:
		payload := t.textExtras[record.payload]
		node.Text, node.Lang = payload.text, payload.extra
	case RawBlock, RawInline:
		payload := t.textExtras[record.payload]
		node.Text, node.Format = payload.text, payload.extra
	case Footnote, FootnoteReference:
		node.Label = t.labels[record.payload]
	}

	next := index + 1
	for next < int(record.subtreeEnd) {
		child, after := t.materializeNode(next, arena)
		node.Children = append(node.Children, child)
		next = after
	}
	return node, next
}

func (t *semanticTape) materializeReferences() map[string]*Node {
	result := make(map[string]*Node, len(t.references))
	for name, ref := range t.references {
		node := &Node{Kind: Link, Target: ref.target, HasTarget: ref.hasTarget, Label: ref.label}
		for _, attr := range ref.attrs {
			node.SetAttr(attr.key, attr.value)
		}
		result[name] = node
	}
	return result
}

func (t *semanticTape) addText(node *parseNode, value string) uint32 {
	if value == "" {
		return 0
	}
	start, end := node.Start.Offset, node.End.Offset+1
	if start >= 0 && end >= start && end <= len(t.source) && t.source[start:end] == value {
		index := checkedSemanticTextIndex(len(t.textSpans), "source-text index")
		t.textSpans = append(t.textSpans, semanticSourceSpan{start: uint32(start), end: uint32(end)})
		return index
	}
	index := checkedSemanticTextIndex(len(t.textValues), "stored-text index")
	t.textValues = append(t.textValues, value)
	return semanticStoredText | index
}

func (t *semanticTape) text(id uint32) string {
	if id == 0 {
		return ""
	}
	if id&semanticStoredText != 0 {
		return t.textValues[id&^semanticStoredText]
	}
	span := t.textSpans[id]
	return t.source[span.start:span.end]
}

// matchesAST performs an exact comparison of every field that can affect the
// default HTML renderer. It lets the v0-compatible exported mutable AST coexist
// safely with a cached tape: changed documents fall back to AST rendering.
func (t *semanticTape) matchesAST(root *Node) bool {
	if t == nil || root == nil || len(t.records) < 2 {
		return false
	}
	index, ok := t.matchNode(root, 0)
	return ok && index == len(t.records)-1
}

func (t *semanticTape) matchNode(node *Node, index int) (int, bool) {
	if index+1 >= len(t.records) {
		return index, false
	}
	record := t.records[index]
	if NodeKind(record.kind) != node.Kind || !t.matchAttrs(node, index) {
		return index, false
	}
	if (record.flags&semanticHasTarget != 0) != node.HasTarget ||
		(record.flags&semanticChecked != 0) != node.Checked ||
		(record.flags&semanticHeader != 0) != node.IsHeader ||
		(record.flags&semanticTight != 0) != node.tight {
		return index, false
	}

	switch node.Kind {
	case Heading:
		if int(record.small) != node.Level {
			return index, false
		}
	case BulletList:
		if byte(record.small) != node.Marker {
			return index, false
		}
	case OrderedList:
		if ListStyle(record.small) != node.ListStyle || t.listStarts[record.payload] != node.ListStart {
			return index, false
		}
	case TableCell:
		if CellAlign(record.small) != node.CellAlign {
			return index, false
		}
	case Text, Verbatim, InlineMath, DisplayMath:
		if t.text(record.payload) != node.Text {
			return index, false
		}
	case Symbol:
		if t.text(record.payload) != node.Name {
			return index, false
		}
	case Link, Image:
		target := ""
		if record.payload != 0 {
			target = t.targets[record.payload]
		}
		if target != node.Target {
			return index, false
		}
	case CodeBlock:
		payload := t.textExtras[record.payload]
		if payload.text != node.Text || payload.extra != node.Lang {
			return index, false
		}
	case RawBlock, RawInline:
		payload := t.textExtras[record.payload]
		if payload.text != node.Text || payload.extra != node.Format {
			return index, false
		}
	case Footnote, FootnoteReference:
		if t.labels[record.payload] != node.Label {
			return index, false
		}
	}

	next := index + 1
	for _, child := range node.Children {
		var ok bool
		next, ok = t.matchNode(child, next)
		if !ok {
			return index, false
		}
	}
	if next != int(record.subtreeEnd) {
		return index, false
	}
	return next, true
}

func (t *semanticTape) matchAttrs(node *Node, index int) bool {
	start := int(t.records[index].attrStart)
	end := int(t.records[index+1].attrStart)
	current := start
	for _, key := range node.attrOrder {
		value, ok := node.Attrs[key]
		if !ok {
			continue
		}
		if current >= end || t.attributes[current].key != key || t.attributes[current].value != value {
			return false
		}
		current++
	}
	return current == end
}
