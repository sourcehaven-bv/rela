package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// An entity ETag is a digest of what the caller may see. Hashing a
// `visible:`-hidden value lets a caller guess it offline against the
// digest (BUG-J01V0Q).

const etagRedactionMeta = `
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: { type: string }
      salary: { type: string }
`

// etagRedactionApp seeds TKT-1 with the given salary. alice may read and
// update tickets, and sees only their title.
func etagRedactionApp(t *testing.T, salary string) (*App, *acl.Declarative) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(etagRedactionMeta))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err = fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	if err = st.CreateEntity(context.Background(), &entity.Entity{
		ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "t", "salary": salary},
	}); err != nil {
		t.Fatal(err)
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"r": {
			Read:    []string{"ticket"},
			Update:  []string{"ticket"},
			Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
		}},
		Assignments: map[string]string{"alice": "r"},
	}, st)
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(d))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	res, err := ResolverFromProfile("", meta, st, d)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = res
	return app, d
}

// patchThenGetETags PATCHes the visible title as alice and returns the PATCH
// response's ETag and a following GET's.
func patchThenGetETags(t *testing.T, salary string) (patched, got string) {
	t.Helper()
	app, d := etagRedactionApp(t, salary)
	alice := principal.With(t.Context(), principal.Principal{User: "alice", Tool: principal.ToolDataEntry})
	rec := patchEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-1", `{"properties":{"title":"changed"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	patched = rec.Header().Get("ETag")
	rec = getEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	var body struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	if _, leaked := body.Properties["salary"]; leaked {
		t.Fatalf("fixture: GET serves the hidden salary: %s", rec.Body)
	}
	got = rec.Header().Get("ETag")

	// The PATCH ETag is a valid If-Match for the next write.
	hdr := http.Header{"If-Match": []string{patched}}
	rec = patchEntityAs(alice, t, app, d, "ticket", "tickets", "TKT-1", `{"properties":{"title":"again"}}`, hdr)
	if rec.Code != http.StatusOK {
		t.Errorf("PATCH with the previous PATCH's ETag = %d %s", rec.Code, rec.Body)
	}
	return patched, got
}

func TestEntityETag_ExcludesHiddenFields(t *testing.T) {
	patchedA, getA := patchThenGetETags(t, "100")
	patchedB, getB := patchThenGetETags(t, "200")
	if patchedA == "" || getA == "" {
		t.Fatalf("missing ETag: PATCH %q, GET %q", patchedA, getA)
	}
	if patchedA != getA {
		t.Errorf("PATCH ETag %s != GET ETag %s", patchedA, getA)
	}
	if patchedA != patchedB {
		t.Errorf("PATCH ETag depends on the hidden salary: %s vs %s", patchedA, patchedB)
	}
	if getA != getB {
		t.Errorf("GET ETag depends on the hidden salary: %s vs %s", getA, getB)
	}
}

// relationETag GETs TKT-001's depends_on edge to TKT-002, whose meta holds
// reason (visible to alice) and secret (hidden from her), and returns the
// response's ETag.
func relationETag(t *testing.T, reason, secret string) string {
	t.Helper()
	app := buildPolicyApp(t, relationRedactionACL, nil)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "a"}})
	seedEntity(app, &entity.Entity{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "b"}})
	if _, err := app.store.CreateRelation(context.Background(),
		entity.RelationKey{From: "TKT-001", Type: "depends_on", To: "TKT-002"},
		&store.RelationData{Properties: map[string]any{"reason": reason, "secret": secret}}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001/relations/depends_on/TKT-002", http.NoBody)
	req = req.WithContext(acltest.Bind(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}), t, app.acl))
	rec := httptest.NewRecorder()
	handleV1GetRelationTarget(app, rec, req, "ticket", "TKT-001", "depends_on", "TKT-002")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET relation = %d %s", rec.Code, rec.Body)
	}
	return rec.Header().Get("ETag")
}

func TestRelationETag_ExcludesHiddenMeta(t *testing.T) {
	a := relationETag(t, "blocked", "s3cr3t")
	if a == "" {
		t.Fatal("missing ETag")
	}
	if b := relationETag(t, "blocked", "other"); a != b {
		t.Errorf("relation ETag depends on the hidden meta: %s vs %s", a, b)
	}
	if c := relationETag(t, "unblocked", "s3cr3t"); a == c {
		t.Errorf("relation ETag ignores the visible meta: both %s", a)
	}
}
