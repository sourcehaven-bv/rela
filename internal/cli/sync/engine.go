package sync

import (
	"context"
	"errors"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LocalApplier is the id-preserving, automation-suppressed write path the pull
// command uses to land remote records locally. Declared at the call site
// (CLAUDE.md): *entitymanager.Manager satisfies it, but the broad EntityManager
// interface deliberately omits ApplyEntity/ApplyRelation — sync is their only
// consumer, mirroring the server side. Delete uses the manager's standard
// delete (a mirrored remote delete). Exported so the CLI wiring can type-assert
// the entity manager to it.
type LocalApplier interface {
	ApplyEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	ApplyRelation(ctx context.Context, r *entity.Relation) (*entity.Relation, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}

// Engine carries the collaborators shared by push and pull: the remote client,
// the local store (read side, for snapshotting + hashing), the local applier
// (write side, for pull), and the in-memory index. The caller loads the index,
// runs push and/or pull, then saves the index so progress is durable even on a
// partial run.
type Engine struct {
	client  *Client
	store   store.Store
	applier LocalApplier
	idx     *State
}

// NewEngine constructs a sync engine. applier may be nil for a push-only run
// (push never writes locally); pull requires it and errors if it is nil.
func NewEngine(client *Client, st store.Store, applier LocalApplier, idx *State) (*Engine, error) {
	if client == nil {
		return nil, errors.New("sync engine: client is required")
	}
	if st == nil {
		return nil, errors.New("sync engine: store is required")
	}
	if idx == nil {
		return nil, errors.New("sync engine: index is required")
	}
	return &Engine{client: client, store: st, applier: applier, idx: idx}, nil
}

// Index returns the engine's in-memory index so the caller can Save it.
func (e *Engine) Index() *State { return e.idx }

// splitRelationKey reverses RelationKey: "from/type/to" -> (from, type, to).
func splitRelationKey(key string) (from, relType, to string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
