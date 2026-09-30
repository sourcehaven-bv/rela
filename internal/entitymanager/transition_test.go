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
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// transitionMetaYAML defines a snapshot type whose status is a state machine.
const transitionMetaYAML = `version: "1.0"
entities:
  snapshot:
    label: Snapshot
    plural: snapshots
    id_prefix: "SNAP-"
    id_type: sequential
    properties:
      title:
        type: string
      status:
        type: snapshot-status
types:
  snapshot-status:
    values: [in-review, approved, established, obsolete]
    initial: in-review
    transitions:
      - from: in-review
        to: approved
        guard: approve
      - from: approved
        to: established
        guard: establish
`

// allowAllGuard grants every permission; denyAllGuard grants none.
type allowAllGuard struct{}

func (allowAllGuard) HoldsPermission(context.Context, string, string) bool { return true }

type denyAllGuard struct{}

func (denyAllGuard) HoldsPermission(context.Context, string, string) bool { return false }

func newTransitionManager(t *testing.T, guard statemachine.Guard) *entitymanager.Manager {
	t.Helper()
	mgr, _ := newTransitionManagerWithStore(t, guard)
	return mgr
}

// newTransitionManagerWithStore is newTransitionManager, also returning the
// store so a test can read what a write persisted.
func newTransitionManagerWithStore(t *testing.T, guard statemachine.Guard) (*entitymanager.Manager, store.Store) {
	t.Helper()
	return newTransitionManagerAudited(t, guard, audit.Nop{})
}

// newTransitionManagerAudited is newTransitionManagerWithStore recording to
// sink.
func newTransitionManagerAudited(
	t *testing.T, guard statemachine.Guard, sink audit.Audit,
) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(transitionMetaYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	machines, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:           st,
		Meta:            meta,
		Templater:       nopTemplater{},
		Audit:           sink,
		ACL:             acl.NopACL{},
		Transitions:     machines,
		FieldGate:       entitymanager.AllowAllFieldGate{},
		TransitionGuard: guard,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return mgr, st
}

func seedSnapshot(t *testing.T, mgr *entitymanager.Manager, status string) *entity.Entity {
	t.Helper()
	e := entity.New("", "snapshot")
	e.SetString("title", "Q3 register")
	if status != "" {
		e.SetString("status", status)
	}
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("seed create(status=%q): %v", status, err)
	}
	return res.Entity
}

func TestTransition_LegalMovePasses(t *testing.T) {
	mgr := newTransitionManager(t, allowAllGuard{})
	snap := seedSnapshot(t, mgr, "") // enters at initial in-review

	snap.SetString("status", "approved")
	if _, err := mgr.UpdateEntity(context.Background(), snap); err != nil {
		t.Fatalf("legal in-review→approved rejected: %v", err)
	}
}

func TestTransition_IllegalMoveIs422(t *testing.T) {
	mgr := newTransitionManager(t, allowAllGuard{})
	snap := seedSnapshot(t, mgr, "")

	snap.SetString("status", "established") // skips approved
	_, err := mgr.UpdateEntity(context.Background(), snap)
	if !errors.Is(err, statemachine.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition, got %v", err)
	}
	// An illegal transition is NOT an ACL denial — it must not surface as 403.
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) {
		t.Fatal("illegal transition must not map to a ForbiddenError (403)")
	}
}

func TestTransition_GuardDeniedIs403(t *testing.T) {
	mgr := newTransitionManager(t, denyAllGuard{})
	snap := seedSnapshot(t, mgr, "")

	snap.SetString("status", "approved") // legal edge, but guard "approve" denied
	_, err := mgr.UpdateEntity(context.Background(), snap)
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *acl.ForbiddenError (403), got %v", err)
	}
	if fe.Decision.RuleKind != "transition-guard" {
		t.Errorf("RuleKind = %q, want transition-guard", fe.Decision.RuleKind)
	}
}

func TestTransition_IllegalEntryOnCreateIs422(t *testing.T) {
	mgr := newTransitionManager(t, allowAllGuard{})
	e := entity.New("", "snapshot")
	e.SetString("title", "bad entry")
	e.SetString("status", "established") // not the initial value
	_, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if !errors.Is(err, statemachine.ErrIllegalEntry) {
		t.Fatalf("expected ErrIllegalEntry, got %v", err)
	}
}

func TestTransition_LegalEntryOnCreatePasses(t *testing.T) {
	mgr := newTransitionManager(t, allowAllGuard{})
	// Explicit initial value is fine.
	seedSnapshot(t, mgr, "in-review")
	// Absent is fine (default applies).
	seedSnapshot(t, mgr, "")
}

// BUG-KK1UXH: a restore brings back a record that already existed, so the
// entry rule does not apply to it. A deleted `established` snapshot comes back
// `established` for a principal who holds the guard of the edge into it.
func TestTransition_RecreateEntity_RestoresPastEntry(t *testing.T) {
	mgr, st := newTransitionManagerWithStore(t, allowAllGuard{})
	ctx := context.Background()
	snap := seedSnapshot(t, mgr, "") // in-review

	back := snap.Clone()
	back.ID = snap.ID + "0"
	back.SetString("status", "established") // not the initial value
	if _, err := entitymanager.RecreateEntity(ctx, mgr, back); err != nil {
		t.Fatalf("RecreateEntity of a status past the entry value: %v", err)
	}
	got, err := st.GetEntity(ctx, back.Ref())
	if err != nil {
		t.Fatalf("read the restored row: %v", err)
	}
	if status := got.GetString("status"); status != "established" {
		t.Errorf("restored status = %q, want %q", status, "established")
	}
}

// The guards still bind on restore. A principal who could not reach the
// recorded state by transitions cannot reach it by deleting the entity and
// restoring an old version: a 403 naming the first unheld guard on the path
// (`approve`, not the last hop's `establish`), a denied-write audit record,
// and no row.
func TestTransition_RecreateEntity_GuardedStateIs403(t *testing.T) {
	sink := audit.NewMemory()
	mgr, st := newTransitionManagerAudited(t, denyAllGuard{}, sink)
	ctx := context.Background()
	back := entity.New("SNAP-9", "snapshot")
	back.SetString("title", "restored")
	back.SetString("status", "established") // in-review -approve-> approved -establish-> established

	_, err := entitymanager.RecreateEntity(ctx, mgr, back)
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("RecreateEntity past an unheld guard = %v, want *acl.ForbiddenError", err)
	}
	if fe.Decision.RuleKind != "transition-guard" || fe.Decision.RuleID != "approve" {
		t.Errorf("decision = %s/%s, want transition-guard/approve", fe.Decision.RuleKind, fe.Decision.RuleID)
	}
	if _, gerr := st.GetEntity(ctx, back.Ref()); !errors.Is(gerr, store.ErrNotFound) {
		t.Errorf("a refused restore left a row behind: %v", gerr)
	}
	var denied []audit.Record
	for _, rec := range sink.Records() {
		if rec.Op == audit.OpDeniedWrite {
			denied = append(denied, rec)
		}
	}
	if len(denied) != 1 || denied[0].Subject == nil || denied[0].Subject.ID != back.ID {
		t.Errorf("denied-write records = %+v, want one naming %s", denied, back.ID)
	}
}

// A value no declared edge enters cannot be restored: `obsolete` is declared
// but unreachable, `shredded` is not declared at all. Either would leave the
// row in a state no transition can leave.
func TestTransition_RecreateEntity_UnenterableStatusIs422(t *testing.T) {
	for _, status := range []string{"obsolete", "shredded"} {
		t.Run(status, func(t *testing.T) {
			mgr := newTransitionManager(t, allowAllGuard{})
			back := entity.New("SNAP-9", "snapshot")
			back.SetString("title", "restored")
			back.SetString("status", status)
			_, err := entitymanager.RecreateEntity(context.Background(), mgr, back)
			if !errors.Is(err, statemachine.ErrIllegalEntry) {
				t.Fatalf("RecreateEntity of %q = %v, want ErrIllegalEntry", status, err)
			}
		})
	}
}

// RR-HETEE: a rejected illegal-entry create must NOT persist a row.
func TestTransition_IllegalEntry_DoesNotPersist(t *testing.T) {
	meta, err := metamodel.Parse([]byte(transitionMetaYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	machines, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:           st,
		Meta:            meta,
		Templater:       nopTemplater{},
		Audit:           audit.Nop{},
		ACL:             acl.NopACL{},
		Transitions:     machines,
		FieldGate:       entitymanager.AllowAllFieldGate{},
		TransitionGuard: allowAllGuard{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	e := entity.New("", "snapshot")
	e.SetString("title", "bad")
	e.SetString("status", "established") // illegal entry
	if _, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{}); !errors.Is(err, statemachine.ErrIllegalEntry) {
		t.Fatalf("want ErrIllegalEntry, got %v", err)
	}
	// No snapshot row must exist — the check runs before the store write, so a
	// rejected illegal entry never persists (and thus never emits a store event).
	count := 0
	for range st.ListEntities(context.Background(), store.EntityQuery{Type: "snapshot", Faces: store.InWorld(store.TrivialScope())}) {
		count++
	}
	if count != 0 {
		t.Fatalf("illegal-entry create persisted %d row(s) (RR-HETEE regression)", count)
	}
}

// facedTransitionYAML is a faced type whose status is a state machine.
const facedTransitionYAML = `
entities:
  beleid:
    label: Beleid
    id_prefix: "POL-"
    faces:
      concept: {label: Concept}
      vastgesteld: {label: Vastgesteld}
    properties:
      title: {type: string}
      status: {type: beleid-status}
types:
  beleid-status:
    values: [draft, adopted, withdrawn]
    initial: draft
    transitions:
      - from: draft
        to: adopted
        guard: adopt
      - from: adopted
        to: withdrawn
        guard: withdraw
`

func newFacedTransitionManager(t *testing.T) (*entitymanager.Manager, store.Store) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(facedTransitionYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	machines, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: machines,
		FieldGate: entitymanager.AllowAllFieldGate{}, TransitionGuard: allowAllGuard{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return mgr, st
}

// BUG-KK1UXH on a faced type: a deleted face comes back at its recorded
// status, past the entry value, while an ordinary create of the same row
// is still refused.
func TestTransition_FacedRestorePastEntry(t *testing.T) {
	ctx := context.Background()
	row := func() *entity.Entity {
		return &entity.Entity{
			ID: "POL-1", Type: "beleid", Face: entity.Face("vastgesteld"),
			Properties: map[string]any{"title": "Access policy", "status": "withdrawn"},
		}
	}

	t.Run("create is refused", func(t *testing.T) {
		mgr, _ := newFacedTransitionManager(t)
		_, err := mgr.CreateEntity(ctx, row(), entity.CreateOptions{Face: entity.Face("vastgesteld")})
		if !errors.Is(err, statemachine.ErrIllegalEntry) {
			t.Fatalf("CreateEntity at a non-entry status = %v, want ErrIllegalEntry", err)
		}
	})

	t.Run("restore succeeds", func(t *testing.T) {
		mgr, st := newFacedTransitionManager(t)
		if _, err := entitymanager.RecreateEntity(ctx, mgr, row()); err != nil {
			t.Fatalf("RecreateEntity at a non-entry status: %v", err)
		}
		got, err := st.GetEntity(ctx, entity.Ref{ID: "POL-1", Face: entity.Face("vastgesteld")})
		if err != nil {
			t.Fatalf("read the restored face: %v", err)
		}
		if status := got.GetString("status"); status != "withdrawn" {
			t.Errorf("restored status = %q, want %q", status, "withdrawn")
		}
	})
}
