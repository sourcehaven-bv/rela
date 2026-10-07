package analysis_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer/tracertest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

func TestCheckOwning(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"task": {Label: "Task"}},
		Relations: map[string]metamodel.RelationDef{
			"subtask": {Label: "subtask", From: []string{"task"}, To: []string{"task"}, Owning: true},
			"relates": {Label: "relates", From: []string{"task"}, To: []string{"task"}},
		},
	}
	link := func(s store.Store, from, typ, to string) {
		if _, err := s.CreateRelation(context.Background(),
			entity.RelationKey{From: from, Type: typ, To: to}, &store.RelationData{}); err != nil {
			panic(err)
		}
	}
	svc := newServiceWith(t, meta, func(s store.Store) {
		for _, id := range []string{"T1", "T2", "T3", "T4", "T5", "T6"} {
			addEntity(s, id, "task", nil)
		}
		link(s, "T1", "subtask", "T2") // regular
		link(s, "T3", "subtask", "T4") // T4 has two owners
		link(s, "T5", "subtask", "T4")
		link(s, "T2", "subtask", "T6") // T2 is owned and owns
		link(s, "T6", "subtask", "T6") // self
		link(s, "T1", "relates", "T3") // not owning: ignored
	})

	issues, err := svc.CheckOwning(context.Background(), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, iss := range issues {
		got = append(got, fmt.Sprintf("%s:%s:%v", iss.EntityID, iss.Kind, iss.Owners))
	}
	// T6 owns only itself, which the self finding already reports.
	want := []string{"T6:self:[T6]", "T2:nested:[T1]", "T4:multiple-owners:[T3 T5]"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("issues = %v, want %v", got, want)
	}
}

// A failing edge read aborts the check rather than reporting clean data, the
// same policy as CheckCardinality (TKT-RNBLAC).
func TestCheckOwning_StoreErrorFailsLoudly(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"task": {Label: "Task"}},
		Relations: map[string]metamodel.RelationDef{
			"subtask": {Label: "subtask", From: []string{"task"}, To: []string{"task"}, Owning: true},
		},
	}
	readErr := errors.New("backend down")
	broken := &failingCountStore{Store: memstore.New(), err: readErr}
	tr := tracertest.Must(broken, store.TrivialScope())
	svc, err := analysis.New(analysis.Deps{Store: broken, Meta: meta, Tracer: tr,
		LuaReadDeps: lua.ReadDeps{
			VisibleReader: visibility.Unrestricted(broken).WithWorld(visibility.WorldOf(store.TrivialScope())),
			Tracer:        tr, Meta: meta, World: store.TrivialScope(),
		}})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	if issues, err := svc.CheckOwning(context.Background(), analysis.Options{}); !errors.Is(err, readErr) {
		t.Fatalf("CheckOwning = %v, %v; want the store error", issues, err)
	}
}
