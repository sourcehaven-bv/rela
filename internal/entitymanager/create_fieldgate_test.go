package entitymanager_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestFieldGated_DefaultHandleSkipsGate: only the FieldGated handle asks
// the gate; the default one is for callers that check fields themselves or
// act for the operator.
func TestFieldGated_DefaultHandleSkipsGate(t *testing.T) {
	t.Parallel()
	gate := &recordingGate{denied: map[string]bool{"salary": true}}
	meta, err := metamodel.Parse([]byte(patchMetamodelYAML))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: gate,
	})
	if err != nil {
		t.Fatal(err)
	}
	seedTask(t, st, "TASK-1", map[string]any{"title": "T"}, "")
	if _, err := mgr.PatchEntity(context.Background(), "TASK-1", entity.Patch{
		Properties: map[string]any{"salary": "1"},
	}); err != nil {
		t.Fatalf("default handle: %v", err)
	}
	if gate.calls != 0 {
		t.Errorf("gate consulted %d times on the default handle, want 0", gate.calls)
	}
}

// TestCreateEntity_FieldGate: create checks the caller's properties like
// PatchEntity does, so a field the caller may not write cannot be set by
// creating the entity with it (TKT-0XL8MF, BUG-Q60V).
func TestCreateEntity_FieldGate(t *testing.T) {
	t.Parallel()

	t.Run("denied property refuses the create", func(t *testing.T) {
		t.Parallel()
		gate := &recordingGate{denied: map[string]bool{"salary": true}}
		mgr, st := newPatchManager(t, gate)
		e := &entity.Entity{Type: "task", Properties: map[string]any{"title": "T", "salary": "999"}}
		if _, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{ID: "TASK-1"}); err == nil {
			t.Fatal("expected the field gate to refuse the create")
		}
		if n, _ := st.CountEntities(context.Background(), store.EntityQuery{}); n != 0 {
			t.Errorf("store holds %d entities, want 0 after a refused create", n)
		}
	})

	t.Run("gate sees the caller's properties only", func(t *testing.T) {
		t.Parallel()
		gate := &recordingGate{denied: map[string]bool{"salary": true}}
		mgr, _ := newPatchManager(t, gate)
		e := &entity.Entity{Type: "task", Properties: map[string]any{"title": "T"}}
		if _, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{ID: "TASK-1"}); err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}
		if gate.calls != 1 || len(gate.lastSet) != 1 || gate.lastSet["title"] != "T" {
			t.Errorf("gate calls=%d set=%v, want one call with just title", gate.calls, gate.lastSet)
		}
	})

	t.Run("elevation skips the gate", func(t *testing.T) {
		t.Parallel()
		gate := &recordingGate{denied: map[string]bool{"salary": true}}
		mgr, _ := newPatchManager(t, gate)
		elevated, ok := mgr.Elevated().(interface {
			CreateEntity(context.Context, *entity.Entity, entity.CreateOptions) (*entity.CreateResult, error)
		})
		if !ok {
			t.Fatalf("elevated mutator (%T) does not expose CreateEntity", mgr.Elevated())
		}
		e := &entity.Entity{Type: "task", Properties: map[string]any{"title": "T", "salary": "999"}}
		if _, err := elevated.CreateEntity(context.Background(), e, entity.CreateOptions{ID: "TASK-1"}); err != nil {
			t.Fatalf("elevated CreateEntity: %v", err)
		}
		if gate.calls != 0 {
			t.Errorf("field gate consulted %d times under elevation, want 0", gate.calls)
		}
	})
}

// TestFieldGate_DenialIsAudited: a field refusal leaves a denied-write
// record, like a row-level refusal does.
func TestFieldGate_DenialIsAudited(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(patchMetamodelYAML))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: sink, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   &recordingGate{denied: map[string]bool{"salary": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	seedTask(t, st, "TASK-1", map[string]any{"title": "T"}, "")
	if _, err := entitymanager.FieldGated(mgr).PatchEntity(context.Background(), "TASK-1", entity.Patch{
		Properties: map[string]any{"salary": "1"},
	}); err == nil {
		t.Fatal("expected a refusal")
	}
	var denied []audit.Record
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			denied = append(denied, r)
		}
	}
	ok := len(denied) == 1 && denied[0].Subject != nil && denied[0].Subject.ID == "TASK-1" &&
		strings.Contains(denied[0].Summary, "salary")
	if !ok {
		t.Errorf("denied-write records = %+v, want one naming TASK-1 and salary", denied)
	}
}
