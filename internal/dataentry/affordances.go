package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// translateVerb maps a wire-format verb to the [acl.WriteRequest] that
// authorizes that operation. It is the single source of truth for the
// "same code path" invariant: both the affordance serializer and the
// write handlers route their [acl.WriteRequest] construction through
// here. A grep test enforces that no other site in internal/dataentry
// constructs `acl.WriteRequest{Op:` directly.
//
// Adding a verb requires (a) an entry here, (b) an [acl.Op] that
// represents it, and (c) a docs/api.md update.
func translateVerb(verb, entityType string) (acl.WriteRequest, bool) {
	switch verb {
	case "create":
		return acl.WriteRequest{Op: acl.OpCreate, EntityType: entityType}, true
	case "update":
		return acl.WriteRequest{Op: acl.OpUpdate, EntityType: entityType}, true
	case "delete":
		return acl.WriteRequest{Op: acl.OpDelete, EntityType: entityType}, true
	case "rename":
		return acl.WriteRequest{Op: acl.OpRename, EntityType: entityType}, true
	default:
		return acl.WriteRequest{}, false
	}
}

// perItemVerbs are the verbs computed per entity instance.
var perItemVerbs = []string{"update", "delete", "rename"}

// perCollectionVerbs are the verbs computed for the collection root.
var perCollectionVerbs = []string{"create"}

// computeActions returns the per-item verb verdict map for entity e.
// Returns nil when the principal is anonymous (no Principal stamped on
// ctx) so the serializer can omit the field — the "show all + warn"
// fallback in the SPA distinguishes anonymous from authenticated-but-
// missing.
func (a *App) computeActions(ctx context.Context, e *entityPkg.Entity) map[string]bool {
	if isAnonymous(ctx) {
		return nil
	}
	out := make(map[string]bool, len(perItemVerbs))
	for _, v := range perItemVerbs {
		req, ok := translateVerb(v, e.Type)
		if !ok {
			continue
		}
		out[v] = a.acl.AuthorizeWrite(ctx, req).Allow
	}
	return out
}

// computeCollectionActions returns the collection-scope verb verdict
// map for an entity type. nil for anonymous principals.
func (a *App) computeCollectionActions(ctx context.Context, entityType string) map[string]bool {
	if isAnonymous(ctx) {
		return nil
	}
	out := make(map[string]bool, len(perCollectionVerbs))
	for _, v := range perCollectionVerbs {
		req, ok := translateVerb(v, entityType)
		if !ok {
			continue
		}
		out[v] = a.acl.AuthorizeWrite(ctx, req).Allow
	}
	return out
}

// isAnonymous returns true when ctx has no [principal.Principal]
// stamped — i.e. [principal.From] would fall through to its default.
// In production rela's data-entry router always stamps a principal,
// so HTTP requests always include `_actions`. The anonymous branch
// exists for tests and for any future code path that calls into
// serialization without going through the router (where the SPA's
// "show all, silent" fallback is the right default).
func isAnonymous(ctx context.Context) bool {
	return !principal.HasPrincipal(ctx)
}
