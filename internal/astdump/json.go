package astdump

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/danielledeleo/djot-go"
	"github.com/danielledeleo/djot-go/ast"
)

// JSON writes doc's AST to w as indented JSON. Each node becomes an object
// with a "tag" (the same tag names as Text), its kind-specific fields, an
// optional "attributes" object, an optional "pos" object, and a "children"
// array. If positions is true, every node except the document root carries a
// "pos" object with start and end line, column, and offset. Inline end offsets
// are inclusive. The output ends with a newline.
//
// The shape follows djot-go's own AST rather than reproducing djot.js's
// reference-resolution tables, so it is not byte-for-byte identical to
// djot.js's "ast" JSON.
func JSON(w io.Writer, doc *djot.Doc, positions bool) error {
	root := astJSONNode(doc, doc.AST(), positions)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(out); err != nil {
		return err
	}
	_, err = io.WriteString(w, "\n")
	return err
}

func astJSONNode(doc *djot.Doc, n ast.Node, positions bool) *jsonObj {
	o := &jsonObj{}
	o.set("tag", astTagName(n))

	if positions && n.Kind() != ast.KindDocument {
		if pos := astJSONPos(doc, n); pos != nil {
			o.set("pos", pos)
		}
	}

	for _, f := range astFields(n) {
		o.set(f.key, f.val)
	}

	if n.Attributes().Len() > 0 {
		attrs := &jsonObj{}
		for _, attribute := range n.Attributes().Entries() {
			attrs.set(attribute.Key, attribute.Value)
		}
		o.set("attributes", attrs)
	}

	var kids []any
	ast.ForEachChild(n, func(child ast.Node) {
		kids = append(kids, astJSONNode(doc, child, positions))
	})
	if len(kids) > 0 {
		o.set("children", kids)
	}

	return o
}

func astJSONPos(doc *djot.Doc, n ast.Node) *jsonObj {
	startPos, endPos, ok := astSpan(doc, n)
	if !ok {
		return nil
	}

	start := &jsonObj{}
	start.set("line", startPos.Line)
	start.set("col", startPos.Column)
	start.set("offset", n.Span().Start.Offset)

	end := &jsonObj{}
	end.set("line", endPos.Line)
	end.set("col", endPos.Column)
	end.set("offset", astOutputEnd(n))

	pos := &jsonObj{}
	pos.set("start", start)
	pos.set("end", end)
	return pos
}

// jsonObj is a JSON object that preserves key insertion order, which the AST
// field order relies on (encoding/json sorts map keys alphabetically).
type jsonObj struct {
	keys []string
	vals []any
}

func (o *jsonObj) set(key string, val any) {
	o.keys = append(o.keys, key)
	o.vals = append(o.vals, val)
}

func (o *jsonObj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
