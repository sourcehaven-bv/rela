package visibility_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// countingTagGate is tagGate that counts its gate and permission calls.
type countingTagGate struct {
	tagGate
	many, perms, single *int
}

func (g countingTagGate) PermitsRead(ctx context.Context, typ, id string) (bool, error) {
	*g.single++
	return g.tagGate.PermitsRead(ctx, typ, id)
}

func (g countingTagGate) ReadableFacesMany(ctx context.Context, typ string, ids []string) (acl.FaceVerdicts, error) {
	*g.many++
	return g.tagGate.ReadableFacesMany(ctx, typ, ids)
}

func (g countingTagGate) HoldsPermission(ctx context.Context, perm string) bool {
	*g.perms++
	return g.tagGate.HoldsPermission(ctx, perm)
}

// TestTaggedEntities_Budget: the gate costs one header query, one batch gate
// call per stored type and one permission check, at 10 rows and at 50.
func TestTaggedEntities_Budget(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := memstore.New()
			read := map[string]bool{}
			var tags listedTags
			for i := range n {
				live := fmt.Sprintf("TKT-%03d", i)
				if err := base.CreateEntity(context.Background(), entity.New(live, "ticket")); err != nil {
					t.Fatal(err)
				}
				read[live] = true
				tags = append(tags, tagRow(live, "ticket", false), tagRow(fmt.Sprintf("OLD-%03d", i), "ticket", true))
			}
			st := storetest.NewCounting(base)
			var many, perms, single int
			gate := countingTagGate{tagGate: tagGate{read: read, history: true}, many: &many, perms: &perms, single: &single}
			policy, err := visibility.NewPolicyReader(gate, visibility.NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			script, err := visibility.NewScriptReader(policy, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := script.WithWorld(visibility.WorldOf(store.TrivialScope())).
				WithHistory(cannedHistory{}, tags)
			st.Reset()
			got, err := r.TaggedEntities(context.Background(), mustTag(t), "")
			if err != nil || len(got) != 2*n {
				t.Fatalf("got %d rows, %v; want %d", len(got), err, 2*n)
			}
			if st.Reads() != 1 || st.Calls()["ListEntityHeaders"] != 1 {
				t.Errorf("reads = %s, want exactly one ListEntityHeaders", st)
			}
			if many != 1 || single != 0 || perms != 1 {
				t.Errorf("gate calls = %d many, %d single, %d permission; want 1, 0, 1", many, single, perms)
			}
		})
	}
}
