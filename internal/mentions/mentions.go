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

// All returns the distinct entity ids that body mentions, sorted. A span
// counts only when its whole text is plain and passes [entity.ValidateID];
// whether the id exists is the caller's question. Code blocks are never
// CodeSpan nodes in goldmark, so they are excluded; a span inside link text
// is skipped because the link already points somewhere else.
//
// Nil: returned when body has no mentions.
func All(body string) []string {
	if body == "" {
		return nil
	}
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
		if id, complete := codeSpanText(cs, source); complete && entity.ValidateID(id) == nil {
			seen[id] = struct{}{}
		}
		return ast.WalkSkipChildren, nil
	})
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	slices.Sort(out)
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

// codeSpanText concatenates the literal text segments inside a code span.
// It returns complete=false when the span holds any child that is not a
// plain text segment, so a partial reconstruction is never matched against
// an entity id.
func codeSpanText(cs *ast.CodeSpan, source []byte) (string, bool) {
	var b []byte
	for c := cs.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			return "", false
		}
		b = append(b, t.Segment.Value(source)...)
	}
	return string(b), true
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
