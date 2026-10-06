package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The relation writes outside the PATCH and /relations routes must ask the
// questions those routes ask: the affordance gate over the edge's source
// family ([affordanceService.relationSources]) and the manager's ACL request.

// restoreAs posts a relation-history restore of version 1 of
// POL-1 --implements--> FEAT-2 as alice.
func restoreAs(t *testing.T, app *App, d *acl.Declarative) *httptest.ResponseRecorder {
	t.Helper()
	req := asAlice(t, d, httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/api/v1/_relation_history/policy/POL-1/implements/FEAT-2/1/restore", http.NoBody))
	rec := httptest.NewRecorder()
	handleV1RelationHistory(app, rec, req)
	return rec
}

// TestRelationHistoryRestore_RunsTheRelationAffordanceGate pins that a
// restore is gated like the PATCH that would make the same change: a
// re-create needs the edge to be creatable and a restored meta value to be
// writable, judged over every face of the faced source. A face the caller
// cannot read is judged on the policy alone, and the denial does not name it.
func TestRelationHistoryRestore_RunsTheRelationAffordanceGate(t *testing.T) {
	const when = "entity.title ~= 'LOCKED'"
	readPub := []string{"feature", "policy@published"}
	readAll := []string{"feature", "policy"}
	conditional := acl.RelationGrant{Relation: "implements", When: when}
	noteWhen := acl.RelationGrant{Relation: "implements", Fields: []acl.FieldGrant{{Field: "note", When: when}}}
	for _, tc := range []struct {
		name  string
		read  []string
		grant acl.RelationGrant
		draft string
		live  bool // the edge exists, so the restore is a meta update
		want  int
	}{
		{"recreate, the draft denies", readAll, conditional, "LOCKED", false, http.StatusForbidden},
		{"recreate, the draft allows", readAll, conditional, "ok", false, http.StatusOK},
		{"recreate, a hidden face is judged on policy", readPub, conditional, "ok", false, http.StatusForbidden},
		{"recreate, unconditional grant", readPub, acl.RelationGrant{Relation: "implements"}, "ok", false,
			http.StatusOK},
		{"meta onto a live edge, the draft denies", readAll, noteWhen, "LOCKED", true, http.StatusForbidden},
		{"meta onto a live edge, the draft allows", readAll, noteWhen, "ok", true, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := policyGrantsApp(t, tc.read, []acl.RelationGrant{tc.grant}, tc.draft)
			key := entity.RelationKey{From: "POL-1", Type: "implements", To: "FEAT-2"}
			if !tc.live {
				if err := app.store.DeleteRelation(t.Context(), key); err != nil {
					t.Fatal(err)
				}
			}
			app.versions = relHistoryStore{versions: map[string][]store.RelationVersionSnapshot{
				bareRelKey("POL-1", "implements", "FEAT-2"): {{
					RelationVersionMeta: store.RelationVersionMeta{
						Version: 1, Op: store.VersionOpCreate, From: "POL-1", Type: "implements", To: "FEAT-2",
					},
					Properties: map[string]any{"note": "x"},
				}},
			}}

			rec := restoreAs(t, app, d)
			if rec.Code != tc.want {
				t.Fatalf("restore = %d %s, want %d", rec.Code, rec.Body, tc.want)
			}
			if strings.Contains(strings.ToLower(rec.Body.String()), "draft") {
				t.Errorf("restore body names a face: %s", rec.Body)
			}
			rel, err := app.store.GetRelation(t.Context(), key)
			restored := err == nil && rel.Properties["note"] == "x"
			if restored != (tc.want == http.StatusOK) {
				t.Errorf("restore took effect = %v, want %v", restored, tc.want == http.StatusOK)
			}
		})
	}
}

// TestConflictResolve_RelationAsksTheManagersACLQuestion pins that resolving
// a conflicted relation file is authorized as the manager authorizes a
// relation update. An identity-scoped edge from a faced source belongs to
// every face the type declares, so a grant on the bare type alone, which
// covers only the zero face, is not enough for it on either path.
func TestConflictResolve_RelationAsksTheManagersACLQuestion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write []string
		want  int
	}{
		{"bare type grant", []string{"feature", "policy"}, http.StatusForbidden},
		{"every face granted", []string{"feature", "policy@draft", "policy@published"}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := facedAppWith(t, linkableMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
				if err := st.CreateEntity(context.Background(), &entity.Entity{ID: "FEAT-2", Type: "feature",
					Properties: map[string]any{"title": "f2"}}); err != nil {
					t.Fatal(err)
				}
				if _, err := st.CreateRelation(context.Background(),
					entity.RelationKey{From: "POL-1", Type: "implements", To: "FEAT-2"},
					&store.RelationData{}); err != nil {
					t.Fatal(err)
				}
				return mustNewACL(t, &acl.Policy{
					Roles: map[string]acl.RoleDef{"r": {
						Read: []string{"feature", "policy"}, Create: tc.write, Update: tc.write, Delete: tc.write,
					}},
					Assignments: map[string]string{"alice": "r"},
				}, st)
			})

			patch := relationAs(t, app, d, http.MethodPatch, "policy", "policys", "POL-1@published",
				"implements", "FEAT-2", `{"meta":{"note":"x"}}`)
			if patch.Code != tc.want {
				t.Fatalf("PATCH = %d %s, want %d", patch.Code, patch.Body, tc.want)
			}

			root := t.TempDir()
			app.write.paths = &project.Context{Root: root}
			path := filepath.Join(root, "relations", "POL-1--implements--FEAT-2.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			conflicted := strings.NewReplacer("TKT-001", "POL-1", "TKT-002", "FEAT-2", "blocks", "implements").
				Replace(conflictedRelation)
			if err := os.WriteFile(path, []byte(conflicted), 0o644); err != nil {
				t.Fatal(err)
			}
			req := asAlice(t, d, httptest.NewRequestWithContext(t.Context(), http.MethodPost,
				"/api/v1/_conflicts/resolve",
				strings.NewReader(`{"path":"relations/POL-1--implements--FEAT-2.md","content_choice":"ours"}`)))
			rec := httptest.NewRecorder()
			app.write.handleV1ConflictResolve(rec, req)
			if rec.Code != tc.want {
				t.Errorf("conflict resolve = %d %s, want %d as the PATCH", rec.Code, rec.Body, tc.want)
			}
		})
	}
}
