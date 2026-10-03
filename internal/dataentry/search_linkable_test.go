package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// linkableMeta is facedMeta plus a person type and an `owns` relation that
// confers a role ON a policy, for the local-role case.
func linkableMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(`
entities:
  policy:
    label: Policy
    id_prefix: POL
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
  person:
    label: Person
    id_prefix: PER
    properties:
      title: { type: string }
relations:
  cites:
    from: [policy]
    to: [feature]
    scope: content
    inverse: { id: cited-by }
  implements:
    from: [policy]
    to: [feature]
  owns:
    from: [person]
    to: [policy]
`))
	if err != nil {
		t.Fatalf("parse metamodel: %v", err)
	}
	return m
}

// faceWorld serves policy at face only, the shape of one world the SPA
// picker searches.
func faceWorld(face entity.Face) worldHandle {
	return worldHandle{name: string(face), scope: store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{face}, Fallback: store.FallbackExclude},
	})}
}

// linkableRead is one collection read a picker makes with relation context:
// RelationCards searches, RelationPicker lists. url is the request prefix;
// a query string is appended to it.
type linkableRead struct {
	url   string
	serve func(app *App, rec *httptest.ResponseRecorder, req *http.Request)
}

var linkableReads = map[string]linkableRead{
	"search": {"/api/v1/_search?q=type:policy&type=policy", func(app *App, rec *httptest.ResponseRecorder, req *http.Request) {
		app.handleV1Search(rec, req)
	}},
	"list": {"/api/v1/policys?per_page=100", func(app *App, rec *httptest.ResponseRecorder, req *http.Request) {
		app.handleV1ListEntities(rec, req, "policy", "policys")
	}},
}

// readLinkable runs read for policies as alice with the relation context,
// once per face world as the SPA pickers do, and returns `linkable` per
// served address.
func readLinkable(
	t *testing.T, app *App, d *acl.Declarative, relType string,
	read linkableRead,
) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for _, face := range []entity.Face{"draft", "published"} {
		req := asAlice(t, d, httptest.NewRequest(http.MethodGet,
			read.url+"&relation="+relType+"&direction=incoming", http.NoBody))
		req = req.WithContext(withWorld(req.Context(), faceWorld(face)))
		rec := httptest.NewRecorder()
		read.serve(app, rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("read = %d %s", rec.Code, rec.Body)
		}
		var resp v1.LinkListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, row := range resp.Data {
			if row.Linkable == nil {
				t.Fatalf("row %s carries no linkable: %s", row.ID, rec.Body)
			}
			// The served address is the last segment of `_self`, as the SPA reads it.
			addr, err := url.PathUnescape(row.Self[strings.LastIndex(row.Self, "/")+1:])
			if err != nil {
				t.Fatalf("row %s: _self %q: %v", row.ID, row.Self, err)
			}
			got[addr] = *row.Linkable
		}
	}
	return got
}

// patchCitedBy adds POL-1@face as a source of FEAT-1's incoming `cites`
// through the unified PATCH, the write the SPA picker performs.
func patchCitedBy(t *testing.T, app *App, d *acl.Declarative, addr string) int {
	t.Helper()
	body := `{"relations":{"cited-by":{"add":[{"type":"policy","id":"` + addr + `"}]}}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/features/FEAT-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = asAlice(t, d, req)
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, req, "feature", "features", "FEAT-1")
	return rec.Code
}

// TestSearch_LinkableMatchesTheWrite pins `linkable` on /_search and list rows
// against the write it predicts, in both directions the old `_actions.update` hint got
// wrong: a row is linkable exactly when the PATCH that adds its edge succeeds,
// and a refused PATCH is a 403.
func TestSearch_LinkableMatchesTheWrite(t *testing.T) {
	faces := []string{"policy@draft", "policy@published"}
	withFeature := func(grants ...string) []string { return append([]string{"feature"}, grants...) }
	readAll := withFeature(append([]string{"person"}, faces...)...)
	denyCites := fakeResolver{rv: RelationVerdicts{Types: map[string]RelationVerdict{
		"cites": {Creatable: false, Removable: true},
	}}}

	for _, tc := range []struct {
		name     string
		policy   acl.Policy
		resolver FieldVerdictResolver
		want     map[string]bool
	}{
		{
			name: "editor links every face",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{"r": {
				Read: readAll, Create: withFeature(faces...), Update: withFeature(faces...),
			}}},
			want: map[string]bool{"POL-1@draft": true, "POL-1@published": true},
		},
		{
			name: "update without create links nothing",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{"r": {
				Read: readAll, Create: withFeature(), Update: withFeature(faces...),
			}}},
			want: map[string]bool{"POL-1@draft": false, "POL-1@published": false},
		},
		{
			name: "create without update links every face",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{"r": {
				Read: readAll, Create: withFeature(faces...), Update: withFeature(),
			}}},
			want: map[string]bool{"POL-1@draft": true, "POL-1@published": true},
		},
		{
			name: "create on one face links that face",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{"r": {
				Read: readAll, Create: withFeature("policy@draft"), Update: withFeature(faces...),
			}}},
			want: map[string]bool{"POL-1@draft": true, "POL-1@published": false},
		},
		{
			name: "a relation grant needs update on the tail",
			policy: acl.Policy{
				Roles: map[string]acl.RoleDef{"r": {
					Read: readAll, Create: withFeature(), Update: withFeature("policy@draft"),
					Permissions: []string{"cite"},
				}},
				RelationWriteGrants: map[string]acl.RelationWriteGrant{"cites": {Create: "cite"}},
			},
			want: map[string]bool{"POL-1@draft": true, "POL-1@published": false},
		},
		{
			name: "a local role does not authorize the edge",
			policy: acl.Policy{
				Roles: map[string]acl.RoleDef{
					"r":     {Read: readAll, Update: withFeature()},
					"owner": {Create: faces, Update: faces},
				},
				RoleRelations: map[string]acl.RoleRelationDef{"owns": {Confers: "owner"}},
			},
			want: map[string]bool{"POL-1@draft": false, "POL-1@published": false},
		},
		{
			name: "an affordance that refuses create",
			policy: acl.Policy{Roles: map[string]acl.RoleDef{"r": {
				Read: readAll, Create: withFeature(faces...), Update: withFeature(faces...),
			}}},
			resolver: denyCites,
			want:     map[string]bool{"POL-1@draft": false, "POL-1@published": false},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := facedAppWith(t, linkableMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
				ctx := context.Background()
				if err := st.CreateEntity(ctx, &entity.Entity{
					ID: "alice", Type: "person", Properties: map[string]any{"title": "Alice"},
				}); err != nil {
					t.Fatal(err)
				}
				k := entity.RelationKey{From: "alice", Type: "owns", To: "POL-1"}
				if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
					t.Fatal(err)
				}
				p := tc.policy
				p.Assignments = map[string]string{"alice": "r"}
				return mustNewACL(t, &p, st)
			})
			if tc.resolver != nil {
				app.fieldResolver = tc.resolver
			}

			got := readLinkable(t, app, d, "cites", linkableReads["search"])
			if !maps.Equal(got, tc.want) {
				t.Fatalf("search linkable = %v, want %v", got, tc.want)
			}
			if listed := readLinkable(t, app, d, "cites", linkableReads["list"]); !maps.Equal(listed, tc.want) {
				t.Fatalf("list linkable = %v, want %v", listed, tc.want)
			}
			for addr, linkable := range got {
				code := patchCitedBy(t, app, d, addr)
				switch {
				case linkable && code != http.StatusOK:
					t.Errorf("%s: linkable but PATCH = %d", addr, code)
				case !linkable && code != http.StatusForbidden:
					t.Errorf("%s: not linkable but PATCH = %d, want 403", addr, code)
				}
			}
		})
	}
}

// TestSearch_RelationContextIsValidated pins the parameter contract on both
// reads: no
// context serves no `linkable`, and a bad relation or direction is a 400.
func TestSearch_RelationContextIsValidated(t *testing.T) {
	app, d := facedApp(t, func(st store.Store) *acl.Declarative {
		return mustNewACL(t, &acl.Policy{
			Roles:       map[string]acl.RoleDef{"r": {Read: []string{"*", "policy@draft", "policy@published"}}},
			Assignments: map[string]string{"alice": "r"},
		}, st)
	})
	for _, tc := range []struct {
		name, query string
		wantCode    int
		wantError   string
	}{
		{"no context", "", http.StatusOK, ""},
		{"unknown relation", "&relation=nope&direction=incoming", http.StatusBadRequest, "invalid_relation"},
		{"inverse name", "&relation=cited-by&direction=incoming", http.StatusBadRequest, "invalid_relation"},
		{"outgoing", "&relation=cites&direction=outgoing", http.StatusBadRequest, "invalid_direction"},
		{"missing direction", "&relation=cites", http.StatusBadRequest, "invalid_direction"},
		{"direction alone", "&direction=incoming", http.StatusBadRequest, "invalid_relation"},
	} {
		for readName, read := range linkableReads {
			t.Run(readName+"/"+tc.name, func(t *testing.T) {
				req := asAlice(t, d, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
					read.url+tc.query, http.NoBody))
				req = req.WithContext(withWorld(req.Context(), faceWorld("published")))
				rec := httptest.NewRecorder()
				read.serve(app, rec, req)
				if rec.Code != tc.wantCode {
					t.Fatalf("= %d %s, want %d", rec.Code, rec.Body, tc.wantCode)
				}
				if tc.wantError != "" && !strings.Contains(rec.Body.String(), tc.wantError) {
					t.Fatalf("body %s, want %s", rec.Body, tc.wantError)
				}
				if tc.wantCode == http.StatusOK {
					if strings.Contains(rec.Body.String(), `"linkable"`) {
						t.Fatalf("a read without relation context serves linkable: %s", rec.Body)
					}
					if !strings.Contains(rec.Body.String(), `"POL-1"`) {
						t.Fatalf("read served no policy: %s", rec.Body)
					}
				}
			})
		}
	}
}
