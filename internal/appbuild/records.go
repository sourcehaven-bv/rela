package appbuild

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/records"
	"github.com/Sourcehaven-BV/rela/internal/records/filerecords"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// recordsDir is the file backend's directory in the project root: records
// are project data, so on the fs tier they travel with the project in git.
// recordsLockFile is its cross-process lock, in the cache directory.
const (
	recordsDir      = "records"
	recordsLockFile = "records.lock"
)

// recordBackend returns the record store for st. In order: the store a
// recipe supplied (sqlite, from rela.db), the store's own database backend
// (postgres, see storeRecordsFor), the file backend in the project root,
// and an in-memory file backend when the project has none.
//
// Any other backend plugs in the same way: implement [records.Store], pass
// recordstest.RunAll, and return it from a recipe override.
func recordBackend(
	fs storage.FS, paths *project.Context, st store.Store, override records.Store,
) (records.Store, error) {
	if override != nil {
		return override, nil
	}
	if s := storeRecordsFor(st); s != nil {
		return s, nil
	}
	if fs == nil || paths == nil || paths.Root == "" || paths.CacheDir == "" {
		slog.Warn("records: no project directory; records are kept in this process only")
		return memoryRecords(), nil
	}
	var locker filerecords.Locker = filelock.ProcessPrivate{}
	if !inMemoryFS(fs) {
		l, err := filelock.New(filepath.Join(paths.CacheDir, recordsLockFile))
		if err != nil {
			return nil, fmt.Errorf("records: %w", err)
		}
		locker = l
	}
	return filerecords.New(fs, filepath.Join(paths.Root, recordsDir), locker)
}

// memoryRecords returns a file backend over an in-memory filesystem: the
// records live as long as the process.
func memoryRecords() records.Store {
	s, err := filerecords.New(storage.NewMemFS(), "/records", filelock.ProcessPrivate{})
	if err != nil { // coverage-ignore: unreachable: every argument is set
		panic(err)
	}
	return s
}

// newRecordsService builds the record service over backend, with the owner
// gate of the current ACL. It returns nil when the schema declares no record
// types: the surfaces then offer nothing.
func newRecordsService(
	backend records.Store, meta *metamodel.Metamodel, st store.Store,
	d *acl.Declarative, redactor visibility.FieldRedactor, writes acl.ACL, w worlds.Compiled,
) (*records.Service, error) {
	if meta == nil || len(meta.Records) == 0 {
		return nil, nil //nolint:nilnil // no record types: the alias fanout drops a nil subscriber
	}
	if writes == nil {
		return nil, errors.New("records: a write ACL is required")
	}
	rows, _, _ := scriptReads(st, d, redactor, w)
	owners := recordOwners{rows: rows, writes: writes, raw: st}
	if d != nil {
		owners.bound = d
	}
	return records.NewService(records.Deps{
		Store:  backend,
		Owners: owners,
		Meta:   meta,
		Lock:   recordLock(st, backend),
	})
}

// recordOwners is the owner gate of the record service. Reads go through
// the gated reader with the Request bound at the request edge, so a caller
// without one reads nothing. Rows belong to the whole family, so both checks
// cover every stored face: a read needs read access on each, and a write
// needs update access on each.
type recordOwners struct {
	rows interface {
		Family(ctx context.Context, id string) (visibility.Family, bool, error)
		ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
	}
	writes interface {
		AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision
	}
	// raw reads the owner's stored properties for write checks such as a
	// ledger's closed period. It is the write-prep read the entity manager
	// also makes: a redacted read could hide the lock date and admit a
	// posting into a closed period. Nothing it reads reaches a response.
	// Inside the append's transaction the read goes to the transaction view
	// instead (see rawReader).
	raw rawOwnerReader
	// bound checks that ctx carries the Request bound at the request edge.
	// The gated reader answers a missing Request as "nothing is readable",
	// which would turn an unbound aggregate into an empty success; this
	// check makes every call fail with acl.ErrNoIdentity instead.
	//
	// Nil: accepted, and then nothing is checked. It is nil exactly when
	// there is no declarative policy (no acl.yaml, or a read-only override),
	// where reads are unrestricted and writes are decided by the write ACL
	// alone.
	bound interface {
		RequestFrom(ctx context.Context) (*acl.Request, error)
	}
}

// requireIdentity returns the error of a call made without the Request the
// request edge binds. See recordOwners.bound.
func (o recordOwners) requireIdentity(ctx context.Context) error {
	if o.bound == nil {
		return nil
	}
	if _, err := o.bound.RequestFrom(ctx); err != nil {
		return fmt.Errorf("records: %w", err)
	}
	return nil
}

// rawOwnerReader reads an owner's stored faces.
type rawOwnerReader interface {
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// rawReader returns the transaction view on ctx when the append runs in a
// store transaction (see txRecordLock), and the raw store otherwise. The
// lock date must be read on the view: a read on another connection could
// miss an update the transaction is already ordered after, and on sqlite it
// would not see the transaction's own state.
func (o recordOwners) rawReader(ctx context.Context) rawOwnerReader {
	if view := store.TxView(ctx); view != nil {
		return view
	}
	return o.raw
}

// Readable reports the owners in ids whose every stored face the caller may
// read. An owner with a hidden face is unreadable: its rows belong to that
// face too, and Writable refuses it for the same reason.
//
// The cost does not grow with len(ids): one gated header resolve counts the
// readable faces per id, and one raw header query over the ids it found
// counts the stored faces. A failed read is returned; it is the same for
// every id, so it tells no id apart.
func (o recordOwners) Readable(ctx context.Context, ids []string) (map[string]bool, error) {
	if err := o.requireIdentity(ctx); err != nil {
		return nil, err
	}
	refs := make([]entity.Ref, 0, len(ids))
	for _, id := range ids {
		// A bare id parses to the implicit face, the form ResolveHeaders
		// reads as the whole family. An id that does not parse is a miss.
		if ref, err := entity.ParseRef(id); err == nil && ref.Face.IsImplicit() {
			refs = append(refs, ref)
		}
	}
	got, err := o.rows.ResolveHeadersErr(ctx, refs)
	if err != nil {
		return nil, err
	}
	readable := make(map[string]int, len(got))
	found := make([]string, 0, len(got))
	for ref, h := range got {
		if h.Family && h.ReadableFaces > 0 {
			readable[ref.ID] = h.ReadableFaces
			found = append(found, ref.ID)
		}
	}
	out := make(map[string]bool, len(found))
	if len(found) == 0 {
		return out, nil
	}
	stored := make(map[string]int, len(found))
	q := store.EntityQuery{IDs: found, Faces: store.AllFaces()}
	for h, err := range store.ListEntityHeaders(ctx, o.rawReader(ctx), q) {
		if err != nil {
			return nil, fmt.Errorf("records: read owners: %w", err)
		}
		// A backend may match ids case-insensitively; a family is the
		// exact id, as in store.FamilyHeaders.
		if _, ok := readable[h.ID]; ok {
			stored[h.ID]++
		}
	}
	for id, n := range readable {
		if stored[id] == n {
			out[id] = true
		}
	}
	return out, nil
}

// Writable checks update access on every stored face of id. Rows belong to
// the whole family, not to one face, so no face is picked as a write target,
// and a face the caller cannot read refuses the write: its lock date still
// governs the rows.
func (o recordOwners) Writable(ctx context.Context, id string) (records.Owner, error) {
	if err := o.requireIdentity(ctx); err != nil {
		return records.Owner{}, err
	}
	fam, ok, err := o.rows.Family(ctx, id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return records.Owner{}, err
	}
	if !ok || err != nil {
		return records.Owner{}, fmt.Errorf("%w: %s", records.ErrNotFound, id)
	}
	raw := o.rawReader(ctx)
	stored, err := store.FamilyHeaders(ctx, raw, id)
	if err != nil {
		return records.Owner{}, fmt.Errorf("records: read owner %s: %w", id, err)
	}
	if len(stored) == 0 {
		// Deleted since the gated read: the owner is gone.
		return records.Owner{}, fmt.Errorf("%w: %s", records.ErrNotFound, id)
	}
	if len(stored) != len(fam.Faces) {
		return records.Owner{}, fmt.Errorf("%w: update %s", records.ErrForbidden, id)
	}
	owner := records.Owner{ID: id, Type: fam.Type, Faces: make([]map[string]any, 0, len(stored))}
	for _, h := range stored {
		face := h.Face
		dec := o.writes.AuthorizeWrite(ctx, acl.WriteRequest{
			Op: acl.OpUpdate, Subject: acl.NewEntitySubject(fam.Type, id, face),
		})
		if !dec.Allow {
			return records.Owner{}, fmt.Errorf("%w: update %s", records.ErrForbidden, id)
		}
		e, err := raw.GetEntity(ctx, entity.Ref{ID: id, Face: face})
		if err != nil {
			return records.Owner{}, fmt.Errorf("records: read owner %s: %w", id, err)
		}
		owner.Faces = append(owner.Faces, e.Properties)
	}
	return owner, nil
}

// Records returns the record service of s. It is a package function rather
// than a method because [Services] is at its plimsoll cap.
//
// Nil: returned when the schema declares no record types, and for a
// Services built by [NewFromCollaborators].
func Records(s *Services) *records.Service {
	if s == nil {
		return nil
	}
	return s.records
}

// recordStoreAliases keeps file-backed records attached to their owner
// across a rename and drops them with a deleted owner. It wraps the store
// rather than the service so rows of a type the schema no longer declares
// still follow their owner. It is an alias subscriber (see newAliasFanout).
//
// It runs after the entity write has committed, and a failure is only
// logged, so a missed drop would let a later entity with the same id inherit
// the rows. On postgres and sqlite the records live in the entity store's
// database, and triggers on the entity tables drop and rekey them inside the
// delete or rename itself (pgstore migration 0048, sqlitedb v40): a failure
// there rolls the entity write back, and no write path can skip it. Those
// tiers get no alias subscriber, because a late post-commit drop could
// remove the rows of a new entity that reused the id meanwhile. The fs and
// memory tiers keep this best-effort hook; a single local user is the only
// writer there.
type recordStoreAliases struct{ store records.Store }

// recordAliases returns the alias subscriber for backend over st.
//
// Nil: returns a nil pointer for a nil backend, and for a backend in the
// entity store's database, whose triggers do the work; newAliasFanout drops
// it.
func recordAliases(st store.Store, backend records.Store) *recordStoreAliases {
	if backend == nil || joinRecordsFor(st, backend) != nil {
		return nil
	}
	return &recordStoreAliases{store: backend}
}

// EntityRenamed moves the rows of oldID to newID. The rename has committed,
// so the move does not depend on the requester staying connected.
func (a *recordStoreAliases) EntityRenamed(ctx context.Context, oldID, newID string) error {
	if err := a.store.RekeyOwner(context.WithoutCancel(ctx), oldID, newID); err != nil {
		return fmt.Errorf("records: move rows from %s to %s: %w", oldID, newID, err)
	}
	return nil
}

// EntityDeleted drops the rows of id: ids can be reused, and a new entity
// must not inherit a stranger's rows.
func (a *recordStoreAliases) EntityDeleted(ctx context.Context, id string) error {
	if err := a.store.DropOwner(context.WithoutCancel(ctx), id); err != nil {
		return fmt.Errorf("records: drop rows of %s: %w", id, err)
	}
	return nil
}

// EntityFaceDeleted keeps the rows: they belong to the whole family, which
// still exists.
func (a *recordStoreAliases) EntityFaceDeleted(context.Context, string, entity.Face) error {
	return nil
}
