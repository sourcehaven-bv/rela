package dataentry

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// translateVerb maps a wire-format verb to the [acl.WriteRequest] that
// authorizes that operation. It is the single source of truth for the
// "same code path" invariant: both the affordance serializer and the
// write handlers route their [acl.WriteRequest] construction through
// here. A grep test (`lint_test.go`) enforces that no other site in
// internal/dataentry constructs `acl.WriteRequest{Op:` directly.
//
// The verb set is closed and lives next to its callers; the only sites
// that pass verbs in are [perItemVerbs] and [perCollectionVerbs] in
// this file. Adding a new verb requires an entry here plus an
// [acl.Op] constant.
func translateVerb(verb, entityType string) acl.WriteRequest {
	switch verb {
	case "create":
		return acl.WriteRequest{Op: acl.OpCreate, EntityType: entityType}
	case "update":
		return acl.WriteRequest{Op: acl.OpUpdate, EntityType: entityType}
	case "delete":
		return acl.WriteRequest{Op: acl.OpDelete, EntityType: entityType}
	case "rename":
		return acl.WriteRequest{Op: acl.OpRename, EntityType: entityType}
	}
	// Unreachable for the closed verb set above. A panic here would
	// signal a bug in a future commit — better than silently returning
	// a zero WriteRequest that maps every verb to OpCreate.
	panic("dataentry.translateVerb: unknown verb: " + verb)
}

// perItemVerbs are the verbs computed per entity instance.
var perItemVerbs = []string{"update", "delete", "rename"}

// perCollectionVerbs are the verbs computed for the collection root.
var perCollectionVerbs = []string{"create"}

// computeActions returns the per-item verb verdict map for entity e.
// Every authenticated data-entry request reaches this through the
// router middleware, so the map is always populated for HTTP traffic;
// callers that synthesize their own context (tests, future non-HTTP
// callers) get the same `{verb: bool}` shape evaluated against
// whatever Principal is on ctx, defaulting to `principal.From`'s
// "unknown" sentinel.
func (a *App) computeActions(ctx context.Context, e *entityPkg.Entity) map[string]bool {
	out := make(map[string]bool, len(perItemVerbs))
	for _, v := range perItemVerbs {
		out[v] = a.acl.AuthorizeWrite(ctx, translateVerb(v, e.Type)).Allow
	}
	return out
}

// computeCollectionActions returns the collection-scope verb verdict
// map for an entity type — currently just `create`.
func (a *App) computeCollectionActions(ctx context.Context, entityType string) map[string]bool {
	out := make(map[string]bool, len(perCollectionVerbs))
	for _, v := range perCollectionVerbs {
		out[v] = a.acl.AuthorizeWrite(ctx, translateVerb(v, entityType)).Allow
	}
	return out
}
