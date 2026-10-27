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
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

func TestNewRollups_RejectsNil(t *testing.T) {
	t.Parallel()
	set := &rollup.Set{}
	st := memstore.New()
	if _, err := visibility.NewRollups(set, nil, visibility.AllowAllReadQueries{}, visibility.NopRedactor{}); err == nil {
		t.Error("nil store accepted")
	}
	if _, err := visibility.NewRollups(set, st, nil, visibility.NopRedactor{}); err == nil {
		t.Error("nil read queries accepted")
	}
	if _, err := visibility.NewRollups(set, st, visibility.AllowAllReadQueries{}, nil); err == nil {
		t.Error("nil field scope accepted")
	}
}

// TestMergeRollups pins that a stored value under a roll-up name is never
// served: it is removed, and only a computed value takes its place.
func TestMergeRollups(t *testing.T) {
	t.Parallel()
	names := []string{"count", "latest"}
	got := visibility.MergeRollups(map[string]any{"title": "t", "count": 99, "latest": "stale"},
		names, map[string]any{"count": int64(3)})
	want := map[string]any{"title": "t", "count": int64(3)}
	if !maps.Equal(got, want) {
		t.Errorf("MergeRollups = %v, want %v", got, want)
	}
	if got := visibility.MergeRollups(nil, names, map[string]any{"latest": "x"}); got["latest"] != "x" {
		t.Errorf("MergeRollups(nil) = %v", got)
	}
}

const rollupValuesMeta = `version: "1.0"
entities:
  risk:
    label: Risk
    id_prefix: "RIS-"
    properties:
      title: {type: string}
      open:
        type: integer
        rollup: {via: treatedBy, count: true, where: {done: "false"}}
      latest:
        type: string
        rollup: {via: treatedBy, pick: title, latest_by: due}
  plan:
    label: Plan
    id_prefix: "PLN-"
    properties:
      title: {type: string}
      done: {type: boolean}
      due: {type: date}
relations:
  treats:
    from: [plan]
    to: [risk]
    inverse: treatedBy
`

// gatedFields hides the listed properties of every type from the reader,
// with no grant that shows them.
type gatedFields []string

func (g gatedFields) SearchFieldScope(context.Context, string) (*search.FieldScope, error) {
	return &search.FieldScope{Gated: g}, nil
}

// TestRollups_Values pins the gate-before-fold rules of [visibility.Rollups]:
// a related type the reader cannot read counts nothing, a related row whose
// where property is hidden is left out, and a hidden pick on the latest row
// leaves the roll-up without a value.
func TestRollups_Values(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	meta, err := metamodel.Parse([]byte(rollupValuesMeta))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rollup.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	risk := entity.New("RIS-1", "risk")
	if err = st.CreateEntity(ctx, risk); err != nil {
		t.Fatal(err)
	}
	for id, p := range map[string]map[string]any{
		"PLN-1": {"title": "a", "done": false, "due": "2026-01-01"},
		"PLN-2": {"title": "b", "done": false, "due": "2026-03-01"},
		"PLN-3": {"title": "c", "done": true, "due": "2026-02-01"},
	} {
		e := entity.New(id, "plan")
		maps.Copy(e.Properties, p)
		if err = st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
		if _, err = st.CreateRelation(ctx, entity.RelationKey{From: id, Type: "treats", To: "RIS-1"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	denyAll := visibility.ReadQueryFunc(func(context.Context, string) (acl.ReadQueryResult, error) {
		return acl.ReadQueryResult{DenyAll: true}, nil
	})
	cases := []struct {
		name   string
		rows   visibility.ReadQueryProvider
		fields visibility.FieldScoper
		want   map[string]any
	}{
		{"everything readable", visibility.AllowAllReadQueries{}, visibility.NopRedactor{},
			map[string]any{"open": int64(2), "latest": "b"}},
		{"plans unreadable", denyAll, visibility.NopRedactor{},
			map[string]any{"open": int64(0)}},
		{"done hidden", visibility.AllowAllReadQueries{}, gatedFields{"done"},
			map[string]any{"open": int64(0), "latest": "b"}},
		{"pick hidden", visibility.AllowAllReadQueries{}, gatedFields{"title"},
			map[string]any{"open": int64(2)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, nErr := visibility.NewRollups(set, st, tc.rows, tc.fields)
			if nErr != nil {
				t.Fatal(nErr)
			}
			got := r.Values(ctx, "risk", []entity.Ref{risk.Ref(), risk.Ref()}, store.TrivialScope(), nil)
			if fmt.Sprint(got["RIS-1"]) != fmt.Sprint(tc.want) { // a count may be int or int64
				t.Errorf("Values = %v, want %v", got["RIS-1"], tc.want)
			}
		})
	}
	r, err := visibility.NewRollups(set, st, visibility.AllowAllReadQueries{}, visibility.NopRedactor{})
	if err != nil {
		t.Fatal(err)
	}
	onlyOpen := func(p string) bool { return p == "open" }
	only := r.Values(ctx, "risk", []entity.Ref{risk.Ref()}, store.TrivialScope(), onlyOpen)
	if fmt.Sprint(only["RIS-1"]) != "map[open:2]" {
		t.Errorf("Values(want open) = %v", only["RIS-1"])
	}
}
