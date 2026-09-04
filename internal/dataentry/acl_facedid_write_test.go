package dataentry

import (
	"context"
	"net/http"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestFacedIDWrite_AuthorizesTheFaceItWrites pins the fix for BUG-Y0GNSB.
//
// # The defect
//
// `ID@face` is the boundary serialization of a state reference, and
// fsstore/memstore key their index on exactly that string
// (stateKey = entity.FormatStateRef). Because FormatStateRef(id, "") returns
// the id verbatim, `GetEntity("POL-1@published")` resolves to the SAME row as
// `GetEntityState("POL-1", "published")` — it returns the published face, with
// `Face` correctly populated.
//
// The write path then dropped that face on the floor: every
// `acl.EntitySubject` literal in entitymanager was built `{Type, ID}` with no
// `Face`, and the zero Face means "the default state" (see acl.EntitySubject).
// So the ACL was asked "may this principal update the DEFAULT face?" while the
// store wrote the PUBLISHED one — and a role holding a plain `update: [policy]`
// grant (or `update: ["*"]`, which every admin policy holds) could edit a face
// no grant covered. That inverts the invariant GrantsVerbOnState exists to
// hold, and it is the ISMS guarantee this epic is built on: a published policy
// changes only by promoting a draft through an audited copy.
//
// # Why no existing test caught it
//
// internal/acl's face tests set `Face` as an explicit STRUCT FIELD, so they
// exercise GrantsVerbOnState correctly and pass — the bug was never in the
// grant matcher. It was in the subject the production code failed to build.
// internal/dataentry's facegrant tests are all read-side. Nothing drove a
// WRITE addressed to a faced id, which is why an escalation survived a suite
// this thorough.
//
// # Why this is not covered by attachWorld's `world_read_only`
//
// That refusal fires on an explicit `?world=` on a non-GET. This attack
// carries no world at all — the face rides in the PATH. The two are
// independent, which is precisely why the world guard did not blunt this.
func TestFacedIDWrite_AuthorizesTheFaceItWrites(t *testing.T) {
	t.Parallel()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Label: "Policy", IDPrefix: "POL-",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
	}

	// grant is the role's update list. The published face is never named by
	// any of these, so none of them may write it.
	for _, tc := range []struct {
		name  string
		grant []string
	}{
		{"bare type grant covers the default face only", []string{"policy"}},
		{"type wildcard ranges over types, not faces", []string{"*"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := storage.NewMemFS()
			paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
			if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
				t.Fatal(err)
			}
			bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
			ctx := context.Background()
			st := bootstrap.Store()

			if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
				Properties: map[string]any{"title": "draft text"}}); err != nil {
				t.Fatal(err)
			}
			published := entity.Face("published")
			if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
				Face: published, Properties: map[string]any{"title": "PUBLISHED-ORIGINAL"}}); err != nil {
				t.Fatal(err)
			}

			d := mustNewACL(t, &acl.Policy{
				Roles: map[string]acl.RoleDef{"editor": {
					Read:   []string{"policy"},
					Update: tc.grant,
					Delete: tc.grant,
				}},
				Assignments: map[string]string{"bob": "editor"},
			}, st)

			svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths),
				appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
			app := newAppFromParts(&Config{}, nil, newFixture())
			rebindApp(app, fs, paths, svc)
			app.acl = d
			app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})

			bobCtx := principal.With(ctx, principal.Principal{
				User: "bob", Tool: principal.ToolDataEntry})

			// Control: the grant DOES cover the default face, so this must
			// succeed. Without it, a blanket-deny bug would pass this test.
			if rec := patchEntityAs(bobCtx, t, app, d, "policy", "policys", "POL-1",
				`{"properties":{"title":"edited draft"}}`, nil); rec.Code != http.StatusOK {
				t.Fatalf("control PATCH of the default face = %d, want 200 (body %s)",
					rec.Code, rec.Body.String())
			}

			// The attack: same grant, faced id.
			rec := patchEntityAs(bobCtx, t, app, d, "policy", "policys", "POL-1@published",
				`{"properties":{"title":"TAMPERED"}}`, nil)
			if rec.Code == http.StatusOK {
				t.Errorf("PATCH POL-1@published = 200; a grant that names no face "+
					"must not reach the published face (body %s)", rec.Body.String())
			}

			// The status alone is not the contract — assert the bytes on disk.
			// A denial that still wrote would be the worst outcome, and is
			// exactly what the pre-fix code did while returning 200.
			after, err := st.GetEntityState(ctx, "POL-1", published)
			if err != nil {
				t.Fatalf("re-read published face: %v", err)
			}
			if got := after.Properties["title"]; got != "PUBLISHED-ORIGINAL" {
				t.Errorf("published face title = %v, want PUBLISHED-ORIGINAL "+
					"(the denied write reached the store)", got)
			}
		})
	}
}

// TestFacedIDWrite_ExplicitFaceGrantStillWorks is the other half of the
// contract: the fix must DENY the ungranted face without breaking the granted
// one. A fix that simply rejected every faced id would pass the test above and
// silently make faced writes impossible — so this pins that a role explicitly
// granted `policy@published` can still write it.
func TestFacedIDWrite_ExplicitFaceGrantStillWorks(t *testing.T) {
	t.Parallel()

	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Label: "Policy", IDPrefix: "POL-",
				Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}}},
		},
	}
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	if err := fs.MkdirAll(paths.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bootstrap := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths))
	ctx := context.Background()
	st := bootstrap.Store()

	if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
		Properties: map[string]any{"title": "draft text"}}); err != nil {
		t.Fatal(err)
	}
	published := entity.Face("published")
	if err := st.CreateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy",
		Face: published, Properties: map[string]any{"title": "PUBLISHED-ORIGINAL"}}); err != nil {
		t.Fatal(err)
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{"publisher": {
			Read:   []string{"policy"},
			Update: []string{"policy@published"},
		}},
		Assignments: map[string]string{"pat": "publisher"},
	}, st)

	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths),
		appbuildtest.WithStore(st), appbuildtest.WithDeclarative(d))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	app.acl = d
	app.schema.Publish(&Schema{Cfg: &Config{}, Meta: meta})

	patCtx := principal.With(ctx, principal.Principal{
		User: "pat", Tool: principal.ToolDataEntry})

	rec := patchEntityAs(patCtx, t, app, d, "policy", "policys", "POL-1@published",
		`{"properties":{"title":"published v2"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH POL-1@published with an explicit policy@published grant = %d, "+
			"want 200 (body %s)", rec.Code, rec.Body.String())
	}
	after, err := st.GetEntityState(ctx, "POL-1", published)
	if err != nil {
		t.Fatalf("re-read published face: %v", err)
	}
	if got := after.Properties["title"]; got != "published v2" {
		t.Errorf("published face title = %v, want published v2", got)
	}

	// The mirror image: this grant names the published face and therefore
	// does NOT cover the default one.
	if rec := patchEntityAs(patCtx, t, app, d, "policy", "policys", "POL-1",
		`{"properties":{"title":"nope"}}`, nil); rec.Code == http.StatusOK {
		t.Errorf("PATCH of the default face with a policy@published-only grant = 200; " +
			"a face-specific grant must not cover the default state")
	}
}
