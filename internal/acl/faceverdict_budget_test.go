package acl

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// ReadableFacesMany answers a whole batch with ONE MatchingFaces query,
// whatever its size, and answers per face row: the reviewer role grants
// published only, so each policy is readable at published and not at draft.
func TestReadableFacesMany_OneQueryPerBatch(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			ctx := context.Background()
			base := memstore.New()
			mustCreate := func(e *entity.Entity) {
				t.Helper()
				if err := base.CreateEntity(ctx, e); err != nil {
					t.Fatal(err)
				}
			}
			mustCreate(&entity.Entity{ID: "alice", Type: "user"})
			ids := make([]string, 0, n)
			for i := range n {
				id := fmt.Sprintf("POL-%03d", i)
				ids = append(ids, id)
				mustCreate(&entity.Entity{ID: id, Type: "policy", Face: "draft"})
				mustCreate(&entity.Entity{ID: id, Type: "policy", Face: "published"})
				if _, err := base.CreateRelation(ctx,
					entity.RelationKey{From: "alice", Type: "reviews", To: id}, nil); err != nil {
					t.Fatal(err)
				}
			}
			st := storetest.NewCounting(base)
			d, err := NewDeclarative(&Policy{
				Roles:         map[string]RoleDef{"reviewer": {Read: []string{"policy@published"}}},
				RoleRelations: map[string]RoleRelationDef{"reviews": {Confers: "reviewer"}},
			}, aclstore.MustNewGraph(base), st)
			if err != nil {
				t.Fatal(err)
			}
			req, err := d.ForIdentity(identityFor(t.Context(), t, d, principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
			if err != nil {
				t.Fatal(err)
			}
			st.Reset()
			vs, err := req.ReadableFacesMany(ctx, "policy", ids)
			if err != nil {
				t.Fatal(err)
			}
			if got := st.Calls()["MatchingFaces"]; got != 1 || st.Reads() != 1 {
				t.Errorf("store reads = %s, want exactly one MatchingFaces", st)
			}
			for _, id := range ids {
				v := vs.For(id)
				if !v.Contains("published") || v.Contains("draft") {
					t.Fatalf("%s: verdict %v, want published only", id, v.Faces())
				}
			}
		})
	}
}

// TestReadableFacesMany_RowsScaleFree pins the rows a batch of 10 reads:
// the faces of those 10, however many other policies alice reviews.
func TestReadableFacesMany_RowsScaleFree(t *testing.T) {
	ctx := context.Background()
	got := storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
		base := memstore.New()
		mustCreate := func(e *entity.Entity) {
			t.Helper()
			if err := base.CreateEntity(ctx, e); err != nil {
				t.Fatal(err)
			}
		}
		mustCreate(&entity.Entity{ID: "alice", Type: "user"})
		ids := make([]string, 0, 10)
		for i := range n {
			id := fmt.Sprintf("POL-%04d", i)
			if i < 10 {
				ids = append(ids, id)
			}
			mustCreate(&entity.Entity{ID: id, Type: "policy", Face: "draft"})
			mustCreate(&entity.Entity{ID: id, Type: "policy", Face: "published"})
			if _, err := base.CreateRelation(ctx,
				entity.RelationKey{From: "alice", Type: "reviews", To: id}, nil); err != nil {
				t.Fatal(err)
			}
		}
		st := storetest.NewCounting(base)
		d, err := NewDeclarative(&Policy{
			Roles:         map[string]RoleDef{"reviewer": {Read: []string{"policy@published"}}},
			RoleRelations: map[string]RoleRelationDef{"reviews": {Confers: "reviewer"}},
		}, aclstore.MustNewGraph(st), st)
		if err != nil {
			t.Fatal(err)
		}
		req, err := d.ForIdentity(identityFor(ctx, t, d, principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
		if err != nil {
			t.Fatal(err)
		}
		st.Reset()
		if _, err := req.ReadableFacesMany(ctx, "policy", ids); err != nil {
			t.Fatal(err)
		}
		return st.Cost(), st.String()
	})
	// One MatchingFaces whose rows are the 10 published faces asked for.
	if want := (storetest.Cost{Calls: 1, Rows: 10}); got != want {
		t.Errorf("a batch of 10 ids cost %s, want %s", got, want)
	}
}
