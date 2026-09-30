package visibility

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// orphanTracer builds a VisibleTracer over st for gate.
func orphanTracer(t *testing.T, st store.Store, gate RowGate) *VisibleTracer {
	t.Helper()
	res, err := NewResolver(gate, NopRedactor{}, st)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := NewVisibleTracer(tracer.New(st, store.WorldScope{}), res, st, store.WorldScope{})
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// seedFamilies stores n faced families, each at two faces, and links half
// of them in pairs, so the other half are orphans.
func seedFamilies(t *testing.T, n int) *memstore.MemStore {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for i := range n {
		for _, f := range []entity.Face{"draft", published} {
			if err := st.CreateEntity(ctx, &entity.Entity{ID: fmt.Sprintf("POL-%d", i), Type: "policy", Face: f}); err != nil {
				t.Fatal(err)
			}
		}
		if i%4 == 1 {
			if _, err := st.CreateRelation(ctx, entity.RelationKey{From: fmt.Sprintf("POL-%d", i-1), Type: "links", To: fmt.Sprintf("POL-%d", i)}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	return st
}

// FindOrphans reads the store a fixed number of times, however many
// families it holds: one header scan, one relation scan and one batched
// title read, never a read per family.
func TestVisibleTracer_FindOrphansReadBudget(t *testing.T) {
	reads := func(n int) int {
		c := storetest.NewCounting(seedFamilies(t, n))
		tr := orphanTracer(t, c, faceRowGate{})
		c.Reset()
		got, err := tr.FindOrphans(context.Background())
		if err != nil || len(got) == 0 {
			t.Fatalf("n=%d: orphans = %v, %v; want some", n, got, err)
		}
		return c.Reads()
	}
	small, large := reads(10), reads(50)
	if small != large {
		t.Fatalf("reads grow with the store: %d at 10 families, %d at 50", small, large)
	}
}

func TestVisibleTracer_FindOrphansEmptyStore(t *testing.T) {
	got, err := orphanTracer(t, memstore.New(), faceRowGate{}).FindOrphans(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("orphans = %v, %v; want none", got, err)
	}
}

// A gate error hides the type, fail-closed: its families are neither
// reported nor able to connect anything.
func TestVisibleTracer_FindOrphansGateErrorHides(t *testing.T) {
	st := seedFamilies(t, 3)
	got, err := orphanTracer(t, st, faceRowGate{err: errors.New("gate down")}).FindOrphans(context.Background())
	if err != nil {
		t.Fatalf("a gate error must hide, not fail the scan: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("orphans = %v; a type whose gate failed must be absent", got)
	}
}
