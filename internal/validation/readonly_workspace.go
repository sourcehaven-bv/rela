package validation

import (
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// ErrReadOnly is returned when a mutation is attempted on a read-only workspace.
var ErrReadOnly = errors.New("mutations not allowed in validation scripts")

// readOnlyWorkspace wraps a WorkspaceInterface and blocks all mutation operations.
// This provides a safe read-only view of the workspace for Lua validation scripts.
type readOnlyWorkspace struct {
	ws lua.WorkspaceInterface
}

// newReadOnlyWorkspace creates a read-only wrapper around the given workspace.
func newReadOnlyWorkspace(ws lua.WorkspaceInterface) *readOnlyWorkspace {
	return &readOnlyWorkspace{ws: ws}
}

// Read operations - delegate to underlying workspace

func (r *readOnlyWorkspace) GetEntity(id string) (*model.Entity, bool) {
	return r.ws.GetEntity(id)
}

func (r *readOnlyWorkspace) EntitiesByType(entityType string) []*model.Entity {
	return r.ws.EntitiesByType(entityType)
}

func (r *readOnlyWorkspace) AllRelations() []*model.Relation {
	return r.ws.AllRelations()
}

func (r *readOnlyWorkspace) TraceFrom(id string, maxDepth int) *model.TraceResult {
	return r.ws.TraceFrom(id, maxDepth)
}

func (r *readOnlyWorkspace) TraceTo(id string, maxDepth int) *model.TraceResult {
	return r.ws.TraceTo(id, maxDepth)
}

func (r *readOnlyWorkspace) FindPath(from, to string) []model.PathStep {
	return r.ws.FindPath(from, to)
}

func (r *readOnlyWorkspace) SearchSimple(query string, limit int) ([]*model.Entity, error) {
	return r.ws.SearchSimple(query, limit)
}

// Mutation operations - blocked

func (r *readOnlyWorkspace) CreateEntityLua(
	_, _ string,
	_ map[string]interface{},
	_ string,
) (*model.Entity, error) {
	return nil, ErrReadOnly
}

func (r *readOnlyWorkspace) UpdateEntityLua(_, _ *model.Entity) error {
	return ErrReadOnly
}

func (r *readOnlyWorkspace) DeleteEntityLua(_, _ string, _ bool) error {
	return ErrReadOnly
}

func (r *readOnlyWorkspace) CreateRelationLua(_, _, _ string) (*model.Relation, error) {
	return nil, ErrReadOnly
}

func (r *readOnlyWorkspace) DeleteRelation(_, _, _ string) error {
	return ErrReadOnly
}

// SyncLua is blocked because it mutates in-memory state by reloading from disk.
func (r *readOnlyWorkspace) SyncLua() error {
	return ErrReadOnly
}

// Ensure readOnlyWorkspace implements lua.WorkspaceInterface.
var _ lua.WorkspaceInterface = (*readOnlyWorkspace)(nil)
