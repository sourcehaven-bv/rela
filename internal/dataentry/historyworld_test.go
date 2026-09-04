package dataentry

import (
	"context"
	"net/http"
	"testing"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestHistoryFace_ResolvesTheFaceOnScreen is the core of BUG-2: the face whose
// history is served must be the face the WORLD resolved, not the default one.
//
// Versioning is per-face, so getting this wrong shows a genuinely different
// record — the draft's edits under a page claiming to be published.
func TestHistoryFace_ResolvesTheFaceOnScreen(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedEntity(app, &entityPkg.Entity{
		ID: "TKT-H", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	if err := app.store.CreateEntity(ctx, &entityPkg.Entity{
		ID: "TKT-H", Type: "ticket", Face: entityPkg.Face("published"),
		Properties: map[string]any{"title": "published"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}

	pubScope := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {
			Chain:    []entityPkg.Face{entityPkg.Face("published")},
			Fallback: store.FallbackExclude,
		},
	})

	// The default world addresses the default face, spelled as the zero
	// face — byte-identical to the pre-BUG-2 unscoped read.
	p, ok, err := historyFace(context.Background(), app.store, "TKT-H")
	if err != nil || !ok || p != "" {
		t.Fatalf("default world: got (%q,%v,%v), want (\"\",true,nil)", p, ok, err)
	}

	p, ok, err = historyFace(worldCtx(pubScope), app.store, "TKT-H")
	if err != nil || !ok {
		t.Fatalf("published world: got (%q,%v,%v)", p, ok, err)
	}
	if p != entityPkg.Face("published") {
		t.Errorf("the history face must be the face the WORLD resolved, not the "+
			"default one — versioning is per-face, so this is the difference "+
			"between the right record and a plausible wrong one; got %q", p)
	}
}

// TestHistoryFace_AbsentWhenTheWorldResolvesNothing pins the third answer: a
// draft with no published face has no history IN the published world.
//
// Distinct from an error. The entity exists and the caller may read it, so the
// handler renders an empty timeline rather than a 404 — which would contradict
// the entity view, that answers the same question with `_world_absent`.
func TestHistoryFace_AbsentWhenTheWorldResolvesNothing(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entityPkg.Entity{
		ID: "TKT-DONLY", Type: "ticket", Properties: map[string]any{"title": "draft only"},
	})
	pubScope := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {
			Chain:    []entityPkg.Face{entityPkg.Face("published")},
			Fallback: store.FallbackExclude,
		},
	})

	p, ok, err := historyFace(worldCtx(pubScope), app.store, "TKT-DONLY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("a draft with no published face must resolve NO face in the "+
			"published world; got face %q", p)
	}
}

// stubHistory records which face-scoped calls it received, so a test can prove
// the face reached the store rather than being dropped on the way.
type stubHistory struct {
	gotList entityPkg.Face
	gotGet  entityPkg.Face
	// unscopedCalls counts reads through the plain HistoryReader shape — the
	// default-face path.
	unscopedCalls int
}

func (s *stubHistory) ListVersions(context.Context, string) ([]store.VersionMeta, error) {
	s.unscopedCalls++
	return nil, nil
}

func (s *stubHistory) GetVersion(context.Context, string, int) (*store.VersionSnapshot, error) {
	s.unscopedCalls++
	return &store.VersionSnapshot{}, nil
}

func (s *stubHistory) ListStateVersions(
	_ context.Context, _ string, p entityPkg.Face,
) ([]store.VersionMeta, error) {
	s.gotList = p
	return nil, nil
}

func (s *stubHistory) GetStateVersion(
	_ context.Context, _ string, p entityPkg.Face, _ int,
) (*store.VersionSnapshot, error) {
	s.gotGet = p
	return &store.VersionSnapshot{}, nil
}

// TestFaceHistoryReader_ScopesBothReadsToTheFace pins that BOTH history reads
// carry the face. A timeline scoped to the face while the snapshot silently
// read the default one would be the worst shape: the list would look right and
// clicking a row would show another face's content.
func TestFaceHistoryReader_ScopesBothReadsToTheFace(t *testing.T) {
	stub := &stubHistory{}
	scoped, ok := faceHistoryReader(stub, entityPkg.Face("published"))
	if !ok {
		t.Fatal("a StateHistoryReader-capable backend must be narrowable")
	}
	if _, err := scoped.ListVersions(context.Background(), "TKT-1"); err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if _, err := scoped.GetVersion(context.Background(), "TKT-1", 1); err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if stub.gotList != entityPkg.Face("published") {
		t.Errorf("the timeline read must carry the face; got %q", stub.gotList)
	}
	if stub.gotGet != entityPkg.Face("published") {
		t.Errorf("the snapshot read must carry the face too — a scoped timeline "+
			"over unscoped snapshots would show another face's content behind a "+
			"correct-looking list; got %q", stub.gotGet)
	}
	if stub.unscopedCalls != 0 {
		t.Errorf("a face-scoped reader must never fall through to the unscoped "+
			"calls; got %d", stub.unscopedCalls)
	}
}

// TestFaceHistoryReader_DefaultFaceStaysUnscoped pins the byte-identical
// default path: the zero face IS the default face, so it must not be
// wrapped — the adapter would otherwise require StateHistoryReader on every
// backend that has history at all.
func TestFaceHistoryReader_DefaultFaceStaysUnscoped(t *testing.T) {
	stub := &stubHistory{}
	scoped, ok := faceHistoryReader(stub, "")
	if !ok {
		t.Fatal("the default face must always be readable")
	}
	if _, err := scoped.ListVersions(context.Background(), "TKT-1"); err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if stub.unscopedCalls != 1 || stub.gotList != "" {
		t.Errorf("the default face must read through the plain HistoryReader; "+
			"unscoped=%d faceScoped=%q", stub.unscopedCalls, stub.gotList)
	}
}

// plainHistory implements HistoryReader ONLY — the fs/mem-shaped backend that
// has no per-face capability.
type plainHistory struct{}

func (plainHistory) ListVersions(context.Context, string) ([]store.VersionMeta, error) {
	return nil, nil
}

func (plainHistory) GetVersion(context.Context, string, int) (*store.VersionSnapshot, error) {
	return &store.VersionSnapshot{}, nil
}

// TestFaceHistoryReader_RefusesWhenTheBackendCannotScope is the fail-closed
// half. A backend without the face-scoped capability must REFUSE a face-scoped
// read, never fall back to the default face — that fallback is precisely the
// wrong-record bug, and it would be invisible because the response looks
// perfectly well-formed.
func TestFaceHistoryReader_RefusesWhenTheBackendCannotScope(t *testing.T) {
	if _, ok := faceHistoryReader(plainHistory{}, entityPkg.Face("published")); ok {
		t.Error("a backend with no StateHistoryReader must not silently serve " +
			"the DEFAULT face's history under a world")
	}
	// It must still serve the default face, which needs no narrowing.
	if _, ok := faceHistoryReader(plainHistory{}, ""); !ok {
		t.Error("the default face needs no face-scoped capability")
	}
}

// TestHistoryRouteAcceptsAWorld is the routing half of BUG-2, end to end
// through the real router.
//
// Version history is postgres-only, so on this fs/mem-backed test app the
// response is the named 501 — the assertion is that the request is not
// rejected as `world_unsupported` FIRST, which is what `worldCapablePath`
// refusing history used to do and what made the client-side fix impossible.
func TestHistoryRouteAcceptsAWorld(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entityPkg.Entity{
		ID: "TKT-R", Type: "ticket", Properties: map[string]any{"title": "t"},
	})
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})

	rec := viewRecord(t, app, "/api/v1/_history/ticket/TKT-R?world=published")
	if rec.Code == http.StatusUnprocessableEntity {
		t.Fatalf("history must ACCEPT a world — refusing it is what forced the "+
			"History button to show the default face's record; got %d %s",
			rec.Code, rec.Body)
	}
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("this backend has no version history, so the honest answer is "+
			"the named 501; got %d %s", rec.Code, rec.Body)
	}
}

// TestFaceHistoryReader_ReachesThroughTheVersionServiceInterface guards the
// wiring shape BUG-2's fix depends on.
//
// `App.versions` is a [store.VersionService] — an interface — and the face
// narrowing type-asserts the value inside it to [store.StateHistoryReader]. A
// Go type assertion on an interface value tests the DYNAMIC type, so this works
// only because the concrete value (pgstore's *VersionStore) implements both.
//
// Worth pinning because the failure mode is silent and remote: nothing here
// breaks, and instead every world-scoped history request on the one backend
// that HAS history refuses with 501. That would read as "the feature is not
// built" rather than "the wiring lost a capability".
func TestFaceHistoryReader_ReachesThroughTheVersionServiceInterface(t *testing.T) {
	// A value satisfying both capabilities, held behind the narrow interface
	// the handler actually has — the same shape appbuild hands the App.
	var held store.HistoryReader = &stubHistory{}
	if _, ok := faceHistoryReader(held, entityPkg.Face("published")); !ok {
		t.Error("the face narrowing must see through the HistoryReader " +
			"interface to the concrete type's StateHistoryReader methods")
	}
}

// TestHistoryFace_DefaultWorldNeverProbesTheStore pins the property that keeps
// DELETED-entity history working.
//
// A deleted entity has surviving versions but no live row, and
// authorizeHistoryRead admits it on the global acl.PermHistoryRead. If the
// default-world path probed the store for a face, it would report absence and
// serve an empty timeline for a record the caller is entitled to read — a
// regression invisible from the response, which would look like "no versions
// recorded yet".
func TestHistoryFace_DefaultWorldNeverProbesTheStore(t *testing.T) {
	app := newTestAppV1(t)

	// Nothing seeded: the deleted-entity shape, as the handler sees it.
	p, ok, err := historyFace(t.Context(), app.store, "GONE-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || p != "" {
		t.Errorf("under the default world an absent LIVE row must still resolve "+
			"the default face — a deleted entity's history is a supported read; "+
			"got (%q,%v)", p, ok)
	}
}
