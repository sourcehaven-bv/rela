package lua

import "github.com/Sourcehaven-BV/rela/internal/model"

// WorkspaceInterface defines the workspace operations needed by the Lua runtime.
// This follows the Go idiom of "accept interfaces, return structs" - the consumer
// (lua) defines the interface it needs, and the provider (workspace) implements it.
//
// This enables dependency inversion: lua doesn't know about workspace, but can
// still call workspace methods through this interface.
type WorkspaceInterface interface {
	// Entity queries
	GetEntity(id string) (*model.Entity, bool)
	EntitiesByType(entityType string) []*model.Entity

	// Entity mutations (using primitives to avoid import cycles)
	CreateEntityLua(entityType, id string, props map[string]interface{}, content string) (*model.Entity, error)
	UpdateEntityLua(entity, oldEntity *model.Entity) error
	DeleteEntityLua(entityType, id string, cascade bool) error

	// Relation queries
	AllRelations() []*model.Relation

	// Relation mutations
	CreateRelationLua(from, relType, to string) (*model.Relation, error)
	DeleteRelation(from, relType, to string) error

	// Graph operations
	TraceFrom(id string, maxDepth int) *model.TraceResult
	TraceTo(id string, maxDepth int) *model.TraceResult
	FindPath(from, to string) []model.PathStep

	// Search
	SearchSimple(query string, limit int) ([]*model.Entity, error)

	// Sync
	SyncLua() error
}
