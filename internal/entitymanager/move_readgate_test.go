package entitymanager_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A move is planned among the siblings the caller can read, even when the
// caller passes no Among. A sibling whose other endpoint is hidden is not
// authorized, rewritten or audited, and a Before naming it fails as one
// naming no sibling.
func TestUpdateRelation_PositionSkipsUnreadableSiblings(t *testing.T) {
	t.Parallel()

	t.Run("incoming, hidden source", func(t *testing.T) {
		t.Parallel()
		_, st, step, rs := incomingFixture(t, hideOneACL{})
		// Without order values every placement densifies, so a hidden
		// sibling in the plan would be rewritten.
		stripIncomingOrder(t, st, step)
		hidden := rs[2]
		mem := audit.NewMemory()
		mgr := readGatedOrderManager(t, st, "incoming", hidden, mem)

		before := snapshotIncoming(t, st, step)
		if err := moveIn(mgr, rs[3], step, entity.OrderPosition{After: rs[0]}); err != nil {
			t.Fatalf("move with a hidden sibling: %v", err)
		}
		if got := snapshotIncoming(t, st, step)[hidden]; got != before[hidden] {
			t.Errorf("hidden sibling's order = %v, want it untouched (%v)", got, before[hidden])
		}
		requireNoAuditOf(t, mem, hidden)

		requireSameRefError(t, func(ref string) error {
			return moveIn(mgr, rs[3], step, entity.OrderPosition{Before: ref})
		}, hidden)
		requireNoAuditOf(t, mem, hidden)
	})

	t.Run("outgoing, hidden target", func(t *testing.T) {
		t.Parallel()
		_, st, _, recipe, ids := moveFixture(t, "outgoing")
		stripOutgoingOrder(t, st, recipe)
		hidden := ids[2]
		mem := audit.NewMemory()
		mgr := readGatedOrderManager(t, st, "outgoing", hidden, mem)

		before := snapshotOutgoing(t, st, recipe)
		if err := move(mgr, recipe, ids[3], entity.OrderPosition{After: ids[0]}); err != nil {
			t.Fatalf("move with a hidden sibling: %v", err)
		}
		after := snapshotOutgoing(t, st, recipe)
		if after[hidden] != before[hidden] {
			t.Errorf("hidden sibling's order = %v, want it untouched (%v)", after[hidden], before[hidden])
		}
		if maps.Equal(after, before) {
			t.Error("the move wrote nothing")
		}
		requireNoAuditOf(t, mem, hidden)

		requireSameRefError(t, func(ref string) error {
			return move(mgr, recipe, ids[3], entity.OrderPosition{Before: ref})
		}, hidden)
	})
}

// readGatedOrderManager builds a manager over st that allows every write
// and hides one id from every reader.
func readGatedOrderManager(
	t *testing.T, st store.Store, mode, hidden string, sink audit.Audit,
) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: orderableMetamodel(t, mode), Templater: nopTemplater{}, Audit: sink,
		ACL:         hideOneACL{hidden: hidden},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// requireSameRefError fails unless a move relative to hidden answers exactly
// as one relative to an id that does not exist.
func requireSameRefError(t *testing.T, moveRel func(ref string) error, hidden string) {
	t.Helper()
	got, missing := moveRel(hidden), moveRel("NOPE-404")
	if !errors.Is(got, entitymanager.ErrOrderRefNotSibling) {
		t.Fatalf("Before a hidden sibling: err = %v, want ErrOrderRefNotSibling", got)
	}
	if missing == nil || got.Error() != missing.Error() {
		t.Errorf("hidden sibling answers %v, missing id %v", got, missing)
	}
}

func requireNoAuditOf(t *testing.T, mem *audit.Memory, id string) {
	t.Helper()
	for _, r := range mem.Records() {
		if s := r.Subject; s != nil && (s.ID == id || s.FromID == id || s.ToID == id) {
			t.Errorf("audit row names the hidden %s: %+v", id, r)
		}
	}
}

func stripOutgoingOrder(t *testing.T, st store.Store, from string) {
	t.Helper()
	ctx := context.Background()
	for r, err := range st.ListRelations(ctx, store.RelationQuery{From: from, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		props := maps.Clone(r.Properties)
		delete(props, metamodel.OrderPropertyOut)
		if _, err := st.UpdateRelation(ctx, r.Identity(), store.RelationData{Properties: props}); err != nil {
			t.Fatalf("strip order: %v", err)
		}
	}
}

// snapshotOutgoing maps each target to its edge's stored outgoing order.
func snapshotOutgoing(t *testing.T, st store.Store, from string) map[string]any {
	t.Helper()
	out := map[string]any{}
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{From: from, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out[r.To] = r.Properties[metamodel.OrderPropertyOut]
	}
	return out
}
