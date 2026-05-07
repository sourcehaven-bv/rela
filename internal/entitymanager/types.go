package entitymanager

import "github.com/Sourcehaven-BV/rela/internal/model"

// UpdateWithRelationsRequest is the input to Manager.UpdateWithRelations.
// It mirrors the wire format of PATCH /api/v1/{plural}/{id} but uses
// neutral types — no JSON tags, no HTTP concepts. Callers (HTTP handler,
// MCP tool, tests) translate to/from their own shapes.
type UpdateWithRelationsRequest struct {
	// EntityID identifies the entity to update. Required.
	EntityID string

	// ExpectedType, if non-empty, is checked against the live entity's
	// type. A mismatch returns ErrEntityNotFound to avoid leaking the
	// existence of an entity at the same ID under a different type.
	ExpectedType string

	// Properties is a partial property map. Keys present are upserted;
	// keys absent are left alone. Use PropertiesUnset to clear keys.
	Properties map[string]interface{}

	// PropertiesUnset names properties to clear. Unknown keys against
	// the entity's type return *ValidationError.
	PropertiesUnset []string

	// Content, if non-nil, replaces the entity's markdown body.
	// Pass a pointer to "" to clear; pass nil to leave alone.
	Content *string

	// Relations is the desired-state map for outgoing relations,
	// keyed by relation type. Absent key = leave alone. Empty Edges
	// = remove all of that type. Non-empty Edges = upsert/replace.
	Relations map[string]RelationDesiredState

	// IfMatch, if non-empty, is compared against the entity's current
	// ETag (computed via ETagFn). On mismatch, returns ErrETagMismatch.
	// Callers without optimistic-concurrency needs leave this empty.
	IfMatch string

	// ETagFn computes the ETag for a given entity. Required when
	// IfMatch is set; ignored otherwise. The function must be
	// deterministic on the entity's persisted state (id + type +
	// content + properties).
	ETagFn func(*model.Entity) string
}

// RelationDesiredState is the wrapper for one relation type's desired
// state. Edges is the full replacement set: edges in the slice are
// kept/upserted, current edges absent from the slice are removed. An
// empty (or nil) slice means "remove all of this type".
//
// The wire-level distinction between "field absent" (leave alone) and
// "data: []" (remove all) is enforced upstream — when the caller maps
// the wire request to a manager request, it must omit a relation type
// from the map to mean "leave alone".
type RelationDesiredState struct {
	Edges []RelationRef
}

// RelationRef is one resource identifier with rela's per-edge upsert
// fields. Type and ID are required; the rest are optional and follow
// the wire-format upsert semantics.
type RelationRef struct {
	// Type is the target entity's type. Must match both the live
	// entity's type and the relation's allowed targets.
	Type string

	// ID is the target entity's ID.
	ID string

	// Meta merges into the existing relation's properties (upsert).
	// Absent = leave existing meta alone. Unknown keys against the
	// relation type's closed schema return *ValidationError.
	Meta map[string]interface{}

	// MetaUnset clears the named keys after the merge.
	MetaUnset []string

	// Content, if non-nil, replaces the relation's markdown body.
	// Only valid for relation types declared with Content: true;
	// otherwise returns *ValidationError.
	Content *string
}

// UpdateWithRelationsResult is what Manager.UpdateWithRelations returns
// on success. It carries enough information for the caller to render
// a response and emit observability events.
type UpdateWithRelationsResult struct {
	// Entity is the post-commit entity (or, on a no-op, the unchanged
	// live entity). Always non-nil on success.
	Entity *model.Entity

	// NoOp is true when no writes occurred — the request was
	// equivalent to the current state. SSE callers should suppress
	// events in this case; HTTP callers can still return 200.
	NoOp bool

	// Counterparties is the set of entity IDs whose relations were
	// modified by symmetric/inverse propagation. The primary entity
	// (request.EntityID) is NOT included. SSE callers fan out one
	// `entity:updated` event per ID here in addition to the one for
	// the primary.
	Counterparties []string

	// Side-effect bookkeeping mirrored from automation.
	AutomationWarnings []string
	AutomationErrors   []string
	RelationsCreated   []*model.Relation
	EntitiesCreated    []*model.Entity
}
