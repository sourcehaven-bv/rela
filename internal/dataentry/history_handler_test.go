package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// historyStore is a canned version-history service, so the handler's
// HistoryReader path can be exercised without a pgstore. Keyed by entity id; each
// snapshot carries a type so cross-type checks can be tested. A named face
// keys as `ID@face`, the zero face by the bare id. It embeds
// stubVersionService to satisfy the whole store.VersionService umbrella and
// overrides only the two reader methods these tests exercise; assign it to
// App.versions.
type historyStore struct {
	stubVersionService
	versions map[string][]store.VersionSnapshot
}

func (h historyStore) ListVersions(_ context.Context, ref entity.Ref) ([]store.VersionMeta, error) {
	snaps := h.versions[entity.FormatStateRef(ref.ID, ref.Face)]
	metas := make([]store.VersionMeta, 0, len(snaps))
	for _, s := range snaps {
		metas = append(metas, s.VersionMeta)
	}
	return metas, nil
}

func (h historyStore) GetVersion(_ context.Context, ref entity.Ref, version int) (*store.VersionSnapshot, error) {
	snaps := h.versions[entity.FormatStateRef(ref.ID, ref.Face)]
	if version < 1 || version > len(snaps) {
		return nil, store.ErrNotFound
	}
	s := snaps[version-1]
	return &s, nil
}

// snapshot builds a version-1 create snapshot. Version is fixed at 1 (every
// caller uses a single-version timeline); typ stays an explicit parameter to
// document each case's entity type and keep the cross-type test readable.
func snapshot(typ, content string, props map[string]any) store.VersionSnapshot {
	return store.VersionSnapshot{
		VersionMeta: store.VersionMeta{Version: 1, Op: store.VersionOpCreate, Type: typ},
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

// TestResolveHistorySubject_AbsentEntityNoPermissionIs404 pins the no-oracle
// invariant: a caller without history:read asking for a non-existent (or
// deleted) entity's history gets the SAME 404 as a nonexistent id, never a
// 403 that would confirm the entity exists.
func TestResolveHistorySubject_AbsentEntityNoPermissionIs404(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	app.versions = historyStore{}
	// nopReadGate would GRANT the permission, so use a fake that withholds it
	// to exercise the deny branch.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket/GONE-1", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), fakeGate{holdsPermission: false}))
	rec := httptest.NewRecorder()
	handleV1History(app, rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("absent-entity history without permission: got %d, want 404 (no existence oracle)", rec.Code)
	}
}

// TestResolveHistorySubject_AbsentEntityWithPermissionAllowed confirms the
// holder of history:read is allowed through for a deleted or absent entity
// (the auditor).
func TestResolveHistorySubject_AbsentEntityWithPermissionAllowed(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	ctx := withReadGate(context.Background(), fakeGate{holdsPermission: true})

	subject, ok, err := resolveHistorySubject(ctx, app.visibleReader, "ticket", entity.Ref{ID: "GONE-1"})
	if err != nil || !ok || subject.live != nil || subject.worldAbsent {
		t.Fatalf("history:read holder = %+v ok %v err %v; want the deleted lineage", subject, ok, err)
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
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-SECRET": {snapshot("ticket", "body", map[string]any{"title": "secret ticket"})},
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

// TestConfigHistoryEnabled_AgreesWithTheHandler pins the two gates together.
//
// `/_config`.history_enabled exists so the SPA can render the History button
// ABSENT on a backend that cannot serve history, rather than present and
// certain to 501. That only works while the flag says what the handler does.
//
// The failure mode if they drift is quiet and one-directional in the worst
// direction: the flag reports true, the button renders, and every click 501s
// — an affordance that lies, which is exactly what the flag was added to
// remove. The reverse (flag false, handler serving) merely hides a working
// feature, which is also wrong but at least visible to whoever looks for it.
//
// So this asserts the PAIR on the same App, in both states, rather than
// asserting the flag's value against a constant.
func TestConfigHistoryEnabled_AgreesWithTheHandler(t *testing.T) {
	tests := []struct {
		name     string
		versions store.VersionService
		want     bool
	}{
		{
			name: "fs-style backend without the optional capability",
			// nil is how appbuild leaves it on fs/mem — the capability is
			// type-asserted, not part of store.Store.
			versions: nil,
			want:     false,
		},
		{
			name:     "backend with version history wired",
			versions: historyStore{versions: map[string][]store.VersionSnapshot{}},
			want:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := newAppFromParts(nil, testMeta(), &fixture{})
			app.versions = tc.versions

			rec := httptest.NewRecorder()
			app.handleV1Config(rec, httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody))
			if rec.Code != http.StatusOK {
				t.Fatalf("config: got %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			var cfg struct {
				App struct {
					HistoryEnabled bool `json:"history_enabled"`
				} `json:"app"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
				t.Fatalf("decode config: %v", err)
			}
			if cfg.App.HistoryEnabled != tc.want {
				t.Errorf("history_enabled = %v, want %v", cfg.App.HistoryEnabled, tc.want)
			}

			// And the handler's own answer, on the SAME App. A 501 means the
			// capability is absent; anything else means it was consulted.
			hrec := httptest.NewRecorder()
			handleV1History(app, hrec, httptest.NewRequest(
				http.MethodGet, "/api/v1/_history/ticket/TKT-1", http.NoBody))
			servedHistory := hrec.Code != http.StatusNotImplemented
			if servedHistory != cfg.App.HistoryEnabled {
				t.Errorf("`/_config`.history_enabled = %v but /_history answered %d "+
					"(capability present = %v). The flag drives whether the SPA "+
					"renders the History button at all, so a disagreement ships an "+
					"affordance that can only fail.",
					cfg.App.HistoryEnabled, hrec.Code, servedHistory)
			}
		})
	}
}
