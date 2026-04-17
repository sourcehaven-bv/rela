// Package storemanage defines the EntityManager service — the "human intent"
// write path that runs automations, validation, and any policy concerns
// (future: ACL, audit logging, rate limiting).
//
// This sits above the Store: reads and writes still go through a store.Store
// backend, but an EntityManager adds workflow concerns that shouldn't live in
// raw storage. Consumers use the manager; the store stays focused on CRUD.
//
// Not all consumers need a manager. Importers, bulk sync, and formatters
// bypass automations and talk to the store directly.
package storemanage

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// CreateOptions configure entity creation.
type CreateOptions struct {
	// ID is an optional explicit ID. If empty, the manager generates one.
	ID string
	// Variant selects an entity template variant (empty = default).
	Variant string
	// SkipAutomation suppresses on-create automations. Defaults to false.
	SkipAutomation bool
}

// CreateResult describes the outcome of a create, including automation
// side-effects.
type CreateResult struct {
	Entity             *entity.Entity
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
	AutomationWarnings []string
	AutomationErrors   []string
}

// UpdateResult describes the outcome of an update.
type UpdateResult struct {
	Entity             *entity.Entity
	RelationsCreated   []*entity.Relation
	EntitiesCreated    []*entity.Entity
	AutomationWarnings []string
	AutomationErrors   []string
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

// RelationOptions configure relation creation.
type RelationOptions struct {
	Properties map[string]interface{}
	Content    string
}

// EntityManager provides the high-level write API for entities and relations.
// Implementations run automations, validate, and may add policy concerns
// (ACL, audit, etc.) on top of the underlying Store.
//
// Read operations are intentionally NOT on this interface today — consumers
// read directly from store.Store. If the manager needs to enforce read
// policies later, reads can be added here.
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
