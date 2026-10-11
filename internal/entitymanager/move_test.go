package entitymanager_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// moveFixture creates one recipe with four steps on an outgoing-orderable
// relation and returns the step ids in creation order.
func moveFixture(t *testing.T, mode string) (
	mgr *entitymanager.Manager, st store.Store, mem *audit.Memory, recipeID string, ids []string,
) {
	t.Helper()
	mgr, st, mem = newOrderableManagerWithAudit(t, mode)
	ctx := context.Background()
	recipe := mkRecipe(t, mgr, "R")
	for _, title := range []string{"S1", "S2", "S3", "S4"} {
		s := mkStep(t, mgr, title)
		if _, err := mgr.CreateRelation(ctx, entity.RelationKey{From: recipe.ID, Type: "has-step", To: s.ID},
			entity.RelationOptions{}); err != nil {
			t.Fatalf("create relation: %v", err)
		}
		ids = append(ids, s.ID)
	}
	return mgr, st, mem, recipe.ID, ids
}

// outgoingOrder reads the recipe's steps in relation order.
func outgoingOrder(t *testing.T, st store.Store, from string) []string {
	t.Helper()
	var rels []entity.Relation
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{From: from, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list relations: %v", err)
		}
		rels = append(rels, *r)
	}
	var ids []string
	for _, r := range entitymanager.SortRelations(rels, metamodel.OrderPropertyOut) {
		ids = append(ids, r.To)
	}
	return ids
}

func move(mgr *entitymanager.Manager, from, to string, pos entity.OrderPosition) error {
	_, err := mgr.UpdateRelation(context.Background(), entity.RelationKey{From: from, Type: "has-step", To: to},
		entity.RelationOptions{Position: &pos})
	return err
}

func TestUpdateRelation_PositionMoves(t *testing.T) {
	t.Parallel()
	mgr, st, mem, recipe, ids := moveFixture(t, "outgoing")
	before := len(mem.Records())
	if err := move(mgr, recipe, ids[3], entity.OrderPosition{Before: ids[1]}); err != nil {
		t.Fatalf("move: %v", err)
	}
	want := []string{ids[0], ids[3], ids[1], ids[2]}
	if got := outgoingOrder(t, st, recipe); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	var moved int
	for _, r := range mem.Records()[before:] {
		if r.Op == audit.OpUpdateRelation && strings.HasPrefix(r.Summary, "moved ") {
			moved++
		}
	}
	if moved != 1 {
		t.Errorf("moved audit records = %d, want 1", moved)
	}

	if err := move(mgr, recipe, ids[0], entity.OrderPosition{Step: 1}); err != nil {
		t.Fatalf("step: %v", err)
	}
	want = []string{ids[3], ids[0], ids[1], ids[2]}
	if got := outgoingOrder(t, st, recipe); !slices.Equal(got, want) {
		t.Errorf("after step: order = %v, want %v", got, want)
	}
}

// TestUpdateRelation_PositionDensifiesMissingValues pins the Atlas case:
// edges that predate `orderable:` carry no value, and the first move must
// put the row where it was dropped rather than first.
func TestUpdateRelation_PositionDensifiesMissingValues(t *testing.T) {
	t.Parallel()
	mgr, st, mem, recipe, ids := moveFixture(t, "outgoing")
	ctx := context.Background()
	for _, id := range ids {
		key := entity.RelationKey{From: recipe, Type: "has-step", To: id}
		r, err := st.GetRelation(ctx, key)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		props := maps.Clone(r.Properties)
		delete(props, metamodel.OrderPropertyOut)
		if _, err := st.UpdateRelation(ctx, key, store.RelationData{Properties: props}); err != nil {
			t.Fatalf("strip order: %v", err)
		}
	}
	shown := outgoingOrder(t, st, recipe) // missing values: by target id
	before := len(mem.Records())
	if err := move(mgr, recipe, shown[3], entity.OrderPosition{After: shown[0]}); err != nil {
		t.Fatalf("move: %v", err)
	}
	want := []string{shown[0], shown[3], shown[1], shown[2]}
	if got := outgoingOrder(t, st, recipe); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	var renumbered int
	for _, r := range mem.Records()[before:] {
		if strings.HasPrefix(r.TriggeredBy, "renumber:") {
			renumbered++
		}
	}
	if renumbered == 0 {
		t.Error("densify wrote siblings without renumber audit records")
	}
}

func TestUpdateRelation_PositionErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode string
		opts func(ids []string) entity.RelationOptions
		want error
	}{
		{"not orderable", "", func(ids []string) entity.RelationOptions {
			return entity.RelationOptions{Position: &entity.OrderPosition{After: ids[1]}}
		}, entitymanager.ErrRelationNotOrderable},
		{"incoming only", "incoming", func(ids []string) entity.RelationOptions {
			return entity.RelationOptions{Position: &entity.OrderPosition{After: ids[1]}}
		}, entitymanager.ErrRelationNotOrderable},
		{"with properties", "outgoing", func(ids []string) entity.RelationOptions {
			return entity.RelationOptions{
				Position:   &entity.OrderPosition{After: ids[1]},
				Properties: map[string]any{"x": 1},
			}
		}, entitymanager.ErrInvalidOrderPosition},
		{"unknown sibling", "outgoing", func([]string) entity.RelationOptions {
			return entity.RelationOptions{Position: &entity.OrderPosition{After: "STP-999"}}
		}, entitymanager.ErrOrderRefNotSibling},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr, _, _, recipe, ids := moveFixture(t, tt.mode)
			_, err := mgr.UpdateRelation(context.Background(),
				entity.RelationKey{From: recipe, Type: "has-step", To: ids[0]}, tt.opts(ids))
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpdateRelation_PositionMissingEdge(t *testing.T) {
	t.Parallel()
	mgr, _, _, recipe, ids := moveFixture(t, "outgoing")
	err := move(mgr, recipe, "STP-999", entity.OrderPosition{After: ids[0]})
	if !errors.Is(err, entitymanager.ErrRelationNotFound) {
		t.Errorf("err = %v, want ErrRelationNotFound", err)
	}
}

// orderValue reads one edge's stored outgoing order value.
func orderValue(t *testing.T, st store.Store, from, to string) any {
	t.Helper()
	r, err := st.GetRelation(context.Background(), entity.RelationKey{From: from, Type: "has-step", To: to})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return r.Properties[metamodel.OrderPropertyOut]
}

// stepKeys returns the has-step edges from recipe to steps.
func stepKeys(recipe string, steps ...string) []entity.RelationKey {
	keys := make([]entity.RelationKey, 0, len(steps))
	for _, s := range steps {
		keys = append(keys, entity.RelationKey{From: recipe, Type: "has-step", To: s})
	}
	return keys
}

// With Among set, a move plans among those edges only: a step skips the
// edges outside it, the new value comes from visible neighbors, and the
// edges outside it keep their values.
func TestUpdateRelation_PositionAmongVisible(t *testing.T) {
	t.Parallel()
	mgr, st, _, recipe, ids := moveFixture(t, "outgoing")
	hidden := ids[1]
	visible := stepKeys(recipe, ids[0], ids[2], ids[3])
	hiddenBefore := orderValue(t, st, recipe, hidden)

	if err := move(mgr, recipe, ids[0], entity.OrderPosition{Step: 1, Among: visible}); err != nil {
		t.Fatalf("step: %v", err)
	}
	want := []string{hidden, ids[2], ids[0], ids[3]}
	if got := outgoingOrder(t, st, recipe); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if got := orderValue(t, st, recipe, hidden); got != hiddenBefore {
		t.Errorf("hidden edge value = %v, want %v", got, hiddenBefore)
	}
}

// A densify renumbers only the edges in Among.
func TestUpdateRelation_PositionAmongDensifiesVisibleOnly(t *testing.T) {
	t.Parallel()
	mgr, st, _, recipe, ids := moveFixture(t, "outgoing")
	ctx := context.Background()
	for _, id := range ids {
		key := entity.RelationKey{From: recipe, Type: "has-step", To: id}
		r, err := st.GetRelation(ctx, key)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		props := maps.Clone(r.Properties)
		delete(props, metamodel.OrderPropertyOut)
		if _, err := st.UpdateRelation(ctx, key, store.RelationData{Properties: props}); err != nil {
			t.Fatalf("strip order: %v", err)
		}
	}
	shown := outgoingOrder(t, st, recipe)
	hidden := shown[1]
	visible := stepKeys(recipe, shown[0], shown[2], shown[3])
	if err := move(mgr, recipe, shown[3], entity.OrderPosition{After: shown[0], Among: visible}); err != nil {
		t.Fatalf("move: %v", err)
	}
	if v := orderValue(t, st, recipe, hidden); v != nil {
		t.Errorf("hidden edge got order value %v; the densify must not write it", v)
	}
	for i, id := range []string{shown[0], shown[3], shown[2]} {
		if v, _ := entitymanager.FiniteOrder(orderValue(t, st, recipe, id)); v != float64(i+1) {
			t.Errorf("%s value = %v, want %d", id, v, i+1)
		}
	}
}
