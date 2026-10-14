package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestCreateFacedRow_JudgesFieldsOnTheCreatedFace (BUG-Y5UF3K): bob holds
// `drafter`, which may create a draft policy and declares no `fields:` block,
// and through his team `clerk`, whose block names only title. The union lets
// him write every field of a draft he creates. That only holds when the create
// candidate carries the face: on the unnamed face drafter holds no create, and
// the clerk's block would make secret read-only. The form and the POST must
// agree.
func TestCreateFacedRow_JudgesFieldsOnTheCreatedFace(t *testing.T) {
	t.Parallel()
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {
				Label: "Policy", IDPrefix: "POL-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: metamodel.PropertyTypeString},
					"secret": {Type: metamodel.PropertyTypeString},
				},
				Faces: map[string]metamodel.FaceDef{"draft": {}, "published": {}},
			},
			"person": {Label: "Person"},
			"team":   {Label: "Team"},
		},
		Relations: map[string]metamodel.RelationDef{
			"member-of": {From: []string{"person"}, To: []string{"team"}},
		},
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	for _, e := range []*entity.Entity{entity.New("bob", "person"), entity.New("clerks", "team")} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "bob", Type: "member-of", To: "clerks"}, nil); err != nil {
		t.Fatal(err)
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"drafter": {Read: []string{"policy"}, Create: []string{"policy@draft"}},
			"clerk": {
				Read: []string{"policy"}, Create: []string{"policy@draft"},
				Fields: map[string][]acl.FieldGrant{"policy": {{Field: "title"}}},
			},
		},
		Assignments: map[string]string{"bob": "drafter", "clerks": "clerk"},
	}, st)
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, d)
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})
	resolver, err := affordances.New(meta, storeRelationLookup{st: st}, d)
	if err != nil {
		t.Fatal(err)
	}
	setTestResolver(t, app, &policyResolver{inner: resolver})
	bobCtx := principal.With(ctx, principal.Principal{User: "bob", Tool: principal.ToolDataEntry})
	body := `{"face":"draft","properties":{"title":"T","secret":"S"}}`

	send := func(handler func(http.ResponseWriter, *http.Request, string, string), path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(gateCtxFor(bobCtx, t, d))
		rec := httptest.NewRecorder()
		handler(rec, req, "policy", "policys")
		return rec
	}

	dry := send(app.write.handleV1DryRunCreate, "/api/v1/policys/_dry-run")
	if dry.Code != http.StatusOK {
		t.Fatalf("dry-run = %d, want 200 (body %s)", dry.Code, dry.Body.String())
	}
	var form struct {
		Fields map[string]struct {
			Writable *bool `json:"writable"`
		} `json:"_fields"`
	}
	if err := json.Unmarshal(dry.Body.Bytes(), &form); err != nil {
		t.Fatal(err)
	}
	if f, ok := form.Fields["secret"]; ok && f.Writable != nil && !*f.Writable {
		t.Errorf("dry-run marks secret read-only on a draft bob may create")
	}

	if rec := send(app.write.handleV1CreateEntity, "/api/v1/policys"); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
}
