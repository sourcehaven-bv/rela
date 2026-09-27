package statemachine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// stubBinder answers every traversal with answer[rowID] and counts binds.
type stubBinder struct {
	answer map[string]bool
	err    error
	binds  int
}

func (b *stubBinder) Bind(
	_ context.Context, _ string, _ []string, _ ...*predicate.Program,
) (func(string) predicate.TraversalFunc, error) {
	b.binds++
	if b.err != nil {
		return nil, b.err
	}
	return func(rowID string) predicate.TraversalFunc {
		return func(predicate.Value, predicate.TraversalSpec) (bool, error) { return b.answer[rowID], nil }
	}, nil
}

// relatedMeta shares one machine between ticket and task. Only ticket has the
// owned-by relation unless both is set.
func relatedMeta(both bool) *metamodel.Metamodel {
	from := []string{"ticket"}
	if both {
		from = append(from, "task")
	}
	status := metamodel.PropertyDef{Type: "work-status"}
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"work-status": {
				Values:  []string{"open", "done", "dropped"},
				Initial: "open",
				Transitions: []metamodel.TransitionDef{
					{From: "open", To: "done", When: "related(entity, 'owned-by')"},
					{From: "open", To: "dropped", When: "not related(entity, 'owned-by')"},
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": status}},
			"task":   {Properties: map[string]metamodel.PropertyDef{"status": status}},
			"person": {},
		},
		Relations: map[string]metamodel.RelationDef{
			"owned-by": {From: from, To: []string{"person"}},
		},
	}
}

// A machine shared by two types is checked per (type, property): the path
// resolves from ticket but not from task.
func TestCompile_RelatedValidatedPerType(t *testing.T) {
	_, err := Compile(relatedMeta(false), WithTraversals(&stubBinder{}))
	if err == nil || !strings.Contains(err.Error(), `entity "task"`) || strings.Contains(err.Error(), `entity "ticket"`) {
		t.Fatalf("err = %v, want a problem for task only", err)
	}
	if _, err := Compile(relatedMeta(true), WithTraversals(&stubBinder{})); err != nil {
		t.Fatalf("both types have the relation: %v", err)
	}
}

func TestCompile_RelatedWithoutBinderRefused(t *testing.T) {
	if _, err := Compile(relatedMeta(true)); err == nil {
		t.Fatal("want a compile problem when no binder answers related()")
	}
}

func TestEnforceUpdate_Related(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		owned   bool
		to      string
		wantErr error
	}{
		{"owned can close", true, "done", nil},
		{"unowned cannot close", false, "done", ErrPreconditionFailed},
		{"unowned can drop", false, "dropped", nil},
		{"owned cannot drop", true, "dropped", ErrPreconditionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &stubBinder{answer: map[string]bool{"T-1": tc.owned}}
			set, err := Compile(relatedMeta(true), WithTraversals(b))
			if err != nil {
				t.Fatal(err)
			}
			err = set.EnforceUpdate(ctx, ent("T-1", "ticket", "open"), ent("T-1", "ticket", tc.to), inertGuard{}, nil)
			if !errors.Is(err, tc.wantErr) && (tc.wantErr != nil || err != nil) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if b.binds != 1 {
				t.Fatalf("binds = %d, want 1", b.binds)
			}
		})
	}
}

// A failed bind fails the precondition; it must not read as "not related",
// which would pass the negated edge.
func TestEnforceUpdate_RelatedBindErrorFailsClosed(t *testing.T) {
	b := &stubBinder{err: errors.New("store down")}
	set, err := Compile(relatedMeta(true), WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	err = set.EnforceUpdate(context.Background(), ent("T-1", "ticket", "open"), ent("T-1", "ticket", "dropped"),
		inertGuard{}, nil)
	if !errors.Is(err, ErrPreconditionFailed) || !strings.Contains(err.Error(), "store down") {
		t.Fatalf("err = %v, want a precondition failure carrying the bind error", err)
	}
}

func TestEnforceUpdate_RelatedOnNamedFaceFailsClosed(t *testing.T) {
	b := &stubBinder{}
	set, err := Compile(relatedMeta(true), WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	old, nw := ent("T-1", "ticket", "open"), ent("T-1", "ticket", "dropped")
	old.Face, nw.Face = entity.Face("en"), entity.Face("en")
	err = set.EnforceUpdate(context.Background(), old, nw, inertGuard{}, nil)
	if !errors.Is(err, ErrPreconditionFailed) || b.binds != 0 {
		t.Fatalf("err = %v, binds = %d; want a precondition failure and no bind", err, b.binds)
	}
}

// Performable answers every out-edge with one bind, and agrees with
// enforcement.
func TestPerformable_RelatedBindsOnce(t *testing.T) {
	b := &stubBinder{answer: map[string]bool{"T-1": true}}
	set, err := Compile(relatedMeta(true), WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	vs := set.Performable(context.Background(), ent("T-1", "ticket", "open"), "status", inertGuard{}, nil)
	if b.binds != 1 {
		t.Fatalf("binds = %d, want 1", b.binds)
	}
	got := map[string]bool{}
	for _, v := range vs {
		got[v.To] = v.Allowed
	}
	if !got["done"] || got["dropped"] || len(got) != 2 {
		t.Fatalf("verdicts = %v, want done allowed and dropped refused", got)
	}
}

// A machine without related() never binds.
func TestEnforceUpdate_NoRelatedNoBind(t *testing.T) {
	b := &stubBinder{}
	set, err := Compile(snapshotMeta(), WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	_ = set.EnforceUpdate(context.Background(), ent("S-1", "snapshot", "in-review"),
		ent("S-1", "snapshot", "approved"), inertGuard{}, nil)
	if b.binds != 0 {
		t.Fatalf("binds = %d, want 0", b.binds)
	}
}
