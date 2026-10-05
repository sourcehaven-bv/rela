package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// relationAffordanceOf is `_relations[relType]` on alice's GET of addr, a
// policy address.
func relationAffordanceOf(t *testing.T, app *App, d *acl.Declarative, addr, relType string) v1.RelationAffordance {
	t.Helper()
	req := asAlice(t, d, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/v1/policys/"+addr, http.NoBody))
	rec := httptest.NewRecorder()
	app.handleV1GetEntity(rec, req, "policy", "policys", addr)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", addr, rec.Code, rec.Body)
	}
	var got v1.Entity
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.RelationAffordances == nil {
		t.Fatalf("GET %s carries no _relations: %s", addr, rec.Body)
	}
	return (*got.RelationAffordances)[relType]
}

// allowed reads a sparse affordance flag: absent is allowed.
func allowed(flag *bool) bool { return flag == nil || *flag }

// outgoingOp is one relation write on POL-1@published, with the affordance
// flag the GET must report for it.
type outgoingOp struct {
	name      string
	relations string
	// grant builds alice's relation grant from the `when:` it carries.
	grant func(relType, when string) acl.RelationGrant
	// flag reads the affordance the GET reports for the write.
	flag func(v1.RelationAffordance) bool
	// done reports whether the write took effect.
	done func(t *testing.T, app *App) bool
}

func outgoingOps(relType string, content bool) []outgoingOp {
	tail := entity.ImplicitFace
	if content {
		tail = "published"
	}
	plain := func(rel, when string) acl.RelationGrant { return acl.RelationGrant{Relation: rel, When: when} }
	edge := func(to string) entity.RelationKey {
		return entity.RelationKey{From: "POL-1", FromFace: tail, Type: relType, To: to}
	}
	exists := func(t *testing.T, app *App, to string) bool {
		t.Helper()
		_, err := app.store.GetRelation(t.Context(), edge(to))
		return err == nil
	}
	return []outgoingOp{
		{
			name:      "add",
			relations: `{"` + relType + `":{"add":[{"type":"feature","id":"FEAT-1"}]}}`,
			grant:     plain,
			flag:      func(a v1.RelationAffordance) bool { return allowed(a.Creatable) },
			done:      func(t *testing.T, app *App) bool { t.Helper(); return exists(t, app, "FEAT-1") },
		},
		{
			name:      "remove",
			relations: `{"` + relType + `":{"remove":[{"type":"feature","id":"FEAT-2"}]}}`,
			grant:     plain,
			flag:      func(a v1.RelationAffordance) bool { return allowed(a.Removable) },
			done:      func(t *testing.T, app *App) bool { t.Helper(); return !exists(t, app, "FEAT-2") },
		},
		{
			name:      "meta update",
			relations: `{"` + relType + `":{"add":[{"type":"feature","id":"FEAT-2","meta":{"note":"x"}}]}}`,
			grant: func(rel, when string) acl.RelationGrant {
				return acl.RelationGrant{Relation: rel, Fields: []acl.FieldGrant{{Field: "note", When: when}}}
			},
			flag: func(a v1.RelationAffordance) bool { return allowed(a.Fields["note"].Writable) },
			done: func(t *testing.T, app *App) bool {
				t.Helper()
				rel, err := app.store.GetRelation(t.Context(), edge("FEAT-2"))
				return err == nil && rel.Properties["note"] == "x"
			},
		},
	}
}

// outgoingApp is policyGrantsApp with an extra edge of relType from
// POL-1@published to FEAT-2 when relType is content-scoped (policyGrantsApp
// seeds only the identity edge).
func outgoingApp(
	t *testing.T, read []string, grant acl.RelationGrant, draft string, content bool,
) (*App, *acl.Declarative) {
	t.Helper()
	app, d := policyGrantsApp(t, read, []acl.RelationGrant{grant}, draft)
	if content {
		if _, err := app.store.CreateRelation(t.Context(),
			entity.RelationKey{From: "POL-1", FromFace: "published", Type: "cites", To: "FEAT-2"},
			&store.RelationData{}); err != nil {
			t.Fatal(err)
		}
	}
	return app, d
}

// TestPatchRelations_OutgoingIdentityEdgeIsFamilyWide pins the affordance
// gate of an OUTGOING identity-scoped edge written through one face of a
// faced entity. The edge belongs to the whole entity, so it is judged as an
// incoming one from a faced source is: each face the caller reads on its row,
// every other declared face on the policy alone. A content-scoped edge
// belongs to the served face and is judged there only. The GET's
// `_relations` flag must agree with the write in every case.
func TestPatchRelations_OutgoingIdentityEdgeIsFamilyWide(t *testing.T) {
	readPub := []string{"feature", "policy@published"}
	readAll := []string{"feature", "policy"}
	const when = "entity.title ~= 'LOCKED'"
	for _, tc := range []struct {
		name        string
		relType     string
		read        []string
		conditional bool
		draft       string
		want        bool
	}{
		{"full reader, conditional grant, draft denies", "implements", readAll, true, "LOCKED", false},
		{"full reader, conditional grant, draft allows", "implements", readAll, true, "ok", true},
		{"full reader, conditional grant, no draft", "implements", readAll, true, "", true},
		{"published reader, conditional grant, draft would deny", "implements", readPub, true, "LOCKED", false},
		{"published reader, conditional grant, draft would allow", "implements", readPub, true, "ok", false},
		{"published reader, conditional grant, no draft", "implements", readPub, true, "", false},
		{"published reader, unconditional grant, draft stored", "implements", readPub, false, "LOCKED", true},
		{"content edge, full reader, draft would deny", "cites", readAll, true, "LOCKED", true},
		{"content edge, published reader, draft would deny", "cites", readPub, true, "LOCKED", true},
	} {
		content := tc.relType == "cites"
		for _, op := range outgoingOps(tc.relType, content) {
			t.Run(tc.name+"/"+op.name, func(t *testing.T) {
				w := ""
				if tc.conditional {
					w = when
				}
				app, d := outgoingApp(t, tc.read, op.grant(tc.relType, w), tc.draft, content)
				if got := op.flag(relationAffordanceOf(t, app, d, "POL-1@published", tc.relType)); got != tc.want {
					t.Errorf("affordance = %v, want %v", got, tc.want)
				}
				rec := patchRelationsAs(t, app, d, "policy", "policys", "POL-1@published", op.relations)
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
				if got := op.done(t, app); got != tc.want {
					t.Errorf("write took effect = %v, want %v", got, tc.want)
				}
			})
		}
	}
}

// TestPatchRelations_OutgoingIdentityDenialDisclosesNoHiddenFace pins that a
// caller who reads only the published face gets one answer whatever the
// hidden draft holds, and whether it exists: the refusal depends on the
// grants alone. It is the answer an unconditional refusal on the type gives.
func TestPatchRelations_OutgoingIdentityDenialDisclosesNoHiddenFace(t *testing.T) {
	readPub := []string{"feature", "policy@published"}
	conditional := acl.RelationGrant{Relation: "implements", When: "entity.title ~= 'LOCKED'"}
	refused := false
	unconditionalDeny := acl.RelationGrant{Relation: "implements", Create: &refused}
	body := `{"implements":{"add":[{"type":"feature","id":"FEAT-1"}]}}`
	answer := func(grant acl.RelationGrant, draft string) (int, string) {
		app, d := policyGrantsApp(t, readPub, []acl.RelationGrant{grant}, draft)
		rec := patchRelationsAs(t, app, d, "policy", "policys", "POL-1@published", body)
		return rec.Code, problemShape(t, rec.Body.Bytes())
	}
	wantCode, wantBody := answer(unconditionalDeny, "LOCKED")
	if wantCode != http.StatusForbidden {
		t.Fatalf("unconditional deny = %d %s, want 403", wantCode, wantBody)
	}
	for _, draft := range []string{"LOCKED", "ok", ""} {
		code, got := answer(conditional, draft)
		if code != wantCode || got != wantBody {
			t.Errorf("draft %q: %d %s\nwant %d %s", draft, code, got, wantCode, wantBody)
		}
	}
}

// TestRelationRoutes_OutgoingIdentityEdgeIsFamilyWide pins the same rule on
// the single-relation routes: a `when:` that denies on the draft refuses an
// identity edge written through the published face, and a content edge from
// the published face is judged there only.
func TestRelationRoutes_OutgoingIdentityEdgeIsFamilyWide(t *testing.T) {
	const when = "entity.title ~= 'LOCKED'"
	readAll := []string{"feature", "policy"}
	for _, tc := range []struct {
		name, method, relType, target, body string
		content                             bool
		want                                int
	}{
		{"identity create", http.MethodPost, "implements", "", `{"id":"FEAT-1"}`, false, http.StatusForbidden},
		{"identity delete", http.MethodDelete, "implements", "FEAT-2", "", false, http.StatusForbidden},
		{"content create", http.MethodPost, "cites", "", `{"id":"FEAT-1"}`, true, http.StatusCreated},
		{"content delete", http.MethodDelete, "cites", "FEAT-2", "", true, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := outgoingApp(t, readAll, acl.RelationGrant{Relation: tc.relType, When: when}, "LOCKED", tc.content)
			rec := relationAs(t, app, d, tc.method, "policy", "policys", "POL-1@published", tc.relType, tc.target, tc.body)
			if rec.Code != tc.want {
				t.Errorf("%s = %d %s, want %d", tc.method, rec.Code, rec.Body, tc.want)
			}
		})
	}
}
