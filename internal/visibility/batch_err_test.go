package visibility_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestResolver_ResolveHeadersErr pins the error-returning twin: it answers
// exactly what ResolveHeaders answers, but a failed header read is an error
// rather than an empty result, and a gate error still hides only its type.
func TestResolver_ResolveHeadersErr(t *testing.T) {
	refs := []entity.Ref{
		{ID: "POL-1"}, {ID: "POL-1", Face: faceDraft}, {ID: "POL-1", Face: facePublished},
		{ID: "TKT-1"}, {ID: "TKT-404"},
	}

	t.Run("agrees with ResolveHeaders", func(t *testing.T) {
		for _, w := range []visibility.World{
			visibility.WorldOf(store.TrivialScope()), visibility.WorldOf(publishedWorld()),
		} {
			r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, resolverStore(t))
			want := r.ResolveHeaders(context.Background(), w, refs)
			got, err := r.ResolveHeadersErr(context.Background(), w, refs)
			if err != nil {
				t.Fatalf("ResolveHeadersErr: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("got %d answers, want %d", len(got), len(want))
			}
			for ref, h := range want {
				g, ok := got[ref]
				if !ok || g.Served() != h.Served() || g.Family != h.Family || g.Header.Face != h.Header.Face {
					t.Errorf("%s: got %+v, want %+v", ref, g, h)
				}
			}
		}
	})

	t.Run("a failed header read is returned", func(t *testing.T) {
		boom := errors.New("disk")
		r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, failingLoader{err: boom})
		got, err := r.ResolveHeadersErr(context.Background(), visibility.WorldOf(store.TrivialScope()),
			[]entity.Ref{{ID: "TKT-1"}})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the store fault", err)
		}
		if got != nil {
			t.Errorf("got %v alongside an error, want nil", got)
		}
	})

	t.Run("a gate error hides only its type", func(t *testing.T) {
		buf := captureWarn(t)
		r := mustResolver(t, typeErrGate{failType: "policy"}, visibility.NopRedactor{}, resolverStore(t))
		got, err := r.ResolveHeadersErr(context.Background(), visibility.WorldOf(store.TrivialScope()),
			[]entity.Ref{{ID: "POL-1", Face: faceDraft}, {ID: "TKT-1"}})
		if err != nil {
			t.Fatalf("a gate error must not fail the batch: %v", err)
		}
		if _, ok := got[entity.Ref{ID: "POL-1", Face: faceDraft}]; ok {
			t.Error("a ref of the failing type was answered")
		}
		if !got[entity.Ref{ID: "TKT-1"}].Served() {
			t.Error("a ref of another type was hidden")
		}
		if !strings.Contains(buf.String(), "gate failed") {
			t.Errorf("the gate failure was not logged: %s", buf)
		}
	})

	t.Run("an unset world is an error and reads nothing", func(t *testing.T) {
		st := resolverStore(t)
		r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
		got, err := r.ResolveHeadersErr(context.Background(), visibility.World{}, []entity.Ref{{ID: "TKT-1"}})
		if !errors.Is(err, store.ErrInvalidQuery) || got != nil {
			t.Errorf("got %v, %v; want nil and store.ErrInvalidQuery", got, err)
		}
		if st.Reads() != 0 {
			t.Errorf("an unset world read the store: %s", st)
		}
	})

	t.Run("an empty batch reads nothing", func(t *testing.T) {
		st := resolverStore(t)
		r := mustResolver(t, visibility.NopGate{}, visibility.NopRedactor{}, st)
		got, err := r.ResolveHeadersErr(context.Background(), visibility.WorldOf(store.TrivialScope()), nil)
		if err != nil || len(got) != 0 || st.Reads() != 0 {
			t.Errorf("got %v, %v, %d reads; want nothing", got, err, st.Reads())
		}
	})
}
