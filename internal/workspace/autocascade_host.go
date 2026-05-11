package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Workspace implements [autocascade.Host] during the transition off
// the god-object pattern (FEAT-workspace). The implementation
// forwards to Workspace's existing entity/relation operations.
//
// The forwarders intentionally do not export private workspace
// methods or rename them — they are thin shims so that future
// migrations can swap Workspace out for entitymanager.Manager without
// touching the Host contract.
var _ autocascade.Host = (*Workspace)(nil)

// CreateEntityNoCascade creates an entity without running automations
// on it. Runner invokes this for each automation.EntityToCreate;
// Runner itself manages the cascade depth and follow-up evaluation.
func (w *Workspace) CreateEntityNoCascade(
	entityType string, opts autocascade.CreateEntityOptions,
) (*entity.Entity, error) {
	return w.createEntityCore(entityType, createEntityCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.IDPrefix,
		TemplateVariant: opts.TemplateVariant,
		Properties:      opts.Properties,
		Content:         opts.Content,
	})
}

// WriteEntity satisfies [autocascade.Host.WriteEntity] by forwarding
// to the existing writeEntity private method.
func (w *Workspace) WriteEntity(e *entity.Entity) error {
	return w.writeEntity(e)
}

// WriteRelation satisfies [autocascade.Host.WriteRelation] by
// forwarding to writeRelationCore (which adds the error-context
// wrapping the older dispatch code expected).
func (w *Workspace) WriteRelation(r *entity.Relation) error {
	return w.writeRelationCore(r)
}

// DeleteEntity satisfies [autocascade.Host.DeleteEntity]. The first
// parameter (entityType) is ignored because Workspace.deleteEntity
// looks up the type from the store; it is included in the Host
// signature so alternative implementations have the type at hand.
func (w *Workspace) DeleteEntity(_ context.Context, entityType, id string, cascade bool) error {
	_, err := w.deleteEntity(entityType, id, cascade)
	return err
}
