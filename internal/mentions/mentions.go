// Package mentions extracts entity-id mentions from markdown bodies.
//
// A mention is an inline code span whose entire content is a well-formed
// entity id, such as `TKT-123`. The data-entry renderer turns such a span
// into a link (forward), and every store indexes the targets of each body so
// an entity can list who mentions it (backward). Both directions share one
// parser ([All] forward, [Targets] and [Index] backward), so they cannot
// disagree on what counts as a mention. Only the backward direction is
// capped, at [MaxTargets].
//
// The package is a leaf: it depends on goldmark and internal/entity only, so
// the store backends may import it without pulling in the metamodel.
package mentions

import (
	"log/slog"
	"slices"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// MaxTargets bounds how many distinct targets one body contributes to the
// index. Bodies have no general size limit, so without a bound one save
// could write an unbounded number of index entries, and do so again on
// every later save.
const MaxTargets = 1000

// All returns the distinct entity ids that body mentions, sorted. It is
// [Spans] filtered by [entity.ValidateID]; whether the id exists is the
// caller's question.
//
// Nil: returned when body has no mentions.
func All(body string) []string {
	var out []string
	for _, s := range Spans(body) {
		if entity.ValidateID(s) == nil {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// Spans returns the distinct texts of the complete, plain code spans of
// body, in document order. A span in link text, or one that holds a line
// break, is skipped; code blocks are never spans. It is the one walk [All]
// and principal mentions share.
//
// A span that holds a line break is refused because the server and the SPA
// would read its text differently.
//
// Nil: returned when body has no spans.
func Spans(body string) []string {
	if body == "" {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	source := []byte(body)
	doc := parser.Parser().Parse(text.NewReader(source))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, isLink := n.(*ast.Link); isLink {
			// A span in link text already points somewhere else.
			return ast.WalkSkipChildren, nil
		}
		cs, ok := n.(*ast.CodeSpan)
		if !ok {
			return ast.WalkContinue, nil
		}
		if s, plain := codeSpanText(cs, source); plain {
			if _, dup := seen[s]; !dup {
				seen[s] = struct{}{}
				out = append(out, s)
			}
		}
		return ast.WalkSkipChildren, nil
	})
	return out
}

// Targets is [All] capped at [MaxTargets]: the lowest ids in sort order, so
// the result is deterministic. It is what a store indexes.
func Targets(body string) []string {
	out := All(body)
	if len(out) > MaxTargets {
		out = out[:MaxTargets]
	}
	return out
}

// Index is [Targets] for a store write that persists the result: it also
// logs when the cap drops mentions, once per write rather than on every
// read.
func Index(body string) []string {
	out := All(body)
	if len(out) > MaxTargets {
		slog.Warn("mentions: body exceeds the target cap; extra mentions are not indexed",
			"targets", len(out), "cap", MaxTargets)
		out = out[:MaxTargets]
	}
	return out
}

// codeSpanText returns the text of a code span. It returns plain=false
// when the span holds anything but one text segment without a line break,
// so a partial reconstruction is never matched.
func codeSpanText(cs *ast.CodeSpan, source []byte) (string, bool) {
	c := cs.FirstChild()
	if c == nil || c.NextSibling() != nil {
		return "", false
	}
	t, ok := c.(*ast.Text)
	if !ok {
		return "", false
	}
	v := t.Segment.Value(source)
	if slices.Contains(v, '\n') || slices.Contains(v, '\r') {
		return "", false
	}
	return string(v), true
}

// parser uses the same GFM extension set as the data-entry document
// renderer, so a span inside a table cell is found exactly when the SPA
// renders it as one. goldmark.Markdown is safe for concurrent use: each
// Parse builds its own state.
var parser = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
)
