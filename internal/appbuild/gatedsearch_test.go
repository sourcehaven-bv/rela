package appbuild_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// gatedSearchServices seeds a ticket (readable, with a hidden `secret`) and a
// feature (unreadable) for `alice`. withPolicy=false wires no Declarative, the
// NopACL path.
func gatedSearchServices(t *testing.T, withPolicy bool) *appbuild.Services {
	t.Helper()
	ctx := context.Background()
	str := metamodel.PropertyDef{Type: "string"}
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"ticket": {Label: "Ticket", IDPrefix: "TKT", Properties: map[string]metamodel.PropertyDef{
			"title": str, "secret": str,
		}},
		"feature": {Label: "Feature", IDPrefix: "FEAT", Properties: map[string]metamodel.PropertyDef{"title": str}},
	}}

	st := memstore.New()
	tkt := entity.New("TKT-1", "ticket")
	tkt.SetString("title", "alpha ticket")
	tkt.SetString("secret", "sesame")
	feat := entity.New("FEAT-1", "feature")
	feat.SetString("title", "alpha feature")
	for _, e := range []*entity.Entity{tkt, feat} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	opts := []appbuildtest.Option{appbuildtest.WithStore(st)}
	if withPolicy {
		d, err := acl.NewDeclarative(&acl.Policy{
			Roles: map[string]acl.RoleDef{"viewer": {
				Read:    []string{"ticket"},
				Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
			}},
			Assignments: map[string]string{"alice": "viewer"},
		}, acl.NewStoreGraph(st), st)
		if err != nil {
			t.Fatalf("acl.NewDeclarative: %v", err)
		}
		opts = append(opts, appbuildtest.WithDeclarative(d))
	}
	svc := appbuildtest.New(meta, opts...)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func searchIDs(ctx context.Context, s search.Searcher, text string) ([]string, error) {
	var ids []string
	for h, err := range s.Search(ctx, search.Query{Text: text}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, h.ID)
	}
	slices.Sort(ids)
	return ids, nil
}

// TestGatedReads_Searcher covers the searcher GatedReads hands the remote MCP
// server: row scope, the hidden-field match filter, fail-closed on a missing
// principal, and NopACL parity.
func TestGatedReads_Searcher(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		policy  bool
		asAlice bool
		query   string
		want    []string
		wantErr bool
	}{
		{name: "policy hides unreadable row", policy: true, asAlice: true, query: "alpha", want: []string{"TKT-1"}},
		{name: "policy drops hidden-field match", policy: true, asAlice: true, query: "sesame"},
		{name: "policy without principal fails closed", policy: true, query: "alpha", wantErr: true},
		{name: "no policy is the raw searcher", query: "alpha", want: []string{"FEAT-1", "TKT-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tc.asAlice {
				ctx = principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolMCP})
			}
			svc := gatedSearchServices(t, tc.policy)
			got, err := searchIDs(ctx, svc.GatedReads().Searcher, tc.query)
			if tc.wantErr {
				if !errors.Is(err, search.ErrScope) {
					t.Fatalf("err = %v, want search.ErrScope", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGatedReads_SearcherHitShape pins what a gated hit carries: no index
// title, a limit that counts only readable hits, and a type filter that cannot
// widen past the policy.
func TestGatedReads_SearcherHitShape(t *testing.T) {
	t.Parallel()
	alice := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolMCP})
	svc := gatedSearchServices(t, true)
	gated := svc.GatedReads().Searcher

	t.Run("title is cleared", func(t *testing.T) {
		t.Parallel()
		for h, err := range gated.Search(alice, search.Query{Text: "alpha"}) {
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			if h.Title != "" {
				t.Errorf("hit %s carries index title %q", h.ID, h.Title)
			}
		}
	})
	t.Run("limit counts readable hits only", func(t *testing.T) {
		t.Parallel()
		var ids []string
		for h, err := range gated.Search(alice, search.Query{Text: "alpha", Limit: 1}) {
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			ids = append(ids, h.ID)
		}
		if !slices.Equal(ids, []string{"TKT-1"}) {
			t.Errorf("ids = %v, want [TKT-1]", ids)
		}
	})
	t.Run("denied type filter returns nothing", func(t *testing.T) {
		t.Parallel()
		for h, err := range gated.Search(alice, search.Query{Text: "alpha", Types: []string{"feature"}}) {
			t.Errorf("got hit %v (err %v), want none", h.ID, err)
		}
	})
}
