package visibility

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestResolver_WriteTarget pins TKT-7IZHP0 §6: a bare id names the one face
// the world admits and the principal may read, else the write is refused
// with the readable faces; a named face is gated on read (A14).
func TestResolver_WriteTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "POL-1", Type: "policy", Face: "draft"},
		{ID: "POL-1", Type: "policy", Face: "published"},
		{ID: "POL-2", Type: "policy", Face: "draft"},
		{ID: "TKT-1", Type: "ticket"},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	published := WorldOf(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published"}, Fallback: store.FallbackExclude},
	}))
	ranked := WorldOf(store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	}))
	all := faceRowGate{}
	publishedOnly := faceRowGate{permitted: map[string][]entity.Face{"policy": {"published"}}}

	bare := entity.BareAddress
	named := func(id string, f entity.Face) entity.Address { return entity.AddressOf(entity.Ref{ID: id, Face: f}) }
	cases := []struct {
		name      string
		gate      RowGate
		world     World
		typ       string
		addr      entity.Address
		want      entity.Ref
		miss      bool
		ambiguous []entity.Face
	}{
		{name: "faceless type", gate: all, world: published, typ: "ticket", addr: bare("TKT-1"),
			want: entity.Ref{ID: "TKT-1"}},
		// A faced type's bare id names no face, whatever the world admits.
		{name: "world admits one face", gate: all, world: published, typ: "policy", addr: bare("POL-1"),
			ambiguous: []entity.Face{"draft", "published"}},
		{name: "two candidates", gate: all, world: ranked, typ: "policy", addr: bare("POL-1"),
			ambiguous: []entity.Face{"draft", "published"}},
		{name: "no candidate", gate: all, world: published, typ: "policy", addr: bare("POL-2"),
			ambiguous: []entity.Face{"draft"}},
		// One readable face is not enough either; the error lists only it.
		{name: "one readable face", gate: publishedOnly, world: ranked, typ: "policy",
			addr: bare("POL-1"), ambiguous: []entity.Face{"published"}},
		{name: "no readable face is a miss", gate: publishedOnly, world: ranked, typ: "policy",
			addr: bare("POL-2"), miss: true},
		{name: "named face", gate: all, world: published, typ: "policy", addr: named("POL-1", "draft"),
			want: entity.Ref{ID: "POL-1", Face: "draft"}},
		{name: "named unreadable face is a miss (A14)", gate: publishedOnly, world: published, typ: "policy",
			addr: named("POL-1", "draft"), miss: true},
		{name: "wrong type is a miss", gate: all, world: published, typ: "ticket", addr: bare("POL-1"), miss: true},
		{name: "absent id is a miss", gate: all, world: published, typ: "policy", addr: bare("POL-9"), miss: true},
		{name: "denied world is a miss", gate: all, world: DeniedWorld(), typ: "ticket",
			addr: bare("TKT-1"), miss: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := NewResolver(tc.gate, NopRedactor{}, st)
			if err != nil {
				t.Fatal(err)
			}
			got, ok, err := r.WriteTarget(ctx, tc.world, tc.typ, tc.addr)
			var amb *AmbiguousAddressError
			switch {
			case tc.ambiguous != nil:
				if !errors.As(err, &amb) || !slices.Equal(amb.Faces, tc.ambiguous) {
					t.Fatalf("err = %v, want ambiguous over %q", err, tc.ambiguous)
				}
			case err != nil:
				t.Fatalf("err = %v", err)
			case tc.miss:
				if ok {
					t.Fatalf("got %v, want a miss", got)
				}
			case !ok || got != tc.want:
				t.Fatalf("got (%v, %v), want %v", got, ok, tc.want)
			}
		})
	}
}
