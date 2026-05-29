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

// EntityManager provides the high-level write API for entities and relations.
//
// **Transitional.** This is a producer-side interface (defined alongside
// its sole implementation, [Manager]), which the project explicitly
// avoids per CLAUDE.md "consumer-side interfaces" rule. Slated for
// removal: each call site should declare its own narrow consumer-side
// interface naming only the methods it invokes.
//
// Read operations are intentionally NOT on this interface — consumers
// read directly from [store.Store].
type EntityManager interface {
	// CreateEntity creates a new entity, running on-create automations.
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)

	// ValidateCreate runs the create path's defaults + validation against
	// a candidate WITHOUT persisting, authorizing, auditing, or running
	// automation. Returns the would-be entity (post-defaults) and soft
	// warnings. Advisory only — the real CreateEntity remains the sole
	// authorization/audit point.
	ValidateCreate(
		ctx context.Context, e *entity.Entity, opts entity.CreateOptions,
	) (*entity.Entity, []entity.Warning, error)

	// UpdateEntity updates an existing entity and runs on-update automations.
	// The caller passes the modified entity; the manager detects changes.
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)

	// DeleteEntity removes an entity and optionally cascades to its relations.
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)

	// RenameEntity changes an entity's ID, updating all relations.
	RenameEntity(ctx context.Context, oldID, newID string, opts entity.RenameOptions) (*entity.RenameResult, error)

	// CreateRelation creates a new relation.
	CreateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)

	// UpdateRelation updates an existing relation's data.
	UpdateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)

	// DeleteRelation removes a relation.
	DeleteRelation(ctx context.Context, from, relType, to string) error
}
