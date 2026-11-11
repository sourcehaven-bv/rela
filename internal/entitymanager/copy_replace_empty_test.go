package entitymanager_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestCopyReplace_EmptySourceEmptiesTarget pins BUG-FCQBONH: `replace`
// makes the target face's edges of a type equal the source face's, also when
// the source has none. Before the fix a type with no source edges was never
// replaced, so removing the last edge on the source face did not carry over.
func TestCopyReplace_EmptySourceEmptiesTarget(t *testing.T) {
	tests := []struct {
		name       string
		sourceTo   []string
		targetTo   []string
		wantTarget []string
		// denyCreate refuses creating any edge, as the affordance gate of a
		// principal who may remove but not add edges would.
		denyCreate bool
	}{
		{name: "empty source empties target", targetTo: []string{"OLD-1", "OLD-2"}},
		{name: "one source edge replaces target", sourceTo: []string{"NEW-1"},
			targetTo: []string{"OLD-1", "OLD-2"}, wantTarget: []string{"NEW-1"}},
		{name: "empty source and target", wantTarget: nil},
		{name: "empty source empties target without create right", targetTo: []string{"OLD-1"},
			denyCreate: true},
		{name: "uncreatable source edges leave target alone", sourceTo: []string{"NEW-1"},
			targetTo: []string{"OLD-1"}, wantTarget: []string{"OLD-1"}, denyCreate: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var st store.Store = memstore.New()
			meta, err := metamodel.Parse([]byte(replaceTailMetaYAML))
			if err != nil {
				t.Fatalf("metamodel.Parse: %v", err)
			}
			mgr, err := entitymanager.New(entitymanager.Deps{
				Store:       st,
				Meta:        meta,
				Templater:   nopTemplater{},
				Audit:       audit.Nop{},
				ACL:         acl.NopACL{},
				Transitions: statemachine.EmptySet(),
				FieldGate:   entitymanager.AllowAllFieldGate{},
				CopyGuard:   allowGuard{allow: true},
				AnchorGuard: entitymanager.NopAnchorGuard{},
			})
			if err != nil {
				t.Fatalf("entitymanager.New: %v", err)
			}

			ctx := context.Background()
			seed := func(id string, f entity.Face) {
				t.Helper()
				e := &entity.Entity{ID: id, Type: "page", Face: f, Properties: map[string]any{"title": id}}
				if cerr := st.CreateEntity(ctx, e); cerr != nil {
					t.Fatalf("seed %s@%s: %v", id, f, cerr)
				}
			}
			seed("PAGE-1", "live")
			seed("PAGE-1", "review")
			for _, id := range []string{"OLD-1", "OLD-2", "NEW-1"} {
				seed(id, "live")
			}
			link := func(f entity.Face, to string) {
				t.Helper()
				key := entity.RelationKey{From: "PAGE-1", FromFace: f, Type: "references", To: to}
				if _, cerr := st.CreateRelation(ctx, key, &store.RelationData{}); cerr != nil {
					t.Fatalf("seed edge @%s->%s: %v", f, to, cerr)
				}
			}
			for _, to := range tc.sourceTo {
				link("live", to)
			}
			for _, to := range tc.targetTo {
				link("review", to)
			}

			copies, err := entitymanager.NewCopyAffordances(mgr, createGate{allow: !tc.denyCreate})
			if err != nil {
				t.Fatalf("NewCopyAffordances: %v", err)
			}
			if _, cerr := copies.CopyState(ctx, entitymanager.CopyRequest{
				Definition: "stage-review", SourceID: "PAGE-1",
			}); cerr != nil {
				t.Fatalf("copy: %v", cerr)
			}

			var got []string
			review := entity.Face("review")
			for r, lerr := range st.ListRelations(ctx, store.RelationQuery{
				From: "PAGE-1", Type: "references", FromFace: &review,
			}) {
				if lerr != nil {
					t.Fatalf("list: %v", lerr)
				}
				got = append(got, r.To)
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.wantTarget) {
				t.Errorf("target edges = %v, want %v", got, tc.wantTarget)
			}
		})
	}
}

// createGate reads every peer and removes every edge, and creates edges only
// when allow is set.
type createGate struct{ allow bool }

func (createGate) ReadablePeers(_ context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (g createGate) RelationCreatable(context.Context, *entity.Entity, string) (bool, error) {
	return g.allow, nil
}

func (createGate) RelationRemovable(context.Context, *entity.Entity, string) (bool, error) {
	return true, nil
}
