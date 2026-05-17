package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// cascadeHost satisfies [autocascade.Host]. It is the surface
// [autocascade.Runner] calls back into during a cascade. cascadeHost
// is constructed per-call inside [Manager.CreateEntity] /
// [Manager.UpdateEntity] (never held as a field) so its lifetime is
// scoped to a single Process call — the form CLAUDE.md's
// "consumer-side interfaces for callbacks" pattern endorses for
// dissolving cycles.
//
// **Important contract:** [cascadeHost.CreateEntity] must NOT fire
// follow-up automation cascades. Runner is the one that schedules
// cascade evaluation on the returned entity; double-cascading would
// enforce [autocascade.MaxDepth] twice and reorder creations.
//
// Audit: cascadeHost emits audit records directly (bypassing
// Manager's recordEntityAudit / recordRelationAudit) because it
// bypasses Manager itself — going through createCore / upsertEntity
// to avoid double-cascading. Records carry triggered_by="automation"
// (or the cascade-delete label when invoked from IfExistsReplace) to
// distinguish them from direct writes.
type cascadeHost struct {
	deps Deps
}

// Compile-time assertion.
var _ autocascade.Host = (*cascadeHost)(nil)

// CreateEntity satisfies [autocascade.Host.CreateEntity]. It calls
// the package-level [createCore] helper directly, **without**
// running automations afterward (Runner manages follow-up cascade
// scheduling on the result).
func (h *cascadeHost) CreateEntity(
	ctx context.Context, entityType string, opts autocascade.CreateEntityOptions,
) (*entity.Entity, error) {
	// Cascade-driven creates discard warnings — the autocascade.Host
	// contract returns only (*entity.Entity, error). The Runner doesn't
	// propagate per-step warnings; they'd be merged into the trigger's
	// entity.CreateResult.Warnings if we extended Outcome, but that's a
	// separate change.
	e, _, err := createCore(ctx, h.deps, entityType, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.IDPrefix,
		TemplateVariant: opts.TemplateVariant,
		Properties:      opts.Properties,
		Content:         opts.Content,
	})
	if err == nil {
		h.recordCascade(ctx, audit.OpCreateEntity, entitySubject(e), "created")
	}
	return e, err
}

// WriteEntity satisfies [autocascade.Host.WriteEntity] by performing
// a bare upsert against the store.
//
// Note: no audit record here. WriteEntity is invoked by the Runner
// to persist post-cascade property changes onto an entity that was
// just created via CreateEntity in the same cascade — the create
// already produced one audit record. Emitting another for the
// property-set step would double-count the same entity creation in
// the audit log.
func (h *cascadeHost) WriteEntity(ctx context.Context, e *entity.Entity) error {
	return upsertEntity(ctx, h.deps.Store, e)
}

// GetEntity satisfies [autocascade.Host.GetEntity] by forwarding to
// the store.
func (h *cascadeHost) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return h.deps.Store.GetEntity(ctx, id)
}

// WriteRelation satisfies [autocascade.Host.WriteRelation] by
// performing a bare upsert against the store.
func (h *cascadeHost) WriteRelation(ctx context.Context, r *entity.Relation) error {
	if err := upsertRelation(ctx, h.deps.Store, r); err != nil {
		return err
	}
	h.recordCascade(ctx, audit.OpCreateRelation, relationSubject(r), "created")
	return nil
}

// ValidateRelation satisfies [autocascade.Host.ValidateRelation] by
// delegating to the metamodel.
func (h *cascadeHost) ValidateRelation(relType, fromType, toType string) error {
	return h.deps.Meta.ValidateRelation(relType, fromType, toType)
}

// DeleteEntity satisfies [autocascade.Host.DeleteEntity]. It mirrors
// [Manager.DeleteEntity]'s incident-relation handling. The entityType
// parameter is informational — the store looks up the type from the
// entity itself.
//
// triggered_by attribution: invoked only from the IfExistsReplace
// path. Stamp `cascade:delete-entity:<id>` on the ctx so the
// cascaded relation deletes are attributed to the replacement
// operation, matching the direct-DeleteEntity convention in Manager.
func (h *cascadeHost) DeleteEntity(ctx context.Context, _, id string, cascade bool) error {
	current, err := h.deps.Store.GetEntity(ctx, id)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}

	incoming := collectIncidentRelations(ctx, h.deps.Store, id, store.DirectionIncoming)
	outgoing := collectIncidentRelations(ctx, h.deps.Store, id, store.DirectionOutgoing)
	if (len(incoming)+len(outgoing)) > 0 && !cascade {
		return ErrHasRelations
	}

	cascadeCtx := ctx
	if cascade && (len(incoming)+len(outgoing)) > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
	}

	for _, rel := range incoming {
		if delErr := h.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		h.recordCascade(cascadeCtx, audit.OpDeleteRelation, relationSubject(rel), "deleted")
	}
	for _, rel := range outgoing {
		if delErr := h.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
		h.recordCascade(cascadeCtx, audit.OpDeleteRelation, relationSubject(rel), "deleted")
	}

	if _, delErr := h.deps.Store.DeleteEntity(ctx, id, false); delErr != nil &&
		!errors.Is(delErr, store.ErrNotFound) {

		return fmt.Errorf("delete entity: %w", delErr)
	}
	h.recordCascade(ctx, audit.OpDeleteEntity, entitySubject(current), "deleted")
	return nil
}

// FindExistingRelationTarget satisfies
// [autocascade.Host.FindExistingRelationTarget].
func (h *cascadeHost) FindExistingRelationTarget(
	ctx context.Context, sourceID, relationType, targetType string,
) *entity.Entity {
	return findExistingRelationTarget(ctx, h.deps.Store, sourceID, relationType, targetType)
}

// entitySubject builds the Subject for an entity-shaped audit record.
func entitySubject(e *entity.Entity) *audit.Subject {
	return &audit.Subject{
		Kind: "entity",
		Type: e.Type,
		ID:   e.ID,
	}
}

// relationSubject builds the Subject for a relation-shaped audit record.
func relationSubject(r *entity.Relation) *audit.Subject {
	return &audit.Subject{
		Kind:         "relation",
		RelationType: r.Type,
		FromID:       r.From,
		ToID:         r.To,
	}
}

// recordCascade emits one audit record from the cascade path. If ctx
// doesn't already carry a triggered_by, "automation" is stamped as
// the generic label (per runner.go applyRelationCreations rationale —
// automation.Result doesn't carry per-action names through the
// engine). Callers that already wrapped ctx with a specific label
// (e.g. cascade:delete-entity:<id>) keep that label.
//
// One emitter for both entity and relation subjects — the Subject
// pointer carries the shape; the rest of the Record envelope is
// identical.
func (h *cascadeHost) recordCascade(
	ctx context.Context, op string, subject *audit.Subject, summary string,
) {
	if subject == nil {
		return
	}
	if audit.TriggeredByFrom(ctx) == "" {
		ctx = audit.WithTriggeredBy(ctx, "automation")
	}
	h.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}
