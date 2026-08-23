package djot_test

import (
	"encoding/binary"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/danielledeleo/djot-go"
)

// semanticColumnsPrototype is a pure structure-of-arrays alternative to the
// 16-byte semanticRecordPrototype. Its logical storage is also 16 bytes per
// record, but kind-only passes touch one narrow stream instead of whole records.
type semanticColumnsPrototype struct {
	kindFlags  []uint16
	small      []uint16
	subtreeEnd []uint32
	payload    []uint32
	attrStart  []uint32
}

func columnsFromTapePrototype(tape *semanticTapePrototype) semanticColumnsPrototype {
	n := len(tape.records)
	columns := semanticColumnsPrototype{
		kindFlags:  make([]uint16, n),
		small:      make([]uint16, n),
		subtreeEnd: make([]uint32, n),
		payload:    make([]uint32, n),
		attrStart:  make([]uint32, n),
	}
	for i, record := range tape.records {
		columns.kindFlags[i] = uint16(record.kind) | uint16(record.flags)<<8
		columns.small[i] = record.small
		columns.subtreeEnd[i] = record.subtreeEnd
		columns.payload[i] = record.payload
		columns.attrStart[i] = record.attrStart
	}
	return columns
}

func scanAoSKindsPrototype(tape *semanticTapePrototype) uint64 {
	var checksum uint64
	for i := 0; i+1 < len(tape.records); i++ {
		checksum += uint64(tape.records[i].kind)
	}
	return checksum
}

func scanSoAKindsPrototype(columns semanticColumnsPrototype) uint64 {
	var checksum uint64
	for i := 0; i+1 < len(columns.kindFlags); i++ {
		checksum += uint64(uint8(columns.kindFlags[i]))
	}
	return checksum
}

func scanSoAFullPrototype(columns semanticColumnsPrototype, tape *semanticTapePrototype) uint64 {
	var checksum uint64
	for i := 0; i+1 < len(columns.kindFlags); i++ {
		checksum += uint64(uint8(columns.kindFlags[i])) + uint64(columns.subtreeEnd[i])
		if payloadIndex := columns.payload[i]; payloadIndex != 0 {
			payload := tape.payloads[payloadIndex]
			checksum += uint64(len(payload.text) + len(payload.target) + len(payload.extra))
		}
		for j := columns.attrStart[i]; j < columns.attrStart[i+1]; j++ {
			attr := tape.attributes[j]
			checksum += uint64(len(attr.key) + len(attr.value))
		}
	}
	return checksum
}

type splitPayloadRefPrototype struct {
	textID   uint32
	targetID uint32
	extraID  uint32
	number   int32
}

type splitPayloadsPrototype struct {
	refs    []splitPayloadRefPrototype
	texts   []string
	targets []string
	extras  []string
}

func splitPayloadsFromTapePrototype(tape *semanticTapePrototype) splitPayloadsPrototype {
	textCount, targetCount, extraCount := 1, 1, 1
	for _, payload := range tape.payloads[1:] {
		if payload.text != "" {
			textCount++
		}
		if payload.target != "" {
			targetCount++
		}
		if payload.extra != "" {
			extraCount++
		}
	}
	store := splitPayloadsPrototype{
		refs:    make([]splitPayloadRefPrototype, len(tape.payloads)),
		texts:   make([]string, 1, textCount),
		targets: make([]string, 1, targetCount),
		extras:  make([]string, 1, extraCount),
	}
	for i, payload := range tape.payloads[1:] {
		ref := splitPayloadRefPrototype{number: payload.number}
		if payload.text != "" {
			ref.textID = uint32(len(store.texts))
			store.texts = append(store.texts, payload.text)
		}
		if payload.target != "" {
			ref.targetID = uint32(len(store.targets))
			store.targets = append(store.targets, payload.target)
		}
		if payload.extra != "" {
			ref.extraID = uint32(len(store.extras))
			store.extras = append(store.extras, payload.extra)
		}
		store.refs[i+1] = ref
	}
	return store
}

func splitPayloadLogicalBytesPrototype(store splitPayloadsPrototype) int {
	return len(store.refs)*int(unsafe.Sizeof(splitPayloadRefPrototype{})) +
		(len(store.texts)+len(store.targets)+len(store.extras))*int(unsafe.Sizeof(""))
}

func scanSplitPayloadsPrototype(store splitPayloadsPrototype) uint64 {
	var checksum uint64
	for _, ref := range store.refs[1:] {
		if ref.textID != 0 {
			checksum += uint64(len(store.texts[ref.textID]))
		}
		if ref.targetID != 0 {
			checksum += uint64(len(store.targets[ref.targetID]))
		}
		if ref.extraID != 0 {
			checksum += uint64(len(store.extras[ref.extraID]))
		}
	}
	return checksum
}

type sourceSpanPrototype struct {
	start uint32
	end   uint32
}

type sourceTextPayloadsPrototype struct {
	refs       []splitPayloadRefPrototype
	textSpans  []sourceSpanPrototype
	textValues []string
	targets    []string
	extras     []string
}

type textExtraPayloadPrototype struct {
	text  string
	extra string
}

// kindPayloadsPrototype treats semanticRecord.payload as a tagged union whose
// meaning follows record.kind. Text nodes index texts, links index targets,
// footnotes index labels, and text+metadata nodes index textExtras. There is no
// generic payload-ref object between a record and the data it actually uses.
type kindPayloadsPrototype struct {
	payloadIDs []uint32 // models replacement values for record.payload
	texts      []string
	targets    []string
	labels     []string
	textExtras []textExtraPayloadPrototype
}

type kindSourcePayloadsPrototype struct {
	payloadIDs []uint32
	textSpans  []sourceSpanPrototype
	textValues []string
	targets    []string
	labels     []string
	textExtras []textExtraPayloadPrototype
	source     string
}

func kindSourcePayloadsFromASTPrototype(doc *djot.Doc, tape *semanticTapePrototype) kindSourcePayloadsPrototype {
	store := kindSourcePayloadsPrototype{
		payloadIDs: make([]uint32, len(tape.records)),
		textSpans:  []sourceSpanPrototype{{}},
		textValues: []string{""},
		targets:    []string{""},
		labels:     []string{""},
		textExtras: []textExtraPayloadPrototype{{}},
		source:     string(doc.Files[0].Source),
	}
	recordIndex := 0
	var visit func(*djot.Node)
	visit = func(node *djot.Node) {
		record := tape.records[recordIndex]
		current := recordIndex
		recordIndex++
		if record.payload != 0 {
			payload := tape.payloads[record.payload]
			switch djot.NodeKind(record.kind) {
			case djot.Text, djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol:
				start, end := node.Start.Offset, node.End.Offset+1
				if start >= 0 && end >= start && end <= len(store.source) && store.source[start:end] == payload.text {
					store.payloadIDs[current] = uint32(len(store.textSpans))
					store.textSpans = append(store.textSpans, sourceSpanPrototype{uint32(start), uint32(end)})
				} else {
					store.payloadIDs[current] = sourceTextValueBitPrototype | uint32(len(store.textValues))
					store.textValues = append(store.textValues, payload.text)
				}
			case djot.Link, djot.Image:
				store.payloadIDs[current] = uint32(len(store.targets))
				store.targets = append(store.targets, payload.target)
			case djot.CodeBlock, djot.RawBlock, djot.RawInline:
				store.payloadIDs[current] = uint32(len(store.textExtras))
				store.textExtras = append(store.textExtras, textExtraPayloadPrototype{
					text: payload.text, extra: payload.extra,
				})
			case djot.Footnote, djot.FootnoteReference:
				store.payloadIDs[current] = uint32(len(store.labels))
				store.labels = append(store.labels, payload.extra)
			case djot.OrderedList:
				store.payloadIDs[current] = uint32(payload.number)
			default:
				panic("unhandled source payload kind: " + djot.NodeKind(record.kind).String())
			}
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(doc.Root())
	return store
}

func kindSourcePayloadLogicalBytesPrototype(store kindSourcePayloadsPrototype) int {
	return len(store.textSpans)*int(unsafe.Sizeof(sourceSpanPrototype{})) +
		(len(store.textValues)+len(store.targets)+len(store.labels))*int(unsafe.Sizeof("")) +
		len(store.textExtras)*int(unsafe.Sizeof(textExtraPayloadPrototype{}))
}

func kindPayloadsFromTapePrototype(tape *semanticTapePrototype) kindPayloadsPrototype {
	store := kindPayloadsPrototype{
		payloadIDs: make([]uint32, len(tape.records)),
		texts:      []string{""},
		targets:    []string{""},
		labels:     []string{""},
		textExtras: []textExtraPayloadPrototype{{}},
	}
	for i, record := range tape.records[:len(tape.records)-1] {
		if record.payload == 0 {
			continue
		}
		payload := tape.payloads[record.payload]
		switch djot.NodeKind(record.kind) {
		case djot.Text, djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol:
			store.payloadIDs[i] = uint32(len(store.texts))
			store.texts = append(store.texts, payload.text)
		case djot.Link, djot.Image:
			store.payloadIDs[i] = uint32(len(store.targets))
			store.targets = append(store.targets, payload.target)
		case djot.CodeBlock, djot.RawBlock, djot.RawInline:
			store.payloadIDs[i] = uint32(len(store.textExtras))
			store.textExtras = append(store.textExtras, textExtraPayloadPrototype{
				text: payload.text, extra: payload.extra,
			})
		case djot.Footnote, djot.FootnoteReference:
			store.payloadIDs[i] = uint32(len(store.labels))
			store.labels = append(store.labels, payload.extra)
		case djot.OrderedList:
			store.payloadIDs[i] = uint32(payload.number)
		default:
			panic("unhandled prototype payload kind: " + djot.NodeKind(record.kind).String())
		}
	}
	return store
}

func kindPayloadLogicalBytesPrototype(store kindPayloadsPrototype) int {
	// payloadIDs replace the existing record.payload column and therefore are
	// not additional logical storage.
	return (len(store.texts)+len(store.targets)+len(store.labels))*int(unsafe.Sizeof("")) +
		len(store.textExtras)*int(unsafe.Sizeof(textExtraPayloadPrototype{}))
}

func scanKindPayloadsPrototype(tape *semanticTapePrototype, store kindPayloadsPrototype) uint64 {
	var checksum uint64
	for i, record := range tape.records[:len(tape.records)-1] {
		id := store.payloadIDs[i]
		if id == 0 {
			continue
		}
		switch djot.NodeKind(record.kind) {
		case djot.Text, djot.Verbatim, djot.InlineMath, djot.DisplayMath, djot.Symbol:
			checksum += uint64(len(store.texts[id]))
		case djot.Link, djot.Image:
			checksum += uint64(len(store.targets[id]))
		case djot.CodeBlock, djot.RawBlock, djot.RawInline:
			payload := store.textExtras[id]
			checksum += uint64(len(payload.text) + len(payload.extra))
		case djot.Footnote, djot.FootnoteReference:
			checksum += uint64(len(store.labels[id]))
		case djot.OrderedList:
			// The comparison checksum, like scanSemanticTapePrototype, ignores
			// numeric values and accounts only for string bytes.
		}
	}
	return checksum
}

const sourceTextValueBitPrototype = uint32(1 << 31)

func sourceTextPayloadsFromASTPrototype(doc *djot.Doc, tape *semanticTapePrototype) sourceTextPayloadsPrototype {
	store := sourceTextPayloadsPrototype{
		refs:       make([]splitPayloadRefPrototype, len(tape.payloads)),
		textSpans:  []sourceSpanPrototype{{}},
		textValues: []string{""},
		targets:    []string{""},
		extras:     []string{""},
	}
	source := string(doc.Files[0].Source)
	payloadIndex := 1
	var visit func(*djot.Node)
	visit = func(node *djot.Node) {
		hasPayload := node.Text != "" || node.Target != "" || node.Name != "" ||
			node.Lang != "" || node.Format != "" || node.Label != "" || node.ListStart != 0
		if hasPayload {
			payload := tape.payloads[payloadIndex]
			ref := splitPayloadRefPrototype{number: payload.number}
			if payload.text != "" {
				start, end := node.Start.Offset, node.End.Offset+1
				if start >= 0 && end >= start && end <= len(source) && source[start:end] == payload.text {
					ref.textID = uint32(len(store.textSpans))
					store.textSpans = append(store.textSpans, sourceSpanPrototype{uint32(start), uint32(end)})
				} else {
					ref.textID = sourceTextValueBitPrototype | uint32(len(store.textValues))
					store.textValues = append(store.textValues, payload.text)
				}
			}
			if payload.target != "" {
				ref.targetID = uint32(len(store.targets))
				store.targets = append(store.targets, payload.target)
			}
			if payload.extra != "" {
				ref.extraID = uint32(len(store.extras))
				store.extras = append(store.extras, payload.extra)
			}
			store.refs[payloadIndex] = ref
			payloadIndex++
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(doc.Root())
	return store
}

func sourceTextLogicalBytesPrototype(store sourceTextPayloadsPrototype) int {
	return len(store.refs)*int(unsafe.Sizeof(splitPayloadRefPrototype{})) +
		len(store.textSpans)*int(unsafe.Sizeof(sourceSpanPrototype{})) +
		(len(store.textValues)+len(store.targets)+len(store.extras))*int(unsafe.Sizeof(""))
}

func scanSourceTextPayloadsPrototype(store sourceTextPayloadsPrototype, source string) uint64 {
	var checksum uint64
	for _, ref := range store.refs[1:] {
		if ref.textID&sourceTextValueBitPrototype != 0 {
			checksum += uint64(len(store.textValues[ref.textID&^sourceTextValueBitPrototype]))
		} else if ref.textID != 0 {
			span := store.textSpans[ref.textID]
			checksum += uint64(len(source[span.start:span.end]))
		}
		if ref.targetID != 0 {
			checksum += uint64(len(store.targets[ref.targetID]))
		}
		if ref.extraID != 0 {
			checksum += uint64(len(store.extras[ref.extraID]))
		}
	}
	return checksum
}

func BenchmarkLayoutStrategyMatrix(b *testing.B) {
	doc := djot.Parse(hugeDoc())
	tape := buildHintedSemanticTapePrototype(doc)
	columns := columnsFromTapePrototype(tape)
	split := splitPayloadsFromTapePrototype(tape)
	sourceSplit := sourceTextPayloadsFromASTPrototype(doc, tape)
	kindPayloads := kindPayloadsFromTapePrototype(tape)
	kindSourcePayloads := kindSourcePayloadsFromASTPrototype(doc, tape)
	source := string(doc.Files[0].Source)

	if scanAoSKindsPrototype(tape) != scanSoAKindsPrototype(columns) {
		b.Fatal("AoS and SoA kind scans differ")
	}
	if scanSemanticTapePrototype(tape) != scanSoAFullPrototype(columns, tape) {
		b.Fatal("AoS and SoA full scans differ")
	}
	if scanSplitPayloadsPrototype(split) != scanSourceTextPayloadsPrototype(sourceSplit, source) {
		b.Fatal("split payload strategies differ")
	}
	if scanSplitPayloadsPrototype(split) != scanKindPayloadsPrototype(tape, kindPayloads) {
		b.Fatal("generic and kind-specific payload strategies differ")
	}
	if got, want := renderSemanticHTMLWithPayloadsPrototype(tape, &kindPayloads), djot.RenderHTML(doc); got != want {
		b.Fatal("kind-specific payload renderer differs from production renderer")
	}
	if got, want := renderSemanticHTMLWithSourcePayloadsPrototype(tape, &kindSourcePayloads), djot.RenderHTML(doc); got != want {
		b.Fatal("source-backed kind payload renderer differs from production renderer")
	}

	b.Run("Shape", func(b *testing.B) {
		baseBytes := len(tape.records)*int(unsafe.Sizeof(semanticRecordPrototype{})) +
			len(tape.attributes)*int(unsafe.Sizeof(semanticAttributePrototype{}))
		b.ReportMetric(float64(len(tape.payloads)*int(unsafe.Sizeof(semanticPayloadPrototype{}))), "fat-payload-B")
		b.ReportMetric(float64(splitPayloadLogicalBytesPrototype(split)), "split-payload-B")
		b.ReportMetric(float64(sourceTextLogicalBytesPrototype(sourceSplit)), "source-split-payload-B")
		b.ReportMetric(float64(kindPayloadLogicalBytesPrototype(kindPayloads)), "kind-payload-B")
		b.ReportMetric(float64(kindSourcePayloadLogicalBytesPrototype(kindSourcePayloads)), "kind-source-payload-B")
		b.ReportMetric(float64(baseBytes+splitPayloadLogicalBytesPrototype(split)), "split-tape-B")
		b.ReportMetric(float64(baseBytes+sourceTextLogicalBytesPrototype(sourceSplit)), "source-split-tape-B")
		b.ReportMetric(float64(baseBytes+kindPayloadLogicalBytesPrototype(kindPayloads)), "kind-tape-B")
		b.ReportMetric(float64(baseBytes+kindSourcePayloadLogicalBytesPrototype(kindSourcePayloads)), "kind-source-tape-B")
		b.ReportMetric(float64(len(sourceSplit.textSpans)-1), "source-backed-texts")
		b.ReportMetric(float64(len(sourceSplit.textValues)-1), "stored-texts")
		for i := 0; i < b.N; i++ {
			runtime.KeepAlive(sourceSplit)
		}
	})

	benchScan := func(name string, fn func() uint64) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			var checksum uint64
			for i := 0; i < b.N; i++ {
				checksum += fn()
			}
			runtime.KeepAlive(checksum)
		})
	}
	benchScan("Kinds/AoS", func() uint64 { return scanAoSKindsPrototype(tape) })
	benchScan("Kinds/SoA", func() uint64 { return scanSoAKindsPrototype(columns) })
	benchScan("Full/AoS", func() uint64 { return scanSemanticTapePrototype(tape) })
	benchScan("Full/SoA", func() uint64 { return scanSoAFullPrototype(columns, tape) })
	benchScan("Payload/Fat", func() uint64 {
		var checksum uint64
		for _, payload := range tape.payloads[1:] {
			checksum += uint64(len(payload.text) + len(payload.target) + len(payload.extra))
		}
		return checksum
	})
	benchScan("Payload/Split", func() uint64 { return scanSplitPayloadsPrototype(split) })
	benchScan("Payload/SourceSplit", func() uint64 { return scanSourceTextPayloadsPrototype(sourceSplit, source) })
	benchScan("Payload/KindSpecific", func() uint64 { return scanKindPayloadsPrototype(tape, kindPayloads) })
}

var inlineSpecialTablePrototype = func() [256]bool {
	var table [256]bool
	for _, c := range []byte{'\\', '`', '$', '*', '_', '^', '~', '[', '!', ']', '{', '}', '<', ':', '\n', '"', '\'', '-', '.', '+', '='} {
		table[c] = true
	}
	return table
}()

func scanInlineSwitchPrototype(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '`', '$', '*', '_', '^', '~', '[', '!', ']', '{', '}', '<', ':', '\n', '"', '\'', '-', '.', '+', '=':
			return i
		}
	}
	return len(s)
}

func scanInlineTablePrototype(s string) int {
	for i := 0; i < len(s); i++ {
		if inlineSpecialTablePrototype[s[i]] {
			return i
		}
	}
	return len(s)
}

func scanInlineUnrolledPrototype(s string) int {
	i := 0
	for ; i+8 <= len(s); i += 8 {
		chunk := s[i : i+8]
		if inlineSpecialTablePrototype[chunk[0]] || inlineSpecialTablePrototype[chunk[1]] ||
			inlineSpecialTablePrototype[chunk[2]] || inlineSpecialTablePrototype[chunk[3]] ||
			inlineSpecialTablePrototype[chunk[4]] || inlineSpecialTablePrototype[chunk[5]] ||
			inlineSpecialTablePrototype[chunk[6]] || inlineSpecialTablePrototype[chunk[7]] {
			for j := 0; j < 8; j++ {
				if inlineSpecialTablePrototype[chunk[j]] {
					return i + j
				}
			}
		}
	}
	for ; i < len(s); i++ {
		if inlineSpecialTablePrototype[s[i]] {
			return i
		}
	}
	return len(s)
}

func scanInlinePeeledUnrolledPrototype(s string) int {
	i := 0
	// Dense markup usually finds a delimiter immediately. Peeling one word
	// avoids paying the wide OR chain before that early exit.
	limit := 8
	if len(s) < limit {
		limit = len(s)
	}
	for ; i < limit; i++ {
		if inlineSpecialTablePrototype[s[i]] {
			return i
		}
	}
	for ; i+8 <= len(s); i += 8 {
		chunk := s[i : i+8]
		if inlineSpecialTablePrototype[chunk[0]] || inlineSpecialTablePrototype[chunk[1]] ||
			inlineSpecialTablePrototype[chunk[2]] || inlineSpecialTablePrototype[chunk[3]] ||
			inlineSpecialTablePrototype[chunk[4]] || inlineSpecialTablePrototype[chunk[5]] ||
			inlineSpecialTablePrototype[chunk[6]] || inlineSpecialTablePrototype[chunk[7]] {
			for j := 0; j < 8; j++ {
				if inlineSpecialTablePrototype[chunk[j]] {
					return i + j
				}
			}
		}
	}
	for ; i < len(s); i++ {
		if inlineSpecialTablePrototype[s[i]] {
			return i
		}
	}
	return len(s)
}

func scanInlineIndexAnyPrototype(s string) int {
	i := strings.IndexAny(s, "\\`$*_^[!]{}<:\n\"'-.+=~")
	if i < 0 {
		return len(s)
	}
	return i
}

func swarHasBytePrototype(word uint64, c byte) uint64 {
	x := word ^ uint64(c)*0x0101010101010101
	return (x - 0x0101010101010101) &^ x & 0x8080808080808080
}

func scanInlineSWARPrototype(s string) int {
	i := 0
	for ; i+8 <= len(s); i += 8 {
		word := binary.LittleEndian.Uint64([]byte(s[i : i+8]))
		var matches uint64
		for _, c := range [...]byte{'\\', '`', '$', '*', '_', '^', '~', '[', '!', ']', '{', '}', '<', ':', '\n', '"', '\'', '-', '.', '+', '='} {
			matches |= swarHasBytePrototype(word, c)
		}
		if matches != 0 {
			for j := 0; j < 8; j++ {
				if inlineSpecialTablePrototype[s[i+j]] {
					return i + j
				}
			}
		}
	}
	for ; i < len(s); i++ {
		if inlineSpecialTablePrototype[s[i]] {
			return i
		}
	}
	return len(s)
}

func scanAllInlinePrototype(s string, scan func(string) int) uint64 {
	var checksum uint64
	for pos := 0; pos < len(s); {
		n := scan(s[pos:])
		checksum += uint64(pos + n)
		pos += n
		if pos < len(s) {
			pos++
		}
	}
	return checksum
}

func scannerCorporaPrototype() map[string]string {
	return map[string]string{
		"PlainASCII_128KB":  strings.Repeat("The quick brown fox jumps over the lazy dog, with ordinary prose and numbers 12345; ", 1600),
		"MarkupDense_128KB": pathologicalInline(128 * 1024),
		"Unicode_128KB":     strings.Repeat("Καλημέρα κόσμε — naïve café 日本語 ordinary prose; ", 2600),
	}
}

func TestInlineScannerStrategies(t *testing.T) {
	strategies := map[string]func(string) int{
		"table": scanInlineTablePrototype, "unrolled": scanInlineUnrolledPrototype,
		"peeled-unrolled": scanInlinePeeledUnrolledPrototype,
		"index-any":       scanInlineIndexAnyPrototype, "swar": scanInlineSWARPrototype,
	}
	inputs := scannerCorporaPrototype()
	allBytes := make([]byte, 256*4)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	inputs["all-bytes"] = string(allBytes)
	for corpus, input := range inputs {
		want := scanAllInlinePrototype(input, scanInlineSwitchPrototype)
		for name, strategy := range strategies {
			if got := scanAllInlinePrototype(input, strategy); got != want {
				t.Errorf("%s/%s checksum = %d, want %d", corpus, name, got, want)
			}
		}
	}
}

func BenchmarkInlineScannerStrategyMatrix(b *testing.B) {
	strategies := []struct {
		name string
		fn   func(string) int
	}{
		{"Switch", scanInlineSwitchPrototype},
		{"Table", scanInlineTablePrototype},
		{"Unrolled", scanInlineUnrolledPrototype},
		{"PeeledUnrolled", scanInlinePeeledUnrolledPrototype},
		{"IndexAny", scanInlineIndexAnyPrototype},
		{"SWAR", scanInlineSWARPrototype},
	}
	for corpus, input := range scannerCorporaPrototype() {
		for _, strategy := range strategies {
			b.Run(corpus+"/"+strategy.name, func(b *testing.B) {
				b.SetBytes(int64(len(input)))
				b.ReportAllocs()
				var checksum uint64
				for i := 0; i < b.N; i++ {
					checksum += scanAllInlinePrototype(input, strategy.fn)
				}
				runtime.KeepAlive(checksum)
			})
		}
	}
}
