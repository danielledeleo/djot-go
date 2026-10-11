package djot

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/danielledeleo/djot-go/ast"
)

var errNilHTMLWriter = errors.New("djot: RenderHTMLTo called with a nil writer")

// RenderOption configures HTML rendering. Pass to [RenderHTML] or [RenderHTMLTo].
type RenderOption func(*renderConfig)

type nodeRenderFunc func(n ast.Node, r NodeRenderer)

// NodeRenderer controls output from inside a [WithNodeRenderer] callback. It is
// valid only during the callback; its zero value writes nothing.
type NodeRenderer struct {
	r      *semanticHTMLRenderer
	record int
}

// Write writes raw HTML to the output. It does not escape s.
func (r NodeRenderer) Write(s string) {
	if r.r != nil {
		r.r.write(s)
	}
}

// Children renders the node's children, including any hooks registered for
// their kinds, without the node's own wrapper element.
func (r NodeRenderer) Children() {
	if r.r != nil {
		r.r.renderChildren(r.record)
	}
}

// Default renders the node as if no hook were registered for its kind. Hooks
// registered for its descendants still run.
func (r NodeRenderer) Default() {
	if r.r != nil {
		r.r.renderDefault(r.record)
	}
}

type renderConfig struct {
	hooks                 map[ast.Kind]nodeRenderFunc
	multiBacklinks        bool
	footnoteID            func(num int) string
	footnoteRefID         func(num, k int) string
	footnoteBacklinkLabel func(num, k, total int) string
}

// Default footnote id/label producers; these reproduce djot-go's standard output.

func defaultFootnoteID(num int) string { return "fn" + strconv.Itoa(num) }

func defaultFootnoteRefID(num, k int) string { return fnrefID(num, k) }

func defaultFootnoteBacklinkLabel(_, k, total int) string {
	if total <= 1 {
		return "↩︎"
	}
	return backrefLabel(k)
}

// WithFootnoteBacklinkLabel overrides the visible text of the k-th backlink for
// a footnote, where total is the number of backlinks that footnote has (1 unless
// [WithMultiBacklinks] is set). The default is "↩︎" when total is 1 and the
// letters a, b, c, … otherwise. Use this for numeric or other label schemes.
// The returned value is escaped as HTML text.
func WithFootnoteBacklinkLabel(fn func(num, k, total int) string) RenderOption {
	requireRenderCallback(fn != nil, "WithFootnoteBacklinkLabel")
	return func(cfg *renderConfig) { cfg.footnoteBacklinkLabel = fn }
}

// WithFootnotePrefix namespaces every footnote id: footnote ids become
// prefix+"fnN" and reference ids prefix+"fnrefN"(-k). Use it when embedding
// output in a page that may already use fn/fnref ids. The prefix is
// HTML-attribute escaped.
func WithFootnotePrefix(prefix string) RenderOption {
	return func(cfg *renderConfig) {
		cfg.footnoteID = func(num int) string { return prefix + defaultFootnoteID(num) }
		cfg.footnoteRefID = func(num, k int) string { return prefix + defaultFootnoteRefID(num, k) }
	}
}

// WithMultiBacklinks renders MediaWiki-style backlinks for footnotes that are
// referenced more than once: every reference gets a unique id (fnref1, fnref1-2,
// fnref1-3, …) and the footnote's entry links back to each reference with
// lettered backlinks (a, b, c, …) instead of a single ↩︎ to the first reference.
// Footnotes referenced only once are unchanged (a single ↩︎ backlink).
//
// Without this option the renderer matches djot.js: only the first reference
// carries the id (so the HTML has no duplicate ids) and the footnote has one
// backlink, pointing to that first reference.
func WithMultiBacklinks() RenderOption {
	return func(cfg *renderConfig) {
		cfg.multiBacklinks = true
	}
}

// WithNodeRenderer registers a render hook for the given node kind.
// The hook receives the [ast.Node] and a [NodeRenderer] for full control over output.
// If called multiple times for the same kind, the last one wins.
//
// KindDocument and KindFootnote are not directly rendered elements; use
// [WithDocumentRenderer] for the document and render hooks for a footnote's
// block children. WithNodeRenderer panics for either synthetic root kind, an
// invalid kind, or a nil callback.
//
// Use this when you need access to [NodeRenderer.Children] or [NodeRenderer.Default].
// For a symbol hook that can avoid AST materialization, see [WithSymbolRenderer].
func WithNodeRenderer(kind ast.Kind, fn func(n ast.Node, r NodeRenderer)) RenderOption {
	requireElementHook(kind, fn != nil, "WithNodeRenderer")
	return func(cfg *renderConfig) {
		if cfg.hooks == nil {
			cfg.hooks = make(map[ast.Kind]nodeRenderFunc)
		}
		cfg.hooks[kind] = fn
	}
}

func requireElementHook(kind ast.Kind, callbackPresent bool, option string) {
	requireRenderCallback(callbackPresent, option)
	if kind < 0 || kind > ast.KindEnDash {
		panic("djot: " + option + " requires a valid node kind")
	}
	if kind == ast.KindDocument || kind == ast.KindFootnote {
		panic("djot: " + option + " cannot target a synthetic rendering root")
	}
}

func requireRenderCallback(present bool, option string) {
	if !present {
		panic("djot: " + option + " requires a non-nil callback")
	}
}

// RenderHTML renders a parsed document to an HTML string. Optional
// [RenderOption] values customize rendering through node hooks and footnote
// options.
func RenderHTML(doc *Doc, opts ...RenderOption) string {
	return renderSemanticHTMLWithHooks(doc.tape(), makeRenderConfig(opts).semanticHooks())
}

// RenderHTMLTo renders a parsed document as HTML to the given writer.
// It returns the first write error encountered, if any.
func RenderHTMLTo(w io.Writer, doc *Doc, opts ...RenderOption) error {
	if w == nil {
		return errNilHTMLWriter
	}
	return renderSemanticHTMLToWithHooks(w, doc.tape(), makeRenderConfig(opts).semanticHooks())
}

func makeRenderConfig(opts []RenderOption) renderConfig {
	var cfg renderConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func (cfg renderConfig) semanticHooks() semanticRenderHooks {
	return semanticRenderHooks{
		nodes:          cfg.hooks,
		multiBacklinks: cfg.multiBacklinks, footnoteID: cfg.footnoteID,
		footnoteRefID: cfg.footnoteRefID, footnoteBacklinkLabel: cfg.footnoteBacklinkLabel,
	}
}

// fnrefID returns the id of the k-th reference (1-based) to footnote number num:
// "fnrefN" for the first reference and "fnrefN-k" for later ones.
func fnrefID(num, k int) string {
	id := "fnref" + strconv.Itoa(num)
	if k > 1 {
		id += "-" + strconv.Itoa(k)
	}
	return id
}

// backrefLabel maps a 1-based index to a spreadsheet-style letter label:
// 1→a, 2→b, …, 26→z, 27→aa, 28→ab, ….
func backrefLabel(i int) string {
	var b []byte
	for i > 0 {
		i--
		b = append([]byte{byte('a' + i%26)}, b...)
		i /= 26
	}
	return string(b)
}

func escapeHTML(s string) string {
	// Fast path: no escaping needed.
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>':
			return escapeHTMLSlow(s, i)
		}
	}
	return s
}

func escapeHTMLSlow(s string, first int) string {
	var b strings.Builder
	b.Grow(len(s) + 10)
	b.WriteString(s[:first])
	for i := first; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func escapeAttr(s string) string {
	// Fast path: no escaping needed.
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"':
			return escapeAttrSlow(s, i)
		}
	}
	return s
}

func escapeAttrSlow(s string, first int) string {
	var b strings.Builder
	b.Grow(len(s) + 10)
	b.WriteString(s[:first])
	for i := first; i < len(s); i++ {
		switch s[i] {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
