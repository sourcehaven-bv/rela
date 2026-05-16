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
// **Why seven methods.** The interface mirrors all seven methods of
// entitymanager.EntityManager because today's `lua.WriteDeps.EntityManager`
// is typed as that wide interface and the Lua script adapter assigns
// `m` straight into the field. If Mutator were narrower (the five
// methods Lua actually calls — RenameEntity and UpdateRelation are
// unused from scripts) the assignment wouldn't type-check.
//
// TKT-IF37 tracks narrowing both surfaces to five — after that the
// interface body shrinks. Until then the two extra methods are
// carried for assignment compatibility, not because any script
// invokes them.
//
// **Transport-vocabulary, not engine-runtime, agnostic.** The
// interface is independent of which script runtime is doing the
// invoking (Lua today; Python/JS later would receive a Mutator the
// same way). It is *not* free of rela's domain vocabulary: the
// method signatures reference entity.CreateOptions / CreateResult etc.
// Any new engine adapter still speaks rela's write-API types.
type Mutator interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	RenameEntity(ctx context.Context, oldID, newID string, opts entity.RenameOptions) (*entity.RenameResult, error)
	CreateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	UpdateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}
