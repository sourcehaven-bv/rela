package queryplan_test

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// scopeIndexMeta declares scopes spanning the lowering vocabulary: one that
// pushes fully (`==` on a literal), one identity scope (which also pushes),
// one that does not lower at all (`~=`), and one over a LIST property — which
// lowers to a jsonb containment probe the composite btree does not serve.
func scopeIndexMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"taak": {
				Label:      "Taak",
				IDPrefixes: []string{"TAAK-"},
				Properties: map[string]metamodel.PropertyDef{
					"title":          {Type: "string"},
					"status":         {Type: "string"},
					"toegewezen_aan": {Type: "string"},
					"volgers":        {Type: "string", List: true},
				},
				QueryScopes: map[string]string{
					"default":   "entity.status == 'open'",
					"mijn":      "is_current_user(entity.toegewezen_aan)",
					"nietklaar": "entity.status ~= 'gereed'",
					"volgend":   "has_current_user(entity.volgers)",
				},
			},
		},
	}
}

func specFor(t *testing.T, list dataentryconfig.List, meta *metamodel.Metamodel) (store.DerivedObjectSpec, bool) {
	t.Helper()
	cfg := &dataentryconfig.Config{Lists: map[string]dataentryconfig.List{"l": list}}
	for _, spec := range queryplan.StaticIndexSpecs(cfg, meta) {
		if spec.Kind == store.DerivedListIndex {
			return spec, true
		}
	}
	return store.DerivedObjectSpec{}, false
}

// TestListIndexSpec_AccountsForQueryScope is AC11 (TKT-EVR2TU).
//
// Every page of a scoped list carries the scope's conjuncts, whether or not
// the list names the scope. An index derived without them describes a query
// nobody issues, so the page scans — and nothing goes red, which is what makes
// this worth a test at all.
func TestListIndexSpec_AccountsForQueryScope(t *testing.T) {
	meta := scopeIndexMeta()
	sorted := []dataentryconfig.SortSpec{{Property: "title"}}

	tests := []struct {
		name  string
		list  dataentryconfig.List
		want  []string
		order []string
	}{
		{
			// The case the ticket calls the drift: the list names no scope,
			// so it silently inherits `default` and probes `status`.
			name:  "inherited default contributes its column",
			list:  dataentryconfig.List{EntityType: "taak", Sort: sorted},
			want:  []string{"status"},
			order: []string{"title"},
		},
		{
			// The sharp case: `mijn:` lowers to an equality on the assignee,
			// so it derives a real column. Missing it is a full scan on the
			// most-used list shape there is.
			name:  "identity scope derives its column",
			list:  dataentryconfig.List{EntityType: "taak", QueryScope: "mijn", Sort: sorted},
			want:  []string{"toegewezen_aan"},
			order: []string{"title"},
		},
		{
			// `all` withdraws the default, so the page carries no scope
			// conjuncts and must not derive a column for one.
			name:  "all withdraws the default",
			list:  dataentryconfig.List{EntityType: "taak", QueryScope: "all", Sort: sorted},
			want:  nil,
			order: []string{"title"},
		},
		{
			// `~=` does not lower, so the store never probes the column and
			// an index over it would be dead weight.
			name:  "non-lowering scope derives nothing",
			list:  dataentryconfig.List{EntityType: "taak", QueryScope: "nietklaar", Sort: sorted},
			want:  nil,
			order: []string{"title"},
		},
		{
			// `has_current_user` IS pushed, but as a jsonb containment probe
			// the composite btree over `properties ->> p` cannot serve — the
			// exclusion ConditionIndexProperties already documents. So the
			// scope contributes no column while the list's own filter still
			// derives its own: a scope must not disqualify the whole spec.
			name: "membership scope derives no column but does not disqualify",
			list: dataentryconfig.List{
				EntityType: "taak", QueryScope: "volgend", Sort: sorted,
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
			},
			want:  []string{"status"},
			order: []string{"title"},
		},
		{
			// Scope and static filter both contribute, deduplicated and
			// sorted like any other composite.
			name: "scope and filter columns combine",
			list: dataentryconfig.List{
				EntityType: "taak", QueryScope: "mijn", Sort: sorted,
				Filters: []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
			},
			want:  []string{"status", "toegewezen_aan"},
			order: []string{"title"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := specFor(t, tc.list, meta)
			if !ok {
				t.Fatalf("no list index derived")
			}
			if !slices.Equal(spec.Properties, tc.want) {
				t.Errorf("properties = %v, want %v", spec.Properties, tc.want)
			}
			if !slices.Equal(spec.OrderBy, tc.order) {
				t.Errorf("order by = %v, want %v", spec.OrderBy, tc.order)
			}
		})
	}
}

// TestListIndexSpec_NoScopesIsUnchanged pins that a project declaring no
// query scopes derives exactly what it did before. The feature is opt-in per
// type, so an existing deployment must not see its indexes churn — a changed
// spec means a DROP and CREATE on the next reconcile.
func TestListIndexSpec_NoScopesIsUnchanged(t *testing.T) {
	meta := scopeIndexMeta()
	def := meta.Entities["taak"]
	def.QueryScopes = nil
	meta.Entities["taak"] = def

	spec, ok := specFor(t, dataentryconfig.List{
		EntityType: "taak",
		Sort:       []dataentryconfig.SortSpec{{Property: "title"}},
		Filters:    []dataentryconfig.FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
	}, meta)
	if !ok {
		t.Fatalf("no list index derived")
	}
	if !slices.Equal(spec.Properties, []string{"status"}) {
		t.Errorf("properties = %v, want [status]", spec.Properties)
	}
}

// TestListIndexSpec_UnknownScopeDerivesNothing covers a scope name the type
// does not declare. The config loader already refuses this, so reaching here
// means a Config built in code; contributing nothing keeps the derivation
// non-destructive rather than pretending to a guarantee it cannot make.
func TestListIndexSpec_UnknownScopeDerivesNothing(t *testing.T) {
	spec, ok := specFor(t, dataentryconfig.List{
		EntityType: "taak", QueryScope: "bestaatniet",
		Sort: []dataentryconfig.SortSpec{{Property: "title"}},
	}, scopeIndexMeta())
	if !ok {
		t.Fatalf("no list index derived")
	}
	if len(spec.Properties) != 0 {
		t.Errorf("properties = %v, want none", spec.Properties)
	}
}

// TestNavEntitiesDeriveTheListIndex pins TKT-PEKL8L: a navigation `entities:`
// entry sends the list endpoint the same request a list with its type, scope
// and sort sends, so it must derive the same index — including when its order
// comes from the type's default_sort rather than its own `sort:`.
func TestNavEntitiesDeriveTheListIndex(t *testing.T) {
	meta := scopeIndexMeta()
	def := meta.Entities["taak"]
	def.DefaultSort = []dataentryconfig.SortSpec{{Property: "title"}}
	meta.Entities["taak"] = def

	navOnly := func(nav dataentryconfig.NavigationEntry) *dataentryconfig.Config {
		return &dataentryconfig.Config{Navigation: []dataentryconfig.NavigationEntry{
			{Group: "G", Items: []dataentryconfig.NavigationEntry{nav}},
		}}
	}
	listSpecs := func(cfg *dataentryconfig.Config) []store.DerivedObjectSpec {
		var out []store.DerivedObjectSpec
		for _, spec := range queryplan.StaticIndexSpecs(cfg, meta) {
			if spec.Kind == store.DerivedListIndex {
				out = append(out, spec)
			}
		}
		return out
	}

	tests := []struct {
		name string
		nav  dataentryconfig.NavigationEntry
		list dataentryconfig.List
	}{
		{
			name: "own sort and identity scope",
			nav: dataentryconfig.NavigationEntry{Entities: "taak", QueryScope: "mijn",
				Sort: []dataentryconfig.SortSpec{{Property: "status"}}},
			list: dataentryconfig.List{EntityType: "taak", QueryScope: "mijn",
				Sort: []dataentryconfig.SortSpec{{Property: "status"}}},
		},
		{
			name: "default_sort and inherited default scope",
			nav:  dataentryconfig.NavigationEntry{Entities: "taak"},
			list: dataentryconfig.List{EntityType: "taak", Sort: []dataentryconfig.SortSpec{{Property: "title"}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := listSpecs(navOnly(tc.nav))
			want := listSpecs(&dataentryconfig.Config{Lists: map[string]dataentryconfig.List{"l": tc.list}})
			if len(want) != 1 {
				t.Fatalf("equivalent list derived %d specs, want 1", len(want))
			}
			if len(got) != 1 || !slices.Equal(got[0].Properties, want[0].Properties) ||
				!slices.Equal(got[0].OrderBy, want[0].OrderBy) {

				t.Errorf("nav entry specs = %+v, want %+v", got, want)
			}
		})
	}

	// A nav entry and a list of the same shape collapse to one index.
	cfg := navOnly(tests[0].nav)
	cfg.Lists = map[string]dataentryconfig.List{"l": tests[0].list}
	if got := listSpecs(cfg); len(got) != 1 {
		t.Errorf("same-shape list and nav entry derived %d specs, want 1", len(got))
	}
}
