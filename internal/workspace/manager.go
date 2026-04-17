package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
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

	created, result, err := m.w.CreateEntity(e.Type, createOpts)
	if err != nil {
		return nil, err
	}

	return &entitymanager.CreateResult{
		Entity:             model.EntityToDomain(created),
		RelationsCreated:   relationsToDomain(result.RelationsCreated),
		EntitiesCreated:    entitiesToDomain(result.EntitiesCreated),
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

	// Build updated model.Entity by applying entity.Entity's fields over current.
	updated := current.Clone()
	updated.Properties = make(map[string]interface{}, len(e.Properties))
	for k, v := range e.Properties {
		updated.Properties[k] = v
	}
	updated.Content = e.Content

	result, err := m.w.UpdateEntity(updated, current)
	if err != nil {
		return nil, err
	}

	return &entitymanager.UpdateResult{
		Entity:             model.EntityToDomain(updated),
		RelationsCreated:   relationsToDomain(result.RelationsCreated),
		EntitiesCreated:    entitiesToDomain(result.EntitiesCreated),
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

	_, err := m.w.DeleteEntity(current.Type, id, cascade)
	if err != nil {
		return nil, err
	}

	// Workspace.DeleteEntity returns only a count, not the deleted relations.
	// We return just the deleted entity.
	return &entitymanager.DeleteResult{
		DeletedEntities:  []*entity.Entity{model.EntityToDomain(current)},
		DeletedRelations: nil,
	}, nil
}

func (m *wsEntityManager) RenameEntity(
	_ context.Context, oldID, newID string, opts entitymanager.RenameOptions,
) (*entitymanager.RenameResult, error) {
	current, ok := m.w.GetEntity(oldID)
	if !ok {
		return nil, &entityNotFoundError{ID: oldID}
	}

	result, err := m.w.Rename(current.Type, oldID, newID, rename.Options{DryRun: opts.DryRun})
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
	r, err := m.w.CreateRelation(from, relType, to, CreateRelationOptions{
		Properties: opts.Properties,
		Content:    opts.Content,
	})
	if err != nil {
		return nil, err
	}
	return model.RelationToDomain(r), nil
}

func (m *wsEntityManager) UpdateRelation(
	_ context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	r, err := m.w.UpdateRelation(from, relType, to, CreateRelationOptions{
		Properties: opts.Properties,
		Content:    opts.Content,
	})
	if err != nil {
		return nil, err
	}
	return model.RelationToDomain(r), nil
}

func (m *wsEntityManager) DeleteRelation(_ context.Context, from, relType, to string) error {
	return m.w.DeleteRelation(from, relType, to)
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

// entitiesToDomain converts []*model.Entity → []*entity.Entity.
func entitiesToDomain(models []*model.Entity) []*entity.Entity {
	if models == nil {
		return nil
	}
	out := make([]*entity.Entity, len(models))
	for i, m := range models {
		out[i] = model.EntityToDomain(m)
	}
	return out
}

// relationsToDomain converts []*model.Relation → []*entity.Relation.
func relationsToDomain(models []*model.Relation) []*entity.Relation {
	if models == nil {
		return nil
	}
	out := make([]*entity.Relation, len(models))
	for i, m := range models {
		out[i] = model.RelationToDomain(m)
	}
	return out
}
