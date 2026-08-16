package dataentry

import (
	"context"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// --- Consumer-side interfaces (declared at the call site per CLAUDE.md) ---

// syncStore is the read surface the manifest filter needs from the store: a
// single entity get, used to resolve a relation entry's source type for the
// row-level read gate. The manifest capability itself is resolved by
// type-asserting the concrete store in newSyncHandler (only pgstore satisfies
// manifestProvider).
type syncStore interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
}

// syncHandler serves the sync CHANGE FEED under /api/sync/manifest (fs-client ↔
// pg-server replication, FEAT-NJ9FEN). The record read/write channel it used to
// own (/api/sync/entities|relations) was retired in TKT-8P1TM7: the sync client
// now reads and writes through the authorized /api/v1 API, so there is one
// content channel with one authorization decision, and the manifest is the only
// sync-specific surface that remains (a plain GET cannot express a tombstone,
// which is what the feed is for).
//
// The manifest source is pgstore-only (manifestProvider), resolved once by
// newSyncHandler and left nil on the fs/memory builds, where the endpoint
// degrades to 501.
type syncHandler struct {
	store    syncStore
	manifest manifestProvider // nil unless the store is pgstore
}

// newSyncHandler wires the manifest route. It resolves the optional manifest
// source by type-asserting the concrete store (only pgstore satisfies
// manifestProvider); it stays nil on the fs/memory builds, where the endpoint
// returns 501.
func newSyncHandler(st syncStore) *syncHandler {
	h := &syncHandler{store: st}
	if mp, ok := st.(manifestProvider); ok {
		h.manifest = mp
	}
	return h
}

// registerSyncRoutes mounts the sync change feed under /api/sync/manifest. The
// route inherits the data-entry security middleware EXCEPT the same-origin
// check, from which /api/sync/ is exempted (a non-browser sync client sends no
// Origin) — see middleware_security.go.
func (h *syncHandler) registerSyncRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sync/manifest", h.handleSyncManifest)
}
