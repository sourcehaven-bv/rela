package dataentry

import (
	"context"
	"errors"
	"log/slog"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// collectMentions scans the supplied markdown blobs for inline code spans
// whose entire content is an entity ID known to the store, and returns a
// deduplicated map keyed by ID. Self-references (the viewer's own ID) are
// included — the renderer treats them like any other reference and the SPA
// route handles them as no-op navigation. Returns nil when no mentions
// are found so callers can rely on JSON `omitempty` for empty maps.
//
// Store failures other than ErrNotFound are logged and skipped: a flaky
// per-ID lookup must not break the whole view-fetch response. The
// fall-through degrades to "code span stays as <code>" which matches the
// unknown-ID UX. Context cancellation is honored — callers (HTTP handlers)
// have already bound the request context and abandoning further lookups
// after the client disconnects saves wasted work.
func collectMentions(ctx context.Context, s store.EntityReader, meta *metamodel.Metamodel, contents ...string) map[string]v1.Mention {
	candidates := scanCodeSpanCandidates(contents...)
	if len(candidates) == 0 {
		return nil
	}
	out := make(map[string]v1.Mention, len(candidates))
	for id := range candidates {
		if err := ctx.Err(); err != nil {
			if len(out) == 0 {
				return nil
			}
			return out
		}
		ent, err := s.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			slog.WarnContext(ctx, "mentions: store lookup failed",
				"id", id, "err", err)
			continue
		}
		out[id] = buildMention(ent, meta)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildMention turns a resolved entity into a wire-shape v1.Mention. Title
// uses the metamodel's DisplayTitle so entity types whose primary
// property is something other than `title` (e.g. concept's `name`)
// still produce readable link text. Inaccessibility flips on only when
// the display-title source is itself unreadable — a partial lock on an
// unrelated property must not turn a link into a lock affordance.
func buildMention(e *entityPkg.Entity, meta *metamodel.Metamodel) v1.Mention {
	m := v1.Mention{Type: e.Type}
	if meta != nil {
		m.Title = meta.DisplayTitle(e.ID, e.Type, e.Properties)
	} else {
		m.Title = e.Title()
	}

	primary := displayProperty(meta, e.Type)
	if reason, locked := lockedReasonFor(e, primary); locked {
		m.Inaccessible = true
		m.InaccessibleReason = reason
	}
	return m
}

// displayProperty returns the property name whose value backs the display
// title for this entity type. Empty when the metamodel has no entry for
// the type or no primary property is configured — falls back to the ID
// being the source of truth, in which case the entity can't really be
// "locked behind its title."
func displayProperty(meta *metamodel.Metamodel, entityType string) string {
	if meta == nil {
		return "title"
	}
	def, ok := meta.GetEntityDef(entityType)
	if !ok {
		return ""
	}
	return def.GetPrimaryProperty()
}

// lockedReasonFor reports whether the entity's display title is
// unreadable (and the matching reason). The entity's whole content body
// being inaccessible (`InaccessibleFieldContent`) counts too, because the
// lookup of the title property may have failed for the same reason —
// markdown loaders produce a `content` inaccessible field for the whole
// file when git-crypt blocks the read. A property unrelated to the title
// being locked does not affect the link.
func lockedReasonFor(e *entityPkg.Entity, displayProp string) (string, bool) {
	if e == nil {
		return "", false
	}
	for _, f := range e.Inaccessible {
		if f.Name == displayProp || f.Name == entityPkg.InaccessibleFieldContent {
			return string(f.Reason), true
		}
	}
	return "", false
}

// scanCodeSpanCandidates parses each markdown blob and returns the set of
// distinct strings that appear as the entire text of an inline code span.
// Code blocks (fenced and indented) are walked over by goldmark as
// FencedCodeBlock / CodeBlock — never CodeSpan — so they are naturally
// excluded.
func scanCodeSpanCandidates(contents ...string) map[string]struct{} {
	if len(contents) == 0 {
		return nil
	}
	candidates := make(map[string]struct{})
	parser := mentionsMarkdown.Parser()
	for _, src := range contents {
		if src == "" {
			continue
		}
		source := []byte(src)
		doc := parser.Parse(text.NewReader(source))
		_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			cs, ok := n.(*ast.CodeSpan)
			if !ok {
				return ast.WalkContinue, nil
			}
			text, complete := codeSpanText(cs, source)
			if !complete {
				return ast.WalkSkipChildren, nil
			}
			candidates[text] = struct{}{}
			return ast.WalkSkipChildren, nil
		})
	}
	delete(candidates, "")
	return candidates
}

// codeSpanText concatenates the literal text segments inside a code span.
// goldmark stores the content as one or more `*ast.Text` children whose
// `Segment` slices the source bytes. Returns complete=false (and an
// empty string) when the span contains any child that isn't a plain
// text segment, so we never silently match a partial reconstruction
// against an entity ID.
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

// mentionsMarkdown is a goldmark instance used solely for AST walking.
// Configured with the same GFM extension set as the document renderer
// (internal/dataentry/document.go) so the parse matches what the SPA
// will eventually render — in particular, table cell inlines (which GFM
// enables) yield CodeSpan nodes we expect to scan.
//
// goldmark.Markdown is safe for concurrent use across goroutines: the
// instance is configuration-only and each Parse call constructs its own
// parser state from the source.
var mentionsMarkdown = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
)
