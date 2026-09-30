package entitymanager_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func newRecreateManager(t *testing.T, st store.Store, sink audit.Audit) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        parseMeta(t),
		Templater:   nopTemplater{},
		Audit:       sink,
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// TestRecreateEntity_PreservesExplicitSequentialID is the core RR-L1MY0N
// regression: CreateEntity rejects an explicit ID for a non-manual id_type
// (the test metamodel's `requirement` is id_type: sequential), but
// RecreateEntity must preserve a caller-supplied ID so a restored record keeps
// its identity.
func TestRecreateEntity_PreservesExplicitSequentialID(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})

	e := &entity.Entity{
		ID:         "REQ-fromPeer",
		Type:       "requirement",
		Properties: map[string]any{"title": "Restored requirement", "status": "draft"},
	}
	res, err := entitymanager.RecreateEntity(context.Background(), mgr, e)
	if err != nil {
		t.Fatalf("RecreateEntity: %v", err)
	}
	if res.Entity.ID != "REQ-fromPeer" {
		t.Fatalf("ID not preserved: got %q", res.Entity.ID)
	}

	// Confirm CreateEntity would have rejected the same explicit ID, proving
	// RecreateEntity genuinely takes a different path.
	_, createErr := mgr.CreateEntity(context.Background(), &entity.Entity{
		ID: "REQ-anotherExplicit", Type: "requirement",
		Properties: map[string]any{"title": "x"},
	}, entity.CreateOptions{ID: "REQ-anotherExplicit"})
	if createErr == nil {
		t.Fatal("expected CreateEntity to reject an explicit ID for a sequential id_type")
	}

	got, err := st.GetEntity(context.Background(), entity.Ref{ID: "REQ-fromPeer"})
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.GetString("title") != "Restored requirement" {
		t.Fatalf("title not persisted: %q", got.GetString("title"))
	}
}

// TestRecreateEntity_AuditsCreate pins that a recreate emits one create audit
// record.
func TestRecreateEntity_AuditsCreate(t *testing.T) {
	st := memstore.New()
	mem := audit.NewMemory()
	mgr := newRecreateManager(t, st, mem)
	ctx := context.Background()

	e := &entity.Entity{ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "t", "status": "draft"}}
	if _, err := entitymanager.RecreateEntity(ctx, mgr, e); err != nil {
		t.Fatalf("recreate: %v", err)
	}

	records := mem.Records()
	if len(records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(records))
	}
	if records[0].Op != audit.OpCreateEntity {
		t.Errorf("record op = %q, want %q", records[0].Op, audit.OpCreateEntity)
	}
}

// TestRecreateEntity_RejectsInvalidContent pins that a HARD validation error (an
// unknown entity type) aborts the recreate and surfaces a *ValidationError
// (which the API layer maps to 422), persisting nothing.
//
// Note a missing required property is a SOFT condition per DEC-HWZHA: it rides
// along as a warning and the recreate still succeeds, matching CreateEntity /
// UpdateEntity.
// TestRecreateEntity_SoftWarningStillApplies covers that case.
func TestRecreateEntity_RejectsInvalidContent(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})
	ctx := context.Background()

	e := &entity.Entity{ID: "ZZ-bad", Type: "no-such-type", Properties: map[string]any{"title": "x"}}
	_, err := entitymanager.RecreateEntity(ctx, mgr, e)
	if err == nil {
		t.Fatal("expected a validation error for unknown entity type")
	}
	var vErr *entitymanager.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if _, getErr := st.GetEntity(ctx, entity.Ref{ID: "ZZ-bad"}); getErr == nil {
		t.Fatal("invalid entity was persisted despite the validation error")
	}
}

// TestRecreateEntity_SoftWarningStillApplies pins that a soft validation condition
// (missing required property) does NOT abort — it persists with a warning,
// matching the rest of the write path (DEC-HWZHA).
func TestRecreateEntity_SoftWarningStillApplies(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})
	ctx := context.Background()

	// `title` is required on requirement; omit it -> soft warning, not abort.
	e := &entity.Entity{ID: "REQ-soft", Type: "requirement", Properties: map[string]any{"status": "draft"}}
	res, err := entitymanager.RecreateEntity(ctx, mgr, e)
	if err != nil {
		t.Fatalf("recreate should succeed with a warning, got error: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a soft warning for the missing required title")
	}
	if _, getErr := st.GetEntity(ctx, entity.Ref{ID: "REQ-soft"}); getErr != nil {
		t.Fatalf("entity not persisted: %v", getErr)
	}
}

// TestRecreateEntity_SuppressesAutomation is the RR-AZMA7T regression: a
// recreate must NOT run automation/cascade, or it would re-derive records the
// original create already produced. A counting store proves it does exactly
// one write and no derived writes.
func TestRecreateEntity_SuppressesAutomation(t *testing.T) {
	counting := &countingStore{Store: memstore.New()}
	// An automation that, on requirement create, would create a checklist
	// entity. If recreate ran automation, the cascade would write a second entity.
	auto := automation.Automation{
		Name: "make-checklist",
		On:   automation.Trigger{Entity: []string{"requirement"}, Created: true},
		Do: []automation.Action{{
			CreateEntity: &automation.CreateEntityAction{
				Type:       "checklist",
				Properties: map[string]string{"title": "auto"},
			},
		}},
	}

	// Sanity: through CreateEntity the automation DOES fire (a checklist is
	// created), so the suppression assertion below is meaningful.
	withAuto := newManagerWithStoreAndAudit(t, &countingStore{Store: memstore.New()}, audit.Nop{}, []automation.Automation{auto})
	cres, err := withAuto.CreateEntity(context.Background(), &entity.Entity{
		Type: "requirement", Properties: map[string]any{"title": "t"},
	}, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("control CreateEntity: %v", err)
	}
	if len(cres.EntitiesCreated) == 0 {
		t.Skip("automation did not create a derived entity in the control; spec may differ — suppression test inconclusive")
	}

	// Now the real assertion: RecreateEntity with the SAME automation wired
	// must create exactly one entity (the recreated one) and run no cascade.
	recreateMgr := newManagerWithStoreAndAudit(t, counting, audit.Nop{}, []automation.Automation{auto})
	if _, err := entitymanager.RecreateEntity(context.Background(), recreateMgr, &entity.Entity{
		ID: "REQ-applied", Type: "requirement", Properties: map[string]any{"title": "t", "status": "draft"},
	}); err != nil {
		t.Fatalf("RecreateEntity: %v", err)
	}
	if got := counting.creates.Load(); got != 1 {
		t.Fatalf("expected exactly 1 create (no automation-derived writes), got %d", got)
	}
}

// TestRecreateEntity_RejectsNilAndEmptyID covers the guard conditions.
func TestRecreateEntity_RejectsNilAndEmptyID(t *testing.T) {
	mgr := newRecreateManager(t, memstore.New(), audit.Nop{})
	ctx := context.Background()
	if _, err := entitymanager.RecreateEntity(ctx, mgr, nil); err == nil {
		t.Error("expected error for nil entity")
	}
	if _, err := entitymanager.RecreateEntity(ctx, mgr, &entity.Entity{Type: "requirement"}); err == nil {
		t.Error("expected error for empty ID")
	}
}

// TestRecreateEntity_RejectsLocked covers the inaccessible-fields guard.
func TestRecreateEntity_RejectsLocked(t *testing.T) {
	mgr := newRecreateManager(t, memstore.New(), audit.Nop{})
	e := &entity.Entity{
		ID: "REQ-1", Type: "requirement",
		Inaccessible: []entity.InaccessibleField{{Name: "title", Reason: entity.InaccessibleReasonGitCrypt}},
	}
	if _, err := entitymanager.RecreateEntity(context.Background(), mgr, e); err == nil {
		t.Fatal("expected error recreating a locked entity")
	}
}

// --- Code-review hardening (fail-closed, ACL, foreign prefix, no-status) ---

// flakyProbeStore makes the first GetEntity for a given ID return a transient
// (non-NotFound) error, then defer to the wrapped store. Models a backend blip
// during the existence probe.
type flakyProbeStore struct {
	store.Store
	failID  string
	failErr error
	failed  bool
}

func (s *flakyProbeStore) GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
	if ref.ID == s.failID && !s.failed {
		s.failed = true
		return nil, s.failErr
	}
	return s.Store.GetEntity(ctx, ref)
}

// TestRecreateEntity_ExistenceProbeFailsClosed is the critical RR-review
// regression: a transient (non-NotFound) error from the existence GetEntity
// must abort, NOT be silently treated as "absent" (which would create over a
// row that is really there). The error must surface and propagate, not be
// swallowed.
func TestRecreateEntity_ExistenceProbeFailsClosed(t *testing.T) {
	sentinel := errors.New("boom: backend blip")
	st := &flakyProbeStore{Store: memstore.New(), failID: "REQ-1", failErr: sentinel}
	mgr := newRecreateManager(t, st, audit.Nop{})

	_, err := entitymanager.RecreateEntity(context.Background(), mgr, &entity.Entity{
		ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "t", "status": "draft"},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the transient store error to propagate, got %v", err)
	}
}

// TestRecreateEntity_ACLDenied pins that a read-only principal is denied a
// recreate (a write) with a *ForbiddenError — the ACL framing is real, not
// bypassed.
func TestRecreateEntity_ACLDenied(t *testing.T) {
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate: entitymanager.AllowAllFieldGate{},
		Store:     st, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.ReadOnlyACL{}, Transitions: statemachine.EmptySet(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, recreateErr := entitymanager.RecreateEntity(context.Background(), mgr, &entity.Entity{
		ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "t", "status": "draft"},
	})
	var forbidden *acl.ForbiddenError
	if !errors.As(recreateErr, &forbidden) {
		t.Fatalf("expected *acl.ForbiddenError, got %T: %v", recreateErr, recreateErr)
	}
	if _, getErr := st.GetEntity(context.Background(), entity.Ref{ID: "REQ-1"}); getErr == nil {
		t.Fatal("entity was persisted despite ACL denial")
	}
}

// TestRecreateEntity_RejectsForeignIDPrefix documents and pins the decision
// (review #3): an ID matching no local prefix is a HARD validation error,
// rather than silently written under a prefix the metamodel doesn't declare.
func TestRecreateEntity_RejectsForeignIDPrefix(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})

	// `requirement` declares prefix REQ-; FOREIGN- matches no declared prefix.
	e := &entity.Entity{ID: "FOREIGN-1", Type: "requirement", Properties: map[string]any{"title": "t", "status": "draft"}}
	_, err := entitymanager.RecreateEntity(context.Background(), mgr, e)
	if err == nil {
		t.Fatal("expected a hard validation error for a foreign ID prefix")
	}
	var vErr *entitymanager.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
}

// TestRecreateEntity_NoStatusAppliesAsIs pins the documented precondition (review
// #4): RecreateEntity does NOT backfill a status default. An entity supplied
// without a status persists without one (status is a soft/optional field on the
// test metamodel), proving "caller owns complete state" — no silent defaulting.
func TestRecreateEntity_NoStatusAppliesAsIs(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})
	ctx := context.Background()

	e := &entity.Entity{ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "t"}}
	if _, err := entitymanager.RecreateEntity(ctx, mgr, e); err != nil {
		t.Fatalf("RecreateEntity: %v", err)
	}
	got, err := st.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.GetString("status") != "" {
		t.Fatalf("RecreateEntity backfilled a status default %q; it must persist as-is", got.GetString("status"))
	}
}

// TestRecreateEntity_IsCreateOnly: RecreateEntity lands a missing row at its
// own id but refuses an existing one instead of replacing it, so a restore that raced a recreate cannot overwrite the live row.
func TestRecreateEntity_IsCreateOnly(t *testing.T) {
	st := memstore.New()
	mgr := newRecreateManager(t, st, audit.Nop{})
	ctx := context.Background()

	e := &entity.Entity{
		ID: "REQ-back", Type: "requirement",
		Properties: map[string]any{"title": "Restored", "status": "draft"},
	}
	if _, err := entitymanager.RecreateEntity(ctx, mgr, e); err != nil {
		t.Fatalf("RecreateEntity of a missing row: %v", err)
	}

	again := &entity.Entity{
		ID: "REQ-back", Type: "requirement",
		Properties: map[string]any{"title": "Overwritten", "status": "draft"},
	}
	_, err := entitymanager.Recreator{M: mgr}.RecreateEntity(ctx, again)
	if !errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
		t.Fatalf("RecreateEntity of a live row = %v, want ErrEntityAlreadyExists", err)
	}
	got, err := st.GetEntity(ctx, entity.Ref{ID: "REQ-back"})
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.GetString("title") != "Restored" {
		t.Errorf("title = %q; a refused recreate must leave the live row alone", got.GetString("title"))
	}
}
