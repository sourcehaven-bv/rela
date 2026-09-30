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

// typeConfusionMetamodel declares two manual-id entity types with no id_prefix,
// so the ID-prefix structural guard (a HARD validation error for prefixed,
// sequential types) does NOT fire — this is exactly the shape the BUG-ZWTDH9
// exploit relies on: a `manual` id_type target that skips the prefix check, so
// the ONLY defense against re-typing is the write layer under test.
const typeConfusionMetamodel = `version: "1.0"
entities:
  secret:
    label: Secret
    plural: secrets
    id_type: manual
    properties:
      title:
        type: string
  note:
    label: Note
    plural: notes
    id_type: manual
    properties:
      title:
        type: string
`

// facedConfusionMetamodel is typeConfusionMetamodel with faces: two prefixless
// manual-id types that share face names, so nothing but the store stops a
// recreate from joining an existing family under another type.
const facedConfusionMetamodel = `version: "1.0"
entities:
  secret:
    label: Secret
    plural: secrets
    id_type: manual
    faces:
      concept: {label: Concept}
      vastgesteld: {label: Vastgesteld}
    properties:
      title: {type: string}
  note:
    label: Note
    plural: notes
    id_type: manual
    faces:
      concept: {label: Concept}
      vastgesteld: {label: Vastgesteld}
    properties:
      title: {type: string}
`

// A recreate of a free face whose sibling face holds another type must not
// land: one id is one entity, with one type. The store refuses the write, so
// the recreate fails and the family keeps its type.
func TestRecreateEntity_SiblingFaceOfAnotherTypeIsRefused(t *testing.T) {
	meta, err := metamodel.Parse([]byte(facedConfusionMetamodel))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "X-1", Type: "secret", Face: entity.Face("concept"),
		Properties: map[string]any{"title": "secret draft"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := entitymanager.RecreateEntity(ctx, mgr, &entity.Entity{
		ID: "X-1", Type: "note", Face: entity.Face("vastgesteld"),
		Properties: map[string]any{"title": "note"},
	}); err == nil {
		t.Fatal("a recreate joined a family of another type")
	}
	if _, gerr := st.GetEntity(ctx, entity.Ref{ID: "X-1", Face: entity.Face("vastgesteld")}); gerr == nil {
		t.Error("the refused recreate wrote the row")
	}
}

func typeConfusionMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(typeConfusionMetamodel))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	return m
}

// raceCreateStore models the postgres multi-writer TOCTOU that BUG-ZWTDH9's
// residual rode on: RecreateEntity's existence probe (GetEntity) observes the id
// as ABSENT — so the intent resolves to CREATE — but a concurrent writer lands
// the id between the probe and the durable write, so CreateEntity conflicts.
//
// GetEntity delegates to the wrapped store (which does NOT yet hold the id), and
// CreateEntity returns store.ErrConflict WITHOUT writing. The wrapped store is
// pre-seeded with the "winner" so the test can assert the loser never
// overwrote or re-typed it. UpdateEntity is counted and delegated, so a
// create-that-fell-through-to-update (the removed upsert fallback) would both
// bump the counter and clobber the seeded winner — exactly what must not happen.
type raceCreateStore struct {
	store.Store
	updateCalls int
}

func (s *raceCreateStore) CreateEntity(_ context.Context, _ *entity.Entity) error {
	return store.ErrConflict
}

func (s *raceCreateStore) UpdateEntity(ctx context.Context, e *entity.Entity) error {
	s.updateCalls++
	return s.Store.UpdateEntity(ctx, e)
}

// GetEntity reports the row as absent so RecreateEntity proceeds to the create,
// while the wrapped store separately holds the seeded winner for verification.
func (s *raceCreateStore) GetEntity(_ context.Context, _ entity.Ref) (*entity.Entity, error) {
	return nil, store.ErrNotFound
}

// TestRecreateEntity_CreateConflict_RejectsAndDoesNotClobber pins that a
// RecreateEntity whose durable CreateEntity conflicts (a concurrent
// create of the same id) is REJECTED with ErrEntityAlreadyExists and never
// falls through to an UpdateEntity. This closes both residuals at once: the
// lost-update clobber (a racing create is not blindly overwritten) and the
// type-re-type vector (a create-intent write that conflicts can no longer
// become a blind, re-typing update on the postgres multi-writer backend).
func TestRecreateEntity_CreateConflict_RejectsAndDoesNotClobber(t *testing.T) {
	inner := memstore.New()
	// The "winner": a secret-typed entity a concurrent writer already landed.
	// The recreate under test claims the SAME id with a DIFFERENT type — the
	// re-type attempt must not land.
	meta := typeConfusionMeta(t)
	if err := inner.CreateEntity(context.Background(), &entity.Entity{
		ID: "SECRET-1", Type: "secret", Properties: map[string]any{"title": "winner"},
	}); err != nil {
		t.Fatalf("seed winner: %v", err)
	}

	st := &raceCreateStore{Store: inner}
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate: entitymanager.AllowAllFieldGate{},
		Store:     st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	// A recreate (probe says absent) that races a concurrent create.
	_, recreateErr := entitymanager.RecreateEntity(context.Background(), mgr, &entity.Entity{
		ID: "SECRET-1", Type: "note", Properties: map[string]any{"title": "loser re-types to note"},
	})
	if recreateErr == nil {
		t.Fatal("create-conflict recreate succeeded — a racing create was silently overwritten (lost-update residual)")
	}
	if !errors.Is(recreateErr, entitymanager.ErrEntityAlreadyExists) {
		t.Fatalf("expected ErrEntityAlreadyExists, got %T: %v", recreateErr, recreateErr)
	}
	if st.updateCalls != 0 {
		t.Fatalf("create fell through to UpdateEntity %d time(s) — the upsert fallback re-appeared", st.updateCalls)
	}

	// The seeded winner must be untouched: still a secret, original title.
	got, err := inner.GetEntity(context.Background(), entity.Ref{ID: "SECRET-1"})
	if err != nil {
		t.Fatalf("GetEntity(SECRET-1): %v", err)
	}
	if got.Type != "secret" {
		t.Fatalf("winner was re-typed to %q; the create-conflict became a re-typing overwrite", got.Type)
	}
	if got.GetString("title") != "winner" {
		t.Fatalf("winner title overwritten to %q; the create-conflict clobbered the racing create", got.GetString("title"))
	}
}

// TestRecreateEntity_SameTypeCreateConflict_NoClobber is the same-type variant:
// two concurrent creates of the SAME id and SAME type. There is no re-typing
// here, only the lost-update question — the loser must still be rejected, not
// silently overwrite the winner's content.
func TestRecreateEntity_SameTypeCreateConflict_NoClobber(t *testing.T) {
	inner := memstore.New()
	meta := typeConfusionMeta(t)
	if err := inner.CreateEntity(context.Background(), &entity.Entity{
		ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "winner"},
	}); err != nil {
		t.Fatalf("seed winner: %v", err)
	}

	st := &raceCreateStore{Store: inner}
	mgr, err := entitymanager.New(entitymanager.Deps{
		FieldGate: entitymanager.AllowAllFieldGate{},
		Store:     st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(),
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}

	_, recreateErr := entitymanager.RecreateEntity(context.Background(), mgr, &entity.Entity{
		ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "loser"},
	})
	if !errors.Is(recreateErr, entitymanager.ErrEntityAlreadyExists) {
		t.Fatalf("expected ErrEntityAlreadyExists, got %T: %v", recreateErr, recreateErr)
	}
	if st.updateCalls != 0 {
		t.Fatalf("same-type create fell through to UpdateEntity %d time(s) — must never clobber", st.updateCalls)
	}
	got, err := inner.GetEntity(context.Background(), entity.Ref{ID: "NOTE-1"})
	if err != nil {
		t.Fatalf("GetEntity(NOTE-1): %v", err)
	}
	if got.GetString("title") != "winner" {
		t.Fatalf("winner title overwritten to %q; a same-type racing create was clobbered", got.GetString("title"))
	}
}
