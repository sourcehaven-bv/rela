package transitions

import (
	"context"
	"testing"
)

// PrimeTraversals binds a page once; Performable on its rows reuses it, and
// a row outside the page binds on its own.
func TestPrimeTraversals_OneBindPerPage(t *testing.T) {
	b := &stubBinder{answer: map[string]bool{"T-1": true, "T-2": false}}
	set, err := CompileTransitions(relatedMeta(true), WithTransitionTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	ctx := set.PrimeTraversals(context.Background(), "ticket", []string{"T-1", "T-2"})
	if b.binds != 1 {
		t.Fatalf("binds after prime = %d, want 1", b.binds)
	}
	for id, wantDone := range map[string]bool{"T-1": true, "T-2": false} {
		got := map[string]bool{}
		for _, v := range set.Performable(ctx, ent(id, "ticket", "open"), "status", inertGuard{}, nil) {
			got[v.To] = v.Allowed
		}
		if got["done"] != wantDone || got["dropped"] == wantDone {
			t.Fatalf("%s verdicts = %v", id, got)
		}
	}
	if b.binds != 1 {
		t.Fatalf("binds after primed rows = %d, want 1", b.binds)
	}
	set.Performable(ctx, ent("T-3", "ticket", "open"), "status", inertGuard{}, nil)
	if b.binds != 2 {
		t.Fatalf("binds after unprimed row = %d, want 2", b.binds)
	}
	if got := set.PrimeTraversals(context.Background(), "person", []string{"P-1"}); got.Value(primedTraversalsKey{}) != nil {
		t.Fatal("a type without traversing whens must not be primed")
	}
}
