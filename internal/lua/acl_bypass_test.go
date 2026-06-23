package lua

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// recordingMutator is a minimal Mutator that records create_relation calls,
// used as the ElevatedManager so a test can assert rela.bypass_acl routed a
// write through the elevated handle.
type recordingMutator struct {
	relations []string // "from--type-->to" per CreateRelation call
}

func (r *recordingMutator) CreateEntity(context.Context, *entity.Entity, entity.CreateOptions) (*entity.CreateResult, error) {
	return &entity.CreateResult{}, nil
}
func (r *recordingMutator) UpdateEntity(context.Context, *entity.Entity) (*entity.UpdateResult, error) {
	return &entity.UpdateResult{}, nil
}
func (r *recordingMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return &entity.DeleteResult{}, nil
}
func (r *recordingMutator) CreateRelation(_ context.Context, from, relType, to string, _ entity.RelationOptions) (*entity.Relation, error) {
	r.relations = append(r.relations, from+"--"+relType+"-->"+to)
	return entity.NewRelation(from, relType, to), nil
}
func (r *recordingMutator) DeleteRelation(context.Context, string, string, string) error { return nil }

// writerWithElevated builds a writer runtime whose WriteDeps carry an
// ElevatedManager (so rela.bypass_acl is registered).
func writerWithElevated(t *testing.T, elevated Mutator) *Runtime {
	t.Helper()
	ws := newMockWorkspace(t)
	deps := ws.services("/tmp")
	deps.ElevatedManager = elevated
	var buf bytes.Buffer
	return NewWriter(deps, &buf)
}

// TestBypassACL_RoutesThroughElevatedHandle pins the happy path: inside
// rela.bypass_acl(fn), admin.create_relation routes to the elevated Mutator.
func TestBypassACL_RoutesThroughElevatedHandle(t *testing.T) {
	t.Parallel()
	em := &recordingMutator{}
	r := writerWithElevated(t, em)
	defer r.Close()

	script := `
		rela.bypass_acl(function(admin)
			admin.create_relation("alice", "created-by", "TKT-1")
		end)
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("bypass_acl script: %v", err)
	}
	if len(em.relations) != 1 || em.relations[0] != "alice--created-by-->TKT-1" {
		t.Errorf("elevated CreateRelation calls = %v, want [alice--created-by-->TKT-1]", em.relations)
	}
}

// TestBypassACL_AbsentWithoutElevatedManager pins that rela.bypass_acl is NOT
// registered when no elevated handle was wired — a normal (non-allow_acl_bypass)
// runtime cannot elevate.
func TestBypassACL_AbsentWithoutElevatedManager(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	var buf bytes.Buffer
	r := NewWriter(ws.services("/tmp"), &buf) // no ElevatedManager
	defer r.Close()

	if err := r.RunString(`if rela.bypass_acl ~= nil then error("bypass_acl present") end`); err != nil {
		t.Errorf("rela.bypass_acl should be absent without an elevated handle: %v", err)
	}
}

// TestBypassACL_HandleInvalidatedAfterClosure pins the escaped-handle defense:
// an admin handle captured into a global and used AFTER the closure returns
// raises (the handle is invalidated), so elevation can't leak past the closure's
// dynamic extent.
func TestBypassACL_HandleInvalidatedAfterClosure(t *testing.T) {
	t.Parallel()
	em := &recordingMutator{}
	r := writerWithElevated(t, em)
	defer r.Close()

	// Capture admin into a global inside the closure, then use it afterwards.
	script := `
		local stashed
		rela.bypass_acl(function(admin) stashed = admin end)
		stashed.create_relation("alice", "created-by", "TKT-1")
	`
	err := r.RunString(script)
	if err == nil {
		t.Fatal("using a captured admin handle after the closure should raise, but it succeeded")
	}
	if !strings.Contains(err.Error(), "invalidated") {
		t.Errorf("error = %v, want it to mention the handle is invalidated", err)
	}
	if len(em.relations) != 0 {
		t.Errorf("elevated write happened via an escaped handle: %v — must not", em.relations)
	}
}
