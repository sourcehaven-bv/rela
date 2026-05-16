package autocascade

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Mutator is the graph-mutation surface a scripted automation action
// may invoke. Defined here at the consumer per CLAUDE.md "interfaces
// at the call site" — Runner is the consumer; entitymanager.Manager
// satisfies it structurally.
//
// The interface mirrors the seven methods of entitymanager.EntityManager
// so that the same value (a Manager) can serve both as the public
// write API and as the per-cascade script-mutation handle. Today only
// five of these are exercised by Lua bindings (CreateEntity /
// UpdateEntity / DeleteEntity / CreateRelation / DeleteRelation); the
// other two (RenameEntity, UpdateRelation) are carried for shape
// symmetry. A future hygiene PR may narrow this to the actually-called
// subset.
//
// The interface is engine-agnostic: any future script runtime (Python,
// JS, ...) that mutates the graph receives a Mutator alongside its
// action payload, not a Lua-typed deps bundle.
type Mutator interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	RenameEntity(ctx context.Context, oldID, newID string, opts entity.RenameOptions) (*entity.RenameResult, error)
	CreateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	UpdateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}
