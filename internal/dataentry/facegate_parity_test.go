package dataentry

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestFaceGateParity_SurfacesAgreeUnderOneWorld pins that the per-face row
// gate (TKT-7IZHP0) answers the same on every read surface: the single
// GET, the list, `?include=` neighbors and search. The world prefers draft
// and falls back to published; alice may read tickets at published only.
//
//   - TKT-1 has both faces. Its draft is denied, so every surface serves
//     the published face (the fall-through), never the draft.
//   - TKT-2 has only a draft. No surface may admit it exists.
//
// Both halves of the verdict are run: a global grant (AllowAll with a face
// allowlist) and a relation-conferred one (a GraphQuery with FaceIn).
func TestFaceGateParity_SurfacesAgreeUnderOneWorld(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy *acl.Policy
		edges  bool
	}{
		{
			name: "global grant",
			policy: &acl.Policy{
				Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket@published"}}},
				Assignments: map[string]string{"alice": "viewer"},
			},
		},
		{
			name: "conferred grant",
			policy: &acl.Policy{
				Roles:         map[string]acl.RoleDef{"watcher": {Read: []string{"ticket@published"}}},
				RoleRelations: map[string]acl.RoleRelationDef{"watches": {Confers: "watcher"}},
			},
			edges: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := withWorldNeighbors(t, facedTicketApp(t))
			ctx := context.Background()
			seedFace(t, app, "TKT-1", "ticket", "draft", "alpha draft")
			seedFace(t, app, "TKT-1", "ticket", "published", "alpha published")
			seedFace(t, app, "TKT-2", "ticket", "draft", "alpha hidden")
			seedFace(t, app, "TKT-9", "ticket", "published", "anchor")
			for _, to := range []string{"TKT-1", "TKT-2"} {
				if _, err := app.store.CreateRelation(ctx,
					entity.RelationKey{From: "TKT-9", Type: "blocks", To: to}, nil); err != nil {
					t.Fatal(err)
				}
			}
			if tc.edges {
				for _, to := range []string{"TKT-1", "TKT-2", "TKT-9"} {
					if _, err := app.store.CreateRelation(ctx,
						entity.RelationKey{From: "alice", Type: "watches", To: to}, nil); err != nil {
						t.Fatal(err)
					}
				}
			}
			d := mustNewACL(t, tc.policy, app.store)
			app.acl = d
			wctx := withWorld(aliceCtx(), worldHandle{name: "draft-first", scope: store.NewWorldScope(
				map[string]store.TypeResolution{"ticket": {
					Chain:    []entity.Face{"draft", "published"},
					Fallback: store.FallbackExclude,
				}})})

			// GET: TKT-1 falls through to published; TKT-2 is a miss.
			rec := getEntityAs(wctx, t, app, d, "ticket", "tickets", "TKT-1", "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET TKT-1: %d %s", rec.Code, rec.Body)
			}
			if got := titleOf(t, rec.Body.Bytes()); got != "alpha published" {
				t.Errorf("GET TKT-1 served %q, want the published face", got)
			}
			if miss := getEntityAs(wctx, t, app, d, "ticket", "tickets", "TKT-2", ""); miss.Code != http.StatusNotFound {
				t.Errorf("GET TKT-2: %d, want 404 (its only face is denied)", miss.Code)
			}

			want := map[string]string{"TKT-1": "alpha published"}

			// LIST.
			resp, lrec := listEntitiesAs(wctx, t, app, d, "ticket", "tickets", "")
			if lrec.Code != http.StatusOK {
				t.Fatalf("list: %d %s", lrec.Code, lrec.Body)
			}
			listed := map[string]string{}
			for _, e := range resp.Data {
				if e.ID != "TKT-9" {
					listed[e.ID] = e.Title
				}
			}
			if !maps.Equal(listed, want) {
				t.Errorf("list = %v, want %v", listed, want)
			}

			// ?include=: the anchor's blockers.
			rec = getEntityAs(wctx, t, app, d, "ticket", "tickets", "TKT-9", "include=blocks")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET TKT-9: %d %s", rec.Code, rec.Body)
			}
			var anchor struct {
				Relations map[string][]string `json:"relations"`
				Included  map[string]struct {
					Title string `json:"_title"`
				} `json:"included"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &anchor); err != nil {
				t.Fatal(err)
			}
			if got := anchor.Relations["blocks"]; !slices.Equal(got, []string{"TKT-1"}) {
				t.Errorf("relations.blocks = %v, want [TKT-1]", got)
			}
			included := map[string]string{}
			for id, e := range anchor.Included {
				included[id] = e.Title
			}
			if !maps.Equal(included, want) {
				t.Errorf("included = %v, want %v", included, want)
			}

			// SEARCH.
			sresp, srec := searchAs(wctx, t, app, d, "alpha")
			if srec.Code != http.StatusOK {
				t.Fatalf("search: %d %s", srec.Code, srec.Body)
			}
			found := map[string]string{}
			for _, e := range sresp.Data {
				found[e.ID] = e.Title
			}
			if !maps.Equal(found, want) {
				t.Errorf("search = %v, want %v", found, want)
			}
		})
	}
}

// titleOf decodes the `_title` of a single-entity response.
func titleOf(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Title string `json:"_title"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return e.Title
}
