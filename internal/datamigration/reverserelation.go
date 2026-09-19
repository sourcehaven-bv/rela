package datamigration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// reverseRelationStep rewrites every stored edge of a relation type so it
// points the other way (TKT-HH7PKJ), answering the relation_endpoints_swapped
// delta.
//
// # The rewrite is the store's job
//
// The step decides WHETHER a type may be reversed and the store decides HOW:
// pgstore and sqlitestore do it in one UPDATE, others fall back to a loop
// ([store.SwapRelationEndpoints] picks). That split is not only about speed.
// A relation's endpoints are its ADDRESS, so a rewrite above the store must be
// create-then-delete, and that shape destroys a self-edge, merges a
// both-directions pair, and — where the backend has history — forks every
// edge's version lineage. An in-place re-key has none of those properties: the
// row survives, so it keeps its identity and its history.
//
// # What the step still owns
//
// Everything schema-shaped, because a store must not read the metamodel:
//
//   - content-scoped types are refused (a head has no face slot, so a reversed
//     state-tailed edge is unrepresentable);
//   - types whose endpoint lists overlap are refused (an edge between two
//     entities of a shared type satisfies both orientations, so nothing can
//     tell a reversed edge from an unreversed one);
//   - symmetric types are refused (direction is meaningless, so the rewrite
//     would churn every edge to no effect).
type reverseRelationStep struct {
	Type string `yaml:"type"`
}

func (s *reverseRelationStep) Kind() string   { return "reverse_relation" }
func (s *reverseRelationStep) Target() string { return s.Type }

// resolvedSubject implements resolvingStep. Relation deltas carry a prefixed
// subject, so this must match CompareShapes' "rel:"+name exactly.
func (s *reverseRelationStep) resolvedSubject() string { return "rel:" + s.Type }

// Validate proves the reversal is expressible from the file's own projections.
//
// The endpoint checks run against the FROM shape, because that describes the
// edges as they are stored today — the ones about to be rewritten.
func (s *reverseRelationStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.Type == "" {
		return errors.New("type is required")
	}
	fromRel, ok := from.Relations[s.Type]
	if !ok {
		return fmt.Errorf("relation type %q is not in the from-schema", s.Type)
	}
	toRel, ok := to.Relations[s.Type]
	if !ok {
		return fmt.Errorf("relation type %q is not in the to-schema — reversing a type the new "+
			"schema does not declare would leave every edge orphaned", s.Type)
	}
	if fromRel.Symmetric || toRel.Symmetric {
		return fmt.Errorf("relation type %q is symmetric: direction carries no meaning, so "+
			"reversing every edge would rewrite the whole type to no effect", s.Type)
	}
	if overlappingEndpoints(fromRel) {
		return fmt.Errorf("relation type %q has overlapping endpoints (from %v, to %v): an edge "+
			"between two entities of a shared type satisfies both directions, so nothing can tell "+
			"a reversed edge from one still to reverse and a re-run would flip them back",
			s.Type, fromRel.From, fromRel.To)
	}
	// The schema must actually describe the reversal the step performs.
	// Without this a hand-written step silently corrupts a type whose
	// declaration says the edges already point the right way.
	if !slices.Equal(fromRel.From, toRel.To) || !slices.Equal(fromRel.To, toRel.From) {
		return fmt.Errorf("relation type %q is not reversed between the two schemas "+
			"(from %v--%v, to %v--%v): reverse_relation rewrites stored edges to match a swap "+
			"the schema declares", s.Type, fromRel.From, fromRel.To, toRel.From, toRel.To)
	}
	return nil
}

// Run reverses the edges via the store, after the refusals the store cannot make.
//
// The scope check needs the LIVE metamodel — `scope:` is deliberately absent
// from ShapeProjection, since it does not affect whether a stored edge conforms
// — so it happens here rather than in Validate. Dry-run is the default at every
// layer above, so the refusal still reaches the operator before any write.
func (s *reverseRelationStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}

	// The store owns the preconditions it can see from the data — a state-tailed
	// edge, a pair that would swap onto each other — and answers them
	// identically on every backend, before any write. A dry-run therefore
	// reports the same refusal an apply would, which is what makes reviewing
	// the plan meaningful.
	n, err := store.CheckSwapRelationEndpoints(ctx, x.Store, s.Type)
	if err != nil {
		return res, fmt.Errorf("relation type %q cannot be reversed: %w", s.Type, err)
	}
	res.Affected = n
	if !x.Apply {
		return res, nil
	}
	n, err = store.SwapRelationEndpoints(ctx, x.Store, s.Type)
	res.Affected = n
	return res, err
}

// overlappingEndpoints reports whether any entity type appears on both sides.
func overlappingEndpoints(r metamodel.RelationShape) bool {
	for _, t := range r.From {
		if slices.Contains(r.To, t) {
			return true
		}
	}
	return false
}
