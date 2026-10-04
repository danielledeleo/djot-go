package djot

import (
	"sort"

	"github.com/danielledeleo/djot-go/ast"
)

// compileSemanticTape compiles a typed tree into the compact form; it is the
// inverse of materializeAST. There is no source text, so every text payload is
// stored rather than pointing into a source.
func compileSemanticTape(root *ast.Document, references map[string]*ast.Reference) *semanticTape {
	records, attributes, texts := compileCapacity(root)
	tape := &semanticTape{
		records:    make([]semanticRecord, 0, records+1),
		attributes: make([]semanticAttribute, 0, attributes),
		positions:  make([]semanticPosition, 0, records),
		textSpans:  []semanticSourceSpan{{}},
		textValues: append(make([]string, 0, texts+1), ""),
		targets:    []string{""},
		labels:     []string{""},
		listStarts: []int{0},
		textExtras: []semanticTextExtra{{}},
	}
	tape.compileNode(root)
	tape.records = append(tape.records, semanticRecord{
		attrStart: checkedSemanticUint32(len(tape.attributes), "attribute index"),
	})
	if len(references) > 0 {
		tape.references = make([]semanticNamedReference, 0, len(references))
		for name, ref := range references {
			named := semanticNamedReference{name: name}
			named.target, named.hasTarget = ref.Destination, ref.DestinationSet
			ref.Attributes.Range(func(a ast.Attribute) bool {
				named.attrs = append(named.attrs, semanticAttribute{key: a.Key, value: a.Value})
				return true
			})
			tape.references = append(tape.references, named)
		}
		sort.Slice(tape.references, func(i, j int) bool {
			return tape.references[i].name < tape.references[j].name
		})
	}
	return tape
}

func (t *semanticTape) storeText(value string) uint32 {
	if value == "" {
		return 0
	}
	index := checkedSemanticTextIndex(len(t.textValues), "stored-text index")
	t.textValues = append(t.textValues, value)
	return semanticStoredText | index
}

func (t *semanticTape) compileNode(node ast.Node) {
	index := len(t.records)
	record := semanticRecord{
		kind:      uint8(node.Kind()),
		attrStart: checkedSemanticUint32(len(t.attributes), "attribute index"),
	}
	span := node.Span()
	t.positions = append(t.positions, semanticPosition{
		start: checkedSemanticUint32(max(span.Start.Offset, 0), "start position"),
		end:   checkedSemanticUint32(max(span.End.Offset, 0), "end position"),
	})
	node.Attributes().Range(func(a ast.Attribute) bool {
		t.attributes = append(t.attributes, semanticAttribute{key: a.Key, value: a.Value})
		return true
	})

	switch n := node.(type) {
	case *ast.Heading:
		record.small = uint16(n.Level)
	case *ast.BulletList:
		record.small = uint16(n.Marker)
		if n.Tight {
			record.flags |= semanticTight
		}
	case *ast.OrderedList:
		record.small = uint16(n.Style)
		record.payload = checkedSemanticUint32(len(t.listStarts), "list-start index")
		t.listStarts = append(t.listStarts, n.Start)
		if n.Tight {
			record.flags |= semanticTight
		}
	case *ast.TaskList:
		if n.Tight {
			record.flags |= semanticTight
		}
	case *ast.DefinitionList:
		if n.Tight {
			record.flags |= semanticTight
		}
	case *ast.TaskListItem:
		if n.Checked {
			record.flags |= semanticChecked
		}
	case *ast.TableRow:
		if n.Header {
			record.flags |= semanticHeader
		}
	case *ast.TableCell:
		record.small = uint16(n.Alignment)
		if n.Header {
			record.flags |= semanticHeader
		}
	case *ast.Text:
		record.payload = t.storeText(n.Value)
	case *ast.Verbatim:
		record.payload = t.storeText(n.Text)
	case *ast.InlineMath:
		record.payload = t.storeText(n.Text)
	case *ast.DisplayMath:
		record.payload = t.storeText(n.Text)
	case *ast.Symbol:
		record.payload = t.storeText(n.Name)
	case *ast.Link:
		if n.DestinationSet {
			record.flags |= semanticHasTarget
		}
		if n.Destination != "" {
			record.payload = checkedSemanticUint32(len(t.targets), "target index")
			t.targets = append(t.targets, n.Destination)
		}
	case *ast.Image:
		if n.DestinationSet {
			record.flags |= semanticHasTarget
		}
		if n.Destination != "" {
			record.payload = checkedSemanticUint32(len(t.targets), "target index")
			t.targets = append(t.targets, n.Destination)
		}
	case *ast.CodeBlock:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: n.Text, extra: n.Language})
	case *ast.RawBlock:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: n.Text, extra: n.Format})
	case *ast.RawInline:
		record.payload = checkedSemanticUint32(len(t.textExtras), "text-extra index")
		t.textExtras = append(t.textExtras, semanticTextExtra{text: n.Text, extra: n.Format})
	case *ast.Footnote:
		record.payload = checkedSemanticUint32(len(t.labels), "label index")
		t.labels = append(t.labels, n.Label)
	case *ast.FootnoteReference:
		record.payload = checkedSemanticUint32(len(t.labels), "label index")
		t.labels = append(t.labels, n.Label)
	}

	t.records = append(t.records, record)
	ast.ForEachChild(node, func(child ast.Node) { t.compileNode(child) })
	t.records[index].subtreeEnd = checkedSemanticUint32(len(t.records), "record index")
}

func compileCapacity(node ast.Node) (records, attributes, texts int) {
	records, attributes = 1, node.Attributes().Len()
	switch node.(type) {
	case *ast.Text, *ast.Verbatim, *ast.InlineMath, *ast.DisplayMath, *ast.Symbol:
		texts = 1
	}
	ast.ForEachChild(node, func(child ast.Node) {
		r, a, x := compileCapacity(child)
		records, attributes, texts = records+r, attributes+a, texts+x
	})
	return records, attributes, texts
}
