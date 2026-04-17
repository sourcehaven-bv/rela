package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/rename"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// wsEntityManager adapts the workspace's legacy write API to the
// entitymanager.EntityManager interface. It converts entity.Entity at the
// boundary and delegates to the workspace's automation-aware methods.
type wsEntityManager struct {
	w *Workspace
}

var _ entitymanager.EntityManager = (*wsEntityManager)(nil)

func (m *wsEntityManager) CreateEntity(
	_ context.Context, e *entity.Entity, opts entitymanager.CreateOptions,
) (*entitymanager.CreateResult, error) {
	if e == nil {
		return nil, nil
	}

	createOpts := CreateOptions{
		ID:         opts.ID,
		Properties: e.Properties,
		Content:    e.Content,
	}
	_ = opts.Variant // Variant not yet plumbed through workspace.CreateEntity

	created, result, err := m.w.createEntity(e.Type, createOpts)
	if err != nil {
		return nil, err
	}

	return &entitymanager.CreateResult{
		Entity:             created,
		RelationsCreated:   result.RelationsCreated,
		EntitiesCreated:    result.EntitiesCreated,
		AutomationWarnings: result.AutomationWarnings,
		AutomationErrors:   result.AutomationErrors,
	}, nil
}

func (m *wsEntityManager) UpdateEntity(
	_ context.Context, e *entity.Entity,
) (*entitymanager.UpdateResult, error) {
	if e == nil {
		return nil, nil
	}

	// Find current state for oldEntity.
	current, ok := m.w.GetEntity(e.ID)
	if !ok {
		return nil, &entityNotFoundError{ID: e.ID}
	}

	// Build updated entity by applying e's fields over current.
	updated := current.Clone()
	updated.Properties = make(map[string]interface{}, len(e.Properties))
	for k, v := range e.Properties {
		updated.Properties[k] = v
	}
	updated.Content = e.Content

	result, err := m.w.updateEntity(updated, current)
	if err != nil {
		return nil, err
	}

	return &entitymanager.UpdateResult{
		Entity:             updated,
		RelationsCreated:   result.RelationsCreated,
		EntitiesCreated:    result.EntitiesCreated,
		AutomationWarnings: result.AutomationWarnings,
		AutomationErrors:   result.AutomationErrors,
	}, nil
}

func (m *wsEntityManager) DeleteEntity(
	_ context.Context, id string, cascade bool,
) (*entitymanager.DeleteResult, error) {
	// Workspace.DeleteEntity needs entity type; look it up.
	current, ok := m.w.GetEntity(id)
	if !ok {
		return nil, &entityNotFoundError{ID: id}
	}

	// Capture relations before delete so we can report what got cascaded.
	var deletedRels []*entity.Relation
	if cascade {
		deletedRels = append(deletedRels, m.w.IncomingRelations(id)...)
		deletedRels = append(deletedRels, m.w.OutgoingRelations(id)...)
	}

	_, err := m.w.deleteEntity(current.Type, id, cascade)
	if err != nil {
		return nil, err
	}

	return &entitymanager.DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: deletedRels,
	}, nil
}

func (m *wsEntityManager) RenameEntity(
	_ context.Context, oldID, newID string, opts entitymanager.RenameOptions,
) (*entitymanager.RenameResult, error) {
	current, ok := m.w.GetEntity(oldID)
	if !ok {
		return nil, &entityNotFoundError{ID: oldID}
	}

	result, err := m.w.rename(current.Type, oldID, newID, rename.Options{DryRun: opts.DryRun})
	if err != nil {
		return nil, err
	}

	return &entitymanager.RenameResult{
		OldID:            result.OldID,
		NewID:            result.NewID,
		RelationsUpdated: len(result.RelationsUpdated),
	}, nil
}

func (m *wsEntityManager) CreateRelation(
	_ context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	return m.w.createRelation(from, relType, to, CreateRelationOptions{
		Properties: opts.Properties,
		Content:    opts.Content,
	})
}

func (m *wsEntityManager) UpdateRelation(
	_ context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	return m.w.updateRelation(from, relType, to, CreateRelationOptions{
		Properties: opts.Properties,
		Content:    opts.Content,
	})
}

func (m *wsEntityManager) DeleteRelation(_ context.Context, from, relType, to string) error {
	return m.w.deleteRelation(from, relType, to)
}

// EntityManager returns the entity management service.
func (w *Workspace) EntityManager() entitymanager.EntityManager {
	return &wsEntityManager{w: w}
}

// entityNotFoundError is a local error type for the manager.
type entityNotFoundError struct {
	ID string
}

func (e *entityNotFoundError) Error() string { return "entity not found: " + e.ID }

