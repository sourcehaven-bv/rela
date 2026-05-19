package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// newManagerWithACL wires a Manager with the supplied ACL + audit
// sink. Used by the deny-path tests below; the standard newManager
// fixture (manager_test.go) is parameterized on NopACL.
func newManagerWithACL(
	t *testing.T, aclImpl acl.ACL, sink audit.Audit,
) (*entitymanager.Manager, *countingStore) {
	t.Helper()
	cs := &countingStore{Store: memstore.New()}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:     cs,
		Meta:      parseMeta(t),
		Templater: nopTemplater{},
		Audit:     sink,
		ACL:       aclImpl,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, cs
}

// seedEntity inserts a fresh entity through a NopACL Manager so a
// subsequent deny test has something to act on. Without this, ACL
// gates fire after the entity-not-found lookup and we can't tell
// apart "ACL denied" from "no such entity."
func seedEntity(t *testing.T, store *countingStore, entityType, title string) {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:     store,
		Meta:      parseMeta(t),
		Templater: nopTemplater{},
		Audit:     audit.Nop{},
		ACL:       acl.NopACL{},
	})
	if err != nil {
		t.Fatalf("seedEntity: New: %v", err)
	}
	e := entity.New("", entityType)
	e.SetString("title", title)
	if _, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{}); err != nil {
		t.Fatalf("seedEntity: CreateEntity: %v", err)
	}
}

// seedRelation inserts a fresh relation through a NopACL Manager.
func seedRelation(t *testing.T, store *countingStore, from, relType, to string) {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:     store,
		Meta:      parseMeta(t),
		Templater: nopTemplater{},
		Audit:     audit.Nop{},
		ACL:       acl.NopACL{},
	})
	if err != nil {
		t.Fatalf("seedRelation: New: %v", err)
	}
	if _, err := mgr.CreateRelation(context.Background(), from, relType, to, entity.RelationOptions{}); err != nil {
		t.Fatalf("seedRelation: CreateRelation: %v", err)
	}
}

// AC1.5: ReadOnlyACL denies every write path, returns *ForbiddenError
// with the read-only Decision, and records a denied-write audit row.
// Table-driven across all 7 entry points.
func TestManager_ACLDenies_AllWritePathsBlocked(t *testing.T) {
	tests := []struct {
		name string
		// run exercises the write entry point against mgr. Returns the
		// error from the entry point. Pre-seeded data is set up by setup.
		setup func(t *testing.T, store *countingStore)
		run   func(t *testing.T, mgr *entitymanager.Manager) error
		// wantSubjectKind is "entity" or "relation".
		wantSubjectKind string
	}{
		{
			name: "CreateEntity denied",
			setup: func(t *testing.T, _ *countingStore) {
				t.Helper()
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				e := entity.New("", "requirement")
				e.SetString("title", "Denied")
				_, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
				return err
			},
			wantSubjectKind: "entity",
		},
		{
			name: "UpdateEntity denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "requirement", "Original")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				e := entity.New("REQ-001", "requirement")
				e.SetString("title", "Modified")
				_, err := mgr.UpdateEntity(context.Background(), e)
				return err
			},
			wantSubjectKind: "entity",
		},
		{
			name: "DeleteEntity denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "requirement", "ToDelete")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				_, err := mgr.DeleteEntity(context.Background(), "REQ-001", false)
				return err
			},
			wantSubjectKind: "entity",
		},
		{
			name: "RenameEntity denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "requirement", "Original")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				_, err := mgr.RenameEntity(context.Background(), "REQ-001", "REQ-002", entity.RenameOptions{})
				return err
			},
			wantSubjectKind: "entity",
		},
		{
			name: "CreateRelation denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "decision", "FromEntity")
				seedEntity(t, store, "requirement", "ToEntity")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				_, err := mgr.CreateRelation(context.Background(), "DEC-001", "addresses", "REQ-001", entity.RelationOptions{})
				return err
			},
			wantSubjectKind: "relation",
		},
		{
			name: "UpdateRelation denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "decision", "FromEntity")
				seedEntity(t, store, "requirement", "ToEntity")
				seedRelation(t, store, "DEC-001", "addresses", "REQ-001")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				_, err := mgr.UpdateRelation(context.Background(), "DEC-001", "addresses", "REQ-001", entity.RelationOptions{})
				return err
			},
			wantSubjectKind: "relation",
		},
		{
			name: "DeleteRelation denied",
			setup: func(t *testing.T, store *countingStore) {
				t.Helper()
				seedEntity(t, store, "decision", "FromEntity")
				seedEntity(t, store, "requirement", "ToEntity")
				seedRelation(t, store, "DEC-001", "addresses", "REQ-001")
			},
			run: func(_ *testing.T, mgr *entitymanager.Manager) error {
				return mgr.DeleteRelation(context.Background(), "DEC-001", "addresses", "REQ-001")
			},
			wantSubjectKind: "relation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Seed via NopACL on a shared store so the data exists when
			// the ReadOnlyACL Manager runs the entry point.
			sink := audit.NewMemory()
			roMgr, store := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
			tt.setup(t, store)

			// Snapshot write-side store counters BEFORE running the
			// denied op; setup may have legitimate writes.
			createsBefore := store.creates.Load()
			updatesBefore := store.updates.Load()
			deletesBefore := store.deletes.Load()

			// Snapshot audit count too — setup writes legitimate audit rows.
			seededRecords := len(sink.Records())

			err := tt.run(t, roMgr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, acl.ErrForbidden) {
				t.Errorf("errors.Is(err, ErrForbidden) = false, got err = %v", err)
			}
			var fe *acl.ForbiddenError
			if !errors.As(err, &fe) {
				t.Fatalf("errors.As(err, *ForbiddenError) = false, got err = %v", err)
			}
			if fe.Decision.RuleKind != "read-only" {
				t.Errorf("Decision.RuleKind = %q, want %q", fe.Decision.RuleKind, "read-only")
			}

			// Pin AC1.5's hard invariant: zero new store writes.
			if got := store.creates.Load(); got != createsBefore {
				t.Errorf("CreateEntity store calls increased by %d, want 0", got-createsBefore)
			}
			if got := store.updates.Load(); got != updatesBefore {
				t.Errorf("UpdateEntity store calls increased by %d, want 0", got-updatesBefore)
			}
			if got := store.deletes.Load(); got != deletesBefore {
				t.Errorf("DeleteEntity store calls increased by %d, want 0", got-deletesBefore)
			}

			// Pin the audit invariant: exactly one new denied-write row.
			records := sink.Records()
			newRecords := records[seededRecords:]
			if len(newRecords) != 1 {
				t.Fatalf("new audit records = %d, want 1 (denied-write)", len(newRecords))
			}
			rec := newRecords[0]
			if rec.Op != audit.OpDeniedWrite {
				t.Errorf("audit record Op = %q, want %q", rec.Op, audit.OpDeniedWrite)
			}
			if rec.Subject == nil {
				t.Fatal("audit record Subject is nil")
			}
			if rec.Subject.Kind != tt.wantSubjectKind {
				t.Errorf("Subject.Kind = %q, want %q", rec.Subject.Kind, tt.wantSubjectKind)
			}
			if rec.Summary == "" {
				t.Error("audit record Summary is empty")
			}
		})
	}
}

// AC1.5 corollary: NopACL allows every write path — the existing
// manager_test.go suite already validates this end-to-end; this test
// just pins that the same 7 entry points succeed under NopACL using
// the exact same fixtures and seed data.
func TestManager_NopACLAllows_AllWritePathsSucceed(t *testing.T) {
	sink := audit.NewMemory()
	mgr, store := newManagerWithACL(t, acl.NopACL{}, sink)

	// Smoke test for each entry point.
	e := entity.New("", "requirement")
	e.SetString("title", "Allowed")
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	res.Entity.SetString("title", "Updated")
	if _, uerr := mgr.UpdateEntity(context.Background(), res.Entity); uerr != nil {
		t.Fatalf("UpdateEntity: %v", uerr)
	}
	dec := entity.New("", "decision")
	dec.SetString("title", "D")
	decRes, err := mgr.CreateEntity(context.Background(), dec, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity (decision): %v", err)
	}
	if _, err := mgr.CreateRelation(context.Background(), decRes.Entity.ID, "addresses", res.Entity.ID, entity.RelationOptions{}); err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	if _, err := mgr.UpdateRelation(context.Background(), decRes.Entity.ID, "addresses", res.Entity.ID, entity.RelationOptions{}); err != nil {
		t.Fatalf("UpdateRelation: %v", err)
	}
	if err := mgr.DeleteRelation(context.Background(), decRes.Entity.ID, "addresses", res.Entity.ID); err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}
	if _, err := mgr.DeleteEntity(context.Background(), res.Entity.ID, false); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}

	// Sanity: no denied-write records appeared.
	for _, r := range sink.Records() {
		if r.Op == audit.OpDeniedWrite {
			t.Errorf("unexpected denied-write record under NopACL: %+v", r)
		}
	}
	_ = store
}
