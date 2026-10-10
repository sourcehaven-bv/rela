package appbuild

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/kvpiles"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// refHolders are the services that hold references by entity id, plus the
// piles backend, which a re-assembly shares.
type refHolders struct {
	comments  *comments.Service
	piles     *piles.Service
	pileStore piles.Store
}

// buildRefHolders builds the two services that hold references by entity id:
// comments and piles. Both ride the AliasRewriter hook rather than
// store.EntityObserver for the reason that hook documents: stores fire the
// observer with the error discarded, which is fine for a rebuildable search
// index but not for records that exist only in these services' own stores.
//
// The piles service is built per assembly, so its owner check follows the
// current ACL; its backend comes from the predecessor on a re-assembly.
func buildRefHolders(
	base *SharedBase, st store.Store, commentStore comments.Store,
	d *acl.Declarative, redactor visibility.FieldRedactor,
) (refHolders, error) {
	var h refHolders
	var err error
	if h.comments, err = buildComments(base.cfg.FS, base.cfg.Paths, base.meta, commentStore); err != nil {
		return refHolders{}, err
	}
	if h.pileStore = base.pileStore; h.pileStore == nil {
		if h.pileStore, err = pileBackend(base.cfg.FS, base.cfg.Paths, st); err != nil {
			return refHolders{}, err
		}
	}
	h.piles, err = piles.NewService(h.pileStore, piles.Options{
		OwnerExists: pileOwnerExists(d, redactor, st, base.worlds),
		PersonType:  pilePersonType(d),
	})
	if err != nil {
		return refHolders{}, err
	}
	return h, nil
}

// pileBackend returns the piles backend for st: the store's own (see
// storePilesFor), else the KV backend over the node-local cache directory.
//
// A re-assembly does not call this; it shares its predecessor's backend
// (see [SharedBase.ForReassembly]), while the service over it is rebuilt with
// the new ACL.
func pileBackend(fs storage.FS, paths *project.Context, st store.Store) (piles.Store, error) {
	if backend := storePilesFor(st); backend != nil {
		return backend, nil
	}
	localKV, err := buildStateKV(fs, paths)
	if err != nil {
		return nil, err
	}
	if _, nop := localKV.(nopKV); nop || localKV == nil {
		slog.Warn("piles: no cache directory; piles will not survive a restart")
		return memoryPiles(), nil
	}
	locker, err := pilesLocker(fs, paths.CacheDir)
	if err != nil {
		return nil, err
	}
	return kvpiles.New(localKV, locker)
}

// pilesLockFile is the lock file next to the piles document in the cache
// directory. Every process that opens the project locks it (see
// [kvpiles.FileLocker]).
const pilesLockFile = "piles.lock"

// pilesLocker returns the cross-process lock for the cache-directory KV over
// fs. Only an in-memory fs gets the process-private locker: no other process
// can open it. Every other fs is taken to be the disk, because guessing wrong
// in that direction loses writes between the desktop app, `rela mcp`, CLI
// scripts and the scheduler.
func pilesLocker(fs storage.FS, cacheDir string) (kvpiles.Locker, error) {
	if _, mem := fs.(*storage.MemFS); mem {
		return kvpiles.ProcessPrivate{}, nil
	}
	return kvpiles.NewFileLocker(filepath.Join(cacheDir, pilesLockFile))
}

// pilePersonType is the ACL's person type when person mapping is configured
// (user_entity_type and principal_property both set), else empty. Only then
// is an owner a person entity id that a rename or delete may move or drop.
func pilePersonType(d *acl.Declarative) string {
	if d == nil {
		return ""
	}
	p := d.Policy()
	userType := strings.TrimSpace(p.UserEntityType)
	if userType == "" || strings.TrimSpace(p.PrincipalProperty) == "" {
		return ""
	}
	return userType
}

// memoryPiles returns a KV piles backend that lives in process memory and
// so lasts for the process only. It is the fallback where the no-op KV would
// otherwise accept every write and keep none, as newUserState does.
func memoryPiles() piles.Store {
	s, err := kvpiles.New(memoryKV(), kvpiles.ProcessPrivate{})
	if err != nil { // coverage-ignore: unreachable: both arguments are non-nil
		panic(err)
	}
	return s
}

// memoryKV returns a state.KV that lives in process memory.
func memoryKV() state.KV {
	mem := storage.NewMemFS()
	rfs, err := storage.NewRootedFS(mem, "/")
	if err != nil { // coverage-ignore: unreachable: NewRootedFS only fails on a nil fs or an empty root
		panic(err)
	}
	return state.NewFSKV(rfs)
}

// pileOwnerExists returns the check that lets a push target another user:
// does the ACTING principal on ctx see a person entity with that id.
//
// It answers through the principal's read gate ([visibility.Resolver.Family],
// a header read of the id restricted to the policy's person type), never a
// raw store read, so "unknown owner" and "no such person" are the same answer
// for a person the pusher cannot read.
//
// Nil: returns nil without person mapping (see pilePersonType), which the
// service reads as "only the acting user". Also nil when the gate cannot be
// built, with an error logged: refusing pushes to others fails closed.
func pileOwnerExists(
	d *acl.Declarative, redactor visibility.FieldRedactor, st store.Store, w worlds.Compiled,
) piles.OwnerExistsFunc {
	userType := pilePersonType(d)
	if userType == "" {
		return nil
	}
	if redactor == nil {
		redactor = visibility.NopRedactor{}
	}
	gate, err := visibility.NewDeclarativeGate(d, w.DefaultWorld())
	if err != nil {
		slog.Error("appbuild: ACL gate unavailable; pushes to other users' piles REFUSED", "err", err)
		return nil
	}
	res, err := visibility.NewResolver(gate, redactor, st, familiesOption(w))
	if err != nil {
		slog.Error("appbuild: resolver unavailable; pushes to other users' piles REFUSED", "err", err)
		return nil
	}
	return func(ctx context.Context, id string) (bool, error) {
		_, ok, err := res.Family(ctx, userType, id)
		return ok, err
	}
}

// Piles returns the per-user piles service.
//
// Nil: never returned on an assembled Services; every build has a backend.
func (s *Services) Piles() *piles.Service { return s.piles }
