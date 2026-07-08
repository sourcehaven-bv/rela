package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The fsstore-backed test App's store is NOT a store.HistoryReader, so these
// tests exercise the two behaviors that don't need a pgstore: the
// unsupported-backend response, and the deleted/absent-entity 404 that must not
// become an existence oracle. The redaction and restore-field-validation paths
// (which need a HistoryReader) are covered by the pgstore DB tests plus the
// serializer/affordance contract tests those paths reuse.

func TestHandleV1History_UnsupportedBackend(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket/TKT-1", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1History(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("unsupported backend: got %d, want 501; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleV1History_InvalidPath(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	// Missing the id segment.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1History(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid path: got %d, want 400", rec.Code)
	}
}

// TestAuthorizeHistoryRead_AbsentEntityNoPermissionIs404 pins the no-oracle
// invariant: a caller without history:read asking for a non-existent (or
// deleted) entity's history gets the SAME 404 as a nonexistent id — never a 403
// that would confirm the entity exists. Exercised directly on authorizeHistoryRead
// with a gate that grants no permission, so it doesn't depend on HistoryReader.
func TestAuthorizeHistoryRead_AbsentEntityNoPermissionIs404(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket/GONE-1", http.NoBody)
	// A gate that denies the permission and is not consulted for a live read
	// (the entity is absent). nopReadGate would GRANT the permission, so use a
	// fake that withholds it to exercise the deny branch.
	req = req.WithContext(withReadGate(context.Background(), fakeGate{holdsPermission: false}))
	rec := httptest.NewRecorder()

	ok := app.authorizeHistoryRead(rec, req, "ticket", "GONE-1")
	if ok {
		t.Fatal("authorizeHistoryRead should deny an absent entity when the caller lacks history:read")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("absent-entity history without permission: got %d, want 404 (no existence oracle)", rec.Code)
	}
}

// TestAuthorizeHistoryRead_AbsentEntityWithPermissionAllowed confirms the holder
// of history:read is allowed through for a deleted/absent entity (the auditor).
func TestAuthorizeHistoryRead_AbsentEntityWithPermissionAllowed(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket/GONE-1", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), fakeGate{holdsPermission: true}))
	rec := httptest.NewRecorder()

	if !app.authorizeHistoryRead(rec, req, "ticket", "GONE-1") {
		t.Fatalf("history:read holder should be allowed to read deleted-entity history; body=%s", rec.Body.String())
	}
}
