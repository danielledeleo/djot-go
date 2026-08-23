package djot_test

import (
	"reflect"
	"runtime"
	"testing"
	"unsafe"

	"github.com/danielledeleo/djot-go"
)

// semanticTapePrototype is a flattened, read-only semantic document. It is a
// representation experiment: the current parser still builds its AST first,
// then this benchmark mirrors it into the tape. A production implementation
// would have the block and inline parsers append these records directly.
//
// Records are preorder. subtreeEnd is the first record after the node's
// subtree, so consumers can skip a subtree without following pointers. Payloads
// and attributes live in side tables because most node kinds need neither.
type semanticTapePrototype struct {
	records    []semanticRecordPrototype
	payloads   []semanticPayloadPrototype
	attributes []semanticAttributePrototype
}

// semanticRecordPrototype deliberately stays at 16 bytes on 64-bit targets.
// kind-specific scalar values use small and flags; larger/string values use a
// payload side-table entry. attrStart for record i and record i+1 delimit the
// attributes for i. A final sentinel record supplies the last end offset.
type semanticRecordPrototype struct {
	kind       uint8
	flags      uint8
	small      uint16
	subtreeEnd uint32
	payload    uint32
	attrStart  uint32
}

type semanticPayloadPrototype struct {
	text   string
	target string
	extra  string
	number int32
}

type semanticAttributePrototype struct {
	key   string
	value string
}

const (
	semanticFlagHasTarget = 1 << iota
	semanticFlagChecked
	semanticFlagHeader
	semanticFlagTight
)

func buildSemanticTapePrototype(doc *djot.Doc) *semanticTapePrototype {
	tape := &semanticTapePrototype{
		// Index zero means "no payload". Keeping a real zero entry avoids a
		// parallel presence bit in every record.
		payloads: []semanticPayloadPrototype{{}},
	}
	appendSemanticRecordPrototype(tape, doc.Root())
	// Sentinel: its attrStart terminates the preceding record's attribute span.
	tape.records = append(tape.records, semanticRecordPrototype{
		attrStart: uint32(len(tape.attributes)),
	})
	return tape
}

// buildHintedSemanticTapePrototype uses only information available before a
// direct parse: input byte length. The ratios are deliberately approximate;
// slices still grow normally for unusually node-dense inputs.
func buildHintedSemanticTapePrototype(doc *djot.Doc) *semanticTapePrototype {
	inputBytes := 0
	for i := range doc.Files {
		inputBytes += len(doc.Files[i].Source)
	}
	payloadHint := inputBytes / 12
	if payloadHint < 1 {
		payloadHint = 1
	}
	tape := &semanticTapePrototype{
		records:    make([]semanticRecordPrototype, 0, inputBytes/7),
		payloads:   make([]semanticPayloadPrototype, 1, payloadHint),
		attributes: make([]semanticAttributePrototype, 0, inputBytes/512),
	}
	appendSemanticRecordPrototype(tape, doc.Root())
	tape.records = append(tape.records, semanticRecordPrototype{
		attrStart: uint32(len(tape.attributes)),
	})
	return tape
}

func appendSemanticRecordPrototype(tape *semanticTapePrototype, node *djot.Node) {
	index := len(tape.records)
	record := semanticRecordPrototype{
		kind:      uint8(node.Kind),
		attrStart: uint32(len(tape.attributes)),
	}

	// The current public AST keeps insertion order privately. Reflection is
	// confined to this benchmark adapter; a direct parser would append ordered
	// attributes to the side table as it recognizes them.
	order := reflect.ValueOf(node).Elem().FieldByName("attrOrder")
	for i := 0; i < order.Len(); i++ {
		key := order.Index(i).String()
		value, ok := node.Attrs[key]
		if !ok {
			continue
		}
		tape.attributes = append(tape.attributes, semanticAttributePrototype{
			key: key, value: value,
		})
	}
	if reflect.ValueOf(node).Elem().FieldByName("tight").Bool() {
		record.flags |= semanticFlagTight
	}

	switch node.Kind {
	case djot.Heading:
		record.small = uint16(node.Level)
	case djot.OrderedList:
		record.small = uint16(node.ListStyle)
	case djot.TableCell:
		record.small = uint16(node.CellAlign)
		if node.IsHeader {
			record.flags |= semanticFlagHeader
		}
	case djot.TaskListItem:
		if node.Checked {
			record.flags |= semanticFlagChecked
		}
	}
	if node.HasTarget {
		record.flags |= semanticFlagHasTarget
	}

	if node.Text != "" || node.Target != "" || node.Name != "" ||
		node.Lang != "" || node.Format != "" || node.Label != "" ||
		node.ListStart != 0 {
		record.payload = uint32(len(tape.payloads))
		text := node.Text
		if text == "" {
			text = node.Name
		}
		extra := node.Lang
		if extra == "" {
			extra = node.Format
		}
		if extra == "" {
			extra = node.Label
		}
		tape.payloads = append(tape.payloads, semanticPayloadPrototype{
			text: text, target: node.Target, extra: extra,
			number: int32(node.ListStart),
		})
	}

	tape.records = append(tape.records, record)
	for _, child := range node.Children {
		appendSemanticRecordPrototype(tape, child)
	}
	tape.records[index].subtreeEnd = uint32(len(tape.records))
}

func scanSemanticTapePrototype(tape *semanticTapePrototype) uint64 {
	var checksum uint64
	// The last record is the attribute sentinel, not a semantic node.
	for i := 0; i+1 < len(tape.records); i++ {
		record := tape.records[i]
		checksum += uint64(record.kind) + uint64(record.subtreeEnd)
		if record.payload != 0 {
			payload := tape.payloads[record.payload]
			checksum += uint64(len(payload.text) + len(payload.target) + len(payload.extra))
		}
		for j := record.attrStart; j < tape.records[i+1].attrStart; j++ {
			attr := tape.attributes[j]
			checksum += uint64(len(attr.key) + len(attr.value))
		}
	}
	return checksum
}

func BenchmarkSemanticTapePrototype(b *testing.B) {
	parsed := djot.Parse(hugeDoc())

	b.Run("Shape/Huge_1MB", func(b *testing.B) {
		tape := buildSemanticTapePrototype(parsed)
		records := len(tape.records) - 1
		logicalBytes := len(tape.records)*int(unsafe.Sizeof(semanticRecordPrototype{})) +
			len(tape.payloads)*int(unsafe.Sizeof(semanticPayloadPrototype{})) +
			len(tape.attributes)*int(unsafe.Sizeof(semanticAttributePrototype{}))
		b.ReportMetric(float64(records), "records/doc")
		b.ReportMetric(float64(len(tape.payloads)-1), "payloads/doc")
		b.ReportMetric(float64(len(tape.attributes)), "attributes/doc")
		b.ReportMetric(float64(logicalBytes), "logical-B/doc")
		b.ReportMetric(float64(unsafe.Sizeof(semanticRecordPrototype{})), "record-B")
		for i := 0; i < b.N; i++ {
			runtime.KeepAlive(tape)
		}
	})

	b.Run("BuildFromAST/Unhinted/Huge_1MB", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tape := buildSemanticTapePrototype(parsed)
			runtime.KeepAlive(tape)
		}
	})
	b.Run("BuildFromAST/SourceHint/Huge_1MB", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tape := buildHintedSemanticTapePrototype(parsed)
			runtime.KeepAlive(tape)
		}
	})

	b.Run("Retained/SourceHint/Huge_1MB", func(b *testing.B) {
		var retained uint64
		for i := 0; i < b.N; i++ {
			retained += measureRetainedSemanticTapePrototype(parsed)
		}
		b.ReportMetric(float64(retained)/float64(b.N), "retained-B/doc")
	})

	tape := buildSemanticTapePrototype(parsed)
	b.Run("Scan/Huge_1MB", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var checksum uint64
		for i := 0; i < b.N; i++ {
			checksum += scanSemanticTapePrototype(tape)
		}
		runtime.KeepAlive(checksum)
	})
}

func measureRetainedSemanticTapePrototype(parsed *djot.Doc) uint64 {
	warm := buildHintedSemanticTapePrototype(parsed)
	runtime.KeepAlive(warm)
	warm = nil
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	tape := buildHintedSemanticTapePrototype(parsed)
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(tape)
	runtime.KeepAlive(parsed)
	if after.HeapAlloc <= before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}
