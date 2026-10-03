package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// patchRelationsAs sends a unified PATCH carrying only relations, as alice,
// to the entity at plural/id.
func patchRelationsAs(
	t *testing.T, app *App, d *acl.Declarative, typeName, plural, id, relations string,
) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"relations":` + relations + `}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/api/v1/"+plural+"/"+id,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.write.handleV1UpdateEntity(rec, asAlice(t, d, req), typeName, plural, id)
	return rec
}

// hiddenSourceApp is linkableMeta with POL-2 stored only as a draft, and
// PER-1, as alice who reads the published face of policy only but may write
// every face. The ACL alone would let her link POL-2.
func hiddenSourceApp(t *testing.T) (*App, *acl.Declarative) {
	t.Helper()
	write := []string{"feature", "person", "policy@draft", "policy@published"}
	return facedAppWith(t, linkableMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
		ctx := context.Background()
		for _, e := range []*entity.Entity{
			draftOnly("POL-2"),
			{ID: "POL-3", Type: "policy", Face: "published", Properties: map[string]any{"title": "P3"}},
			{ID: "PER-1", Type: "person", Properties: map[string]any{"title": "p"}},
		} {
			if err := st.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"r": {
				Read: []string{"feature", "person", "policy@published"}, Create: write, Update: write, Delete: write,
			}},
			Assignments: map[string]string{"alice": "r"},
		}, st)
	})
}

// TestPatchRelations_HiddenSourceIsNonexistent pins that a relation write
// naming an entity, or a face, the caller cannot read answers exactly as one
// naming an absent id, and writes nothing. Each case sends the same body
// twice, once with the hidden id and once with an absent one.
func TestPatchRelations_HiddenSourceIsNonexistent(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		typeName, plural, id  string
		relations             string // %s is the peer
		hidden, absent        string
		edge                  entity.RelationKey // the edge the hidden write must not create or touch
		seedEdge, wantSuccess bool
	}{
		{
			name: "incoming identity add", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"implemented-by":{"add":[{"type":"policy","id":%q}]}}`, hidden: "POL-2", absent: "POL-9",
			edge: entity.RelationKey{From: "POL-2", Type: "implements", To: "FEAT-1"},
		},
		{
			name: "incoming identity replace", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"implemented-by":{"data":[{"type":"policy","id":%q}]}}`, hidden: "POL-2", absent: "POL-9",
			edge: entity.RelationKey{From: "POL-2", Type: "implements", To: "FEAT-1"},
		},
		{
			name: "incoming content add by bare id", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"cited-by":{"add":[{"type":"policy","id":%q}]}}`, hidden: "POL-2", absent: "POL-9",
			edge: entity.RelationKey{From: "POL-2", FromFace: "draft", Type: "cites", To: "FEAT-1"},
		},
		{
			name: "incoming content add at a hidden face", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"cited-by":{"add":[{"type":"policy","id":%q}]}}`, hidden: "POL-1@draft", absent: "POL-3@draft",
			edge: entity.RelationKey{From: "POL-1", FromFace: "draft", Type: "cites", To: "FEAT-1"},
		},
		{
			name: "outgoing add to a hidden target", typeName: "person", plural: "persons", id: "PER-1",
			relations: `{"owns":{"add":[{"type":"policy","id":%q}]}}`, hidden: "POL-2", absent: "POL-9",
			edge: entity.RelationKey{From: "PER-1", Type: "owns", To: "POL-2"},
		},
		{
			name: "meta update of an edge from a hidden source", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"implemented-by":{"add":[{"type":"policy","id":%q,"meta":{"note":"x"}}]}}`,
			hidden:    "POL-2", absent: "POL-9", seedEdge: true,
			edge: entity.RelationKey{From: "POL-2", Type: "implements", To: "FEAT-1"},
		},
		{
			name: "remove of an edge from a hidden source", typeName: "feature", plural: "features", id: "FEAT-1",
			relations: `{"implemented-by":{"remove":[{"type":"policy","id":%q}]}}`,
			hidden:    "POL-2", absent: "POL-9", seedEdge: true, wantSuccess: true,
			edge: entity.RelationKey{From: "POL-2", Type: "implements", To: "FEAT-1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := hiddenSourceApp(t)
			ctx := t.Context()
			if tc.seedEdge {
				if _, err := app.store.CreateRelation(ctx, tc.edge, &store.RelationData{}); err != nil {
					t.Fatal(err)
				}
			}
			hidden := patchRelationsAs(t, app, d, tc.typeName, tc.plural, tc.id, fmt.Sprintf(tc.relations, tc.hidden))
			absent := patchRelationsAs(t, app, d, tc.typeName, tc.plural, tc.id, fmt.Sprintf(tc.relations, tc.absent))
			if (hidden.Code == http.StatusOK) != tc.wantSuccess {
				t.Errorf("hidden = %d %s, want success=%v", hidden.Code, hidden.Body, tc.wantSuccess)
			}
			if hidden.Code != absent.Code {
				t.Errorf("hidden = %d %s\nabsent = %d %s", hidden.Code, hidden.Body, absent.Code, absent.Body)
			} else if hidden.Code != http.StatusOK {
				want := strings.ReplaceAll(absent.Body.String(), tc.absent, tc.hidden)
				if problemShape(t, hidden.Body.Bytes()) != problemShape(t, []byte(want)) {
					t.Errorf("hidden body %s differs from absent body %s", hidden.Body, absent.Body)
				}
			}
			if strings.Contains(hidden.Body.String(), "DRAFT") {
				t.Errorf("body discloses the hidden row: %s", hidden.Body)
			}
			rel, err := app.store.GetRelation(ctx, tc.edge)
			switch {
			case !tc.seedEdge && err == nil:
				t.Errorf("the write created an edge from a hidden source")
			case tc.seedEdge && err != nil:
				t.Errorf("the write removed the hidden edge: %v", err)
			case tc.seedEdge && rel.Properties["note"] != nil:
				t.Errorf("the write updated the hidden edge: %v", rel.Properties)
			}
		})
	}
}

// policyRelationApp is hiddenSourceApp's fixture with a real affordance
// resolver over grants. read is alice's read list; grant is her `implements`
// relation grant on policy. draft is the title of POL-1's draft, or "" to
// store no draft.
func policyRelationApp(t *testing.T, read []string, grant acl.RelationGrant, draft string) (*App, *acl.Declarative) {
	t.Helper()
	return policyGrantsApp(t, read, []acl.RelationGrant{grant}, draft)
}

// policyGrantsApp is policyRelationApp with every relation grant alice holds
// on policy.
func policyGrantsApp(t *testing.T, read []string, grants []acl.RelationGrant, draft string) (*App, *acl.Declarative) {
	t.Helper()
	write := []string{"feature", "policy@draft", "policy@published"}
	app, d := facedAppWith(t, linkableMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
		ctx := context.Background()
		if draft == "" {
			if _, err := st.DeleteFace(ctx, entity.Ref{ID: "POL-1", Face: "draft"}); err != nil {
				t.Fatal(err)
			}
		} else if err := st.UpdateEntity(ctx, &entity.Entity{ID: "POL-1", Type: "policy", Face: "draft",
			Properties: map[string]any{"title": draft}}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "POL-1", Type: "implements", To: "FEAT-2"},
			&store.RelationData{}); err != nil {
			t.Fatal(err)
		}
		return mustNewACL(t, &acl.Policy{
			Roles: map[string]acl.RoleDef{"r": {
				Read: read, Create: write, Update: write, Delete: write,
				Relations: map[string][]acl.RelationGrant{"policy": grants},
			}},
			Assignments: map[string]string{"alice": "r"},
		}, st)
	})
	if err := app.store.CreateEntity(t.Context(), &entity.Entity{ID: "FEAT-2", Type: "feature",
		Properties: map[string]any{"title": "f2"}}); err != nil {
		t.Fatal(err)
	}
	resolver, err := affordances.New(app.Meta(), storeRelationLookup{st: app.store}, d)
	if err != nil {
		t.Fatal(err)
	}
	app.fieldResolver = &policyResolver{inner: resolver}
	return app, d
}

// searchLinkable is POL-1's `linkable` for an incoming implements edge, in
// the published world, as alice.
func searchLinkable(t *testing.T, app *App, d *acl.Declarative) bool {
	t.Helper()
	req := asAlice(t, d, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		linkableReads["search"].url+"&relation=implements&direction=incoming", http.NoBody))
	req = req.WithContext(withWorld(req.Context(), faceWorld("published")))
	rec := httptest.NewRecorder()
	app.handleV1Search(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d %s", rec.Code, rec.Body)
	}
	var resp v1.LinkListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, row := range resp.Data {
		if strings.HasPrefix(row.Self, "/api/v1/policys/POL-1") && row.Linkable != nil {
			return *row.Linkable
		}
	}
	t.Fatalf("search rows = %s, want POL-1 with linkable", rec.Body)
	return false
}

// TestPatchRelations_HiddenFaceNeitherLeaksNorBypassesTheAffordance pins
// the affordance gate of an identity-scoped edge from a faced source, which
// belongs to every face. A face the caller can read is judged on its row. A
// face the caller cannot read is judged on the policy alone: a conditional
// grant could deny it, so it denies, whatever the face holds and whether or
// not it is stored. The verdict therefore never depends on a hidden row, and
// a `when:` that would deny on it is never skipped.
func TestPatchRelations_HiddenFaceNeitherLeaksNorBypassesTheAffordance(t *testing.T) {
	readPub := []string{"feature", "policy@published"}
	readAll := []string{"feature", "policy"}
	conditional := acl.RelationGrant{Relation: "implements", When: "entity.title ~= 'LOCKED'"}
	unconditional := acl.RelationGrant{Relation: "implements"}
	for _, tc := range []struct {
		name  string
		read  []string
		grant acl.RelationGrant
		draft string
		want  bool
	}{
		{"published reader, conditional grant, draft would deny", readPub, conditional, "LOCKED", false},
		{"published reader, conditional grant, draft would allow", readPub, conditional, "ok", false},
		{"published reader, conditional grant, no draft", readPub, conditional, "", false},
		{"published reader, unconditional grant, draft stored", readPub, unconditional, "LOCKED", true},
		{"published reader, unconditional grant, no draft", readPub, unconditional, "", true},
		{"full reader, conditional grant, draft denies", readAll, conditional, "LOCKED", false},
		{"full reader, conditional grant, draft allows", readAll, conditional, "ok", true},
		{"full reader, conditional grant, no draft", readAll, conditional, "", true},
	} {
		for _, op := range []struct{ name, relations, edgeTo string }{
			{"add", `{"implemented-by":{"add":[{"type":"policy","id":"POL-1"}]}}`, "FEAT-1"},
			{"remove", `{"implemented-by":{"remove":[{"type":"policy","id":"POL-1"}]}}`, "FEAT-2"},
		} {
			t.Run(tc.name+"/"+op.name, func(t *testing.T) {
				app, d := policyRelationApp(t, tc.read, tc.grant, tc.draft)
				if op.name == "add" {
					if got := searchLinkable(t, app, d); got != tc.want {
						t.Errorf("linkable = %v, want %v", got, tc.want)
					}
				}
				id := "FEAT-1"
				if op.name == "remove" {
					id = "FEAT-2"
				}
				rec := patchRelationsAs(t, app, d, "feature", "features", id, op.relations)
				wantCode := http.StatusForbidden
				if tc.want {
					wantCode = http.StatusOK
				}
				if rec.Code != wantCode {
					t.Errorf("PATCH = %d %s, want %d", rec.Code, rec.Body, wantCode)
				}
				if !slices.Contains(tc.read, "policy") && strings.Contains(strings.ToLower(rec.Body.String()), "draft") {
					t.Errorf("PATCH body names the hidden face: %s", rec.Body)
				}
				_, err := app.store.GetRelation(t.Context(), entity.RelationKey{From: "POL-1", Type: "implements", To: op.edgeTo})
				if exists := err == nil; exists != (tc.want == (op.name == "add")) {
					t.Errorf("edge to %s exists = %v after %s (allowed=%v)", op.edgeTo, exists, op.name, tc.want)
				}
			})
		}
	}
}

// TestLinkable_HiddenSourceIsNotLinkable pins `linkable` for a source with no
// face the caller can read: false, as for an absent one. Such a row is never
// served, so this asks linkableFrom directly.
func TestLinkable_HiddenSourceIsNotLinkable(t *testing.T) {
	app, d := hiddenSourceApp(t)
	ctx := gateCtxFor(aliceCtx(), t, d)
	row, err := app.store.GetEntity(ctx, entity.Ref{ID: "POL-2", Face: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if app.affordances.linkableFrom(ctx, row, "implements", app.Meta().Relations["implements"].Scope) {
		t.Errorf("a source whose every face is hidden is linkable")
	}
}
