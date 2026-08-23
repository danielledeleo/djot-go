package djot

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// RenderASTJSON renders a parsed document to indented JSON that mirrors
// djot-go's AST. Each node becomes an object with a "tag" (the same tag names
// used by [RenderAST]), its kind-specific fields, an optional "attributes"
// object, an optional "pos" object, and a "children" array. If positions is
// true, every node except the document root carries a "pos" object with start
// and end line, column, and offset. The output ends with a newline.
//
// The shape follows djot-go's own AST rather than reproducing djot.js's
// reference-resolution tables, so it is convenient for tooling and diffing but
// not byte-for-byte identical to djot.js's "ast" JSON.
func RenderASTJSON(doc *Doc, positions bool) string {
	var b strings.Builder
	// The only error source is the io.Writer; strings.Builder never fails.
	_ = RenderASTJSONTo(&b, doc, positions)
	return b.String()
}

// RenderASTJSONTo writes the JSON AST (see [RenderASTJSON]) to w.
func RenderASTJSONTo(w io.Writer, doc *Doc, positions bool) error {
	root := astJSONNode(doc, doc.Root(), positions)
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

func astJSONNode(doc *Doc, n Node, positions bool) *jsonObj {
	o := &jsonObj{}
	o.set("tag", astTagName(n))

	if positions && n.Kind() != KindDocument {
		if pos := astJSONPos(doc, n); pos != nil {
			o.set("pos", pos)
		}
	}

	for _, f := range astFields(n) {
		o.set(f.key, f.val)
	}

	if n.Attributes().Len() > 0 {
		attrs := &jsonObj{}
		for _, attribute := range n.Attributes().items {
			attrs.set(attribute.Key, attribute.Value)
		}
		o.set("attributes", attrs)
	}

	var kids []any
	forEachChild(n, func(child Node) {
		kids = append(kids, astJSONNode(doc, child, positions))
	})
	if len(kids) > 0 {
		o.set("children", kids)
	}

	return o
}

func astJSONPos(doc *Doc, n Node) *jsonObj {
	if doc == nil || len(doc.Files) == 0 {
		return nil
	}
	span := n.Span()
	fi := &doc.Files[span.Start.File]
	sLine, sCol := fi.Position(span.Start.Offset)
	eLine, eCol := astEndPosition(fi, span.End.Offset)

	start := &jsonObj{}
	start.set("line", sLine)
	start.set("col", sCol)
	start.set("offset", span.Start.Offset)

	end := &jsonObj{}
	end.set("line", eLine)
	end.set("col", eCol)
	end.set("offset", span.End.Offset)

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
