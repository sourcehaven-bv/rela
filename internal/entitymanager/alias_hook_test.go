package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// recordingRewriter captures the hook calls so a test can assert the
// entitymanager reported old→new, which is the one moment that link is knowable.
type recordingRewriter struct {
	renames   [][2]string
	deletes   []string
	renameErr error
	deleteErr error
}

func (r *recordingRewriter) EntityRenamed(_ context.Context, oldID, newID string) error {
	r.renames = append(r.renames, [2]string{oldID, newID})
	return r.renameErr
}

func (r *recordingRewriter) EntityDeleted(_ context.Context, id string) error {
	r.deletes = append(r.deletes, id)
	return r.deleteErr
}

func aliasHookMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	return &metamodel.Metamodel{
		Version: "1.0",
		Entities: map[string]metamodel.EntityDef{
			"task": {
				Label: "Task", IDPrefix: "TSK-", DisplayProperty: "title",
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: metamodel.PropertyTypeString},
				},
			},
		},
	}
}

func aliasHookManager(t *testing.T, rw entitymanager.AliasRewriter) (*entitymanager.Manager, *memstore.MemStore) {
	t.Helper()
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate:     entitymanager.AllowAllFieldGate{},
		Store:         st,
		Meta:          aliasHookMeta(t),
		Templater:     nopTemplater{},
		Audit:         audit.Nop{},
		ACL:           acl.NopACL{},
		Transitions:   statemachine.EmptySet(),
		AliasRewriter: rw,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

// TestAliasHook_FiresOnRename is the load-bearing case. The rename choke-point
// is the ONLY place old→new is knowable — a later sweep sees an ordinary update
// — so if the hook does not fire here, a CalDAV client sees a delete plus a
// create and the user's to-do silently duplicates.
func TestAliasHook_FiresOnRename(t *testing.T) {
	rw := &recordingRewriter{}
	mgr, st := aliasHookManager(t, rw)
	seedTask(t, st, "TSK-old", map[string]any{"title": "a task"}, "")

	if _, err := mgr.RenameEntity(t.Context(), "TSK-old", "TSK-new", entity.RenameOptions{}); err != nil {
		t.Fatalf("RenameEntity: %v", err)
	}

	if len(rw.renames) != 1 {
		t.Fatalf("rename hook fired %d times, want 1", len(rw.renames))
	}
	if got := rw.renames[0]; got != [2]string{"TSK-old", "TSK-new"} {
		t.Errorf("hook received %v, want [TSK-old TSK-new]", got)
	}
}

// TestAliasHook_NotFiredOnDryRun: a planned rename changes nothing, so
// rewriting an alias would point it at an id that does not exist.
func TestAliasHook_NotFiredOnDryRun(t *testing.T) {
	rw := &recordingRewriter{}
	mgr, st := aliasHookManager(t, rw)
	seedTask(t, st, "TSK-old", map[string]any{"title": "a task"}, "")

	if _, err := mgr.RenameEntity(t.Context(), "TSK-old", "TSK-new",
		entity.RenameOptions{DryRun: true}); err != nil {
		t.Fatalf("RenameEntity(dry-run): %v", err)
	}
	if len(rw.renames) != 0 {
		t.Errorf("a dry-run rename fired the alias hook: %v", rw.renames)
	}
}

func TestAliasHook_FiresOnDelete(t *testing.T) {
	rw := &recordingRewriter{}
	mgr, st := aliasHookManager(t, rw)
	seedTask(t, st, "TSK-gone", map[string]any{"title": "a task"}, "")

	if _, err := mgr.DeleteEntity(t.Context(), "TSK-gone", false); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	if len(rw.deletes) != 1 || rw.deletes[0] != "TSK-gone" {
		t.Errorf("delete hook received %v, want [TSK-gone]", rw.deletes)
	}
}

// TestAliasHook_ErrorDoesNotFailTheWrite pins the best-effort contract: the
// store change already happened and cannot be unwound, so reporting an error
// for a write that DID land would be worse than logging. The residual risk (an
// orphaned alias) is documented on AliasRewriter.
func TestAliasHook_ErrorDoesNotFailTheWrite(t *testing.T) {
	rw := &recordingRewriter{
		renameErr: errors.New("alias store unavailable"),
		deleteErr: errors.New("alias store unavailable"),
	}
	mgr, st := aliasHookManager(t, rw)
	seedTask(t, st, "TSK-old", map[string]any{"title": "a task"}, "")

	if _, err := mgr.RenameEntity(t.Context(), "TSK-old", "TSK-new", entity.RenameOptions{}); err != nil {
		t.Fatalf("a failing alias hook must not fail the rename: %v", err)
	}
	// And the rename really landed.
	if _, err := st.GetEntity(t.Context(), "TSK-new"); err != nil {
		t.Errorf("renamed entity is missing: %v", err)
	}

	if _, err := mgr.DeleteEntity(t.Context(), "TSK-new", false); err != nil {
		t.Fatalf("a failing alias hook must not fail the delete: %v", err)
	}
}

// TestAliasHook_NilIsANoOp: the capability is optional, so a build without a
// CalDAV alias service must behave exactly as before.
func TestAliasHook_NilIsANoOp(t *testing.T) {
	mgr, st := aliasHookManager(t, nil)
	seedTask(t, st, "TSK-old", map[string]any{"title": "a task"}, "")

	if _, err := mgr.RenameEntity(t.Context(), "TSK-old", "TSK-new", entity.RenameOptions{}); err != nil {
		t.Fatalf("RenameEntity with no rewriter: %v", err)
	}
	if _, err := st.GetEntity(t.Context(), "TSK-new"); err != nil {
		t.Errorf("rename did not land: %v", err)
	}
	if _, err := mgr.DeleteEntity(t.Context(), "TSK-new", false); err != nil {
		t.Fatalf("DeleteEntity with no rewriter: %v", err)
	}
}
