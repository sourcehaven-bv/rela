package entitymanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
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
	entityType string, opts autocascade.CreateEntityOptions,
) (*entity.Entity, error) {
	return createCore(context.Background(), h.deps, entityType, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.IDPrefix,
		TemplateVariant: opts.TemplateVariant,
		Properties:      opts.Properties,
		Content:         opts.Content,
	})
}

// WriteEntity satisfies [autocascade.Host.WriteEntity] by performing
// a bare upsert against the store.
func (h *cascadeHost) WriteEntity(e *entity.Entity) error {
	return upsertEntity(context.Background(), h.deps.Store, e)
}

// GetEntity satisfies [autocascade.Host.GetEntity] by forwarding to
// the store.
func (h *cascadeHost) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return h.deps.Store.GetEntity(ctx, id)
}

// WriteRelation satisfies [autocascade.Host.WriteRelation] by
// performing a bare upsert against the store.
func (h *cascadeHost) WriteRelation(r *entity.Relation) error {
	return upsertRelation(context.Background(), h.deps.Store, r)
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
func (h *cascadeHost) DeleteEntity(ctx context.Context, _, id string, cascade bool) error {
	if _, err := h.deps.Store.GetEntity(ctx, id); err != nil {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}

	incoming := collectIncidentRelations(ctx, h.deps.Store, id, store.DirectionIncoming)
	outgoing := collectIncidentRelations(ctx, h.deps.Store, id, store.DirectionOutgoing)
	if (len(incoming)+len(outgoing)) > 0 && !cascade {
		return ErrHasRelations
	}

	for _, rel := range incoming {
		if delErr := h.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
	}
	for _, rel := range outgoing {
		if delErr := h.deps.Store.DeleteRelation(ctx, rel.From, rel.Type, rel.To); delErr != nil &&
			!errors.Is(delErr, store.ErrNotFound) {

			continue
		}
	}

	if _, err := h.deps.Store.DeleteEntity(ctx, id, false); err != nil &&
		!errors.Is(err, store.ErrNotFound) {

		return fmt.Errorf("delete entity: %w", err)
	}
	return nil
}

// FindExistingRelationTarget satisfies
// [autocascade.Host.FindExistingRelationTarget].
func (h *cascadeHost) FindExistingRelationTarget(
	sourceID, relationType, targetType string,
) *entity.Entity {
	return findExistingRelationTarget(context.Background(), h.deps.Store, sourceID, relationType, targetType)
}
