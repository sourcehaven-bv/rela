package visibility_test

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/rollup"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

const rollupPathMeta = `version: "1.0"
entities:
  eis:
    label: Eis
    id_prefix: "NE-"
    properties:
      maatregelen:
        type: integer
        rollup: {via: [verwijst_naar, wordt_geimplementeerd_door], count: true, where: {status: actief}}
  iso:
    label: ISO
    id_prefix: "ISO-"
    properties:
      zichtbaar: {type: boolean}
  maatregel:
    label: Maatregel
    id_prefix: "MAAT-"
    properties:
      status: {type: enum, values: [actief, gepland]}
relations:
  verwijst_naar:
    from: [eis]
    to: [iso]
  implementeert:
    from: [maatregel]
    to: [iso]
    inverse: wordt_geimplementeerd_door
`

// TestRollups_PathGatesEveryHop pins TKT-LTSNHD: a roll-up over a path gates
// the entities on the path as it gates the related rows. A hidden ISO
// control breaks the paths through it, a measure reached through two
// controls counts once, and a hidden where field still excludes the far
// entity.
func TestRollups_PathGatesEveryHop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(rollupPathMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mk := func(id, typ string, props map[string]any) {
		e := entity.New(id, typ)
		maps.Copy(e.Properties, props)
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	rel := func(from, typ, to string) {
		if _, err := st.CreateRelation(ctx, entity.RelationKey{From: from, Type: typ, To: to}, nil); err != nil {
			t.Fatal(err)
		}
	}
	// NE-1 -> ISO-1 (visible), ISO-2 (hidden from the restricted reader).
	// MAAT-1 implements both; MAAT-2 only ISO-2; MAAT-3 only ISO-1.
	mk("NE-1", "eis", nil)
	mk("ISO-1", "iso", map[string]any{"zichtbaar": true})
	mk("ISO-2", "iso", map[string]any{"zichtbaar": false})
	for _, m := range []string{"MAAT-1", "MAAT-2", "MAAT-3"} {
		mk(m, "maatregel", map[string]any{"status": "actief"})
	}
	rel("NE-1", "verwijst_naar", "ISO-1")
	rel("NE-1", "verwijst_naar", "ISO-2")
	rel("MAAT-1", "implementeert", "ISO-1")
	rel("MAAT-1", "implementeert", "ISO-2")
	rel("MAAT-2", "implementeert", "ISO-2")
	rel("MAAT-3", "implementeert", "ISO-1")

	visibleISO := visibility.ReadQueryFunc(func(_ context.Context, typ string) (acl.ReadQueryResult, error) {
		if typ != "iso" {
			return acl.ReadQueryResult{AllowAll: true}, nil
		}
		return acl.ReadQueryResult{Query: &store.GraphQuery{Props: []store.PropPredicate{
			{Property: "zichtbaar", Op: store.PropEqual, Value: "true"},
		}}}, nil
	})
	noISO := visibility.ReadQueryFunc(func(_ context.Context, typ string) (acl.ReadQueryResult, error) {
		return acl.ReadQueryResult{AllowAll: typ != "iso", DenyAll: typ == "iso"}, nil
	})
	cases := []struct {
		name   string
		rows   visibility.ReadQueryProvider
		fields visibility.FieldScoper
		want   string
	}{
		{"everything readable counts MAAT-1 once", visibility.AllowAllReadQueries{}, visibility.NopRedactor{},
			"map[maatregelen:3]"},
		{"hidden control breaks its paths", visibleISO, visibility.NopRedactor{}, "map[maatregelen:2]"},
		{"unreadable control type breaks every path", noISO, visibility.NopRedactor{}, "map[maatregelen:0]"},
		{"hidden where field excludes the far end", visibility.AllowAllReadQueries{}, gatedFields{"status"},
			"map[maatregelen:0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := visibility.NewRollups(set, st, tc.rows, tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			got := r.Values(ctx, "eis", []entity.Ref{{ID: "NE-1"}}, store.TrivialScope(), nil)
			if fmt.Sprint(got["NE-1"]) != tc.want {
				t.Errorf("Values = %v, want %v", got["NE-1"], tc.want)
			}
		})
	}
}
