package entitymanager_test

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// cascadePolicy: alice may delete requirements, but holds NO delete grant on
// `decision` — the source type of the incoming `addresses` edge. So a cascade
// delete of a requirement destroys an edge she could not delete directly.
const cascadePolicy = `
roles:
  req-admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: [requirement]
  full-admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
assignments:
  alice: req-admin
  root: full-admin
`

func cascadeManager(t *testing.T, st store.Store) *entitymanager.Manager {
	t.Helper()
	p, err := acl.LoadPolicyBytes([]byte(cascadePolicy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        parseMeta(t),
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         d,
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

func asUser(user string) context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: user, Tool: principal.ToolCLI})
}

// seedCascadeGraph builds DEC-1 --addresses--> REQ-1, so deleting REQ-1 with
// cascade destroys an edge whose SOURCE is a decision.
func seedCascadeGraph(t *testing.T, st store.Store) {
	t.Helper()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		entity.New("REQ-1", "requirement"),
		entity.New("DEC-1", "decision"),
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	if _, err := st.CreateRelation(ctx, "DEC-1", "addresses", "REQ-1", nil); err != nil {
		t.Fatalf("seed relation: %v", err)
	}
}

// TestCascadeDelete_DeniedWhenRelationNotDeletable is the core of TKT-8HDPQW:
// deleting an entity must not be a back door to destroying edge types the
// principal holds no delete grant on.
func TestCascadeDelete_DeniedWhenRelationNotDeletable(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedCascadeGraph(t, st)
	mgr := cascadeManager(t, st)

	_, err := mgr.DeleteEntity(asUser("alice"), "REQ-1", true)
	if err == nil {
		t.Fatal("cascade succeeded; alice holds delete on requirement but not on " +
			"decision, the source of the incident addresses edge")
	}
	var forbidden *acl.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Errorf("error %v does not wrap *acl.ForbiddenError, so callers cannot "+
			"map it to a 403", err)
	}
	// The message must be entity-delete-shaped: a bare relation denial is
	// baffling when the request was "delete this entity".
	if !strings.Contains(err.Error(), "REQ-1") || !strings.Contains(err.Error(), "addresses") {
		t.Errorf("error %q does not name the entity and the blocking relation", err)
	}
}

// TestCascadeDelete_DeniedLeavesEverythingIntact pins the no-partial-write
// property. Asserted on memstore deliberately: the postgres variant is gated on
// RELA_TEST_DATABASE_URL and skipped in default CI, so if that were the only
// place this held, a regression would ship on the default build.
func TestCascadeDelete_DeniedLeavesEverythingIntact(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedCascadeGraph(t, st)
	mgr := cascadeManager(t, st)

	if _, err := mgr.DeleteEntity(asUser("alice"), "REQ-1", true); err == nil {
		t.Fatal("expected denial")
	}

	ctx := context.Background()
	if _, err := st.GetEntity(ctx, "REQ-1"); err != nil {
		t.Errorf("REQ-1 was deleted despite the denial: %v", err)
	}
	if _, err := st.GetRelation(ctx, "DEC-1", "addresses", "REQ-1"); err != nil {
		t.Errorf("the addresses edge was deleted despite the denial: %v", err)
	}
}

// TestCascadeDelete_AllowedWhenEveryRelationIsDeletable is the counterweight:
// the gate must not break the legitimate case.
func TestCascadeDelete_AllowedWhenEveryRelationIsDeletable(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	seedCascadeGraph(t, st)
	mgr := cascadeManager(t, st)

	res, err := mgr.DeleteEntity(asUser("root"), "REQ-1", true)
	if err != nil {
		t.Fatalf("root holds delete on everything: %v", err)
	}
	if len(res.DeletedRelations) != 1 {
		t.Errorf("DeletedRelations = %d, want 1", len(res.DeletedRelations))
	}
	if _, gErr := st.GetEntity(context.Background(), "REQ-1"); gErr == nil {
		t.Error("REQ-1 still exists after an allowed cascade")
	}
}

// TestCascadeDelete_NoRelationsNeedsNoRelationGrant pins that the gate is
// scoped to what a cascade actually destroys — an entity with no edges must
// not suddenly require relation grants.
func TestCascadeDelete_NoRelationsNeedsNoRelationGrant(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	if err := st.CreateEntity(context.Background(), entity.New("REQ-9", "requirement")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mgr := cascadeManager(t, st)

	if _, err := mgr.DeleteEntity(asUser("alice"), "REQ-9", true); err != nil {
		t.Fatalf("alice holds delete on requirement and there are no edges: %v", err)
	}
}

// raceStore injects a concurrent relation create in the window between the
// manager's collect and the store's own re-derivation.
//
// This is THE test that distinguishes the correct fix from the naive one. The
// original design authorized a snapshot collected OUTSIDE any lock; both
// stores then re-derive the set inside their own lock and delete THAT set. A
// relation added in that window would be deleted with no authorization at all.
// Running collect+authorize+delete inside one Tx closes the window, so the
// injected write cannot land mid-operation.
type raceStore struct {
	store.Store
	once   sync.Once
	inject func()
}

func (s *raceStore) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	// Fire on the FIRST collect — i.e. after authorization has begun but
	// before the store re-derives its own set.
	s.once.Do(func() {
		if s.inject != nil {
			s.inject()
		}
	})
	return s.Store.ListRelations(ctx, q)
}

func (s *raceStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	// Hand the callback THIS decorator so the injection point stays live
	// inside the transaction.
	return s.Store.Tx(ctx, func(store.Store) error { return fn(s) })
}

// TestCascadeDelete_ConcurrentRelationIsNotDeletedUnauthorized pins that a
// relation appearing during the operation is either authorized or blocks the
// delete — never silently destroyed.
func TestCascadeDelete_ConcurrentRelationIsNotDeletedUnauthorized(t *testing.T) {
	t.Parallel()
	inner := memstore.New()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		entity.New("REQ-1", "requirement"),
		entity.New("DEC-1", "decision"),
	} {
		if err := inner.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	st := &raceStore{Store: inner}
	st.inject = func() {
		// A concurrent writer adds an edge alice cannot delete.
		if _, err := inner.CreateRelation(ctx, "DEC-1", "addresses", "REQ-1", nil); err != nil {
			t.Errorf("inject: %v", err)
		}
	}
	mgr := cascadeManager(t, st)

	_, err := mgr.DeleteEntity(asUser("alice"), "REQ-1", true)

	// Whichever way it resolves, the invariant is the same: that edge must not
	// have been deleted without authorization.
	_, relErr := inner.GetRelation(ctx, "DEC-1", "addresses", "REQ-1")
	_, entErr := inner.GetEntity(ctx, "REQ-1")
	if relErr != nil && err == nil {
		t.Fatal("the concurrently-added addresses edge was deleted by a cascade " +
			"that never authorized it — the TOCTOU window is open")
	}
	if err != nil && entErr != nil {
		t.Error("the delete failed but the entity is gone: partial write")
	}
}
