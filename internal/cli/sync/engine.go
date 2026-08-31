package sync

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LocalApplier is the id-preserving, automation-suppressed write path the pull
// command uses to land remote records locally. Declared at the call site
// (CLAUDE.md): *entitymanager.Manager satisfies it, and no other consumer's
// write interface names ApplyEntity/ApplyRelation — sync is their only
// consumer, mirroring the server side. Delete uses the manager's standard
// delete (a mirrored remote delete). Exported so the CLI wiring can type-assert
// the entity manager to it.
type LocalApplier interface {
	ApplyEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	ApplyRelation(ctx context.Context, r *entity.Relation) (*entity.Relation, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
	// RenameEntity re-keys a locally-created entity from its temporary id to the
	// primary-minted id after a push create (TKT-8P1TM7). The manager rewrites
	// every incident relation endpoint as part of the rename (RelationsUpdated),
	// so the replica's reference remap is a single call, not a manual sweep.
	RenameEntity(ctx context.Context, oldID, newID string, opts entity.RenameOptions) (*entity.RenameResult, error)
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

	// schema is the primary's type→plural map, fetched once per run by
	// ensureSchema and reused for every /api/v1/{plural}/{id} URL. nil until the
	// first fetch. The compatibility handshake (CheckSchemaCompatible) runs at
	// the same point.
	schema *RemoteSchema
	// local is the replica's own schema view for the compatibility handshake,
	// supplied by the CLI wiring (nil disables the check — e.g. in unit tests
	// that stub the remote schema directly).
	local *LocalSchema
}

// NewEngine constructs a sync engine.
//
// applier may be nil, but ONLY for a run that never writes locally. That is
// narrower than "push-only": pull and force-pull obviously write, and push does
// too on its id-adoption path (the primary mints an id differing from the local
// temp id, and adopting it is a local rename). Each of those three checks for
// nil and returns errLocalApplierRequired; none dereferences it blind.
//
// Production always supplies one — the CLI wiring holds it as a typed
// writeServices.SyncApplier field, so a missing applier is a compile error
// there. The nil case exists for tests that exercise read-only paths.
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

// SetLocalSchema supplies the replica's own schema view for the compatibility
// handshake. Call before Pull/Push; nil (the default) disables the check.
func (e *Engine) SetLocalSchema(local *LocalSchema) { e.local = local }

// ensureSchema fetches the primary's schema once per run (idempotent) and, when
// a local schema was supplied, runs the compatibility handshake — failing the
// whole run before any record is touched if the schemas diverge.
func (e *Engine) ensureSchema(ctx context.Context) error {
	if e.schema != nil {
		return nil
	}
	rs, err := e.client.Schema(ctx)
	if err != nil {
		return err
	}
	if e.local != nil {
		if cerr := rs.CheckSchemaCompatible(*e.local); cerr != nil {
			return cerr
		}
	}
	e.schema = rs
	return nil
}

// pluralFor resolves an entity type to its /api/v1 URL plural via the fetched
// schema. An unknown type is a hard error — the replica must not guess a route.
func (e *Engine) pluralFor(typeName string) (string, error) {
	if e.schema == nil {
		return "", errors.New("sync: schema not loaded (internal: ensureSchema not called)")
	}
	plural, ok := e.schema.Plural(typeName)
	if !ok {
		return "", fmt.Errorf("sync: no plural for entity type %q in the remote schema", typeName)
	}
	return plural, nil
}

// splitRelationKey reverses RelationKey: "from/type/to" -> (from, type, to).
func splitRelationKey(key string) (from, relType, to string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
