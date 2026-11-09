package remotesync

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

func TestLedger_RoundTripAndMappingChange(t *testing.T) {
	ctx := context.Background()
	meta := testMeta(t)
	m := testMapping(t, meta, nil)
	kv := &mapKV{}
	l, err := loadLedger(ctx, kv, m)
	if err != nil || len(l.rows) != 0 {
		t.Fatalf("missing ledger: %v %v", l, err)
	}
	row := &ledgerRow{Kind: "E", Type: "req", Address: "req/R-1", Hash: digestOf("a"), Local: "REQ-1",
		State: stateLinked, Base: map[string]string{"title<title": "x"}}
	l.put(row)
	if err = l.save(ctx, kv, m); err != nil {
		t.Fatal(err)
	}
	again, err := loadLedger(ctx, kv, m)
	if err != nil {
		t.Fatal(err)
	}
	got := again.get(reconclient.KindEntity, "req/R-1")
	if got == nil || got.Hash != row.Hash || got.Base["title<title"] != "x" {
		t.Fatalf("round trip: %+v", got)
	}
	other := testMapping(t, meta, func(p *Remote) { p.Entities["req"].Fields["note"] = "note" })
	changed, err := loadLedger(ctx, kv, other)
	if err != nil {
		t.Fatal(err)
	}
	got = changed.get(reconclient.KindEntity, "req/R-1")
	if got.Hash != (canonical.Digest{}) || got.Base["title<title"] != "x" {
		t.Fatalf("a mapping change must clear the hash and keep the base: %+v", got)
	}
}

func TestLedger_RejectsCorruptDocuments(t *testing.T) {
	ctx := context.Background()
	m := testMapping(t, testMeta(t), nil)
	for name, doc := range map[string]string{
		"not json":  `{`,
		"version":   `{"version":2,"rows":[]}`,
		"bad kind":  `{"version":1,"rows":[{"k":"X","a":"x","s":"linked"}]}`,
		"no addr":   `{"version":1,"rows":[{"k":"E","a":"","s":"linked"}]}`,
		"bad state": `{"version":1,"rows":[{"k":"E","a":"a/b","s":"odd"}]}`,
		"nil row":   `{"version":1,"rows":[null]}`,
	} {
		t.Run(name, func(t *testing.T) {
			kv := &mapKV{}
			_ = kv.Put(ctx, ledgerKey(m.Name), []byte(doc))
			if _, err := loadLedger(ctx, kv, m); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// TestLedger_LocalAgreesWithReference checks Sum and List against a brute
// force over random rows and ranges.
func TestLedger_LocalAgreesWithReference(t *testing.T) {
	ctx := context.Background()
	rnd := rand.New(rand.NewPCG(1, 2))
	l := &ledger{rows: map[string]*ledgerRow{}}
	types := []string{"a", "b", "c"}
	states := []string{stateLinked, stateDetached, stateRejected, stateDropped}
	for i := range 200 {
		typ := types[rnd.IntN(len(types))]
		kind, addr := "E", fmt.Sprintf("%s/X-%03d", typ, rnd.IntN(500))
		if i%3 == 0 {
			kind, addr = "R", fmt.Sprintf("X-%03d--%s--X-%03d", rnd.IntN(50), typ, rnd.IntN(50))
		}
		l.put(&ledgerRow{Kind: kind, Type: typ, Address: addr, Hash: digestOf(addr),
			State: states[rnd.IntN(len(states))]})
	}
	l.buildIndex()
	lc := local{l: l, sum: visibility.SumHeld}
	scope := reconclient.Types{Entities: []string{"a", "c"}, Relations: []string{"b", "c"}}
	for range 300 {
		lo, hi := fmt.Sprintf("X-%03d", rnd.IntN(60)), fmt.Sprintf("X-%03d", rnd.IntN(60))
		if rnd.IntN(2) == 0 {
			lo, hi = types[rnd.IntN(3)]+"/"+lo, types[rnd.IntN(3)]+"/"+hi
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		r := store.ElementRange{Relations: rnd.IntN(2) == 0, Lo: lo, Hi: hi, Unbounded: rnd.IntN(5) == 0}
		var want []*ledgerRow
		var wantRange reconcile.Range
		for _, row := range l.rows {
			names := scope.Entities
			if r.Relations {
				names = scope.Relations
			}
			if (row.Kind == "R") != r.Relations || row.State == stateDetached || !slices.Contains(names, row.Type) ||
				row.Address < r.Lo || (!r.Unbounded && row.Address >= r.Hi) {

				continue
			}
			want = append(want, row)
			kind := reconcile.KindEntity
			if row.Kind == "R" {
				kind = reconcile.KindRelation
			}
			wantRange = wantRange.Add(reconcile.Element{Kind: kind, Address: row.Address, Hash: row.Hash})
		}
		sum, err := lc.Sum(ctx, scope, r)
		if err != nil {
			t.Fatal(err)
		}
		if sum.Count != wantRange.Count || sum.Fingerprint != wantRange.Sum.String() {
			t.Fatalf("Sum %+v: got %+v, want %d/%s", r, sum, wantRange.Count, wantRange.Sum)
		}
		page, err := lc.List(ctx, scope, r, 5)
		if err != nil {
			t.Fatal(err)
		}
		n := min(len(want), 5)
		if len(page.Rows) != n || page.More != (len(want) > 5) {
			t.Fatalf("List %+v: %d rows more=%v, want %d of %d", r, len(page.Rows), page.More, n, len(want))
		}
		for i := 1; i < len(page.Rows); i++ {
			if page.Rows[i-1].Address >= page.Rows[i].Address {
				t.Fatal("List is not in address order")
			}
		}
	}
}

func TestSplitRelationAddress(t *testing.T) {
	if f, ty, to, ok := splitRelationAddress("A-1--dep--B-2"); !ok || f != "A-1" || ty != "dep" || to != "B-2" {
		t.Fatal("valid address")
	}
	for _, a := range []string{"A-1--dep", "--dep--B", "A--x--y--z", strings.Repeat("-", 4)} {
		if _, _, _, ok := splitRelationAddress(a); ok {
			t.Errorf("%q should not parse", a)
		}
	}
}

// TestPrune_DeletesLedgersOfRemovedRemotes saves the ledgers of two remotes,
// then prunes with one of them configured: only the other's ledger goes.
func TestPrune_DeletesLedgersOfRemovedRemotes(t *testing.T) {
	ctx := context.Background()
	meta := testMeta(t)
	kv := &mapKV{}
	for _, name := range []string{"cat", "dog"} {
		m := testMapping(t, meta, nil)
		m.Name = name
		l, err := loadLedger(ctx, kv, m)
		if err != nil {
			t.Fatal(err)
		}
		l.put(&ledgerRow{Kind: "E", Type: "req", Address: "req/R-1", Local: "REQ-1", State: stateLinked})
		if err := l.save(ctx, kv, m); err != nil {
			t.Fatal(err)
		}
	}
	pruned, err := Prune(ctx, kv, []string{"cat"})
	if err != nil || !slices.Equal(pruned, []string{"dog"}) {
		t.Fatalf("Prune = %v, %v; want [dog]", pruned, err)
	}
	if _, err := kv.Get(ctx, ledgerKey("dog")); err == nil {
		t.Fatal("the ledger of the removed remote is still there")
	}
	if _, err := kv.Get(ctx, ledgerKey("cat")); err != nil {
		t.Fatalf("the ledger of the configured remote is gone: %v", err)
	}
	if pruned, err := Prune(ctx, kv, []string{"cat"}); err != nil || len(pruned) != 0 {
		t.Fatalf("second Prune = %v, %v; want nothing", pruned, err)
	}
	if pruned, err := Prune(ctx, kv, nil); err != nil || !slices.Equal(pruned, []string{"cat"}) {
		t.Fatalf("Prune with no remotes = %v, %v; want [cat]", pruned, err)
	}
}
