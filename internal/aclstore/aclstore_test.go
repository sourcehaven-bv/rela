package aclstore_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func seed(ctx context.Context, t *testing.T) *memstore.MemStore {
	t.Helper()
	ms := memstore.New()
	people := map[string]string{"PERS-1": "a@example.org", "PERS-2": "b@example.org", "PERS-3": "b@example.org"}
	for id, email := range people {
		e := entity.New(id, "person")
		e.SetString("email", email)
		if err := ms.CreateEntity(ctx, e); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	for _, id := range []string{"TEAM-1", "TEAM-2"} {
		if err := ms.CreateEntity(ctx, entity.New(id, "team")); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "PERS-1", Type: "member-of", To: "TEAM-1"},
		{From: "PERS-1", Type: "member-of", To: "TEAM-2"},
		{From: "PERS-1", Type: "owns", To: "TEAM-1"},
	} {
		if _, err := ms.CreateRelation(ctx, k, nil); err != nil {
			t.Fatalf("create relation %v: %v", k, err)
		}
	}
	return ms
}

func TestGraph_HasEdge(t *testing.T) {
	ctx := context.Background()
	g := aclstore.NewGraph(seed(ctx, t))
	tests := []struct {
		name          string
		from, rel, to string
		want          bool
	}{
		{"existing edge", "PERS-1", "member-of", "TEAM-1", true},
		{"non-member", "PERS-2", "member-of", "TEAM-1", false},
		{"reversed", "TEAM-1", "member-of", "PERS-1", false},
		{"unknown entity", "PERS-9", "member-of", "TEAM-1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.HasEdge(ctx, tc.from, tc.rel, tc.to); got != tc.want {
				t.Errorf("HasEdge(%s, %s, %s) = %v, want %v", tc.from, tc.rel, tc.to, got, tc.want)
			}
		})
	}
}

func TestGraph_OutgoingRelations(t *testing.T) {
	ctx := context.Background()
	g := aclstore.NewGraph(seed(ctx, t))
	tests := []struct {
		name      string
		from, rel string
		want      []string
	}{
		{"filters by type", "PERS-1", "member-of", []string{"TEAM-1", "TEAM-2"}},
		{"other type", "PERS-1", "owns", []string{"TEAM-1"}},
		{"no edges", "PERS-2", "member-of", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.OutgoingRelations(ctx, tc.from, tc.rel)
			if err != nil {
				t.Fatalf("OutgoingRelations: %v", err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("OutgoingRelations(%s, %s) = %v, want %v", tc.from, tc.rel, got, tc.want)
			}
		})
	}
}

func TestPrincipalLookup_LookupEntityByProperty(t *testing.T) {
	ctx := context.Background()
	l := aclstore.NewPrincipalLookup(seed(ctx, t))
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"unique match", "a@example.org", []string{"PERS-1"}},
		{"ambiguous key returns all", "b@example.org", []string{"PERS-2", "PERS-3"}},
		{"no match", "c@example.org", nil},
		{"empty value short-circuits", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.LookupEntityByProperty(ctx, "person", "email", tc.value)
			if err != nil {
				t.Fatalf("LookupEntityByProperty: %v", err)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("LookupEntityByProperty(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
