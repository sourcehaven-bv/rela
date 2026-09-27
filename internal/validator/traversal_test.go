package validator_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

func binder(meta *metamodel.Metamodel, st store.GraphQueryer) *relresolve.Binder {
	b, err := relresolve.NewBinder(meta, relresolve.Ungated, st.MatchingIDs)
	if err != nil {
		panic(err)
	}
	return b
}

func mustValidator(v *validator.GenericValidator, err error) *validator.GenericValidator {
	if err != nil {
		panic(err)
	}
	return v
}

// ownedRule: a done ticket must have an owner. The when is plain, the then
// traverses.
var ownedRule = metamodel.ValidationRule{
	Name:          "done-needs-owner",
	EntityType:    "ticket",
	WhenCondition: "entity.status == 'done'",
	ThenCondition: "related(entity, 'owned-by')",
	Severity:      "error",
}

func traversalMeta(rules ...metamodel.ValidationRule) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{"status": {Type: "string"}}},
			"person": {Properties: map[string]metamodel.PropertyDef{"name": {Type: "string"}}},
		},
		Relations: map[string]metamodel.RelationDef{
			"owned-by": {From: []string{"ticket"}, To: []string{"person"}},
		},
		Validations: rules,
	}
}

// seedTickets creates n done tickets; the even ones are owned.
func seedTickets(t *testing.T, st *memstore.MemStore, n int) {
	t.Helper()
	mustCreate(t, st, &entity.Entity{ID: "P-1", Type: "person", Properties: map[string]any{"name": "p"}})
	for i := range n {
		id := fmt.Sprintf("T-%03d", i)
		mustCreate(t, st, &entity.Entity{ID: id, Type: "ticket", Properties: map[string]any{"status": "done"}})
		if i%2 == 0 {
			if _, err := st.CreateRelation(t.Context(), id, "owned-by", "P-1", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

type countingMatch struct {
	store.GraphQueryer
	calls int
}

func (c *countingMatch) MatchingIDs(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
	c.calls++
	return c.GraphQueryer.MatchingIDs(ctx, q, ids)
}

func TestCheckRule_Traversal(t *testing.T) {
	st := memstore.New()
	seedTickets(t, st, 4)
	meta := traversalMeta(ownedRule)
	v := mustValidator(validator.New(st, meta, lua.ReadDeps{Meta: meta}, binder(meta, st)))

	ids, err := v.CheckRule(t.Context(), ownedRule)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "T-001,T-003" {
		t.Fatalf("violations = %v, want the unowned tickets T-001,T-003", ids)
	}
}

// A rule answers its traversals with one store query per distinct traversal,
// however many candidates it has.
func TestCheckRule_TraversalBudgetIsRowIndependent(t *testing.T) {
	for _, n := range []int{10, 50} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			st := memstore.New()
			seedTickets(t, st, n)
			meta := traversalMeta(ownedRule)
			cm := &countingMatch{GraphQueryer: st}
			v := mustValidator(validator.New(st, meta, lua.ReadDeps{Meta: meta}, binder(meta, cm)))
			ids, err := v.CheckRule(t.Context(), ownedRule)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != n/2 {
				t.Fatalf("violations = %d, want %d", len(ids), n/2)
			}
			if cm.calls != 1 {
				t.Fatalf("MatchingIDs calls = %d, want 1", cm.calls)
			}
		})
	}
}

// A traversal that cannot be answered abandons the rule with a load error. It
// must not read as "when did not match" (a silent skip) or as a violation.
func TestCheckRule_TraversalErrorIsLoadErrorNotSkip(t *testing.T) {
	st := memstore.New()
	seedTickets(t, st, 2)
	rule := ownedRule
	rule.WhenCondition = "not related(entity, 'owned-by')"
	rule.ThenCondition = "entity.status == 'open'"
	meta := traversalMeta(rule)
	refuse := func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error) {
		return nil, acl.ErrTraversalUnsupported
	}
	b, err := relresolve.NewBinder(meta, refuse, st.MatchingIDs)
	if err != nil {
		t.Fatal(err)
	}
	v := mustValidator(validator.New(st, meta, lua.ReadDeps{Meta: meta}, b))
	full, err := v.CheckRuleFull(t.Context(), rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Violations) != 0 || len(full.LoadErrors) == 0 {
		t.Fatalf("violations = %d, load errors = %v; want 0 and at least one", len(full.Violations), full.LoadErrors)
	}
}

func TestCheckRule_TraversalInvalidPathIsLoadError(t *testing.T) {
	st := memstore.New()
	seedTickets(t, st, 1)
	rule := ownedRule
	rule.ThenCondition = "related(entity, 'nope')"
	meta := traversalMeta(rule)
	v := mustValidator(validator.New(st, meta, lua.ReadDeps{Meta: meta}, binder(meta, st)))
	full, err := v.CheckRuleFull(t.Context(), rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Violations) != 0 || len(full.LoadErrors) == 0 {
		t.Fatalf("violations = %d, load errors = %v; want 0 and at least one", len(full.Violations), full.LoadErrors)
	}
}

// The store answers a traversal from the default face's edges, so a row on a
// named face is reported instead of evaluated: once per type, not per row.
func TestCheckRule_TraversalOnNamedFaceIsLoadError(t *testing.T) {
	st := memstore.New()
	rule := ownedRule
	meta := traversalMeta(rule)
	def := meta.Entities["ticket"]
	def.Faces = map[string]metamodel.FaceDef{"en": {}}
	meta.Entities["ticket"] = def
	for _, id := range []string{"T-1", "T-2"} {
		mustCreate(t, st, &entity.Entity{ID: id, Type: "ticket", Face: entity.Face("en"),
			Properties: map[string]any{"status": "done"}})
	}
	v := mustValidator(validator.New(st, meta, lua.ReadDeps{Meta: meta}, binder(meta, st)))
	full, err := v.CheckRuleFull(t.Context(), rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Violations) != 0 || len(full.LoadErrors) != 1 {
		t.Fatalf("violations = %d, load errors = %v; want 0 and 1", len(full.Violations), full.LoadErrors)
	}
}

func TestNew_RejectsNilBinder(t *testing.T) {
	st := memstore.New()
	meta := traversalMeta()
	if _, err := validator.New(st, meta, lua.ReadDeps{Meta: meta}, nil); err == nil {
		t.Fatal("want an error for a nil traversal binder")
	}
}
