package automation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func traversalMeta() *metamodel.Metamodel {
	m := datePropMeta()
	m.Entities["person"] = metamodel.EntityDef{Properties: map[string]metamodel.PropertyDef{
		"name": {Type: metamodel.PropertyTypeString},
	}}
	m.Relations = map[string]metamodel.RelationDef{"owned-by": {From: []string{"taak"}, To: []string{"person"}}}
	return m
}

// stubBinder answers every traversal with answer, or fails with err.
type stubBinder struct {
	answer bool
	err    error
	calls  int
	ids    []string
}

func (s *stubBinder) Bind(
	_ context.Context, _ string, ids []string, _ ...*predicate.Program,
) (func(string) predicate.TraversalFunc, error) {
	s.calls++
	s.ids = ids
	if s.err != nil {
		return nil, s.err
	}
	return func(string) predicate.TraversalFunc {
		return func(predicate.Value, predicate.TraversalSpec) (bool, error) { return s.answer, nil }
	}, nil
}

func TestProcess_Traversal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cond     string
		binder   *stubBinder
		wantFire bool
		wantWarn bool
	}{
		{"related holds", "related(entity, 'owned-by')", &stubBinder{answer: true}, true, false},
		{"related fails", "related(entity, 'owned-by')", &stubBinder{answer: false}, false, false},
		{"error does not fire, even negated", "not related(entity, 'owned-by')",
			&stubBinder{err: errors.New("boom")}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, err := NewEngineFromMetamodel(traversalMeta(),
				[]metamodel.AutomationDef{condAutomation(tc.cond)}, WithTraversals(tc.binder))
			if err != nil {
				t.Fatal(err)
			}
			ent := buildEntity(testutil.Entity("taak").ID("taak-1"))
			res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: ent})
			if fired := res.PropertiesSet["status"] == "due-soon"; fired != tc.wantFire {
				t.Fatalf("fired = %v, want %v", fired, tc.wantFire)
			}
			if warned := len(res.Warnings) > 0; warned != tc.wantWarn {
				t.Fatalf("warnings = %v, want some: %v", res.Warnings, tc.wantWarn)
			}
			if tc.binder.calls != 1 || len(tc.binder.ids) != 1 || tc.binder.ids[0] != "taak-1" {
				t.Fatalf("Bind calls/ids = %d/%v, want one call for the triggering entity", tc.binder.calls, tc.binder.ids)
			}
		})
	}
}

func TestProcess_NoTraversalNoBind(t *testing.T) {
	b := &stubBinder{}
	engine, err := NewEngineFromMetamodel(traversalMeta(),
		[]metamodel.AutomationDef{condAutomation("entity.status == nil")}, WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: buildEntity(testutil.Entity("taak"))})
	if b.calls != 0 {
		t.Fatalf("Bind calls = %d, want 0 for a condition without related()", b.calls)
	}
}

func TestNewEngine_RelatedRefusedAtLoad(t *testing.T) {
	for _, tc := range []struct {
		name string
		cond string
		opts []Option
		want string
	}{
		{"no binder", "related(entity, 'owned-by')", nil, "no store"},
		{"unknown relation", "related(entity, 'nope')", []Option{WithTraversals(&stubBinder{})}, "nope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEngineFromMetamodel(traversalMeta(), []metamodel.AutomationDef{condAutomation(tc.cond)}, tc.opts...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
}

// A named face's edges are not the default face's, so the store's answer
// would be about another entity: refuse instead of firing on it.
func TestProcess_TraversalOnNamedFaceDoesNotFire(t *testing.T) {
	b := &stubBinder{answer: false}
	engine, err := NewEngineFromMetamodel(traversalMeta(),
		[]metamodel.AutomationDef{condAutomation("not related(entity, 'owned-by')")}, WithTraversals(b))
	if err != nil {
		t.Fatal(err)
	}
	ent := buildEntity(testutil.Entity("taak").ID("taak-1"))
	ent.Face = "draft"
	res := engine.Process(context.Background(), Event{Type: EventEntityCreated, Entity: ent})
	if res.PropertiesSet["status"] == "due-soon" || len(res.Warnings) == 0 {
		t.Fatalf("fired on a named face (set=%v warnings=%v)", res.PropertiesSet, res.Warnings)
	}
}
