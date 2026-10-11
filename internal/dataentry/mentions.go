package dataentry

import (
	"context"
	"log/slog"
	"slices"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	mentionspkg "github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// collectMentions scans the supplied markdown blobs for mentions
// ([mentionspkg.All]) and returns the ones the principal is served, keyed by
// id. Self-references (the viewer's own ID) are included: the renderer treats
// them like any other reference and the SPA route handles them as no-op
// navigation. Returns nil when no mentions are found so callers can rely on
// JSON `omitempty` for empty maps.
//
// Each id resolves to the face the request's world serves among the faces
// the principal may read ([visibility.Resolver.ResolveIDsErr]), the same
// resolution relation neighbors use, so a mention of an entity whose type
// has faces links like any other. The header comes back gated and redacted
// (DEC-ZBI39P): a mention of an entity the principal may not read is
// dropped, and a survivor whose display property is hidden falls back to
// its id (BUG-R9EHKV).
//
// A store failure is logged and drops every mention: a failed lookup must
// not break the whole view-fetch response. The fall-through degrades to
// "code span stays as <code>", which matches the unknown-ID UX. A cancelled
// context skips the lookup: callers (HTTP handlers) have already bound the
// request context, and a client that disconnected wants no answer.
func collectMentions(
	ctx context.Context, vis visibility.Reader, meta *metamodel.Metamodel, contents ...string,
) map[string]v1.Mention {
	seen := make(map[string]struct{})
	for _, body := range contents {
		for _, id := range mentionspkg.All(body) {
			seen[id] = struct{}{}
		}
	}
	if len(seen) == 0 || ctx.Err() != nil {
		return nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	served, err := vis.Resolver().ResolveIDsErr(ctx, worldFromContext(ctx).visibility(), ids)
	if err != nil {
		slog.WarnContext(ctx, "mentions: store lookup failed", "ids", len(ids), "err", err)
		return nil
	}
	if len(served) == 0 {
		return nil
	}
	out := make(map[string]v1.Mention, len(served))
	for id, h := range served {
		out[id] = buildMention(headerEntity(h), meta)
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

	titleProps := displayProperties(meta, e.Type)
	if reason, locked := lockedReasonFor(e, titleProps); locked {
		m.Inaccessible = true
		m.InaccessibleReason = reason
	}
	return m
}

// displayProperties returns the property names whose values back the display
// title for this entity type — a single name for a bare/autoderived primary,
// or every placeholder for a templated display_property. Empty when the
// metamodel has no entry for the type or no primary property is configured
// (the ID is then the source of truth, so the entity can't be "locked behind
// its title"). Any one of these being inaccessible locks the title, because
// DisplayTitle would render an incomplete name.
func displayProperties(meta *metamodel.Metamodel, entityType string) []string {
	if meta == nil {
		return []string{"title"}
	}
	def, ok := meta.GetEntityDef(entityType)
	if !ok {
		return nil
	}
	return def.DisplayProperties()
}

// lockedReasonFor reports whether the entity's display title is
// unreadable (and the matching reason). The entity's whole content body
// being inaccessible (`InaccessibleFieldContent`) counts too, because the
// lookup of a title property may have failed for the same reason —
// markdown loaders produce a `content` inaccessible field for the whole
// file when git-crypt blocks the read. A property unrelated to the title
// being locked does not affect the link; but any property the title reads
// from (all placeholders of a template) being locked does.
func lockedReasonFor(e *entityPkg.Entity, displayProps []string) (string, bool) {
	if e == nil {
		return "", false
	}
	for _, f := range e.Inaccessible {
		if f.Name == entityPkg.InaccessibleFieldContent {
			return string(f.Reason), true
		}
		if slices.Contains(displayProps, f.Name) {
			return string(f.Reason), true
		}
	}
	return "", false
}
