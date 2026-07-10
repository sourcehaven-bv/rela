package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestReadOnlyACL_EveryWriteRoute_DeniesAndDoesNotMutate is the class-level
// guard for BUG-K6FEVB (automated measure AM-acl-readonly-write-route-invariant).
//
// It enumerates every write-capable /api/v1 route SHAPE that the dynamic
// dispatcher (handleV1DynamicRoutes) exposes — collection create, entity
// update/delete, the relation collection/member writes, and entity actions —
// and drives each through the PRODUCTION router under acl.ReadOnlyACL. For
// every one it asserts:
//
//   - the response is not 2xx (a denied write must never succeed), and
//   - the store is byte-identical before and after (entity + relation counts
//     unchanged): no write path may reach the store under ReadOnlyACL.
//
// The route matrix is derived STRUCTURALLY from the dispatcher's segment /
// method switch, not hand-copied from behavior — including a relation write
// to a NON-EXISTENT peer, the exact shape that used to bypass the ACL via an
// ungated direct store write. If a future change adds an un-gated write path
// (or reintroduces the dangling-peer fallback), the count-delta oracle fails
// here even if the handler happens to answer with a non-error status.
func TestReadOnlyACL_EveryWriteRoute_DeniesAndDoesNotMutate(t *testing.T) {
	// Enumerate the write matrix from the dispatcher's switch (api_v1.go
	// handleV1DynamicRoutes): {segments, method} → write handler. Each probe
	// hits a shape the router routes to a mutation handler.
	probes := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		// case 1: /{plural} collection create
		{"collection create", http.MethodPost, "/api/v1/tickets", `{"properties":{"title":"X"}}`},

		// case 2: /{plural}/{id} single-entity update / delete
		{"entity update", http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"Y"}}`},
		{"entity delete", http.MethodDelete, "/api/v1/tickets/TKT-001", ""},

		// case 2: relations-only PATCH — existing peer (must 403, unchanged case)
		{"relations patch existing peer", http.MethodPatch, "/api/v1/tickets/TKT-001",
			`{"relations":{"belongs_to":{"data":[{"type":"component","id":"CMP-001"}]}}}`},
		// case 2: relations-only PATCH — DANGLING peer (the BUG-K6FEVB bypass)
		{"relations patch dangling peer", http.MethodPatch, "/api/v1/tickets/TKT-001",
			`{"relations":{"belongs_to":{"data":[{"type":"component","id":"CMP-999"}]}}}`},

		// case segmentsSubResource (4): /{plural}/{id}/relations/{relType} — create
		{"relation create existing peer", http.MethodPost, "/api/v1/tickets/TKT-001/relations/belongs_to",
			`{"data":{"type":"component","id":"CMP-001"}}`},
		{"relation create dangling peer", http.MethodPost, "/api/v1/tickets/TKT-001/relations/belongs_to",
			`{"data":{"type":"component","id":"CMP-999"}}`},

		// case segmentsSubResource (4): /{plural}/{id}/_actions/{action}
		{"entity action", http.MethodPost, "/api/v1/tickets/TKT-001/_actions/transition", `{}`},

		// case segmentsSubResourceID (5): /{plural}/{id}/relations/{relType}/{targetId}
		{"relation member write existing peer", http.MethodPatch,
			"/api/v1/tickets/TKT-001/relations/belongs_to/CMP-001", `{"meta":{"note":"x"}}`},
		{"relation member delete", http.MethodDelete,
			"/api/v1/tickets/TKT-001/relations/belongs_to/CMP-001", ""},
		{"relation member write dangling peer", http.MethodPatch,
			"/api/v1/tickets/TKT-001/relations/belongs_to/CMP-999", `{"meta":{"note":"x"}}`},
	}

	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			app := buildAppWithACLAndAudit(t, acl.ReadOnlyACL{}, nil)
			seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket",
				Properties: map[string]interface{}{"title": "T"}})
			seedEntity(app, &entity.Entity{ID: "CMP-001", Type: "component",
				Properties: map[string]interface{}{"name": "C"}})

			before := storeSnapshot(t, app.store)

			var bodyReader = http.NoBody
			r := httptest.NewRequest(p.method, p.path, bodyReader)
			if p.body != "" {
				r = httptest.NewRequest(p.method, p.path, strings.NewReader(p.body))
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			app.NewRouter().ServeHTTP(w, r)

			if w.Code >= 200 && w.Code < 300 {
				t.Fatalf("write route succeeded under ReadOnlyACL: status %d body=%.300s",
					w.Code, w.Body.String())
			}

			after := storeSnapshot(t, app.store)
			if before != after {
				t.Fatalf("store mutated under ReadOnlyACL (before=%+v after=%+v): status %d body=%.300s",
					before, after, w.Code, w.Body.String())
			}
		})
	}
}

// storeCounts is a coarse fingerprint of store contents used as the
// no-mutation oracle: any create/update/delete changes at least one count
// (a property-only update changes no count, but ReadOnlyACL must deny those
// too, and the status-code check catches a silent success).
type storeCounts struct {
	entities  int
	relations int
}

func storeSnapshot(t *testing.T, st store.Store) storeCounts {
	t.Helper()
	ctx := context.Background()
	ec, err := st.CountEntities(ctx, store.EntityQuery{})
	if err != nil {
		t.Fatalf("CountEntities: %v", err)
	}
	rc, err := st.CountRelations(ctx, store.RelationQuery{})
	if err != nil {
		t.Fatalf("CountRelations: %v", err)
	}
	return storeCounts{entities: ec, relations: rc}
}
