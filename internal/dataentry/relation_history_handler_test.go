package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// relHistoryStore is a canned relation-version-history service so the
// RelationHistoryReader path is exercised without a pgstore. Keyed by the
// composite "from|type|to". Embeds stubVersionService to satisfy the whole
// store.VersionService umbrella; assign it to App.versions.
type relHistoryStore struct {
	stubVersionService
	versions map[string][]store.RelationVersionSnapshot
	// lifetimes, when set for a key, is returned by ListRelationLifetimes; else a
	// single synthetic lifetime is derived from `versions`.
	lifetimes map[string][]store.RelationLifetime
}

func relKey(from, relType, to string) string { return from + "|" + relType + "|" + to }

func (h relHistoryStore) ListRelationVersions(
	_ context.Context, q store.RelationHistoryQuery,
) ([]store.RelationVersionMeta, error) {
	snaps := h.versions[relKey(q.From, q.Type, q.To)]
	metas := make([]store.RelationVersionMeta, 0, len(snaps))
	for _, s := range snaps {
		metas = append(metas, s.RelationVersionMeta)
	}
	return metas, nil
}

func (h relHistoryStore) GetRelationVersion(
	_ context.Context, q store.RelationHistoryQuery, version int,
) (*store.RelationVersionSnapshot, error) {
	snaps := h.versions[relKey(q.From, q.Type, q.To)]
	if version < 1 || version > len(snaps) {
		return nil, store.ErrNotFound
	}
	s := snaps[version-1]
	return &s, nil
}

// ListRelationLifetimes reports a single lifetime for any key that has canned
// versions — enough for the handler tests (which exercise the timeline/version
// paths, not multi-lifetime enumeration; that is covered by the pgstore DB tests).
func (h relHistoryStore) ListRelationLifetimes(
	_ context.Context, from, relType, to string,
) ([]store.RelationLifetime, error) {
	if lts, ok := h.lifetimes[relKey(from, relType, to)]; ok {
		return lts, nil
	}
	snaps := h.versions[relKey(from, relType, to)]
	if len(snaps) == 0 {
		return nil, nil
	}
	return []store.RelationLifetime{{Lifetime: 1, RecordID: 1, VersionCount: len(snaps)}}, nil
}

// perEndpointGate permits reads only for ids in `allow`. HoldsPermission is
// controlled independently so deleted-relation tests can toggle history:read.
type perEndpointGate struct {
	allow           map[string]bool
	holdsPermission bool
}

func (g perEndpointGate) PermitsRead(_ context.Context, _, id string) (bool, error) {
	return g.allow[id], nil
}

func (g perEndpointGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = g.allow[id]
	}
	return m, nil
}

func (g perEndpointGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{}
}

func (g perEndpointGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return nil
}

func (g perEndpointGate) HoldsPermission(context.Context, string) bool { return g.holdsPermission }

func (perEndpointGate) PermitsWorld(context.Context, string) (bool, error) { return true, nil }

// newRelHistoryApp builds a test App whose store is a RelationHistoryReader with
// two live endpoints (DEC-1: decision, REQ-1: requirement) and one canned
// relation version.
func newRelHistoryApp(t *testing.T) *App {
	t.Helper()
	f := &fixture{}
	dec := entity.New("DEC-1", "decision")
	req := entity.New("REQ-1", "requirement")
	f.AddNode(dec)
	f.AddNode(req)

	base := newAppFromParts(nil, testMeta(), f)
	rel := relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("DEC-1", "addresses", "REQ-1"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpCreate, From: "DEC-1", Type: "addresses", To: "REQ-1",
				},
				Content: "why", Properties: map[string]any{"weight": "high"},
			}},
		},
	}
	base.versions = rel
	return base
}

func TestRelationHistory_UnsupportedBackend(t *testing.T) {
	app := newAppFromParts(nil, testMeta(), &fixture{})
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1", http.NoBody)
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("unsupported backend: got %d, want 501; body=%s", rec.Code, rec.Body.String())
	}
}

// TestRelationHistory_DualEndpointDeniesOnTo is the RR-SDDYZO regression: a
// caller who can read the FROM endpoint but NOT the TO endpoint must get an
// indistinguishable 404 — the TO side must not be an existence/content oracle.
func TestRelationHistory_DualEndpointDeniesOnTo(t *testing.T) {
	app := newRelHistoryApp(t)
	// Permit FROM (DEC-1) but deny TO (REQ-1).
	gate := perEndpointGate{allow: map[string]bool{"DEC-1": true, "REQ-1": false}}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), gate))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("TO denied must be 404 (no oracle); got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// TestRelationHistory_BothEndpointsPermitted verifies the happy path: both
// endpoints readable → the timeline is served.
func TestRelationHistory_BothEndpointsPermitted(t *testing.T) {
	app := newRelHistoryApp(t)
	gate := perEndpointGate{allow: map[string]bool{"DEC-1": true, "REQ-1": true}}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), gate))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("both permitted must be 200; got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"versions\"") {
		t.Errorf("timeline body missing versions: %s", rec.Body.String())
	}
}

// TestRelationHistory_LifetimesRoutePermitted verifies GET .../_lifetimes serves
// the lifetime list when both endpoints are readable (same gate as the timeline).
func TestRelationHistory_LifetimesRoutePermitted(t *testing.T) {
	app := newRelHistoryApp(t)
	// Give the key two canned lifetimes.
	rel := app.versions.(relHistoryStore)
	rel.lifetimes = map[string][]store.RelationLifetime{
		relKey("DEC-1", "addresses", "REQ-1"): {
			{Lifetime: 1, RecordID: 22, VersionCount: 2, Live: true, FinalOp: store.VersionOpUpdate},
			{Lifetime: 2, RecordID: 11, VersionCount: 3, FinalOp: store.VersionOpDelete},
		},
	}
	app.versions = rel

	gate := perEndpointGate{allow: map[string]bool{"DEC-1": true, "REQ-1": true}}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1/_lifetimes", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), gate))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lifetimes must be 200; got %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"lifetimes\"") || !strings.Contains(body, "\"record_id\":11") {
		t.Errorf("lifetimes body missing expected fields: %s", body)
	}
}

// TestRelationHistory_LifetimesRouteDeniedIsNotOracle: the _lifetimes route is
// gated exactly like the timeline — a denied TO endpoint yields a 404, never a
// leak of how many lifetimes exist.
func TestRelationHistory_LifetimesRouteDeniedIsNotOracle(t *testing.T) {
	app := newRelHistoryApp(t)
	gate := perEndpointGate{allow: map[string]bool{"DEC-1": true, "REQ-1": false}}
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1/_lifetimes", http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), gate))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("denied _lifetimes must be 404 (no oracle); got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// TestRelationHistory_BadRecordIDIs400: a non-empty but unparseable ?record_id=
// is a 400, never silently coerced to the newest lifetime (which would serve a
// different lifetime than asked for, with no signal).
func TestRelationHistory_BadRecordIDIs400(t *testing.T) {
	app := newRelHistoryApp(t)
	gate := perEndpointGate{allow: map[string]bool{"DEC-1": true, "REQ-1": true}}
	for _, bad := range []string{"abc", "-1", "0"} {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/_relation_history/decision/DEC-1/addresses/REQ-1?record_id="+bad, http.NoBody)
		req = req.WithContext(withReadGate(context.Background(), gate))
		rec := httptest.NewRecorder()
		handleV1RelationHistory(app, rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("record_id=%q must be 400; got %d, body=%s", bad, rec.Code, rec.Body.String())
		}
	}
}

// TestRelationHistory_DeletedRelationRequiresPermission asserts a relation whose
// endpoints are gone requires history:read; a non-holder gets a 404.
func TestRelationHistory_DeletedRelationRequiresPermission(t *testing.T) {
	// App with NO live endpoints, but canned history for a gone relation.
	base := newAppFromParts(nil, testMeta(), &fixture{})
	rel := relHistoryStore{
		versions: map[string][]store.RelationVersionSnapshot{
			relKey("GONE-A", "links", "GONE-B"): {{
				RelationVersionMeta: store.RelationVersionMeta{
					Version: 1, Op: store.VersionOpDelete, From: "GONE-A", Type: "links", To: "GONE-B",
				},
			}},
		},
	}
	base.versions = rel

	path := "/api/v1/_relation_history/decision/GONE-A/links/GONE-B"

	// Non-holder → 404.
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), perEndpointGate{holdsPermission: false}))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(base, rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted relation, non-holder: got %d, want 404; body=%s", rec.Code, rec.Body.String())
	}

	// Holder → 200.
	req = httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req = req.WithContext(withReadGate(context.Background(), perEndpointGate{holdsPermission: true}))
	rec = httptest.NewRecorder()
	handleV1RelationHistory(base, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("deleted relation, holder: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
