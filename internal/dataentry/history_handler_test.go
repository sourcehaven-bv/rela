package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// historyStore wraps a store.Store with a canned version history, so the
// handler's HistoryReader path can be exercised without a pgstore. Keyed by
// entity id; each snapshot carries a type so cross-type checks can be tested.
type historyStore struct {
	store.Store
	versions map[string][]store.VersionSnapshot
}

func (h historyStore) ListVersions(_ context.Context, id string) ([]store.VersionMeta, error) {
	snaps := h.versions[id]
	metas := make([]store.VersionMeta, 0, len(snaps))
	for _, s := range snaps {
		metas = append(metas, s.VersionMeta)
	}
	return metas, nil
}

func (h historyStore) GetVersion(_ context.Context, id string, version int) (*store.VersionSnapshot, error) {
	snaps := h.versions[id]
	if version < 1 || version > len(snaps) {
		return nil, store.ErrNotFound
	}
	s := snaps[version-1]
	return &s, nil
}

func snapshot(version int, typ, content string, props map[string]any) store.VersionSnapshot {
	return store.VersionSnapshot{
		VersionMeta: store.VersionMeta{Version: version, Op: store.VersionOpCreate, Type: typ},
		Content:     content,
		Properties:  props,
		Projection:  []byte(`{}`),
	}
}

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
	handleV1History(app, rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("unsupported backend: got %d, want 501; body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleV1History_InvalidPath(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	// Missing the id segment.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket", http.NoBody)
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)
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

	ok := authorizeHistoryRead(app, rec, req, "ticket", "GONE-1")
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

	if !authorizeHistoryRead(app, rec, req, "ticket", "GONE-1") {
		t.Fatalf("history:read holder should be allowed to read deleted-entity history; body=%s", rec.Body.String())
	}
}

// TestHandleV1History_CrossTypeIs404 is the RR-2S0ZP8 regression: a LIVE ticket
// requested under the wrong URL type (/_history/note/<ticket-id>) must 404 —
// otherwise the wrong type's (permissive) read verdict could be borrowed to
// leak the ticket's history (a confused-deputy cross-type leak).
func TestHandleV1History_CrossTypeIs404(t *testing.T) {
	f := newFixture()
	tkt := entity.New("TKT-SECRET", "ticket")
	tkt.Properties = map[string]any{"title": "secret ticket"}
	f.AddNode(tkt)
	app := newAppFromParts(nil, testMeta(), f)
	app.store = historyStore{
		Store: app.store,
		versions: map[string][]store.VersionSnapshot{
			"TKT-SECRET": {snapshot(1, "ticket", "body", map[string]any{"title": "secret ticket"})},
		},
	}

	// Correct type: allowed (nop gate) → 200 with the timeline.
	recOK := doHistoryGet(app, "ticket", "TKT-SECRET")
	if recOK.Code != http.StatusOK {
		t.Fatalf("same-type history: got %d, want 200; body=%s", recOK.Code, recOK.Body.String())
	}

	// Wrong type: must be an indistinguishable 404, NOT the ticket's history.
	recBad := doHistoryGet(app, "note", "TKT-SECRET")
	if recBad.Code != http.StatusNotFound {
		t.Fatalf("cross-type history leak: got %d, want 404 (RR-2S0ZP8)", recBad.Code)
	}
	if bodyMentions(recBad, "secret") {
		t.Fatalf("cross-type response leaked ticket content: %s", recBad.Body.String())
	}

	// Wrong type on a specific version snapshot must also 404 (snap.Type check).
	recVer := doHistoryGet(app, "note", "TKT-SECRET/1")
	if recVer.Code != http.StatusNotFound {
		t.Fatalf("cross-type version snapshot: got %d, want 404", recVer.Code)
	}
}

func doHistoryGet(app *App, typeName, idPath string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/"+typeName+"/"+idPath, http.NoBody)
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)
	return rec
}

func bodyMentions(rec *httptest.ResponseRecorder, substr string) bool {
	return strings.Contains(rec.Body.String(), substr)
}
