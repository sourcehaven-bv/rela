package entitymanager_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
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

func renameGuardManager(t *testing.T, g entitymanager.RenameGuard) (
	*entitymanager.Manager, *memstore.MemStore, *audit.Memory,
) {
	t.Helper()
	st := memstore.New()
	sink := audit.NewMemory()
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate:   entitymanager.AllowAllFieldGate{},
		Store:       st,
		Meta:        aliasHookMeta(t),
		Templater:   nopTemplater{},
		Audit:       sink,
		ACL:         acl.NopACL{},
		Transitions: transitions.NoTransitions(),
		AnchorGuard: entitymanager.NopAnchorGuard{},
		RenameGuard: g,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, st, sink
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
			mgr, st, sink := renameGuardManager(t, g)
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
			// Only the real rename is a refused write; a dry run is not.
			want := 1
			if tc.dryRun {
				want = 0
			}
			if got := len(sink.Records()); got != want {
				t.Fatalf("audit records = %d, want %d denied write(s)", got, want)
			}
		})
	}
}

// TestRenameGuard_Conversion pins that a conversion, which gives the entity
// a new id and type, asks the rename guard: a refusal stops the preview and
// the conversion before anything is written, the conversion asks inside the
// store transaction, and the refusal is audited as a denied write.
func TestRenameGuard_Conversion(t *testing.T) {
	g := &blockingRenameGuard{blocked: map[string]bool{"NOTE-1": true}}
	f := newConversionFixture(t, conversionOptions{renameGuard: g})
	f.seed(t, "NOTE-1", "note", map[string]any{"title": "Quick note"})
	req := entitymanager.ConversionRequest{Definition: "note-to-task", SourceID: "NOTE-1"}

	if _, err := f.conv.Preview(t.Context(), req); !errors.Is(err, entitymanager.ErrRenameBlocked) {
		t.Fatalf("Preview error = %v, want ErrRenameBlocked", err)
	}
	if _, err := f.conv.Convert(t.Context(), req); !errors.Is(err, entitymanager.ErrRenameBlocked) {
		t.Fatalf("Convert error = %v, want ErrRenameBlocked", err)
	}
	if want := []bool{false, true}; !slices.Equal(g.inTx, want) {
		t.Fatalf("guard calls in a transaction = %v, want %v", g.inTx, want)
	}
	if got := mustGet(t, f.st, "NOTE-1"); got.Type != "note" {
		t.Fatalf("NOTE-1 type = %q, want the refused conversion not written", got.Type)
	}
	var denied int
	for _, rec := range f.audit.Records() {
		if rec.Op == audit.OpDeniedWrite && rec.Subject != nil && rec.Subject.ID == "NOTE-1" &&
			strings.Contains(rec.Summary, entitymanager.ErrRenameBlocked.Error()) {

			denied++
		}
	}
	if denied != 1 {
		t.Fatalf("denied-write records for NOTE-1 = %d, want 1", denied)
	}
}
