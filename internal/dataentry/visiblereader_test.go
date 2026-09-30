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

func (configGate) HoldsPermission(context.Context, string) bool { return false }

func (configGate) PermitsWorld(context.Context, string) (bool, error) { return true, nil }

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
	vr, err := newVisibleReader(st, tokenFaceOrder)
	if err != nil {
		t.Fatal(err)
	}
	return vr
}

func TestVisibleReader_InWorld(t *testing.T) {
	vr := seedReader(t)

	t.Run("permitted and present", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-001": true}})
		e, found, err := vr.inWorld(ctx, "ticket", "TKT-001")
		if err != nil || !found || e == nil || e.ID != "TKT-001" {
			t.Fatalf("got (%v, %v, %v), want (TKT-001, true, nil)", e, found, err)
		}
	})

	t.Run("denied is indistinguishable from absent", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-001": false}})
		e, found, err := vr.inWorld(ctx, "ticket", "TKT-001")
		if err != nil || found || e != nil {
			t.Fatalf("denied: got (%v, %v, %v), want (nil, false, nil)", e, found, err)
		}
	})

	t.Run("absent entity that is permitted", func(t *testing.T) {
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-999": true}})
		e, found, err := vr.inWorld(ctx, "ticket", "TKT-999")
		if err != nil || found || e != nil {
			t.Fatalf("absent: got (%v, %v, %v), want (nil, false, nil)", e, found, err)
		}
	})

	t.Run("gate error surfaces (not a deny)", func(t *testing.T) {
		sentinel := errors.New("gate boom")
		ctx := withReadGate(context.Background(), configGate{err: sentinel})
		e, found, err := vr.inWorld(ctx, "ticket", "TKT-001")
		if !errors.Is(err, sentinel) || found || e != nil {
			t.Fatalf("gate error: got (%v, %v, %v), want (nil, false, sentinel)", e, found, err)
		}
	})

	t.Run("the store read happens only after the gate allows", func(t *testing.T) {
		// A deny must NOT touch the store — verified indirectly: a denied
		// read of an absent id returns the same (nil,false,nil) as a denied
		// read of a present id, so no existence signal leaks.
		ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{}})
		ePresent, fp, _ := vr.inWorld(ctx, "ticket", "TKT-001")
		eAbsent, fa, _ := vr.inWorld(ctx, "ticket", "TKT-999")
		if fp || fa || ePresent != nil || eAbsent != nil {
			t.Fatalf("denied present vs absent must be identical: present=(%v,%v) absent=(%v,%v)",
				ePresent, fp, eAbsent, fa)
		}
	})
}

// tokenFaceOrder declares no order, so faces list by token. It is for tests
// whose schema order does not matter.
func tokenFaceOrder(string) []string { return nil }

func TestNewVisibleReader_RejectsNilStore(t *testing.T) {
	if _, err := newVisibleReader(nil, tokenFaceOrder); err == nil {
		t.Fatal("newVisibleReader(nil) = nil error, want a refusal")
	}
	if _, err := newVisibleReader(memstore.New(), nil); err == nil {
		t.Fatal("newVisibleReader with a nil order = nil error, want a refusal")
	}
}

// TestVisibleReader_ReadableType pins the entity-level check a relation
// endpoint named by bare id gets: the stored type when some face is readable,
// and the same empty answer for a hidden id, an absent one and a gate error's
// caller-visible verdict.
func TestVisibleReader_ReadableType(t *testing.T) {
	vr := seedReader(t)
	sentinel := errors.New("gate boom")
	for _, tc := range []struct {
		name    string
		gate    configGate
		id      string
		want    string
		wantErr error
	}{
		{"readable", configGate{permits: map[string]bool{"FEAT-001": true}}, "FEAT-001", "feature", nil},
		{"hidden", configGate{permits: map[string]bool{}}, "FEAT-001", "", nil},
		{"absent", configGate{permits: map[string]bool{"NOPE-1": true}}, "NOPE-1", "", nil},
		{"gate error", configGate{err: sentinel}, "FEAT-001", "", sentinel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := vr.readableType(withReadGate(context.Background(), tc.gate), tc.id)
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("readableType(%s) = (%q, %v), want (%q, %v)", tc.id, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// TestVisibleReader_UntypedAddress pins the typeless read commands and detail
// actions use: the gates run on the STORED type, and a malformed, absent or
// hidden address is the same clean miss.
func TestVisibleReader_UntypedAddress(t *testing.T) {
	vr := seedReader(t)
	ctx := withReadGate(context.Background(), configGate{permits: map[string]bool{"TKT-001": true}})
	if e, ok, err := vr.untypedAddress(ctx, "TKT-001"); err != nil || !ok || e.Type != "ticket" {
		t.Fatalf("readable: got (%v, %v, %v), want the ticket row", e, ok, err)
	}
	for _, addr := range []string{"TKT-002", "TKT-999", "not an id", "TKT-001@@"} {
		if e, ok, err := vr.untypedAddress(ctx, addr); err != nil || ok || e != nil {
			t.Errorf("%q: got (%v, %v, %v), want a clean miss", addr, e, ok, err)
		}
	}
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
