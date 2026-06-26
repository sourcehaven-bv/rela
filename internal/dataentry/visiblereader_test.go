package dataentry

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// configGate is a configurable readGate test double for visibleReader unit
// tests: permits is the per-id verdict; err (if set) is returned from every
// probe to exercise the fail-closed paths.
type configGate struct {
	permits map[string]bool
	err     error
}

func (g configGate) PermitsRead(_ context.Context, _ /*entityType*/, id string) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.permits[id], nil
}

func (g configGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	if g.err != nil {
		return nil, g.err
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = g.permits[id]
	}
	return m, nil
}

func (configGate) ReadQuery(context.Context, string) acl.ReadQueryResult {
	return acl.ReadQueryResult{AllowAll: true}
}

func (configGate) SearchScope(context.Context, []string) map[string]search.TypeScope {
	return map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
}

func seedReader(t *testing.T) visibleReader {
	t.Helper()
	st := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "T2"}},
		{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "F1"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	return newVisibleReader(st)
}

func TestVisibleReader_GetVisible(t *testing.T) {
	vr := seedReader(t)

	t.Run("permitted and present", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-001": true}})
		e, found, err := vr.getVisible(ctx, "ticket", "TKT-001")
		if err != nil || !found || e == nil || e.ID != "TKT-001" {
			t.Fatalf("got (%v, %v, %v), want (TKT-001, true, nil)", e, found, err)
		}
	})

	t.Run("denied is indistinguishable from absent", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-001": false}})
		e, found, err := vr.getVisible(ctx, "ticket", "TKT-001")
		if err != nil || found || e != nil {
			t.Fatalf("denied: got (%v, %v, %v), want (nil, false, nil)", e, found, err)
		}
	})

	t.Run("absent entity that is permitted", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-999": true}})
		e, found, err := vr.getVisible(ctx, "ticket", "TKT-999")
		if err != nil || found || e != nil {
			t.Fatalf("absent: got (%v, %v, %v), want (nil, false, nil)", e, found, err)
		}
	})

	t.Run("gate error surfaces (not a deny)", func(t *testing.T) {
		sentinel := errors.New("gate boom")
		ctx := withReadGate(context.Background(), configGate{err: sentinel})
		e, found, err := vr.getVisible(ctx, "ticket", "TKT-001")
		if !errors.Is(err, sentinel) || found || e != nil {
			t.Fatalf("gate error: got (%v, %v, %v), want (nil, false, sentinel)", e, found, err)
		}
	})

	t.Run("the store read happens only after the gate allows", func(t *testing.T) {
		// A deny must NOT touch the store — verified indirectly: a denied
		// read of an absent id returns the same (nil,false,nil) as a denied
		// read of a present id, so no existence signal leaks.
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{}})
		ePresent, fp, _ := vr.getVisible(ctx, "ticket", "TKT-001")
		eAbsent, fa, _ := vr.getVisible(ctx, "ticket", "TKT-999")
		if fp || fa || ePresent != nil || eAbsent != nil {
			t.Fatalf("denied present vs absent must be identical: present=(%v,%v) absent=(%v,%v)",
				ePresent, fp, eAbsent, fa)
		}
	})
}

func TestVisibleReader_FilterVisible(t *testing.T) {
	vr := seedReader(t)
	candidates := []*entity.Entity{
		{ID: "TKT-001", Type: "ticket"},
		{ID: "TKT-002", Type: "ticket"},
		{ID: "FEAT-001", Type: "feature"},
	}

	t.Run("drops non-permitted, preserves order", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{
			"TKT-001": true, "TKT-002": false, "FEAT-001": true,
		}})
		got := vr.filterVisible(ctx, candidates)
		if len(got) != 2 || got[0].ID != "TKT-001" || got[1].ID != "FEAT-001" {
			t.Fatalf("got %v, want [TKT-001 FEAT-001] in order", ids(got))
		}
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		if got := vr.filterVisible(context.Background(), nil); got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})

	t.Run("gate error fails closed (drops the whole type)", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{err: errors.New("gate down")})
		got := vr.filterVisible(ctx, candidates)
		if len(got) != 0 {
			t.Fatalf("gate error must drop everything fail-closed, got %v", ids(got))
		}
	})

	t.Run("fresh slice, candidates not aliased", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{
			"TKT-001": true, "TKT-002": true, "FEAT-001": true,
		}})
		got := vr.filterVisible(ctx, candidates)
		if len(got) == 0 || &got[0] == &candidates[0] {
			t.Fatal("filterVisible must return a fresh slice, not alias candidates")
		}
	})
}

func ids(es []*entity.Entity) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}
