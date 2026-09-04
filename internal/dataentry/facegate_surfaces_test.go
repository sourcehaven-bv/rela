package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The face half of the read gate, on the surfaces that were still applying
// only the row half (TKT-O7R2A1's fix reached the entity GET, the view entry
// and the relations route; these did not). Each case pairs the denial with a
// positive control on the same fixture under an unrestricted principal, so an
// absence is the gate's doing and not an empty fixture.

// seedDraftAndPublishedTicket seeds TKT-1 with a draft (bare) face and a published face.
func seedDraftAndPublishedTicket(ctx context.Context, t *testing.T, app *App) {
	t.Helper()
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "SECRET DRAFT"},
	}); err != nil {
		t.Fatalf("seed draft face: %v", err)
	}
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Face: "published",
		Properties: map[string]any{"title": "published face"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}
}

// publishedOnly grants alice `ticket@published` only, and admin bob everything.
func publishedOnly(t *testing.T, app *App) (viewer, admin *acl.Declarative) {
	t.Helper()
	viewer = mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket@published"}},
		},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	admin = mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"*"}}},
		Assignments: map[string]string{"bob": "admin"},
	}, app.store)
	return viewer, admin
}

func TestFaceGrant_IncludedNeighboursAreFaceGated(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedEntity(app, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "ticket"},
	})
	// The neighbor has ONLY a draft: a `feature@published` grant reads none
	// of its faces, so it must be absent from every read-out of TKT-1.
	seedEntity(app, &entity.Entity{
		ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "SECRET FEATURE"},
	})
	if _, err := app.store.CreateRelation(ctx, "TKT-1", "implements", "FEAT-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
	viewer := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"viewer": {Read: []string{"ticket", "feature@published"}},
		},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	admin := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"admin": {Read: []string{"*"}}},
		Assignments: map[string]string{"bob": "admin"},
	}, app.store)

	app.acl = admin
	rec := getEntityAs(principalCtx("bob"), t, app, admin, "ticket", "tickets", "TKT-1", "include=implements")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SECRET FEATURE") {
		t.Fatalf("precondition: an unrestricted principal sees the included neighbor; got %d %s",
			rec.Code, rec.Body)
	}

	app.acl = viewer
	rec = getEntityAs(aliceCtx(), t, app, viewer, "ticket", "tickets", "TKT-1", "include=implements")
	if rec.Code != http.StatusOK {
		t.Fatalf("TKT-1 itself is readable; got %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "SECRET FEATURE") {
		t.Errorf("the draft-only neighbor's content leaked through ?include= to a "+
			"principal granted only feature@published; body=%s", rec.Body)
	}
}

func TestFaceGrant_AttachmentDownloadIsFaceGated(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedDraftAndPublishedTicket(ctx, t, app)
	if err := app.store.AttachFile(ctx, "TKT-1", "screenshot", "a.txt",
		strings.NewReader("draft bytes")); err != nil {
		t.Fatalf("attach: %v", err)
	}
	viewer, admin := publishedOnly(t, app)

	download := func(ctx context.Context, d *acl.Declarative) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/tickets/TKT-1/_attachments/screenshot/a.txt", http.NoBody)
		req = req.WithContext(gateCtxFor(ctx, t, d))
		rec := httptest.NewRecorder()
		app.attachments.handleV1GetAttachment(rec, req, "ticket", "TKT-1", "screenshot", "a.txt")
		return rec
	}

	app.acl = admin
	if rec := download(principalCtx("bob"), admin); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "draft bytes") {

		t.Fatalf("precondition: an unrestricted principal downloads the file; got %d %s", rec.Code, rec.Body)
	}

	app.acl = viewer
	if rec := download(aliceCtx(), viewer); rec.Code != http.StatusNotFound {
		t.Errorf("a file on the DRAFT face served to a ticket@published principal: got %d, want 404; body=%s",
			rec.Code, rec.Body)
	}
}

func TestFaceGrant_FacesAffordanceOmitsUnreadableFaces(t *testing.T) {
	// `_faces` enumerates DECLARED faces, so this needs a metamodel that
	// declares them — the shared fixture's ticket has none.
	meta, err := metamodel.Parse([]byte(`
version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    bare_face: draft
    faces:
      draft: {}
      published: {}
    properties:
      title: {type: string}
`))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	app := newAppFromParts(&Config{App: AppConfig{Name: "Faces", Description: "x"}}, meta, newFixture())
	ctx := context.Background()
	seedDraftAndPublishedTicket(ctx, t, app)
	viewer, admin := publishedOnly(t, app)

	faces := func(ctx context.Context, d *acl.Declarative) ([]map[string]any, int) {
		rec := getEntityAs(ctx, t, app, d, "ticket", "tickets", "TKT-1@published", "")
		var body struct {
			Faces []map[string]any `json:"_faces"`
		}
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
		}
		return body.Faces, rec.Code
	}

	app.acl = admin
	got, code := faces(principalCtx("bob"), admin)
	if code != http.StatusOK || len(got) != 1 {
		t.Fatalf("precondition: an unrestricted principal is offered the other (draft) face; got %d %v", code, got)
	}

	app.acl = viewer
	got, code = faces(aliceCtx(), viewer)
	if code != http.StatusOK {
		t.Fatalf("the published face is readable; got %d", code)
	}
	if len(got) != 0 {
		t.Errorf("`_faces` must not name the draft face to a principal who may not read it "+
			"(it discloses the face's existence); got %v", got)
	}
}

func TestFaceGrant_HistoryIsFaceGatedUnderTheDefaultWorld(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedDraftAndPublishedTicket(ctx, t, app)
	app.versions = historyStore{
		versions: map[string][]store.VersionSnapshot{
			"TKT-1": {snapshot("ticket", "SECRET DRAFT BODY", map[string]any{"title": "SECRET DRAFT"})},
		},
	}
	viewer, admin := publishedOnly(t, app)

	history := func(ctx context.Context, d *acl.Declarative, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_history/ticket/"+path, http.NoBody)
		req = req.WithContext(gateCtxFor(ctx, t, d))
		rec := httptest.NewRecorder()
		handleV1History(app, rec, req)
		return rec
	}

	app.acl = admin
	if rec := history(principalCtx("bob"), admin, "TKT-1/1"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "SECRET DRAFT BODY") {

		t.Fatalf("precondition: an unrestricted principal reads the snapshot; got %d %s", rec.Code, rec.Body)
	}

	app.acl = viewer
	for _, path := range []string{"TKT-1", "TKT-1/1"} {
		rec := history(aliceCtx(), viewer, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: the DRAFT face's history served to a ticket@published principal: got %d; body=%s",
				path, rec.Code, rec.Body)
		}
	}
}

// An explicit `?world=` is ANY occurrence of the parameter, including an
// empty value. attachWorld decided "explicit" on the first value while
// resolveWorld decided on the count, so `?world=&world=published` was not
// explicit to the write refusal and the unsupported-route refusal, yet was a
// duplicate to the resolver — which then went unreached.
func TestAttachWorld_EmptyPlusNamedIsExplicit(t *testing.T) {
	t.Parallel()
	app := &App{worlds: stubWorlds{names: map[string]bool{"published": true}}}

	if rec := serveWorldRequest(t, app, "/api/v1/tickets?world=&world=published"); rec.Code != http.StatusBadRequest {
		t.Errorf("GET with an empty and a named value is a duplicate; got %d %s", rec.Code, rec.Body)
	}
	if _, code := boundWorld(context.Background(), t, app, http.MethodPatch,
		"/api/v1/tickets/TKT-1?world=&world=published"); code == http.StatusOK {
		t.Errorf("a write carrying any ?world= must be refused; got %d", code)
	}
	if _, code := boundWorld(context.Background(), t, app, http.MethodGet,
		"/api/v1/tickets/TKT-1/relations?world=&world=published"); code == http.StatusOK {
		t.Errorf("a non-world-capable route must refuse an explicit world; got %d", code)
	}
}

// A declared world this principal may not read must answer the view EXACTLY
// as a permitted world in which the entity has no face: `_world_absent` over
// the default face. The denied handle carries the ZERO scope, so before the
// view honored the denial it resolved as the default world and served the
// full default-face view — which both disclosed the denial and showed content
// under a world the caller was refused.
func TestEntityView_DeniedWorldIsIndistinguishableFromAnEmptyOne(t *testing.T) {
	render := func(t *testing.T, worldGranted bool) map[string]any {
		t.Helper()
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{
			ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "draft"},
		})
		read := []string{"ticket"}
		if worldGranted {
			read = append(read, "world:published")
		}
		app.acl = mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"viewer": {Read: read}},
			Assignments: map[string]string{"alice": "viewer"},
		}, app.store)
		// The world EXCLUDES everything (no resolveDefault), so for the
		// granted principal TKT-1 genuinely has no face in it.
		app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})
		app.SetPrincipalResolver(func(*http.Request) principal.Principal {
			return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
		})
		rec := viewRecord(t, app, "/api/v1/_views/ticket/TKT-1?world=published")
		if rec.Code != http.StatusOK {
			t.Fatalf("granted=%v: want 200, got %d %s", worldGranted, rec.Code, rec.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	granted := render(t, true)
	denied := render(t, false)
	if absent, _ := granted["_world_absent"].(bool); !absent {
		t.Fatalf("precondition: an empty permitted world answers _world_absent; got %v", granted)
	}
	if absent, _ := denied["_world_absent"].(bool); !absent {
		t.Errorf("a DENIED world must answer exactly like an empty one (_world_absent), "+
			"not the default-face view; got %v", denied)
	}
	// Same SHAPE, key for key: a caller comparing the two responses must not
	// be able to tell which one was the denial.
	if g, d := slices.Sorted(maps.Keys(granted)), slices.Sorted(maps.Keys(denied)); !slices.Equal(g, d) {
		t.Errorf("denied and empty worlds must answer with the same keys; granted=%v denied=%v", g, d)
	}
}

// The contract is filter-first: the ACL trims the candidates to the faces
// the reader may see, then the world ranks what is left. The list path ran
// that query; the single-entity path resolved the world over EVERY face and
// 404'd when the prime was a face the grant withheld — so a listed row had no
// readable GET. Both now carry the allowlist into the world query.
func TestFaceGrant_GetFallsThroughToThePermittedFace(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedDraftAndPublishedTicket(ctx, t, app)
	if err := app.store.CreateEntity(ctx, &entity.Entity{
		ID: "TKT-1", Type: "ticket", Face: "review",
		Properties: map[string]any{"title": "SECRET REVIEW"},
	}); err != nil {
		t.Fatalf("seed review face: %v", err)
	}
	viewer, admin := publishedOnly(t, app)
	editorial := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {Chain: []entity.Face{"review", "published"}, Fallback: store.FallbackDefaultState},
	})
	get := func(ctx context.Context, d *acl.Declarative) (*entity.Entity, bool) {
		t.Helper()
		gctx := withWorld(gateCtxFor(ctx, t, d), worldHandle{name: "editorial", scope: editorial})
		e, found, err := app.visibleReader.getVisible(gctx, "ticket", "TKT-1")
		if err != nil {
			t.Fatalf("getVisible: %v", err)
		}
		return e, found
	}

	app.acl = admin
	if e, found := get(principalCtx("bob"), admin); !found || e.Face != "review" {
		t.Fatalf("precondition: an unrestricted reader gets the chain's first face; got %v %v", e, found)
	}
	app.acl = viewer
	e, found := get(aliceCtx(), viewer)
	if !found || e.Face != "published" {
		t.Errorf("a ticket@published reader must be served the published face — the "+
			"world ranks over the faces they may see; got found=%v face=%q", found, e)
	}
}

// The ETag folds the served FACE, not the world name: two worlds serving the
// same face share a validator (same bytes), a different face changes it, and
// a write — which addresses the stored face and refuses `?world=` — computes
// the same fold as the GET that showed that face. Folding the world name made
// every world-bound If-Match a permanent 412.
func TestEntityETag_FoldsTheServedFaceNotTheWorld(t *testing.T) {
	app := newTestAppV1(t)
	ctx := context.Background()
	seedDraftAndPublishedTicket(ctx, t, app)
	bare, _ := app.store.GetEntity(ctx, "TKT-1")
	published, _ := app.store.GetEntityState(ctx, "TKT-1", "published")
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"ticket": {Chain: []entity.Face{"published"}, Fallback: store.FallbackDefaultState},
	})
	wctx := withWorld(ctx, worldHandle{name: "published", scope: scope})

	if a, b := app.computeEntityETag(ctx, bare), app.computeEntityETag(wctx, bare); a != b {
		t.Errorf("the same face under two worlds must share a validator: %s vs %s", a, b)
	}
	if a, b := app.computeEntityETag(ctx, bare), app.computeEntityETag(ctx, published); a == b {
		t.Errorf("two faces must not share a validator")
	}
}
