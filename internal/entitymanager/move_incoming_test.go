package entitymanager_test

import (
	"context"
	"errors"
	"maps"
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

// denyUpdateFromACL refuses an update of any relation from one source, and
// allows every other write.
type denyUpdateFromACL struct {
	acl.NopACL
	from string
}

func (a denyUpdateFromACL) AuthorizeWrite(_ context.Context, req acl.WriteRequest) acl.Decision {
	if s, ok := req.Subject.(acl.RelationSubject); ok && req.Op == acl.OpUpdate && s.FromID == a.from {
		return acl.Decision{Allow: false, RuleKind: "test", Reason: "no update from " + a.from}
	}
	return acl.Decision{Allow: true, RuleKind: "test"}
}

// incomingFixture creates one step that four recipes link to over an
// incoming-orderable relation, and returns the recipe ids in creation
// order. a is the manager's ACL.
func incomingFixture(t *testing.T, a acl.ACL) (
	mgr *entitymanager.Manager, st store.Store, step string, recipes []string,
) {
	t.Helper()
	st = memstore.New()
	var err error
	mgr, err = entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        orderableMetamodel(t, "incoming"),
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         a,
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	step = mkStep(t, mgr, "Shared").ID
	for _, title := range []string{"R1", "R2", "R3", "R4"} {
		r := mkRecipe(t, mgr, title)
		if _, err := mgr.CreateRelation(ctx, entity.RelationKey{From: r.ID, Type: "has-step", To: step},
			entity.RelationOptions{}); err != nil {
			t.Fatalf("create relation: %v", err)
		}
		recipes = append(recipes, r.ID)
	}
	return mgr, st, step, recipes
}

// incomingOrder reads the recipes linking to step in its incoming order.
func incomingOrder(t *testing.T, st store.Store, step string) []string {
	t.Helper()
	var rels []entity.Relation
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{To: step, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list relations: %v", err)
		}
		rels = append(rels, *r)
	}
	var ids []string
	for _, r := range entitymanager.SortRelations(rels, metamodel.OrderPropertyIn) {
		ids = append(ids, r.From)
	}
	return ids
}

func moveIn(mgr *entitymanager.Manager, from, step string, pos entity.OrderPosition) error {
	pos.Incoming = true
	_, err := mgr.UpdateRelation(context.Background(), entity.RelationKey{From: from, Type: "has-step", To: step},
		entity.RelationOptions{Position: &pos})
	return err
}

func TestUpdateRelation_IncomingPositionMoves(t *testing.T) {
	t.Parallel()
	mgr, st, step, rs := incomingFixture(t, acl.NopACL{})
	if err := moveIn(mgr, rs[3], step, entity.OrderPosition{Before: rs[1]}); err != nil {
		t.Fatalf("move: %v", err)
	}
	want := []string{rs[0], rs[3], rs[1], rs[2]}
	if got := incomingOrder(t, st, step); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if err := moveIn(mgr, rs[0], step, entity.OrderPosition{Step: 1}); err != nil {
		t.Fatalf("step: %v", err)
	}
	want = []string{rs[3], rs[0], rs[1], rs[2]}
	if got := incomingOrder(t, st, step); !slices.Equal(got, want) {
		t.Errorf("after step: order = %v, want %v", got, want)
	}
}

func TestUpdateRelation_IncomingPositionErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode string
		pos  entity.OrderPosition
		want error
	}{
		{"outgoing only", "outgoing", entity.OrderPosition{Step: 1}, entitymanager.ErrRelationNotOrderable},
		{"unknown sibling", "incoming", entity.OrderPosition{After: "REC-999"}, entitymanager.ErrOrderRefNotSibling},
		{"bad address", "incoming", entity.OrderPosition{After: "REC-001@"}, entitymanager.ErrInvalidOrderPosition},
		{"itself", "incoming", entity.OrderPosition{After: "REC-001"}, entitymanager.ErrInvalidOrderPosition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr, _ := newOrderableManager(t, tt.mode)
			ctx := context.Background()
			step := mkStep(t, mgr, "S")
			r1, r2 := mkRecipe(t, mgr, "R1"), mkRecipe(t, mgr, "R2")
			for _, r := range []string{r1.ID, r2.ID} {
				if _, err := mgr.CreateRelation(ctx, entity.RelationKey{From: r, Type: "has-step", To: step.ID},
					entity.RelationOptions{}); err != nil {
					t.Fatalf("create relation: %v", err)
				}
			}
			if r1.ID != "REC-001" {
				t.Fatalf("fixture: first recipe is %s, want REC-001", r1.ID)
			}
			if err := moveIn(mgr, r1.ID, step.ID, tt.pos); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// A densify on the incoming side rewrites edges of other sources, so the
// caller must be allowed to update every one of them. One denial fails the
// move before anything is written.
func TestUpdateRelation_IncomingPositionAuthorizesSiblings(t *testing.T) {
	t.Parallel()
	_, st, step, rs := incomingFixture(t, acl.NopACL{})
	stripIncomingOrder(t, st, step)
	before := snapshotIncoming(t, st, step)

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: orderableMetamodel(t, "incoming"), Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL:         denyUpdateFromACL{from: rs[2]},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	shown := incomingOrder(t, st, step)
	err = moveIn(mgr, shown[3], step, entity.OrderPosition{After: shown[0]})
	var denied *acl.ForbiddenError
	if !errors.As(err, &denied) {
		t.Fatalf("err = %v, want an ACL denial", err)
	}
	if got := snapshotIncoming(t, st, step); !maps.Equal(got, before) {
		t.Errorf("a denied move wrote order values: %v, want %v", got, before)
	}

	// With the denied source outside Among, the move plans without it and
	// leaves its edge alone.
	among := []entity.RelationKey{}
	for _, r := range shown {
		if r != rs[2] {
			among = append(among, entity.RelationKey{From: r, Type: "has-step", To: step})
		}
	}
	if err := moveIn(mgr, shown[3], step, entity.OrderPosition{After: shown[0], Among: among}); err != nil {
		t.Fatalf("move among the allowed: %v", err)
	}
	if got := snapshotIncoming(t, st, step)[rs[2]]; got != nil {
		t.Errorf("edge from the denied source got %v; it must not be written", got)
	}
}

// Two faces of one source are two edges of one incoming list. A bare ref
// names the source's first place; `id@face` names that tail.
func TestPlaceOrder_IncomingRefAddresses(t *testing.T) {
	t.Parallel()
	edge := func(from, face string, v float64) entity.Relation {
		return entity.Relation{From: from, FromFace: entity.Face(face), Type: "t", To: "T",
			Properties: map[string]any{metamodel.OrderPropertyIn: v}}
	}
	sibs := []entity.Relation{edge("A", "", 1), edge("B", "draft", 2), edge("C", "", 3), edge("B", "published", 4)}
	moved := sibs[2].Identity()
	tests := []struct {
		name string
		pos  entity.OrderPosition
		want float64
	}{
		{"bare ref: first place", entity.OrderPosition{Before: "B"}, 1.5},
		{"named face", entity.OrderPosition{After: "B@published"}, 5},
		{"named other face", entity.OrderPosition{Before: "B@draft"}, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pos := tt.pos
			pos.Incoming = true
			plan, err := entitymanager.PlaceOrder(sibs, moved, pos, metamodel.OrderPropertyIn)
			if err != nil {
				t.Fatalf("PlaceOrder: %v", err)
			}
			if got := plan[moved]; got != tt.want {
				t.Errorf("value = %v, want %v (plan %v)", got, tt.want, plan)
			}
		})
	}
	if _, err := entitymanager.PlaceOrder(sibs, moved, entity.OrderPosition{Before: "B@gone", Incoming: true},
		metamodel.OrderPropertyIn); !errors.Is(err, entitymanager.ErrOrderRefNotSibling) {
		t.Errorf("unknown face: err = %v, want ErrOrderRefNotSibling", err)
	}
}

func stripIncomingOrder(t *testing.T, st store.Store, step string) {
	t.Helper()
	ctx := context.Background()
	for r, err := range st.ListRelations(ctx, store.RelationQuery{To: step, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		props := maps.Clone(r.Properties)
		delete(props, metamodel.OrderPropertyIn)
		if _, err := st.UpdateRelation(ctx, r.Identity(), store.RelationData{Properties: props}); err != nil {
			t.Fatalf("strip order: %v", err)
		}
	}
}

// snapshotIncoming maps each source to its edge's stored incoming order.
func snapshotIncoming(t *testing.T, st store.Store, step string) map[string]any {
	t.Helper()
	out := map[string]any{}
	for r, err := range st.ListRelations(context.Background(), store.RelationQuery{To: step, Type: "has-step"}) {
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out[r.From] = r.Properties[metamodel.OrderPropertyIn]
	}
	return out
}
