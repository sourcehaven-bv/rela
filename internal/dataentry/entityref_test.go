package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The face is part of the address, exactly as the id is (TKT-SLFURL).
//
// A faced response advertises `_self: /api/v1/policys/POL-1@published`, and
// the write path has accepted that spelling since BUG-Y0GNSB — but the read
// path treated the whole string as an id, so under a configured
// `default_world` (the documented ISMS setup) the server's own `_self` 404'd
// on a GET. A client following `_self` broke on the read, and no form could
// ever open a non-bare face. These tests pin the contract that closes it:
//
//   - `ID@face` reads the named row literally under EVERY world;
//   - `ID@<bare_face>` is the explicit spelling of the bare row;
//   - a PATCH or DELETE addressed to a face touches that face only;
//   - the row gate keys on the bare id and the face gate on the face, so a
//     `type@face` grant withholds exactly what it withheld before.

// facedMeta declares `policy` with a draft (bare) and a published face, an
// identity-scoped `implements` and a content-scoped `cites` relation to
// `feature`, so a PATCH can be tested against both scopes.
func facedMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	// Parsed from YAML rather than built as a literal so the loader's derived
	// indexes (the inverse-name map resolveDirection consults) exist, exactly
	// as in production.
	m, err := metamodel.Parse([]byte(`
entities:
  policy:
    label: Policy
    id_prefix: POL
    bare_face: draft
    faces:
      draft: {}
      published: { label: Published }
    properties:
      title: { type: string }
  feature:
    label: Feature
    id_prefix: FEAT
    properties:
      title: { type: string }
relations:
  implements:
    from: [policy]
    to: [feature]
  cites:
    from: [policy]
    to: [feature]
    scope: content
  # SYMMETRIC and content-scoped, with an inverse spelling: the inverse key
  # still makes the addressed entity the TAIL, so the faced-PATCH guard must
  # refuse it like the canonical name.
  relates-to:
    from: [policy]
    to: [policy]
    scope: content
    symmetric: true
    inverse: { id: related-from }
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return m
}

// policyPublishedScope is `select: published, otherwise: exclude` for policy — the
// public-world shape, under which a bare `POL-1` resolves AWAY from the draft.
func policyPublishedScope() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{entity.Face("published")}, Fallback: store.FallbackExclude},
	})
}

// facedApp builds an App over facedMeta with `default_world: published`
// configured, so a bare request lands in the world that hides drafts — the
// exact deployment shape under which `_self` used to 404. When d is non-nil
// the manager and the affordance service authorize against it.
func facedApp(t *testing.T, d func(st store.Store) *acl.Declarative) (*App, *acl.Declarative) {
	t.Helper()
	meta := facedMeta(t)
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	st := bootstrap.Store()
	seedPolicyFaces(t, st)

	cfg := &Config{}
	cfg.App.DefaultWorld = "published"
	opts := []appbuildtest.Option{appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st)}
	var decl *acl.Declarative
	if d != nil {
		decl = d(st)
		opts = append(opts, appbuildtest.WithDeclarative(decl))
	}
	svc := appbuildtest.New(meta, opts...)
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	if decl != nil {
		app.acl = decl
	}
	app.schema.Publish(&Schema{Cfg: cfg, Meta: meta})
	app.SetWorlds(fixedWorlds{scope: policyPublishedScope()})
	// Neighbor resolution wired as production does (rela-server main): the
	// face-scoped edge seam the write response now uses falls back to the
	// bare-id UNION without it, which is the mixed-face shape under test.
	if err := SetWorldNeighbors(app, st, appbuild.RelationScopes(svc)); err != nil {
		t.Fatal(err)
	}
	return app, decl
}

func seedPolicyFaces(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Properties: map[string]any{"title": "DRAFT TEXT"}},
		{ID: "POL-1", Type: "policy", Face: "published", Properties: map[string]any{"title": "PUBLISHED TEXT"}},
		{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "f"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%q: %v", e.ID, e.Face, err)
		}
	}
}

// getRouted performs a GET through the real router, so attachWorld applies
// the configured default world exactly as it does for a browser or curl.
func getRouted(t *testing.T, app *App, path string) (status int, got v1.Entity, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("GET %s: decode: %v (body %s)", path, err, rec.Body)
		}
	}
	return rec.Code, got, rec.Body.String()
}

func TestParseEntityRef(t *testing.T) {
	m := facedMeta(t)
	for _, tc := range []struct {
		raw  string
		want entityRef
		ok   bool
	}{
		{"POL-1", entityRef{ID: "POL-1"}, true},
		{"POL-1@published", entityRef{ID: "POL-1", Face: "published", Explicit: true}, true},
		// The bare face by its declared name is the bare row, spelled out.
		{"POL-1@draft", entityRef{ID: "POL-1", Face: "", Explicit: true}, true},
		// An undeclared name maps to itself; the store decides existence.
		{"POL-1@nope", entityRef{ID: "POL-1", Face: "nope", Explicit: true}, true},
		{"POL-1@@", entityRef{}, false},
		{"POL-1@Published", entityRef{}, false},
		{"POL-1@a@b", entityRef{}, false},
		{"not an id", entityRef{}, false},
	} {
		got, ok := parseEntityRef(m, "policy", tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseEntityRef(%q) = %+v, %v; want %+v, %v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

// TestFacedAddress_GetServesTheNamedFaceUnderAnyWorld is the acceptance test
// for TKT-SLFURL: the address the server hands out in `_self` reads back the
// row it names, under the configured world and under the default one alike.
func TestFacedAddress_GetServesTheNamedFaceUnderAnyWorld(t *testing.T) {
	app, _ := facedApp(t, nil)

	// The world does what it did: a bare id resolves to the published face.
	code, bare, body := getRouted(t, app, "/api/v1/policys/POL-1")
	if code != http.StatusOK || bare.Properties["title"] != "PUBLISHED TEXT" {
		t.Fatalf("precondition: bare id under default_world=published serves the "+
			"published face; got %d %s", code, body)
	}
	if bare.Self != "/api/v1/policys/POL-1@published" {
		t.Fatalf("precondition: _self names the published row; got %q", bare.Self)
	}

	for _, tc := range []struct {
		path, title, self, face string
	}{
		// The row `_self` names, under the world that used to 404 it.
		{"/api/v1/policys/POL-1@published", "PUBLISHED TEXT", "/api/v1/policys/POL-1@published", "published"},
		// The bare face by its declared name: literal, even though the
		// world resolves the bare id away from it.
		{"/api/v1/policys/POL-1@draft", "DRAFT TEXT", "/api/v1/policys/POL-1", "draft"},
		// Both spellings under the explicit default world too.
		{"/api/v1/policys/POL-1@published?world=default", "PUBLISHED TEXT", "/api/v1/policys/POL-1@published", "published"},
		{"/api/v1/policys/POL-1@draft?world=default", "DRAFT TEXT", "/api/v1/policys/POL-1", "draft"},
	} {
		code, got, body := getRouted(t, app, tc.path)
		if code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 (%s)", tc.path, code, body)
			continue
		}
		if got.Properties["title"] != tc.title {
			t.Errorf("GET %s served %v, want %q", tc.path, got.Properties["title"], tc.title)
		}
		if got.Self != tc.self {
			t.Errorf("GET %s: _self = %q, want %q (it must round-trip)", tc.path, got.Self, tc.self)
		}
		if got.World == nil || got.World.Via != ruleUnscoped || got.World.Face != tc.face {
			t.Errorf("GET %s: _world = %+v, want via=unscoped face=%s — an addressed "+
				"face was not resolved by the world, and labeling it by the chain "+
				"position it happens to hold would badge a page the reader navigated "+
				"to on purpose", tc.path, got.World, tc.face)
		}
	}

	// `_faces` carries an address per face, so a client links to a face
	// without deriving which world leads with it.
	_, got, _ := getRouted(t, app, "/api/v1/policys/POL-1@published")
	if got.Faces == nil || len(*got.Faces) != 1 || (*got.Faces)[0].Ref != "POL-1@draft" {
		t.Errorf("_faces on the published row should offer the draft at its explicit "+
			"address POL-1@draft; got %+v", got.Faces)
	}
	_, got, _ = getRouted(t, app, "/api/v1/policys/POL-1@draft")
	if got.Faces == nil || len(*got.Faces) != 1 || (*got.Faces)[0].Ref != "POL-1@published" {
		t.Errorf("_faces on the draft should offer POL-1@published; got %+v", got.Faces)
	}
}

func TestFacedAddress_RejectsWhatCannotNameARow(t *testing.T) {
	app, _ := facedApp(t, nil)
	for _, path := range []string{
		"/api/v1/policys/POL-1@nope",      // no such face on this entity
		"/api/v1/policys/POL-1@Published", // grammar: uppercase
		"/api/v1/policys/POL-1@@",         // grammar: empty face
		"/api/v1/policys/POL-9@published", // no such entity
	} {
		code, _, body := getRouted(t, app, path)
		if code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (%s)", path, code, body)
		}
		// One body for every miss, so a malformed address discloses nothing a
		// missing row does not.
		if !strings.Contains(body, entityNotFoundTitle) {
			t.Errorf("GET %s: body %q must be the uniform not-found", path, body)
		}
	}
}

// The `_views` surface is what the SPA's detail page reads, so it must accept
// the same addresses the entity route does.
func TestFacedAddress_ViewServesTheNamedFace(t *testing.T) {
	app, _ := facedApp(t, nil)
	for _, tc := range []struct{ path, title, self string }{
		{"/api/v1/_views/policy/POL-1@draft", "DRAFT TEXT", "/api/v1/policys/POL-1"},
		{"/api/v1/_views/policy/POL-1@published", "PUBLISHED TEXT", "/api/v1/policys/POL-1@published"},
		{"/api/v1/_views/policy/POL-1@published?world=default", "PUBLISHED TEXT", "/api/v1/policys/POL-1@published"},
	} {
		rec := viewRecord(t, app, tc.path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 (%s)", tc.path, rec.Code, rec.Body)
			continue
		}
		var resp v1.ViewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Entry.Properties["title"] != tc.title {
			t.Errorf("GET %s: entry title %v, want %q", tc.path, resp.Entry.Properties["title"], tc.title)
		}
		if resp.Entry.Self != tc.self {
			t.Errorf("GET %s: entry _self %q, want %q", tc.path, resp.Entry.Self, tc.self)
		}
		if resp.WorldAbsent {
			t.Errorf("GET %s: an explicitly addressed face is never 'absent from the world'", tc.path)
		}
	}
	if rec := viewRecord(t, app, "/api/v1/_views/policy/POL-1@nope"); rec.Code == http.StatusOK {
		t.Errorf("a face that does not exist must not render; got %d %s", rec.Code, rec.Body)
	}
}

// TestFacedAddress_PatchWritesTheNamedFace pins that the address, not the
// world, is what a write means — and that the one thing a face address cannot
// carry (a content-scoped edge) is refused rather than attached to the wrong
// tail.
func TestFacedAddress_PatchWritesTheNamedFace(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"*"}, Create: []string{"*"}, Update: []string{"policy", "policy@published"},
			}},
			Assignments: map[string]string{"bob": "editor"},
		}, st)
	})
	ctx := context.Background()
	bob := principal.With(ctx, principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	rec := patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published",
		`{"properties":{"title":"PUBLISHED v2"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH POL-1@published = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Self != "/api/v1/policys/POL-1@published" {
		t.Errorf("PATCH response _self = %q, want the face that was written", got.Self)
	}
	pub, err := app.store.GetEntityState(ctx, "POL-1", "published")
	if err != nil || pub.Properties["title"] != "PUBLISHED v2" {
		t.Errorf("published face after PATCH: %v %v, want PUBLISHED v2", pub, err)
	}
	draft, err := app.store.GetEntity(ctx, "POL-1")
	if err != nil || draft.Properties["title"] != "DRAFT TEXT" {
		t.Errorf("the draft must be untouched by a write to the published face; got %v %v", draft, err)
	}

	// The bare face by its declared name is the bare row.
	rec = patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@draft",
		`{"properties":{"title":"DRAFT v2"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH POL-1@draft = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	if draft, _ = app.store.GetEntity(ctx, "POL-1"); draft.Properties["title"] != "DRAFT v2" {
		t.Errorf("POL-1@draft must write the bare row; got %v", draft.Properties["title"])
	}

	// A content-scoped edge attaches to a face's tail, and the relation
	// writers address the bare tail: refused, not silently misfiled.
	rec = patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published",
		`{"relations":{"cites":{"data":[{"type":"feature","id":"FEAT-1"}]}}}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "face_relations_unsupported") {
		t.Errorf("a content-scoped relation through a face address = %d %s, want 422 face_relations_unsupported",
			rec.Code, rec.Body)
	}
	// A SYMMETRIC content-scoped relation spelled by its inverse name still
	// tails at this address (resolveDirection maps it back to outgoing), so
	// the guard must resolve keys the way the writer does.
	rec = patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published",
		`{"relations":{"related-from":{"data":[{"type":"policy","id":"POL-2"}]}}}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "face_relations_unsupported") {
		t.Errorf("a symmetric content-scoped relation via its inverse key = %d %s, want 422 "+
			"face_relations_unsupported — it would attach to the BARE tail", rec.Code, rec.Body)
	}
	// An identity-scoped edge is entity-level: the same edge from every face.
	rec = patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published",
		`{"relations":{"implements":{"data":[{"type":"feature","id":"FEAT-1"}]}}}`, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("an identity-scoped relation through a face address = %d %s, want 200", rec.Code, rec.Body)
	}
	// The response describes the row that was written: the published face's
	// own edges, not the union of every face's. The draft's `cites` edge
	// seeded below must not appear beside the published face.
	if _, err := app.store.CreateRelation(ctx, "POL-1", "cites", "FEAT-1", nil); err != nil {
		t.Fatalf("seed draft-tail edge: %v", err)
	}
	rec = patchEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published",
		`{"properties":{"title":"PUBLISHED v3"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	var after v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if _, leaked := after.Relations["cites"]; leaked {
		t.Errorf("the PATCH response for the published face carried the DRAFT face's "+
			"content-scoped edge: %v", after.Relations)
	}
}

// TestFacedAddress_WritesDenyAsNotFound: a face the principal may not read
// answers a write with the same 404 a missing face does — never a 403 that
// confirms the face exists.
func TestFacedAddress_WritesDenyAsNotFound(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"published-only": {
				Read: []string{"policy@published", "feature"}, Update: []string{"policy@published"},
				Delete: []string{"policy@published"},
			}},
			Assignments: map[string]string{"alice": "published-only"},
		}, st)
	})
	alice := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	// Control: the granted face is writable.
	if rec := patchEntityAs(alice, t, app, d, "policy", "policys", "POL-1@published",
		`{"properties":{"title":"x"}}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("control PATCH on the granted face = %d %s", rec.Code, rec.Body)
	}
	for _, tc := range []struct{ name, exists, absent string }{
		{"PATCH", "POL-1@draft", "POL-1@nope"},
		{"DELETE", "POL-1@draft", "POL-1@nope"},
	} {
		do := func(id string) *httptest.ResponseRecorder {
			if tc.name == "PATCH" {
				return patchEntityAs(alice, t, app, d, "policy", "policys", id, `{"properties":{"title":"x"}}`, nil)
			}
			return deleteEntityAs(alice, t, app, d, "policy", "policys", id)
		}
		existing, missing := do(tc.exists), do(tc.absent)
		if existing.Code != http.StatusNotFound {
			t.Errorf("%s on a denied-but-existing face = %d, want 404 (%s)", tc.name, existing.Code, existing.Body)
		}
		// The problem body echoes the request path in `instance`; every
		// other field must agree, or the difference is the oracle.
		if problemShape(t, existing.Body.Bytes()) != problemShape(t, missing.Body.Bytes()) {
			t.Errorf("%s: denied face body %q differs from missing face body %q — an existence oracle",
				tc.name, existing.Body, missing.Body)
		}
	}
	if _, err := app.store.GetEntity(context.Background(), "POL-1"); err != nil {
		t.Errorf("the denied delete must not have removed the draft: %v", err)
	}
}

// TestFacedAddress_DeleteRemovesOnlyTheFace: DELETE `ID@face` is the unpublish
// the address grammar makes expressible, authorized on the face it removes.
func TestFacedAddress_DeleteRemovesOnlyTheFace(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{
				"bare":      {Read: []string{"*"}, Delete: []string{"policy"}},
				"publisher": {Read: []string{"*"}, Delete: []string{"policy", "policy@published"}},
			},
			Assignments: map[string]string{"alice": "bare", "bob": "publisher"},
		}, st)
	})
	ctx := context.Background()
	alice := principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	bob := principal.With(ctx, principal.Principal{User: "bob", Tool: principal.ToolDataEntry})

	// The affordance and the write agree: alice's bare grant does not cover
	// the published face, and the map says so before she tries.
	rec := getEntityAs(alice, t, app, d, "policy", "policys", "POL-1@published", "")
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%d %s)", err, rec.Code, rec.Body)
	}
	if got.Actions["delete"] {
		t.Errorf("_actions.delete on the published face must be false for a bare grant")
	}
	if rec = deleteEntityAs(alice, t, app, d, "policy", "policys", "POL-1@published"); rec.Code != http.StatusForbidden {
		t.Errorf("DELETE POL-1@published with a bare grant = %d, want 403", rec.Code)
	}
	if _, err := app.store.GetEntityState(ctx, "POL-1", "published"); err != nil {
		t.Fatalf("the denied delete must not have removed the face: %v", err)
	}

	if rec = deleteEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published"); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE POL-1@published = %d, want 204 (%s)", rec.Code, rec.Body)
	}
	if _, err := app.store.GetEntityState(ctx, "POL-1", "published"); err == nil {
		t.Errorf("the published face must be gone")
	}
	if _, err := app.store.GetEntity(ctx, "POL-1"); err != nil {
		t.Errorf("deleting a face must leave the entity standing: %v", err)
	}
	if rec = deleteEntityAs(bob, t, app, d, "policy", "policys", "POL-1@published"); rec.Code != http.StatusNotFound {
		t.Errorf("a second DELETE of the removed face = %d, want 404", rec.Code)
	}

	// The bare face by name is the entity: deleting it is deleting the entity.
	if rec = deleteEntityAs(bob, t, app, d, "policy", "policys", "POL-1@draft"); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE POL-1@draft = %d, want 204 (%s)", rec.Code, rec.Body)
	}
	if _, err := app.store.GetEntity(ctx, "POL-1"); err == nil {
		t.Errorf("POL-1@draft names the bare row, so the entity must be gone")
	}
}

// problemShape renders a problem+json body without its `instance` (which
// echoes the request path) so two responses can be compared for sameness.
func problemShape(t *testing.T, body []byte) string {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode problem body %q: %v", body, err)
	}
	delete(p, "instance")
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestFacedAddress_DeniedWorldStillBlocks: a principal who may not select a
// world cannot reach a face in it by spelling the address — the entity route
// answers the uniform not-found, and the entity view answers exactly what it
// answers a bare id: the world-absent page over the BARE face, never the
// addressed one.
func TestFacedAddress_DeniedWorldStillBlocks(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		// Reads everything, but holds no `world:published` grant.
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"*"}}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	app.acl = d
	// The router is what binds the world handle, so these go through it; its
	// stamper replaces the ctx principal, so identity arrives the way
	// production supplies it.
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		app.NewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		return rec
	}
	if rec := get("/api/v1/policys/POL-1@published?world=default"); rec.Code != http.StatusOK {
		t.Fatalf("control: the address reads under a world alice may select; got %d %s", rec.Code, rec.Body)
	}
	if rec := get("/api/v1/policys/POL-1@published?world=published"); rec.Code != http.StatusNotFound {
		t.Errorf("an address under a DENIED world = %d, want 404 (%s)", rec.Code, rec.Body)
	}
	rec := get("/api/v1/_views/policy/POL-1@published?world=published")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "PUBLISHED TEXT") {
		t.Errorf("the view under a denied world must answer as for a bare id — the absent page "+
			"over the bare face — never the addressed face; got %d %s", rec.Code, rec.Body)
	}
}

// TestFacedAddress_FaceGateStillHolds: spelling the face does not widen a
// `type@face` read grant, and a denied face reads as a missing one.
func TestFacedAddress_FaceGateStillHolds(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"reader": {Read: []string{"policy@published", "feature"}}},
			Assignments: map[string]string{"alice": "reader"},
		}, st)
	})
	alice := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})

	if rec := getEntityAs(alice, t, app, d, "policy", "policys", "POL-1@published", ""); rec.Code != http.StatusOK {
		t.Fatalf("control: the granted face reads; got %d %s", rec.Code, rec.Body)
	}
	rec := getEntityAs(alice, t, app, d, "policy", "policys", "POL-1@draft", "")
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "DRAFT TEXT") {
		t.Errorf("the draft is withheld from a policy@published reader however it is "+
			"spelled; got %d %s", rec.Code, rec.Body)
	}
}
