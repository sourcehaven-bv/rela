package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// blockingRenameGuard refuses the ids in blocked and records whether each
// call ran inside a store transaction.
type blockingRenameGuard struct {
	blocked map[string]bool
	inTx    []bool
}

func (g *blockingRenameGuard) CheckRename(ctx context.Context, id string) error {
	g.inTx = append(g.inTx, store.InTx(ctx))
	if g.blocked[id] {
		return errors.Join(entitymanager.ErrRenameBlocked, errors.New(id+" is held"))
	}
	return nil
}

func renameGuardManager(t *testing.T, g entitymanager.RenameGuard) (*entitymanager.Manager, *memstore.MemStore) {
	t.Helper()
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate:   entitymanager.AllowAllFieldGate{},
		Store:       st,
		Meta:        aliasHookMeta(t),
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		AnchorGuard: entitymanager.NopAnchorGuard{},
		RenameGuard: g,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st
}

// TestRenameGuard pins that a guard refusal stops a rename and a dry run
// before anything is written, that the real rename asks inside the store
// transaction, and that an id the guard admits renames.
func TestRenameGuard(t *testing.T) {
	tests := []struct {
		name     string
		blocked  bool
		dryRun   bool
		wantInTx bool
	}{
		{name: "refused", blocked: true, wantInTx: true},
		{name: "refused dry run", blocked: true, dryRun: true},
		{name: "admitted", wantInTx: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := &blockingRenameGuard{blocked: map[string]bool{"TSK-old": tc.blocked}}
			mgr, st := renameGuardManager(t, g)
			seedTask(t, st, "TSK-old", map[string]any{"title": "a task"}, "")

			_, err := mgr.RenameEntity(t.Context(), "TSK-old", "TSK-new", entity.RenameOptions{DryRun: tc.dryRun})
			if len(g.inTx) != 1 || g.inTx[0] != tc.wantInTx {
				t.Fatalf("guard calls in a transaction = %v, want [%v]", g.inTx, tc.wantInTx)
			}
			if !tc.blocked {
				if err != nil {
					t.Fatalf("RenameEntity: %v", err)
				}
				mustGet(t, st, "TSK-new")
				return
			}
			if !errors.Is(err, entitymanager.ErrRenameBlocked) {
				t.Fatalf("RenameEntity error = %v, want ErrRenameBlocked", err)
			}
			mustGet(t, st, "TSK-old")
		})
	}
}
