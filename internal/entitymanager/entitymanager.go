// Package entitymanager defines the EntityManager service — the "human intent"
// write path that runs automations, validation, and any policy concerns
// (future: ACL, audit logging, rate limiting).
//
// This sits above the Store: reads and writes still go through a store.Store
// backend, but an EntityManager adds workflow concerns that shouldn't live in
// raw storage. Consumers use the manager; the store stays focused on CRUD.
//
// Not all consumers need a manager. Importers, bulk sync, and formatters
// bypass automations and talk to the store directly.
package entitymanager

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// CreateOptions configure entity creation.
type CreateOptions struct {
	// ID is an optional explicit ID. If empty, the manager generates one.
	ID string
	// Prefix overrides the default ID prefix when the entity type declares
	// multiple via `id_prefixes`. Ignored when ID is set or when the entity
	// type uses manual IDs.
	Prefix string
	// Variant selects an entity template variant (empty = default).
	Variant string
	// SkipAutomation suppresses on-create automations. Defaults to false.
	SkipAutomation bool
}

// Warning is a non-blocking finding surfaced to the caller alongside
// a successful write per DEC-HWZHA — a state the storage layer
// tolerated but that an analyze tool would also flag. Code values are
// stable and match the corresponding `analyze_*` finding codes where
// applicable. Path is an RFC 6901 JSON Pointer to the offending field.
//
// Warnings are NOT errors. The write succeeded; the warning is
// advisory. Consumers should surface them non-blockingly (HTTP body,
// MCP result text, CLI stderr, Lua second return).
type Warning struct {
	Code   string `json:"code"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
	// Direction is "outgoing" by default. When the warning was emitted
	// under an inverse body key in the unified PATCH, it's "incoming".
	// Lets UIs disambiguate same-edge warnings without parsing the
	// (free-form) JSON Pointer path. See TKT-GFQK.
	Direction string `json:"direction,omitempty"`
}

// CreateResult describes the outcome of a create, including automation
// side-effects.
type CreateResult struct {
	Entity             *entity.Entity
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
	AutomationWarnings []string
	AutomationErrors   []string
	// Warnings collects DEC-HWZHA soft validation findings on the
	// post-write entity. Nil when there are none. Sorted by Path for
	// stable client-facing ordering.
	Warnings []Warning `json:"warnings,omitempty"`
}

// UpdateResult describes the outcome of an update.
type UpdateResult struct {
	Entity             *entity.Entity
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
	AutomationWarnings []string
	AutomationErrors   []string
	// Warnings collects DEC-HWZHA soft validation findings on the
	// post-write entity. Nil when there are none. Sorted by Path for
	// stable client-facing ordering.
	Warnings []Warning `json:"warnings,omitempty"`
}

// DeleteResult describes entities and relations removed by a delete.
type DeleteResult struct {
	DeletedEntities  []*entity.Entity
	DeletedRelations []*entity.Relation
}

// RenameOptions configure entity renames.
type RenameOptions struct {
	// DryRun plans the rename without applying changes.
	DryRun bool
}

// RenameResult describes what was changed during a rename.
type RenameResult struct {
	OldID            string
	NewID            string
	RelationsUpdated int
}

// RelationOptions configure relation creation and updates.
//
// CreateRelation: Properties is the initial property map. MetaUnset is
// ignored (no existing values to clear). If Content is non-nil, the body
// is set to *Content (including the empty string); if nil, the body is
// empty.
//
// UpdateRelation: Properties MERGES into the existing relation's
// properties (an Update with empty Properties does NOT clear existing
// keys — use MetaUnset for that). After the merge, MetaUnset removes
// the named keys. If Content is non-nil, the body is replaced with
// *Content (including the empty string clears the body); if nil, the
// existing body is left untouched.
//
// The pointer-vs-string distinction on Content is the only way to
// express "leave the body alone" vs "set the body to empty"; callers
// that want to clear must pass a pointer to "".
type RelationOptions struct {
	Properties map[string]interface{}
	MetaUnset  []string
	Content    *string
}

// EntityManager provides the high-level write API for entities and relations.
//
// **Transitional.** This is a producer-side interface (defined alongside
// its sole implementation, [Manager]), which the project explicitly
// avoids per CLAUDE.md "consumer-side interfaces" rule. It exists today
// because the legacy [internal/workspace] shim returns it from
// `Workspace.Manager()` and several call sites depend on it. As call
// sites are migrated off workspace, each one should declare its own
// narrow consumer-side interface naming only the methods it invokes.
// This interface is scheduled for removal after TKT-64R3 deletes the
// workspace shim.
//
// Read operations are intentionally NOT on this interface — consumers
// read directly from [store.Store].
type EntityManager interface {
	// CreateEntity creates a new entity, running on-create automations.
	CreateEntity(ctx context.Context, e *entity.Entity, opts CreateOptions) (*CreateResult, error)

	// UpdateEntity updates an existing entity and runs on-update automations.
	// The caller passes the modified entity; the manager detects changes.
	UpdateEntity(ctx context.Context, e *entity.Entity) (*UpdateResult, error)

	// DeleteEntity removes an entity and optionally cascades to its relations.
	DeleteEntity(ctx context.Context, id string, cascade bool) (*DeleteResult, error)

	// RenameEntity changes an entity's ID, updating all relations.
	RenameEntity(ctx context.Context, oldID, newID string, opts RenameOptions) (*RenameResult, error)

	// CreateRelation creates a new relation.
	CreateRelation(ctx context.Context, from, relType, to string, opts RelationOptions) (*entity.Relation, error)

	// UpdateRelation updates an existing relation's data.
	UpdateRelation(ctx context.Context, from, relType, to string, opts RelationOptions) (*entity.Relation, error)

	// DeleteRelation removes a relation.
	DeleteRelation(ctx context.Context, from, relType, to string) error
}
