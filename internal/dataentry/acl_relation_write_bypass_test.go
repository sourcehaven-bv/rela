package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestReadOnlyACL_DanglingPeerRelationWrite_Refused is the PoC-as-test for
// BUG-K6FEVB: under acl.ReadOnlyACL (i.e. `rela-server --read-only`), a
// relations-only PATCH that adds an edge to a NON-EXISTENT peer must be
// REFUSED — no store mutation, no successful-write audit record, and a
// non-200 response.
//
// Pre-fix, the EntityManager ran the peer-existence check before the ACL
// check, so a missing peer returned a soft "entity not found" (not a
// *acl.ForbiddenError); the dataentry reconciler then fell back to a DIRECT,
// UNGATED store write. Result: 200 + a persisted edge with no audit — a full
// ACL bypass. This test fails against that code and passes after the fix.
//
// The reconciler now refuses a peer the caller cannot read before the manager
// runs (requireReadablePeers), so the refusal is the dangling-peer 422 rather
// than the manager's 403, and no denied-write record is written. A hidden
// peer gets the same answer, which a manager-side 403 could not give: the
// manager's ACL sees a hidden source's type and an absent one's as different.
func TestReadOnlyACL_DanglingPeerRelationWrite_Refused(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.ReadOnlyACL{}, sink)

	// Only the source exists; the peer (CMP-999) does not.
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket",
		Properties: map[string]any{"title": "T"},
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"belongs_to":{"data":[{"type":"component","id":"CMP-999"}]}}}`))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code == http.StatusOK {
		t.Fatalf("read-only ACL let a dangling-peer relation write through (200): %s", rec.Body.String())
	}
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "target_not_found") {
		t.Fatalf("expected 422 target_not_found, got %d: %s", rec.Code, rec.Body.String())
	}

	// The edge must not exist in the store.
	if _, err := app.store.GetRelation(t.Context(), entity.RelationKey{From: "TKT-001", Type: "belongs_to", To: "CMP-999"}); err == nil {
		t.Fatal("dangling-peer edge persisted under ReadOnlyACL; ACL/audit bypass")
	}

	// No successful create-relation audit record.
	for _, r := range sink.Records() {
		if r.Op == audit.OpCreateRelation {
			t.Errorf("unexpected create-relation audit record for a refused write: %+v", sink.Records())
		}
	}
}

// TestReadOnlyACL_ExistingPeerRelationWrite_Forbidden pins that the fix did
// not weaken the pre-existing case: a relation between two EXISTING entities
// under ReadOnlyACL still 403s and writes nothing.
func TestReadOnlyACL_ExistingPeerRelationWrite_Forbidden(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.ReadOnlyACL{}, sink)

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "CMP-001", Type: "component", Properties: map[string]any{"name": "C"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"belongs_to":{"data":[{"type":"component","id":"CMP-001"}]}}}`))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := app.store.GetRelation(t.Context(), entity.RelationKey{From: "TKT-001", Type: "belongs_to", To: "CMP-001"}); err == nil {
		t.Fatal("edge persisted despite ReadOnlyACL deny")
	}
}

// TestDanglingPeerRelationWrite_AllowedACL_422 pins the third invariant of
// BUG-K6FEVB: when the ACL ALLOWS the write but the peer does not exist, the
// write is a HARD 422 (reference did not resolve) rather than a 200-with-
// warning + ungated store write. Uses NopACL so authz passes and the
// missing-peer path is exercised.
func TestDanglingPeerRelationWrite_AllowedACL_422(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"belongs_to":{"data":[{"type":"component","id":"CMP-999"}]}}}`))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (dangling peer, ACL allows), got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "target_not_found") {
		t.Errorf("expected target_not_found in body, got %s", rec.Body.String())
	}
	if _, err := app.store.GetRelation(t.Context(), entity.RelationKey{From: "TKT-001", Type: "belongs_to", To: "CMP-999"}); err == nil {
		t.Fatal("dangling-peer edge persisted despite hard 422")
	}
}

// TestTypeMismatchRelationWrite_RefusedUnaudited pins DEC-0D8UZX3 (GitHub
// #1992) where GitHub #1806 (ISMS CONTROL-8-15) once pinned an audited
// write: a type-allowlist mismatch is refused with 422, so no edge is stored
// and no write is audited.
func TestTypeMismatchRelationWrite_RefusedUnaudited(t *testing.T) {
	sink := audit.NewMemory()
	app := buildAppWithACLAndAudit(t, acl.NopACL{}, sink)

	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "U"}})

	// belongs_to allows ticket -> component only; TKT-002 is a ticket.
	who := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"relations":{"belongs_to":{"data":[{"type":"ticket","id":"TKT-002"}]}}}`))
	req = req.WithContext(principal.With(req.Context(), who))
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "ticket", "tickets", "TKT-001")

	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "target_type_not_allowed") {
		t.Fatalf("expected 422 target_type_not_allowed, got %d: %s", rec.Code, rec.Body.String())
	}
	key := entity.RelationKey{From: "TKT-001", Type: "belongs_to", To: "TKT-002"}
	if _, err := app.store.GetRelation(t.Context(), key); err == nil {
		t.Fatal("refused edge was stored")
	}
	if recs := sink.Records(); len(recs) != 0 {
		t.Errorf("refused write audited: %+v", recs)
	}
}
