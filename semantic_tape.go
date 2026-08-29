package djot

import (
	"sort"

	"github.com/danielledeleo/djot-go/ast"
)

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
	references []semanticNamedReference
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
	attrs     []semanticAttribute
}

type semanticNamedReference struct {
	name string
	semanticReference
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
	case ast.KindHeading:
		if node.Level < 0 || uint64(node.Level) > uint64(^uint16(0)) {
			panic("djot: semantic tape heading level exceeds uint16 capacity")
		}
		record.small = uint16(node.Level)
	case ast.KindBulletList:
		record.small = uint16(node.Marker)
	case ast.KindOrderedList:
		record.small = uint16(node.ListStyle)
		record.payload = checkedSemanticUint32(len(t.listStarts), "list-start index")
		t.listStarts = append(t.listStarts, node.ListStart)
	case ast.KindTableCell:
		record.small = uint16(node.CellAlign)
	case ast.KindText, ast.KindVerbatim, ast.KindInlineMath, ast.KindDisplayMath, ast.KindSymbol:
		value := node.Text
		if node.Kind == ast.KindSymbol {
			value = node.Name
		}
		record.payload = t.addText(node, value)
	case ast.KindLink, ast.KindImage:
		if node.Target != "" {
			record.payload = checkedSemanticUint32(len(t.targets), "target index")
			t.targets = append(t.targets, node.Target)
		}
	case ast.KindCodeBlock:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: node.Text, extra: node.Lang})
	case ast.KindRawBlock, ast.KindRawInline:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: node.Text, extra: node.Format})
	case ast.KindFootnote, ast.KindFootnoteReference:
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
	t.references = make([]semanticNamedReference, 0, len(references))
	for name, node := range references {
		ref := semanticReference{
			target: node.Target, hasTarget: node.HasTarget,
		}
		for _, key := range node.attrOrder {
			if value, ok := node.Attrs[key]; ok {
				ref.attrs = append(ref.attrs, semanticAttribute{key: key, value: value})
			}
		}
		t.references = append(t.references, semanticNamedReference{name: name, semanticReference: ref})
	}
	sort.Slice(t.references, func(i, j int) bool {
		return t.references[i].name < t.references[j].name
	})
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
