package entitymanager_test

import (
	"context"
	"errors"
	"strings"
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

// copyCardinalityManager stages page@live into page@review with
// `references: replace`, the bound given on references (TKT-65LVAK).
func copyCardinalityManager(t *testing.T, bound string) (*entitymanager.Manager, store.Store) {
	t.Helper()
	yaml := strings.Replace(replaceTailMetaYAML, "    to: [page]\n", "    to: [page]\n    "+bound+"\n", 1)
	meta, err := metamodel.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{}, CopyGuard: allowGuard{allow: true},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "PAGE-1", Type: "page", Face: "live", Properties: map[string]any{"title": "live"}},
		{ID: "PAGE-1", Type: "page", Face: "review", Properties: map[string]any{"title": "staged"}},
		{ID: "OLD-1", Type: "page", Face: "live", Properties: map[string]any{"title": "old"}},
		{ID: "NEW-1", Type: "page", Face: "live", Properties: map[string]any{"title": "new"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
		}
	}
	for _, k := range []entity.RelationKey{
		{From: "PAGE-1", FromFace: "live", Type: "references", To: "NEW-1"},
		{From: "PAGE-1", FromFace: "review", Type: "references", To: "OLD-1"},
	} {
		if _, err := st.CreateRelation(ctx, k, &store.RelationData{}); err != nil {
			t.Fatalf("seed %v: %v", k, err)
		}
	}
	return mgr, st
}

func reviewTargets(t *testing.T, st store.Store) []string {
	t.Helper()
	var out []string
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{From: "PAGE-1", Type: "references"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if r.FromFace == "review" {
			out = append(out, r.To)
		}
	}
	return out
}

// A replace frees the target face's slot: max_outgoing counts per face, and
// the edge being replaced does not count.
func TestCopyReplace_FitsMaxOutgoing(t *testing.T) {
	mgr, st := copyCardinalityManager(t, "max_outgoing: 1")
	if _, err := mgr.CopyState(context.Background(), entitymanager.CopyRequest{
		Definition: "stage-review", SourceID: "PAGE-1",
	}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got := reviewTargets(t, st); len(got) != 1 || got[0] != "NEW-1" {
		t.Errorf("review edges = %v, want [NEW-1]", got)
	}
}

// NEW-1 already holds its one incoming edge (from the live face), so the
// copy is refused before it writes anything.
func TestCopyReplace_RefusedOverMaxIncoming(t *testing.T) {
	mgr, st := copyCardinalityManager(t, "max_incoming: 1")
	ctx := context.Background()
	_, err := mgr.CopyState(ctx, entitymanager.CopyRequest{Definition: "stage-review", SourceID: "PAGE-1"})
	if !errors.Is(err, entitymanager.ErrCardinalityExceeded) {
		t.Fatalf("err = %v, want ErrCardinalityExceeded", err)
	}
	if got := reviewTargets(t, st); len(got) != 1 || got[0] != "OLD-1" {
		t.Errorf("review edges = %v, want [OLD-1] unchanged", got)
	}
	e, gErr := st.GetEntity(ctx, entity.Ref{ID: "PAGE-1", Face: "review"})
	if gErr != nil || e.Properties["title"] != "staged" {
		t.Errorf("review face = %v (%v), want it unchanged", e, gErr)
	}
}
