package dataentry

import (
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestV1FilteringListProperty pins the semantics of filtering a LIST-typed
// property through the HTTP list endpoint (BUG-AMK38R).
//
// Before the fix, applyV1Filters compared fmt.Sprintf("%v", prop) — rendering
// []any{"urgent","blocker"} as "[urgent blocker]" — so eq/in could never match
// and ne returned the very rows it was asked to exclude. Every pre-existing
// filter test seeded a scalar string property, which is why that shipped.
//
// The rule pinned here matches internal/filter.matchList and the static
// `filters:` path (propertyContains): ANY element satisfies eq/in/contains,
// and ne is its exact complement.
func TestV1FilteringListProperty(t *testing.T) {
	const (
		multiID  = "TKT-multi"
		singleID = "TKT-single"
		emptyID  = "TKT-empty"
	)

	newApp := func(t *testing.T) *App {
		t.Helper()
		app := newTestAppV1(t)
		seedEntity(app, &entity.Entity{ID: multiID, Type: "ticket",
			Properties: map[string]any{"tags": []any{"urgent", "blocker"}}})
		seedEntity(app, &entity.Entity{ID: singleID, Type: "ticket",
			Properties: map[string]any{"tags": []any{"later"}}})
		seedEntity(app, &entity.Entity{ID: emptyID, Type: "ticket",
			Properties: map[string]any{"tags": []any{}}})
		return app
	}

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "eq matches any element, not the whole list",
			query: "filter[tags]=urgent",
			want:  []string{multiID},
		},
		{
			name:  "eq matches a later element too",
			query: "filter[tags]=blocker",
			want:  []string{multiID},
		},
		{
			name:  "eq does not match the stringified slice form",
			query: "filter[tags]=%5Burgent+blocker%5D",
			want:  nil,
		},
		{
			name:  "in matches an entity holding any listed value",
			query: "filter[tags][in]=urgent,later",
			want:  []string{multiID, singleID},
		},
		{
			// The wrong-answer case: ne used to return multiID, the row it
			// was explicitly asked to exclude.
			name:  "ne excludes an entity whose list contains the value",
			query: "filter[tags][ne]=urgent",
			want:  []string{singleID, emptyID},
		},
		{
			name:  "ne with a comma list excludes every named value",
			query: "filter[tags][ne]=urgent,later",
			want:  []string{emptyID},
		},
		{
			name:  "contains matches a substring of one element",
			query: "filter[tags][contains]=urg",
			want:  []string{multiID},
		},
		{
			// Guards the accidental half-working behavior of the old code:
			// "urgent blocker" was one string, so a needle spanning the
			// element boundary matched. Elements are separate values.
			name:  "contains does not match across element boundaries",
			query: "filter[tags][contains]=urgent+blocker",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runListFilter(t, newApp(t), tc.query)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("query %q: got %v, want %v", tc.query, got, want)
			}
		})
	}
}

// TestV1FilteringScalarPropertyUnchanged pins that routing list values through
// propertyElements did not alter scalar behavior — the fix must be additive.
func TestV1FilteringScalarPropertyUnchanged(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-open", Type: "ticket",
		Properties: map[string]any{"status": "open"}})
	seedEntity(app, &entity.Entity{ID: "TKT-done", Type: "ticket",
		Properties: map[string]any{"status": "done"}})

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"eq", "filter[status]=open", []string{"TKT-open"}},
		{"ne", "filter[status][ne]=open", []string{"TKT-done"}},
		{"in", "filter[status][in]=open,done", []string{"TKT-open", "TKT-done"}},
		{"contains", "filter[status][contains]=pe", []string{"TKT-open"}},
		{"eq no match", "filter[status]=missing", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runListFilter(t, app, tc.query)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("query %q: got %v, want %v", tc.query, got, want)
			}
		})
	}
}
