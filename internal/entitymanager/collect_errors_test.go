package entitymanager_test

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// highestIDErrStore fails HighestID, modeling a store that cannot say
// which numbers are taken. ID generation must not guess.
type highestIDErrStore struct {
	store.Store
	err error
}

func (s *highestIDErrStore) HighestID(context.Context, string) (int, error) {
	return 0, s.err
}

// listRelationsErrStore yields a terminal error from ListRelations,
// modeling a relation scan that fails before the full incident set is
// counted. The delete-safety gate must not run on partial data.
type listRelationsErrStore struct {
	store.Store
	err error
}

// Tx hands the callback THIS decorator so the ListRelations override stays
// effective inside a transaction; the embedded store's Tx would bypass it.
func (s *listRelationsErrStore) Tx(_ context.Context, fn func(store.Store) error) error {
	return fn(s)
}

func (s *listRelationsErrStore) ListRelations(_ context.Context, _ store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		yield(nil, s.err)
	}
}

func newManagerOver(t *testing.T, st store.Store) *entitymanager.Manager {
	t.Helper()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       st,
		Meta:        parseMeta(t),
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr
}

// TestCreate_FailsWhenIDScanErrors pins that auto-ID create surfaces an
// entity-scan error instead of generating an ID from a partial list. A
// truncated scan can hide a high-numbered existing ID, so generating
// from it risks a collision — which createCore now rejects with
// ErrEntityAlreadyExists (a create never overwrites), so a bad scan
// would surface as a spurious conflict rather than data loss.
func TestCreate_FailsWhenIDScanErrors(t *testing.T) {
	sentinel := errors.New("boom: highest id lookup failed")
	st := &highestIDErrStore{Store: memstore.New(), err: sentinel}
	mgr := newManagerOver(t, st)

	e := entity.New("", "requirement") // auto-ID path → triggers HighestID
	e.SetString("title", "needs an id")
	_, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the entity-scan error to surface, got %v", err)
	}
}

// TestDelete_FailsWhenRelationScanErrors pins that DeleteEntity refuses
// to proceed when the incident-relation scan errors, rather than
// under-counting and deleting an entity that may still have relations
// (the orphan class issue #888 closed at the deletion step).
func TestDelete_FailsWhenRelationScanErrors(t *testing.T) {
	sentinel := errors.New("boom: relation scan failed")

	// Seed the entity in the underlying store so the GetEntity lookup
	// and ACL check pass; the relation scan is what fails.
	mem := memstore.New()
	target := entity.New("REQ-1", "requirement")
	target.SetString("title", "has relations on disk")
	if err := mem.CreateEntity(context.Background(), target); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := &listRelationsErrStore{Store: mem, err: sentinel}
	mgr := newManagerOver(t, st)

	_, err := mgr.DeleteEntity(context.Background(), "REQ-1", false)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the relation-scan error to surface, got %v", err)
	}
}
