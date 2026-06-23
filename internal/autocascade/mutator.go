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
// Five methods — the subset that scripted automations actually call.
// `RenameEntity` and `UpdateRelation` are intentionally absent; if a
// future script binding needs them, extend Mutator at that time.
// (Narrowed from seven to five in TKT-IF37; the wider shape was a
// transitional artifact of TKT-Z9MR matching the pre-narrowing
// lua.WriteDeps.EntityManager type.)
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
	CreateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}

// ElevatedProvider is an OPTIONAL capability a Mutator may expose
// (TKT-D8T148): it hands back a second Mutator whose writes skip the ACL deny,
// for an `allow_acl_bypass` automation action that calls `rela.bypass_acl(...)`.
// The script runner type-asserts the per-cascade Mutator to this interface and
// uses Elevated() ONLY when the action is allow_acl_bypass; the elevated handle
// is scoped to the bypass closure and invalidated after it returns.
//
// Kept separate from Mutator so the elevated capability is opt-in and a
// Mutator implementation without it (e.g. a test double) simply doesn't grant
// bypass — there is no way to elevate a Mutator that doesn't choose to offer it.
type ElevatedProvider interface {
	// Elevated returns a Mutator whose writes bypass the ACL deny. The
	// returned handle must NOT propagate elevation into nested cascades.
	Elevated() Mutator
}
