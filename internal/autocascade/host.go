// Package autocascade executes the side effects of automation results
// produced by [automation.Engine]. Engine is the pure decision-maker;
// Runner here is the executor that turns its [automation.Result] into
// actual writes, recursive automation invocation, and Lua script
// execution.
//
// Runner does not hold a back-reference to whatever invokes it. Instead
// it declares a [Host] interface naming exactly the methods it calls
// back into, and the caller passes a Host on every [Runner.Process]
// call. This is the consumer-side-interface pattern documented in
// CLAUDE.md under "Consumer-side interfaces for callbacks and cycles".
// During the workspace decomposition (FEAT-workspace), Workspace
// satisfies Host directly; once entitymanager.Manager exists, it will
// satisfy Host instead. Per-call passing avoids the constructor cycle
// either way.
package autocascade

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Host is what Runner needs from its caller to execute a cascade.
// Each method documents where Runner invokes it.
//
// Method names describe the contract from Runner's perspective, not
// the caller's implementation. For example, [Host.CreateEntityNoCascade]
// names the property Runner relies on (no recursive automation
// inside it — Runner manages cascade scheduling itself); the
// implementer's own method may be called something else.
type Host interface {
	// Meta returns the active metamodel. Runner uses it to validate
	// automation-generated relations against the schema before
	// writing them.
	Meta() *metamodel.Metamodel

	// Store returns the authoritative store. Runner reads relation
	// targets through it to check that automation-generated relations
	// point at extant entities.
	Store() store.Store

	// CreateEntityNoCascade creates an entity *without* running
	// automations on the newly created entity. Runner calls this for
	// each automation.EntityToCreate; Runner is responsible for
	// queueing follow-up automation evaluation on the result.
	//
	// The "NoCascade" name is load-bearing: if the implementer's
	// version runs automations internally, the cascade depth limit
	// will be enforced twice and entity ordering will drift.
	CreateEntityNoCascade(entityType string, opts CreateEntityOptions) (*entity.Entity, error)

	// WriteEntity upserts the entity to the store without running
	// automations or validation. Runner uses it when applying
	// [automation.Result.PropertiesSet] to a cascaded entity (i.e.,
	// the trigger of a follow-up automation event).
	WriteEntity(e *entity.Entity) error

	// WriteRelation upserts the relation to the store. Runner uses
	// it for entries in [automation.Result.RelationsToCreate] and
	// for trigger relations attached to automation-created entities.
	WriteRelation(r *entity.Relation) error

	// DeleteEntity removes an entity, cascading to its relations
	// when cascade is true. Runner calls this only via the
	// IfExistsReplace path of [automation.EntityToCreate].
	//
	// entityType is informational — the implementer may ignore it
	// (workspace.deleteEntity does today). It's in the signature so
	// alternative implementations have the entity type available
	// without re-fetching.
	DeleteEntity(ctx context.Context, entityType, id string, cascade bool) error

	// FindExistingRelationTarget returns the existing target entity
	// of the given relation type from the source entity, if any.
	// Returns nil if no such relation exists. Runner uses it to
	// implement the IfExists behaviors (Skip / Error / Replace).
	FindExistingRelationTarget(sourceID, relationType, targetType string) *entity.Entity
}

// CreateEntityOptions configures a [Host.CreateEntityNoCascade] call.
// Field names mirror workspace.createEntityCoreOpts so an implementer
// satisfying both can use a simple forwarder.
type CreateEntityOptions struct {
	// ID is a caller-supplied ID. Empty means auto-generate via
	// IDPrefix and the metamodel's prefix rules.
	ID string

	// IDPrefix overrides the default ID prefix from the metamodel.
	// Ignored when ID is non-empty or when the entity type uses
	// manual IDs.
	IDPrefix string

	// TemplateVariant selects an entity template variant. Empty
	// means the default template.
	TemplateVariant string

	// Properties seeds the new entity's properties, overriding
	// template defaults.
	Properties map[string]interface{}

	// Content is the markdown body content. Overrides the template's
	// content when non-empty.
	Content string
}
