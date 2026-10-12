package appbuild

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/kvlease"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// leasesLockFile is the lock file next to the lease document in the cache
// directory, taken by every process that opens the project.
const leasesLockFile = "leases.lock"

// leaseShared is what successive assemblies over one store share for
// leases: the backend, and the hub that wakes subscribers, so a subscriber
// keeps hearing about changes after a schema reload rebuilt the service.
//
// Only a re-assembly over the same store shares it ([SharedBase.ForReassembly]:
// `rela mcp` and the desktop app). rela-server rebuilds on a Configure save
// through Discover, which opens a new store, so each generation gets its own
// backend and hub. That loses nothing on the KV and database backends, whose
// leases live in the cache directory or the database. It does lose every
// lease on the in-process fallback ([memoryLeases], used only when the
// project has no cache directory). Handing the predecessor's leaseShared to
// Discover would need an ownership rule for Close, which closes the backend;
// for a fallback that already warns at startup, the loss is documented
// instead.
type leaseShared struct {
	store lease.Store
	hub   *lease.Hub
}

// buildLeases builds the lease service for one assembly over shared, or over
// a new backend and hub for st when shared is empty (a first assembly).
//
// The service is per assembly, so its access checks follow the current ACL:
// reads go through the gated reader [Services.GatedReads] wraps, built from
// the same inputs, so a claim answers not-found exactly where show_entity
// does; updates go through writes, the resolved write ACL.
func buildLeases(
	base *SharedBase, st store.Store, shared leaseShared,
	d *acl.Declarative, redactor visibility.FieldRedactor, writes acl.ACL,
) (*lease.Service, leaseShared, error) {
	if shared.store == nil {
		backend, err := leaseBackend(base.cfg.FS, base.cfg.Paths, st)
		if err != nil {
			return nil, leaseShared{}, err
		}
		shared = leaseShared{store: backend, hub: lease.NewHub()}
	}
	rows, _ := scriptReads(st, d, redactor, base.worlds)
	svc, err := lease.NewService(lease.Deps{
		Store:      shared.store,
		Gate:       leaseGate{rows: rows},
		Authorizer: writes,
		Clock:      lease.SystemClock{},
		Hub:        shared.hub,
	})
	if err != nil {
		return nil, leaseShared{}, err
	}
	return svc, shared, nil
}

// leaseGate adapts the gated reader to [lease.Gate], so package lease does
// not import visibility. Every read resolves the ctx principal per call.
type leaseGate struct {
	rows interface {
		Family(ctx context.Context, id string) (visibility.Family, bool, error)
		WriteTarget(ctx context.Context, addr string) (entity.Ref, error)
		ResolveHeadersErr(ctx context.Context, refs []entity.Ref) (map[entity.Ref]visibility.ResolvedHeader, error)
	}
}

// Readable answers through one batch header resolve: the row gate on each
// bare id and the readable faces of its stored type, at a cost that does not
// grow with len(ids). [visibility.ResolvedHeader.Family] is the
// world-independent "some face is readable" answer, the question
// [lease.Gate.Family] asks for one id. A failed header read is returned; it
// is the same for every id, so it tells no id apart.
func (g leaseGate) Readable(ctx context.Context, ids []string) (map[string]bool, error) {
	refs := make([]entity.Ref, 0, len(ids))
	for _, id := range ids {
		// A bare id parses to the implicit face, the form ResolveHeaders
		// reads as the whole family. An id that does not parse is a miss.
		if ref, err := entity.ParseRef(id); err == nil && ref.Face.IsImplicit() {
			refs = append(refs, ref)
		}
	}
	got, err := g.rows.ResolveHeadersErr(ctx, refs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(got))
	for ref, h := range got {
		if h.Family {
			out[ref.ID] = true
		}
	}
	return out, nil
}

// WriteTarget wraps an ambiguous address in [lease.AmbiguousError], keeping
// the visibility error in its chain for callers that look for it.
func (g leaseGate) WriteTarget(ctx context.Context, addr string) (entity.Ref, error) {
	ref, err := g.rows.WriteTarget(ctx, addr)
	if _, ok := errors.AsType[*visibility.AmbiguousAddressError](err); ok {
		return entity.Ref{}, &lease.AmbiguousError{Err: err}
	}
	return ref, err
}

func (g leaseGate) Family(ctx context.Context, id string) (lease.Family, bool, error) {
	fam, ok, err := g.rows.Family(ctx, id)
	if err != nil || !ok {
		return lease.Family{}, false, err
	}
	return lease.Family{Type: fam.Type, Faces: fam.Faces}, true, nil
}

// leaseBackend returns the lease backend for st: the store's own (see
// storeLeasesFor), else the KV backend over the node-local cache directory,
// with the cross-process lock chosen as in pilesLocker.
func leaseBackend(fs storage.FS, paths *project.Context, st store.Store) (lease.Store, error) {
	if backend := storeLeasesFor(st); backend != nil {
		return backend, nil
	}
	localKV, err := buildStateKV(fs, paths)
	if err != nil {
		return nil, err
	}
	if _, nop := localKV.(nopKV); nop || localKV == nil {
		slog.Warn("leases: no cache directory; leases are kept in this process only")
		return memoryLeases(), nil
	}
	var locker kvlease.Locker = filelock.ProcessPrivate{}
	if _, mem := fs.(*storage.MemFS); !mem {
		if locker, err = filelock.New(filepath.Join(paths.CacheDir, leasesLockFile)); err != nil {
			return nil, err
		}
	}
	return kvlease.New(localKV, locker)
}

// memoryLeases returns a KV lease backend in process memory. Its leases end
// with the [Services] that built it; a rela-server Configure save therefore
// drops them (see leaseShared).
func memoryLeases() lease.Store {
	s, err := kvlease.New(memoryKV(), filelock.ProcessPrivate{})
	if err != nil { // coverage-ignore: unreachable: both arguments are non-nil
		panic(err)
	}
	return s
}

// WatchLeases runs the lease watcher of s (see [lease.Service.Watch]) until
// ctx is cancelled, and returns a channel closed once it has stopped. Only
// long-running hosts call it.
func WatchLeases(ctx context.Context, s *Services) <-chan struct{} {
	done := make(chan struct{})
	if s.leases == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		if err := s.leases.Watch(ctx); err != nil {
			slog.Warn("leases: watcher stopped", "error", err)
		}
	}()
	return done
}

// Leases returns the advisory lease service of s. It is a package function
// rather than a method because [Services] is at its plimsoll cap.
//
// Nil: returned only for a Services built by [NewFromCollaborators]; every
// assembled build has a backend.
func Leases(s *Services) *lease.Service { return s.leases }
