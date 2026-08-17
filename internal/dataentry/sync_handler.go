package dataentry

import (
	"context"
	"net/http"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// --- Consumer-side interfaces (declared at the call site per CLAUDE.md) ---

// syncStore is the read surface the sync handlers need from the store: single
// entity/relation gets. The manifest/applier capabilities are resolved by
// type-asserting the concrete store/manager in newSyncHandler rather than being
// listed here, since only pgstore / *entitymanager.Manager satisfy them.
type syncStore interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error)
}

// syncDeleter is the delete surface the conditional-delete handler needs from
// the entity manager. Distinct from syncApplier (the id-preserving push path):
// deletes go through the normal write path, not the automation-suppressed apply.
type syncDeleter interface {
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}

// syncHandler serves the /api/sync/ API (fs-client ↔ pg-server replication).
// Extracted from App (TKT-R68TV8): it owns the sync route cluster so the god
// object shrinks by ~16 methods.
//
// It holds narrow read (syncStore) and delete (syncDeleter) surfaces plus the
// two optional capabilities the sync protocol needs — the manifest source
// (pgstore-only) and the id-preserving applier (*entitymanager.Manager-only).
// Both are resolved once by newSyncHandler and left nil on the fs/memory builds,
// where the corresponding endpoints degrade to 501. The store/manager handles
// are fixed for App's lifetime (only the schema snapshot reloads), so there is
// nothing to re-resolve per request.
//
// writeMu is a POINTER to App's mutation mutex, not a private one: sync pushes
// and deletes must serialize against every OTHER data-entry mutation handler,
// not just against each other, so they share the App-wide write lock. (Once
// TKT-R68TV8 M5.4 moves write serialization behind the store, this field goes
// away.)
type syncHandler struct {
	store    syncStore
	deleter  syncDeleter
	manifest manifestProvider // nil unless the store is pgstore
	applier  syncApplier      // nil unless the manager is *entitymanager.Manager
	writeMu  *sync.Mutex

	// provision implements unmatched_principal: provision (TKT-ANUJDS), set by
	// App after construction. Called under writeMu at the top of each sync
	// write; a no-op unless an unmatched verified principal hits a provision
	// policy. See writeHandler.enterWrite for the shared rationale.
	provision func(context.Context) context.Context
}

// enterWrite acquires writeMu and runs the provision seam under it, returning
// the request the handler must use. The caller defers Unlock itself. Mirrors
// writeHandler.enterWrite so sync writes get the same coverage.
func (h *syncHandler) enterWrite(r *http.Request) *http.Request {
	h.writeMu.Lock()
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}

// newSyncHandler wires the sync route cluster. It resolves the two optional
// capabilities up front by type-asserting the concrete store/manager: the
// manifest source (only pgstore satisfies manifestProvider) and the
// id-preserving applier (only *entitymanager.Manager satisfies syncApplier).
// Both stay nil on the fs/memory builds, where the manifest and push endpoints
// return 501. writeMu is App's mutation mutex, shared by pointer.
func newSyncHandler(st syncStore, deleter syncDeleter, writeMu *sync.Mutex) *syncHandler {
	h := &syncHandler{store: st, deleter: deleter, writeMu: writeMu}
	if mp, ok := st.(manifestProvider); ok {
		h.manifest = mp
	}
	if ap, ok := deleter.(syncApplier); ok {
		h.applier = ap
	}
	return h
}

// registerSyncRoutes mounts the sync API under /api/sync/. See the handler
// godocs for the per-route contract. The routes inherit the data-entry
// security middleware EXCEPT the same-origin check, from which /api/sync/ is
// exempted (a non-browser sync client sends no Origin) — see
// middleware_security.go.
func (h *syncHandler) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sync/manifest", h.handleSyncManifest)
	mux.HandleFunc("/api/sync/", h.handleSyncRecord)
}
