package visibility_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
	"github.com/Sourcehaven-BV/rela/internal/visibility/visibilitytest"
)

// TestPolicyReaderConformance runs the shared suite against the
// production PolicyReader.
func TestPolicyReaderConformance(t *testing.T) {
	visibilitytest.RunReaderTests(t, func(
		t *testing.T, gate visibility.RowGate, redact visibility.FieldRedactor, get visibility.EntityGetter,
	) visibility.Reader {
		t.Helper()
		r, err := visibility.NewPolicyReader(gate, redact, get)
		if err != nil {
			t.Fatalf("NewPolicyReader: %v", err)
		}
		return r
	})
}

// TestVisibleTracerConformance runs the shared suite against the
// production VisibleTracer decorator.
func TestVisibleTracerConformance(t *testing.T) {
	visibilitytest.RunTracerTests(t, func(
		t *testing.T, base tracer.Tracer,
		gate visibility.RowGate, redact visibility.FieldRedactor, get visibility.EntityGetter,
	) tracer.Tracer {
		t.Helper()
		tr, err := visibility.NewVisibleTracer(base, gate, redact, get)
		if err != nil {
			t.Fatalf("NewVisibleTracer: %v", err)
		}
		return tr
	})
}

// TestAllowAllReader pins the pass-through contract plus the one
// non-pass-through behavior: the Reader-semantics stored-type check.
func TestAllowAllReader(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	seed := &entity.Entity{ID: "T-1", Type: "ticket", Properties: map[string]any{"title": "One", "secret": "x"}}
	if err := st.CreateEntity(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	r, err := visibility.NewAllowAllReader(st)
	if err != nil {
		t.Fatalf("NewAllowAllReader: %v", err)
	}

	t.Run("PassThroughGet", func(t *testing.T) {
		e, ok, gerr := r.Get(ctx, "ticket", "T-1")
		if gerr != nil || !ok {
			t.Fatalf("Get = (ok=%v, err=%v)", ok, gerr)
		}
		if e.Properties["secret"] != "x" {
			t.Fatalf("AllowAllReader redacted: %v", e.Properties)
		}
	})
	t.Run("StoredTypeCheckStillHolds", func(t *testing.T) {
		if e, ok, gerr := r.Get(ctx, "person", "T-1"); e != nil || ok || gerr != nil {
			t.Fatalf("cross-type Get = (%v,%v,%v), want miss", e, ok, gerr)
		}
	})
	t.Run("PassThroughFilters", func(t *testing.T) {
		ents := []*entity.Entity{seed}
		if got := r.Filter(ctx, ents); len(got) != 1 || got[0] != seed {
			t.Fatalf("Filter altered input: %v", got)
		}
		rels := []*entity.Relation{{From: "T-1", Type: "r", To: "GHOST"}}
		if got := r.FilterRelations(ctx, rels); len(got) != 1 {
			t.Fatalf("FilterRelations altered input: %v", got)
		}
	})
}

// TestConstructorsRejectNil pins the constructors-reject-nil rule for
// every constructor in the package.
func TestConstructorsRejectNil(t *testing.T) {
	st := memstore.New()
	base := tracer.New(st)
	gate := visibility.NopGate{}
	redact := visibility.NopRedactor{}

	cases := []struct {
		name string
		fn   func() error
	}{
		{"PolicyReader nil gate", func() error { _, err := visibility.NewPolicyReader(nil, redact, st); return err }},
		{"PolicyReader nil redact", func() error { _, err := visibility.NewPolicyReader(gate, nil, st); return err }},
		{"PolicyReader nil get", func() error { _, err := visibility.NewPolicyReader(gate, redact, nil); return err }},
		{"AllowAllReader nil get", func() error { _, err := visibility.NewAllowAllReader(nil); return err }},
		{"VisibleTracer nil base", func() error { _, err := visibility.NewVisibleTracer(nil, gate, redact, st); return err }},
		{"VisibleTracer nil gate", func() error { _, err := visibility.NewVisibleTracer(base, nil, redact, st); return err }},
		{"VisibleTracer nil redact", func() error { _, err := visibility.NewVisibleTracer(base, gate, nil, st); return err }},
		{"VisibleTracer nil get", func() error { _, err := visibility.NewVisibleTracer(base, gate, redact, nil); return err }},
		{"DeclarativeGate nil", func() error { _, err := visibility.NewDeclarativeGate(nil); return err }},
		{"PolicyRedactor nil", func() error { _, err := visibility.NewPolicyRedactor(nil); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("expected constructor error for nil collaborator, got nil")
			}
		})
	}
}
