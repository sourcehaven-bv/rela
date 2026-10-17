package acl

import (
	"context"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// primePolicy has two conferring relations, one write gate without
// `confers:` (never probed) and containment through `in`.
func primePolicy() *Policy {
	return &Policy{
		Roles: map[string]RoleDef{
			"owner":   {Update: []string{"doc"}},
			"watcher": {Read: []string{"doc"}},
			"staff":   {Read: []string{"doc"}},
		},
		Assignments: map[string]string{"alice": "staff"},
		RoleRelations: map[string]RoleRelationDef{
			"owns":    {Confers: "owner"},
			"watches": {Confers: "watcher"},
			"gates":   {RequiresPermission: "x"},
		},
		InheritRolesThrough: []string{"in"},
	}
}

// randomPrimeGraph seeds alice, groups, docs and folders with random
// membership, conferring and containment edges, cycles included.
func randomPrimeGraph(t *testing.T, seed uint64, docs int) (st *memstore.MemStore, ids []string) {
	t.Helper()
	ctx := context.Background()
	rng := rand.New(rand.NewPCG(seed, seed))
	st = memstore.New()
	create := func(id, typ string) {
		if err := st.CreateEntity(ctx, &entity.Entity{ID: id, Type: typ}); err != nil {
			t.Fatal(err)
		}
	}
	link := func(from, rel, to string) {
		_, _ = st.CreateRelation(ctx, entity.RelationKey{From: from, Type: rel, To: to}, nil)
	}
	create("alice", "user")
	create("bob", "user")
	groups := []string{"g0", "g1", "g2"}
	for _, g := range groups {
		create(g, "group")
	}
	folders := []string{"f0", "f1", "f2", "f3"}
	for _, f := range folders {
		create(f, "folder")
	}
	ids = make([]string, 0, docs)
	for i := range docs {
		id := fmt.Sprintf("d%02d", i)
		create(id, "doc")
		ids = append(ids, id)
	}
	link("alice", "member-of", groups[rng.IntN(len(groups))])
	link(groups[0], "member-of", groups[1+rng.IntN(2)])
	holders := append([]string{"alice", "bob"}, groups...)
	targets := append(append([]string{}, ids...), folders...)
	for range docs {
		h := holders[rng.IntN(len(holders))]
		tg := targets[rng.IntN(len(targets))]
		link(h, []string{"owns", "watches", "gates"}[rng.IntN(3)], tg)
	}
	for _, id := range targets {
		if rng.IntN(3) > 0 {
			link(id, "in", folders[rng.IntN(len(folders))])
		}
	}
	return st, ids
}

// The prime is the per-entity walk over pre-loaded edges, so it must return
// exactly what the walk returns, attribution order included, on any graph.
func TestPrimeLocalRoles_MatchesPerEntityWalk(t *testing.T) {
	for seed := range uint64(40) {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			st, ids := randomPrimeGraph(t, seed, 20)
			d, err := NewDeclarative(primePolicy(), aclstore.MustNewGraph(st), st)
			if err != nil {
				t.Fatal(err)
			}
			p := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
			plain, _ := d.ForIdentity(identityFor(context.Background(), t, d, p))
			primedReq, _ := d.ForIdentity(identityFor(context.Background(), t, d, p))
			ctx := context.Background()
			pctx := PrimeLocalRoles(WithRequest(ctx, primedReq), append(ids, "f0", "f1"))
			for _, id := range append(ids, "f0", "f1") {
				if _, ok := primedLocalRoles(pctx, primedReq, id); !ok {
					t.Fatalf("%s not primed", id)
				}
				want := plain.computeForEntity(ctx, id)
				got := primedReq.computeForEntity(pctx, id)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: primed %v, walk %v", id, got, want)
				}
			}
		})
	}
}

// A page's local roles cost the same number of store reads at 10 and 50
// rows; the per-entity walk cost 1 + chain probes per row per check. The
// graph has a fixed shape (every doc in f0, f0 in f1, alice owning every
// third doc), so only the row count changes.
func TestPrimeLocalRoles_ReadsAreSizeIndependent(t *testing.T) {
	counts := map[int]int{}
	for _, n := range []int{10, 50} {
		ctx := context.Background()
		base := memstore.New()
		for _, e := range []entity.Entity{{ID: "alice", Type: "user"}, {ID: "f0", Type: "folder"}, {ID: "f1", Type: "folder"}} {
			if err := base.CreateEntity(ctx, &e); err != nil {
				t.Fatal(err)
			}
		}
		link := func(from, rel, to string) {
			if _, err := base.CreateRelation(ctx, entity.RelationKey{From: from, Type: rel, To: to}, nil); err != nil {
				t.Fatal(err)
			}
		}
		link("f0", "in", "f1")
		ids := make([]string, 0, n)
		for i := range n {
			id := fmt.Sprintf("d%02d", i)
			if err := base.CreateEntity(ctx, &entity.Entity{ID: id, Type: "doc"}); err != nil {
				t.Fatal(err)
			}
			link(id, "in", "f0")
			if i%3 == 0 {
				link("alice", "owns", id)
			}
			ids = append(ids, id)
		}
		st := storetest.NewCounting(base)
		d, err := NewDeclarative(primePolicy(), aclstore.MustNewGraph(st), st)
		if err != nil {
			t.Fatal(err)
		}
		alice := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
		req, _ := d.ForIdentity(identityFor(context.Background(), t, d, alice))
		ctx = WithRequest(ctx, req)
		req.Globals()
		st.Reset()
		pctx := PrimeLocalRoles(ctx, ids)
		for _, id := range ids {
			req.computeForEntity(pctx, id)
		}
		counts[n] = st.Reads()
	}
	if counts[10] != counts[50] {
		t.Errorf("reads grow with the page: %d at 10 rows, %d at 50", counts[10], counts[50])
	}
}

// A prime is computed for one Request; another Request on the same ctx
// (a rebind) must not read it.
func TestPrimeLocalRoles_ScopedToRequest(t *testing.T) {
	st, ids := randomPrimeGraph(t, 3, 5)
	d, err := NewDeclarative(primePolicy(), aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	p := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	first, _ := d.ForIdentity(identityFor(context.Background(), t, d, p))
	second, _ := d.ForIdentity(identityFor(context.Background(), t, d, p))
	pctx := PrimeLocalRoles(WithRequest(context.Background(), first), ids)
	if _, ok := primedLocalRoles(pctx, second, ids[0]); ok {
		t.Error("a prime computed for one Request answered another")
	}
	// Priming again adds ids without recomputing the primed ones.
	again := PrimeLocalRoles(pctx, ids)
	if again != pctx {
		t.Error("re-priming covered ids allocated a new prime")
	}
}

// A write strips the prime: once a conferring edge is added, a check on the
// stripped ctx sees it even though the primed ctx predates it.
func TestWithoutLocalRolePrime_SeesLaterEdges(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []entity.Entity{{ID: "alice", Type: "user"}, {ID: "d1", Type: "doc"}} {
		if err := st.CreateEntity(ctx, &e); err != nil {
			t.Fatal(err)
		}
	}
	d, err := NewDeclarative(primePolicy(), aclstore.MustNewGraph(st), st)
	if err != nil {
		t.Fatal(err)
	}
	alice := principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	req, _ := d.ForIdentity(identityFor(context.Background(), t, d, alice))
	pctx := PrimeLocalRoles(WithRequest(ctx, req), []string{"d1"})
	if _, err := st.CreateRelation(ctx, entity.RelationKey{From: "alice", Type: "owns", To: "d1"}, nil); err != nil {
		t.Fatal(err)
	}
	hasOwner := func(attrs []RoleAttribution) bool {
		for _, a := range attrs {
			if a.Role == "owner" {
				return true
			}
		}
		return false
	}
	if hasOwner(req.computeForEntity(pctx, "d1")) {
		t.Fatal("the prime should predate the edge; the test proves nothing")
	}
	if !hasOwner(req.computeForEntity(WithoutLocalRolePrime(pctx), "d1")) {
		t.Error("a stripped ctx still answered from the prime")
	}
}
