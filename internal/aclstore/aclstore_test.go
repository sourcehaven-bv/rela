package aclstore_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
	g := aclstore.MustNewGraph(seed(ctx, t))
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
			if got := g.HasEdge(ctx, tc.from, tc.rel, tc.to, nil); got != tc.want {
				t.Errorf("HasEdge(%s, %s, %s) = %v, want %v", tc.from, tc.rel, tc.to, got, tc.want)
			}
		})
	}
}

func TestGraph_OutgoingRelations(t *testing.T) {
	ctx := context.Background()
	g := aclstore.MustNewGraph(seed(ctx, t))
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
			got, err := g.OutgoingRelations(ctx, tc.from, tc.rel, nil)
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
	l := aclstore.MustNewPrincipalLookup(seed(ctx, t))
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

func TestConstructors_RejectNil(t *testing.T) {
	if g, err := aclstore.NewGraph(nil); err == nil || g != nil {
		t.Errorf("NewGraph(nil) = %v, %v; want an error", g, err)
	}
	if l, err := aclstore.NewPrincipalLookup(nil); err == nil || l != nil {
		t.Errorf("NewPrincipalLookup(nil) = %v, %v; want an error", l, err)
	}
}

// TestGraph_EdgeCondition pins that every lookup acl uses for a conferring
// edge applies the edge condition (TKT-TKSCSBT). A lookup that ignored it
// would keep an expired grant alive on that path.
func TestGraph_EdgeCondition(t *testing.T) {
	ctx := context.Background()
	ms := memstore.New()
	for _, id := range []string{"P", "LIVE", "PARSED", "EXPIRED", "BAD", "NONE"} {
		if err := ms.CreateEntity(ctx, entity.New(id, "x")); err != nil {
			t.Fatal(err)
		}
	}
	for to, until := range map[string]any{
		"LIVE":    "2026-12-31",
		"PARSED":  time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), // a date the fs parser decoded
		"EXPIRED": "2026-01-01",
		"BAD":     "next year",
		"NONE":    nil,
	} {
		var data *store.RelationData
		if until != nil {
			data = &store.RelationData{Properties: map[string]any{"until": until}}
		}
		if _, err := ms.CreateRelation(ctx, entity.RelationKey{From: "P", Type: "grants", To: to}, data); err != nil {
			t.Fatal(err)
		}
	}
	cond := []store.PropPredicate{
		{Property: "until", Op: store.PropIsDate},
		{Property: "until", Op: store.PropGreaterEqual, Value: "2026-11-16"},
		// `<=` on the day itself: a decoded date must compare as a date.
		{Property: "until", Op: store.PropLessEqual, Value: "2026-12-31"},
	}
	g := aclstore.MustNewGraph(ms)
	want := []string{"LIVE", "PARSED"}
	all := []string{"BAD", "EXPIRED", "LIVE", "NONE", "PARSED"}

	var probed []string
	for _, to := range all {
		if g.HasEdge(ctx, "P", "grants", to, cond) {
			probed = append(probed, to)
		}
	}
	if !slices.Equal(probed, want) {
		t.Errorf("HasEdge admits %v, want %v", probed, want)
	}

	out, err := g.OutgoingRelations(ctx, "P", "grants", cond)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	if !slices.Equal(out, want) {
		t.Errorf("OutgoingRelations = %v, want %v", out, want)
	}

	batch, err := g.EdgesBetween(ctx, "grants", cond, []string{"P"}, all)
	if err != nil {
		t.Fatal(err)
	}
	var batched []string
	for _, to := range all {
		if batch["P"][to] {
			batched = append(batched, to)
		}
	}
	if !slices.Equal(batched, want) {
		t.Errorf("EdgesBetween admits %v, want %v", batched, want)
	}

	if !g.HasEdge(ctx, "P", "grants", "NONE", nil) {
		t.Error("no condition must admit every edge")
	}
}

func TestGraph_HasOutgoingEdge(t *testing.T) {
	ctx := context.Background()
	ms := seed(ctx, t)
	// PERS-2's only watches edge sits at a faced tail, which HasEdge does not
	// read either.
	draft := entity.Face("draft")
	if _, err := ms.CreateRelation(ctx,
		entity.RelationKey{From: "PERS-2", FromFace: draft, Type: "watches", To: "TEAM-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.CreateRelation(ctx, entity.RelationKey{From: "PERS-3", Type: "grants", To: "TEAM-1"},
		&store.RelationData{Properties: map[string]any{"until": "2026-01-01"}}); err != nil {
		t.Fatal(err)
	}
	live := []store.PropPredicate{{Property: "until", Op: store.PropGreaterEqual, Value: "2026-11-16"}}
	g := aclstore.MustNewGraph(ms)
	tests := []struct {
		name string
		rel  string
		cond []store.PropPredicate
		from []string
		want bool
	}{
		{"one source holds it", "owns", nil, []string{"PERS-2", "PERS-1"}, true},
		{"no source holds it", "owns", nil, []string{"PERS-2", "PERS-3"}, false},
		{"empty sources", "owns", nil, []string{}, false},
		{"nil sources", "owns", nil, nil, false},
		{"faced tail is not read", "watches", nil, []string{"PERS-2"}, false},
		{"edge fails the condition", "grants", live, []string{"PERS-3"}, false},
		{"edge without a condition", "grants", nil, []string{"PERS-3"}, true},
		{"target is not a source", "owns", nil, []string{"TEAM-1"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.HasOutgoingEdge(ctx, tc.rel, tc.cond, tc.from)
			if err != nil {
				t.Fatalf("HasOutgoingEdge: %v", err)
			}
			if got != tc.want {
				t.Errorf("HasOutgoingEdge(%s, %v) = %v, want %v", tc.rel, tc.from, got, tc.want)
			}
		})
	}
}
