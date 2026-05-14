package workspace

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// wsEntityManager is a transitional adapter that forwards every write
// to the workspace's held [entitymanager.Manager]. It exists because
// consumers reach EntityManager via the workspace shim today; TKT-64R3
// replaces the shim with direct Manager construction at each wiring
// site and deletes this adapter along with [internal/workspace].
type wsEntityManager struct {
	w *Workspace
}

var _ entitymanager.EntityManager = (*wsEntityManager)(nil)

func (m *wsEntityManager) CreateEntity(
	ctx context.Context, e *entity.Entity, opts entitymanager.CreateOptions,
) (*entitymanager.CreateResult, error) {
	return m.w.manager.CreateEntity(ctx, e, opts)
}

func (m *wsEntityManager) UpdateEntity(
	ctx context.Context, e *entity.Entity,
) (*entitymanager.UpdateResult, error) {
	return m.w.manager.UpdateEntity(ctx, e)
}

func (m *wsEntityManager) DeleteEntity(
	ctx context.Context, id string, cascade bool,
) (*entitymanager.DeleteResult, error) {
	return m.w.manager.DeleteEntity(ctx, id, cascade)
}

func (m *wsEntityManager) RenameEntity(
	ctx context.Context, oldID, newID string, opts entitymanager.RenameOptions,
) (*entitymanager.RenameResult, error) {
	return m.w.manager.RenameEntity(ctx, oldID, newID, opts)
}

func (m *wsEntityManager) CreateRelation(
	ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	return m.w.manager.CreateRelation(ctx, from, relType, to, opts)
}

func (m *wsEntityManager) UpdateRelation(
	ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions,
) (*entity.Relation, error) {
	return m.w.manager.UpdateRelation(ctx, from, relType, to, opts)
}

func (m *wsEntityManager) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return m.w.manager.DeleteRelation(ctx, from, relType, to)
}

// EntityManager returns the entity management service.
func (w *Workspace) EntityManager() entitymanager.EntityManager {
	return &wsEntityManager{w: w}
}
